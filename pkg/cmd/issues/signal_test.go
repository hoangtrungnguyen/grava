package issues

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sqlMock_ErrNoRows() error { return sql.ErrNoRows }
func nowStub() time.Time       { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

// ─── worktreeRE: cwd path → issue id extraction ───────────────────────────

func TestWorktreeRE_ExtractsIssueID(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		// Top-level issue id (no dots).
		{"/Users/x/repo/.worktree/grava-abc123/sub/dir", "grava-abc123"},
		{"/repo/.worktree/grava-abc123", "grava-abc123"},
		{"/repo/.worktree/grava-abc123/", "grava-abc123"},
		// Dotted subtask ids — must NOT be truncated at the first dot.
		{"/repo/.worktree/grava-3a8d.1/", "grava-3a8d.1"},
		{"/repo/.worktree/grava-3a8d.1.1", "grava-3a8d.1.1"},
		{"/repo/.worktree/grava-3a8d.1.1/foo/bar", "grava-3a8d.1.1"},
		// Underscore + hyphen still work.
		{"/repo/.worktree/grava-foo_bar-1", "grava-foo_bar-1"},
		// Non-matches.
		{"/repo/some/other/dir", ""},
		{"/repo/.worktree/", ""}, // empty id
	}
	for _, c := range cases {
		m := worktreeRE.FindStringSubmatch(c.path)
		got := ""
		if len(m) >= 2 {
			got = m[1]
		}
		assert.Equal(t, c.want, got, "path=%q", c.path)
	}
}

// ─── resolveTargetPhase: pure-function table-driven tests ──────────────────

