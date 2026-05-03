package issues

import (
	"context"
	"database/sql"
	"errors"
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
		SignalPipelineHalted:     "pipeline_halted_reason",
		SignalPipelineFailed:     "pipeline_failed_reason",
		SignalPlannerNeedsInput:  "planner_needs_input_summary",
		// Signals with no auxiliary record:
		SignalCoderDone:        "",
		SignalReviewerApproved: "",
		SignalPRMerged:         "",
		SignalPlannerDone:      "",
		SignalBugHuntComplete:  "",
	}
	for k, want := range cases {
		assert.Equal(t, want, auxiliaryKey(k), "kind=%s", k)
	}
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