func TestResolveTargetPhase_ForwardOnly(t *testing.T) {
	tests := []struct {
		name        string
		current     string
		kind        SignalKind
		wantPhase   string
		wantWrite   bool
		wantErrSubs string
	}{
		{
			name:      "fresh issue → coding_complete",
			current:   "",
			kind:      SignalCoderDone,
			wantPhase: "coding_complete",
			wantWrite: true,
		},
		{
			name:      "ISSUE_CLAIMED on fresh issue → claimed (forward from no-state)",
			current:   "",
			kind:      SignalIssueClaimed,
			wantPhase: "claimed",
			wantWrite: true,
		},
		{
			name:      "ISSUE_CLAIMED replay on already-claimed → no-op (equal idx)",
			current:   "claimed",
			kind:      SignalIssueClaimed,
			wantPhase: "claimed",
			wantWrite: false,
		},
		{
			name:      "ISSUE_CLAIMED on review_approved is backward, suppressed",
			current:   "review_approved",
			kind:      SignalIssueClaimed,
			wantPhase: "claimed",
			wantWrite: false,
		},
		{
			name:      "PR_AWAITING_MERGE on pr_created → forward",
			current:   "pr_created",
			kind:      SignalPRAwaitingMerge,
			wantPhase: "pr_awaiting_merge",
			wantWrite: true,
		},
		{
			name:      "PR_AWAITING_MERGE on pr_comments_resolved → REARM (terminal-class allows)",
			current:   "pr_comments_resolved",
			kind:      SignalPRAwaitingMerge,
			wantPhase: "pr_awaiting_merge",
			wantWrite: true,
		},
		{
			name:      "claimed → coding_complete (forward)",
			current:   "claimed",
			kind:      SignalCoderDone,
			wantPhase: "coding_complete",
			wantWrite: true,
		},
		{
			name:      "review_approved → coding_complete is BACKWARDS, suppressed",
			current:   "review_approved",
			kind:      SignalCoderDone,
			wantPhase: "coding_complete",
			wantWrite: false,
		},
		{
			name:      "equal phase replay → no rewrite",
			current:   "coding_complete",
			kind:      SignalCoderDone,
			wantPhase: "coding_complete",
			wantWrite: false,
		},
		{
			name:      "review_blocked → review_approved (forward)",
			current:   "review_blocked",
			kind:      SignalReviewerApproved,
			wantPhase: "review_approved",
			wantWrite: true,
		},
		{
			name:      "review_approved → review_blocked is backwards, suppressed",
			current:   "review_approved",
			kind:      SignalReviewerBlocked,
			wantPhase: "review_blocked",
			wantWrite: false,
		},
		{
			name:      "pr_created → pr_merged (forward, skipping intermediate)",
			current:   "pr_created",
			kind:      SignalPRMerged,
			wantPhase: "pr_merged",
			wantWrite: true,
		},
		{
			name:      "terminal: CODER_HALTED overrides any phase",
			current:   "review_approved",
			kind:      SignalCoderHalted,
			wantPhase: "coding_halted",
			wantWrite: true,
		},
		{
			name:      "terminal: PIPELINE_HALTED overrides complete",
			current:   "complete",
			kind:      SignalPipelineHalted,
			wantPhase: "halted_human_needed",
			wantWrite: true,
		},
		{
			name:      "terminal: PR_FAILED overrides pr_created",
			current:   "pr_created",
			kind:      SignalPRFailed,
			wantPhase: "failed",
			wantWrite: true,
		},
		{
			name:      "terminal: PR_CLOSED overrides pr_awaiting_merge",
			current:   "pr_awaiting_merge",
			kind:      SignalPRClosed,
			wantPhase: "failed",
			wantWrite: true,
		},
		{
			name:      "terminal: PR_CLOSED overrides pr_merged (rare race, but recoverable)",
			current:   "pr_merged",
			kind:      SignalPRClosed,
			wantPhase: "failed",
			wantWrite: true,
		},
		{
			name:      "bookkeeping: PLANNER_DONE has no phase",
			current:   "",
			kind:      SignalPlannerDone,
			wantPhase: "",
			wantWrite: false,
		},
		{
			name:      "bookkeeping: BUG_HUNT_COMPLETE has no phase",
			current:   "claimed",
			kind:      SignalBugHuntComplete,
			wantPhase: "",
			wantWrite: false,
		},
		{
			name:      "unknown current phase → treated as before everything",
			current:   "garbage_value",
			kind:      SignalCoderDone,
			wantPhase: "coding_complete",
			wantWrite: true,
		},
		{
			name:        "unknown signal kind → error",
			current:     "claimed",
			kind:        SignalKind("WHATEVER"),
			wantErrSubs: "unknown signal kind",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, write, err := resolveTargetPhase(tc.current, tc.kind)
			if tc.wantErrSubs != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrSubs)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantPhase, got)
			assert.Equal(t, tc.wantWrite, write)
		})
	}
}

// ─── auxiliaryKey ──────────────────────────────────────────────────────────

func TestAuxiliaryKey(t *testing.T) {
	cases := map[SignalKind]string{
		SignalCoderHalted:        "coder_halted",
		SignalReviewerBlocked:    "reviewer_findings",
		SignalPRCreated:          "pr_url",
		SignalPRFailed:           "pr_failed_reason",
		SignalPRClosed:           "pr_close_reason",
		SignalPipelineHalted:     "pipeline_halted_reason",
		SignalPipelineFailed:     "pipeline_failed_reason",
		SignalPlannerNeedsInput:  "planner_needs_input_summary",
		// Signals with no auxiliary record:
		SignalIssueClaimed:     "",
		SignalCoderDone:        "",
		SignalReviewerApproved: "",
		SignalPRAwaitingMerge:  "",
		SignalPRMerged:         "",
		SignalPlannerDone:      "",
		SignalBugHuntComplete:  "",
	}
	for k, want := range cases {
		assert.Equal(t, want, auxiliaryKey(k), "kind=%s", k)
	}
}

// ─── Phase 6 telemetry: emitSignalLog + computeSignalStats ────────────────

func TestEmitSignalLog_AppendsValidJSON(t *testing.T) {
	dir := t.TempDir()
	logPath := dir + "/signal-source.jsonl"
	t.Setenv("GRAVA_SIGNAL_LOG_PATH", logPath)

	emitSignalLog(signalLogEvent{
		TS: "2026-01-01T00:00:00Z", IssueID: "grava-aaaa", Kind: "CODER_DONE",
		Phase: "coding_complete", PhaseWrote: true, Source: "cli", Actor: "agent-coder",
	})
	emitSignalLog(signalLogEvent{
		TS: "2026-01-01T00:00:01Z", IssueID: "grava-bbbb", Kind: "PR_MERGED",
		Phase: "pr_merged", PhaseWrote: true, Source: "cli", Actor: "watcher",
	})

	raw, err := os.ReadFile(logPath)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	require.Len(t, lines, 2)

	var ev1 signalLogEvent
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &ev1))
	assert.Equal(t, "grava-aaaa", ev1.IssueID)
	assert.Equal(t, "CODER_DONE", ev1.Kind)
	assert.Equal(t, "cli", ev1.Source)
	assert.True(t, ev1.PhaseWrote)
}

func TestEmitSignalLog_DisabledByEmptyEnv(t *testing.T) {
	dir := t.TempDir()
	logPath := dir + "/should-not-exist.jsonl"
	t.Setenv("GRAVA_SIGNAL_LOG_PATH", "") // explicitly disable

	emitSignalLog(signalLogEvent{TS: "x", IssueID: "y", Kind: "z", Source: "cli"})

	_, err := os.Stat(logPath)
	assert.True(t, os.IsNotExist(err), "log file should not have been created")
}

func TestComputeSignalStats_MixedSources(t *testing.T) {
	dir := t.TempDir()
	logPath := dir + "/signal-source.jsonl"
	// 90 cli + 10 hook-fallback events = 0.9 cli ratio.
	lines := []string{}
	for i := range 90 {
		lines = append(lines, fmt.Sprintf(
			`{"ts":"2026-05-01T00:00:%02dZ","issue_id":"grava-x%02d","kind":"CODER_DONE","phase":"coding_complete","phase_wrote":true,"source":"cli"}`,
			i%60, i%50))
	}
	for i := range 10 {
		lines = append(lines, fmt.Sprintf(
			`{"ts":"2026-05-02T00:00:%02dZ","issue_id":"grava-y%02d","kind":"REVIEWER_APPROVED","phase":"review_approved","phase_wrote":true,"source":"hook-fallback"}`,
			i, i))
	}
	require.NoError(t, os.WriteFile(logPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644))

	stats, err := computeSignalStats(logPath)
	require.NoError(t, err)
	assert.Equal(t, 100, stats.Total)
	assert.Equal(t, 90, stats.BySource["cli"])
	assert.Equal(t, 10, stats.BySource["hook-fallback"])
	assert.InDelta(t, 0.90, stats.CLIRatio, 0.0001)
	assert.Equal(t, "2026-05-01T00:00:00Z", stats.WindowStart)
	assert.Equal(t, "2026-05-02T00:00:09Z", stats.WindowEnd)
	assert.Equal(t, 0, stats.ParseErrors)
}

func TestComputeSignalStats_MissingFileReturnsZero(t *testing.T) {
	stats, err := computeSignalStats("/nonexistent/path/signal-source.jsonl")
	require.NoError(t, err) // missing log isn't an error — just no traffic yet
	assert.Equal(t, 0, stats.Total)
	assert.Equal(t, 0.0, stats.CLIRatio)
}

// ─── legacyTextLine ────────────────────────────────────────────────────────

func TestLegacyTextLine(t *testing.T) {
	cases := []struct {
		kind, payload, want string
	}{
		{string(SignalCoderDone), "abc123", "CODER_DONE: abc123"},
		{string(SignalCoderDone), "", "CODER_DONE"},
		{string(SignalCoderHalted), "no spec", "CODER_HALTED: no spec"},
		{string(SignalReviewerApproved), "ignored", "REVIEWER_APPROVED"},
		{string(SignalReviewerBlocked), ".review-round-1.md", "REVIEWER_BLOCKED: .review-round-1.md"},
		{string(SignalReviewerBlocked), "", "REVIEWER_BLOCKED"},
		{string(SignalPRMerged), "anything", "PR_MERGED"},
		{string(SignalPRCreated), "https://x/y/pr/1", "PR_CREATED: https://x/y/pr/1"},
		{string(SignalPRClosed), "reviewer-rejected", "PR_CLOSED: reviewer-rejected"},
		{string(SignalPRClosed), "", "PR_CLOSED"},
		{string(SignalIssueClaimed), "", "ISSUE_CLAIMED"},
		{string(SignalPRAwaitingMerge), "", "PR_AWAITING_MERGE"},
		{string(SignalPlannerDone), "ignored", "PLANNER_DONE"},
		{string(SignalBugHuntComplete), "ignored", "BUG_HUNT_COMPLETE"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, legacyTextLine(tc.kind, tc.payload),
			"kind=%s payload=%q", tc.kind, tc.payload)
	}
}

// ─── signalRun: integration with sqlmock ───────────────────────────────────

// expectWispWrite sets up the canonical sequence of mock expectations for a
// successful wispWrite call (matches wisp_test.go's pattern: guardNotArchived
// + WithAuditedTx + INSERT/UPDATE/INSERT events + commit).
func expectWispWrite(mock sqlmock.Sqlmock, issueID, key, value, actor string) {
	mock.ExpectQuery("SELECT status FROM issues WHERE id").
		WithArgs(issueID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("open"))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs(issueID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(issueID))
	mock.ExpectExec("INSERT INTO wisp_entries").
		WithArgs(issueID, key, value, actor).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE issues SET wisp_heartbeat_at").
		WithArgs(issueID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
}

func TestSignalRun_HappyPath_CoderDone_FreshIssue(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	const issueID = "abc123def456"

	// signalRun first calls wispRead to get current pipeline_phase. Wisp
	// "not found" surfaces ISSUE_NOT_FOUND from the existence check OR
	// WISP_NOT_FOUND. Here we simulate "issue exists, wisp absent".
	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs(issueID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(issueID))
	mock.ExpectQuery("SELECT key_name, value, written_by, written_at").
		WithArgs(issueID, "pipeline_phase").
		WillReturnError(sqlMock_ErrNoRows())

	// Then wispWrite for pipeline_phase=coding_complete.
	expectWispWrite(mock, issueID, "pipeline_phase", "coding_complete", "agent-coder")

	store := dolt.NewClientFromDB(db)
	res, err := signalRun(context.Background(), store, SignalParams{
		IssueID: issueID,
		Kind:    SignalCoderDone,
		Payload: "deadbeef", // commit sha — not stored as auxiliary for CODER_DONE
		Actor:   "agent-coder",
	})
	require.NoError(t, err)
	assert.Equal(t, "coding_complete", res.Phase)
	assert.True(t, res.PhaseWrote)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSignalRun_CoderHalted_AlsoWritesAuxiliary(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	const issueID = "abc123def456"

	// Current phase = claimed (from a prior claim).
	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs(issueID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(issueID))
	mock.ExpectQuery("SELECT key_name, value, written_by, written_at").
		WithArgs(issueID, "pipeline_phase").
		WillReturnRows(sqlmock.NewRows([]string{"key_name", "value", "written_by", "written_at"}).
			AddRow("pipeline_phase", "claimed", "agent-coder", nowStub()))

	// Phase write: coding_halted (terminal — overrides forward-only).
	expectWispWrite(mock, issueID, "pipeline_phase", "coding_halted", "agent-coder")
	// Auxiliary: coder_halted = "<reason>".
	expectWispWrite(mock, issueID, "coder_halted", "no spec found", "agent-coder")

	store := dolt.NewClientFromDB(db)
	res, err := signalRun(context.Background(), store, SignalParams{
		IssueID: issueID,
		Kind:    SignalCoderHalted,
		Payload: "no spec found",
		Actor:   "agent-coder",
	})
	require.NoError(t, err)
	assert.Equal(t, "coding_halted", res.Phase)
	assert.True(t, res.PhaseWrote)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSignalRun_PRClosed_OverridesAwaitingMerge(t *testing.T) {
	// Simulates what pr-merge-watcher.sh emits when GitHub reports CLOSED
	// without a merge: pipeline_phase moves from pr_awaiting_merge → failed,
	// pr_close_reason auxiliary wisp captures the rejection category.
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	const issueID = "abc123def456"

	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs(issueID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(issueID))
	mock.ExpectQuery("SELECT key_name, value, written_by, written_at").
		WithArgs(issueID, "pipeline_phase").
		WillReturnRows(sqlmock.NewRows([]string{"key_name", "value", "written_by", "written_at"}).
			AddRow("pipeline_phase", "pr_awaiting_merge", "watcher", nowStub()))

	// Phase write: failed (terminal — overrides forward-only).
	expectWispWrite(mock, issueID, "pipeline_phase", "failed", "watcher")
	// Auxiliary: pr_close_reason = "<category>".
	expectWispWrite(mock, issueID, "pr_close_reason", "reviewer-rejected", "watcher")

	store := dolt.NewClientFromDB(db)
	res, err := signalRun(context.Background(), store, SignalParams{
		IssueID: issueID,
		Kind:    SignalPRClosed,
		Payload: "reviewer-rejected",
		Actor:   "watcher",
	})
	require.NoError(t, err)
	assert.Equal(t, "failed", res.Phase)
	assert.True(t, res.PhaseWrote)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSignalRun_BackwardSignal_SkipsPhaseWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	const issueID = "abc123def456"

	// Current phase already at review_approved; a stray REVIEWER_BLOCKED must NOT roll back.
	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs(issueID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(issueID))
	mock.ExpectQuery("SELECT key_name, value, written_by, written_at").
		WithArgs(issueID, "pipeline_phase").
		WillReturnRows(sqlmock.NewRows([]string{"key_name", "value", "written_by", "written_at"}).
			AddRow("pipeline_phase", "review_approved", "agent-reviewer", nowStub()))

	// Auxiliary `reviewer_findings` IS still written (operator may want to see late findings),
	// even though pipeline_phase doesn't roll back.
	expectWispWrite(mock, issueID, "reviewer_findings", "late nit", "agent-reviewer")

	store := dolt.NewClientFromDB(db)
	res, err := signalRun(context.Background(), store, SignalParams{
		IssueID: issueID,
		Kind:    SignalReviewerBlocked,
		Payload: "late nit",
		Actor:   "agent-reviewer",
	})
	require.NoError(t, err)
	assert.Equal(t, "review_blocked", res.Phase)
	assert.False(t, res.PhaseWrote, "must not roll back review_approved → review_blocked")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSignalRun_UnknownKind_Errors(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	store := dolt.NewClientFromDB(db)
	_, err = signalRun(context.Background(), store, SignalParams{
		IssueID: "abc",
		Kind:    SignalKind("FAKE_SIGNAL"),
		Actor:   "agent",
	})
	require.Error(t, err)
	var gerr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gerr))
	assert.Equal(t, "INVALID_SIGNAL", gerr.Code)
}

func TestSignalRun_BookkeepingSignal_NoPhaseWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	const issueID = "abc123def456"

	// wispRead probes pipeline_phase first — issue exists, wisp absent.
	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs(issueID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(issueID))
	mock.ExpectQuery("SELECT key_name, value, written_by, written_at").
		WithArgs(issueID, "pipeline_phase").
		WillReturnError(sqlMock_ErrNoRows())

	store := dolt.NewClientFromDB(db)
	res, err := signalRun(context.Background(), store, SignalParams{
		IssueID: issueID,
		Kind:    SignalPlannerDone,
		Actor:   "agent-planner",
	})
	require.NoError(t, err)
	assert.Equal(t, "", res.Phase)
	assert.False(t, res.PhaseWrote)
	require.NoError(t, mock.ExpectationsWereMet())
}
