package issues

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/spf13/cobra"
)

// SignalKind is the typed pipeline signal an agent emits to the orchestrator.
// Each kind maps deterministically to a `pipeline_phase` wisp value via
// resolveTargetPhase, replacing the regex / last-line text parsing previously
// done by scripts/hooks/sync-pipeline-status.sh.
type SignalKind string

const (
	// SignalIssueClaimed marks an issue as claimed by the orchestrator.
	// Orchestrator-internal phase write (no agent emission): /ship Phase 1
	// prologue, grava-next-issue skill, and Phase 5 retry use it to mark
	// "I'm taking responsibility for this issue now." Routed through grava
	// signal so atomicity, validation, forward-only logic, and audit logging
	// match every other phase write.
	SignalIssueClaimed        SignalKind = "ISSUE_CLAIMED"
	SignalCoderDone           SignalKind = "CODER_DONE"
	SignalCoderHalted         SignalKind = "CODER_HALTED"
	SignalReviewerApproved    SignalKind = "REVIEWER_APPROVED"
	SignalReviewerBlocked     SignalKind = "REVIEWER_BLOCKED"
	SignalPRCreated           SignalKind = "PR_CREATED"
	SignalPRFailed            SignalKind = "PR_FAILED"
	SignalPRCommentsResolved  SignalKind = "PR_COMMENTS_RESOLVED"
	// SignalPRAwaitingMerge marks the transition from "PR opened" to "PR
	// awaiting merge." Orchestrator-internal: /ship Phase 3 emits it after
	// pr-creator returns success, and Phase 4 resume re-emits after a
	// successful comment-fix round.
	SignalPRAwaitingMerge     SignalKind = "PR_AWAITING_MERGE"
	SignalPRMerged            SignalKind = "PR_MERGED"
	// SignalPRClosed is emitted by pr-merge-watcher.sh when the GitHub PR was
	// closed without being merged. Distinct from SignalPRFailed (which the
	// pr-creator agent emits when it can't open a PR in the first place).
	// Both target pipeline_phase=failed but record different auxiliary wisps:
	// pr_close_reason (rejection category) here, pr_failed_reason (open-time
	// error) for SignalPRFailed.
	SignalPRClosed            SignalKind = "PR_CLOSED"
	SignalPipelineComplete    SignalKind = "PIPELINE_COMPLETE"
	SignalPipelineHalted      SignalKind = "PIPELINE_HALTED"
	SignalPipelineFailed      SignalKind = "PIPELINE_FAILED"
	SignalPlannerNeedsInput   SignalKind = "PLANNER_NEEDS_INPUT"
	SignalPlannerDone         SignalKind = "PLANNER_DONE"
	SignalBugHuntComplete     SignalKind = "BUG_HUNT_COMPLETE"
)

// signalKindIndex is built once at init for O(1) validation.
var signalKindIndex = map[SignalKind]struct{}{
	SignalIssueClaimed:       {},
	SignalCoderDone:          {},
	SignalCoderHalted:        {},
	SignalReviewerApproved:   {},
	SignalReviewerBlocked:    {},
	SignalPRCreated:          {},
	SignalPRFailed:           {},
	SignalPRCommentsResolved: {},
	SignalPRAwaitingMerge:    {},
	SignalPRMerged:           {},
	SignalPRClosed:           {},
	SignalPipelineComplete:   {},
	SignalPipelineHalted:     {},
	SignalPipelineFailed:     {},
	SignalPlannerNeedsInput:  {},
	SignalPlannerDone:        {},
	SignalBugHuntComplete:    {},
}

// phaseOrder mirrors the forward-only progression in sync-pipeline-status.sh.
// Phases not in this list (terminal / out-of-band) are handled separately in
// resolveTargetPhase via shouldOverwrite().
var phaseOrder = []string{
	"claimed",
	"coding_complete",
	"review_blocked",
	"review_approved",
	"pr_created",
	"pr_awaiting_merge",
	"pr_comments_resolved",
	"pr_merged",
	"complete",
}

// signalToPhase maps each SignalKind to the pipeline_phase value it produces.
// A blank value means the signal does not drive pipeline_phase (e.g.
// PLANNER_DONE, BUG_HUNT_COMPLETE — bookkeeping signals only).
var signalToPhase = map[SignalKind]string{
	SignalIssueClaimed:       "claimed",
	SignalCoderDone:          "coding_complete",
	SignalCoderHalted:        "coding_halted",
	SignalReviewerApproved:   "review_approved",
	SignalReviewerBlocked:    "review_blocked",
	SignalPRCreated:          "pr_created",
	SignalPRFailed:           "failed",
	SignalPRCommentsResolved: "pr_comments_resolved",
	SignalPRAwaitingMerge:    "pr_awaiting_merge",
	SignalPRMerged:           "pr_merged",
	SignalPRClosed:           "failed",
	SignalPipelineComplete:   "complete",
	SignalPipelineHalted:     "halted_human_needed",
	SignalPipelineFailed:     "failed",
	SignalPlannerNeedsInput:  "planner_needs_input",
}

// terminalPhases overwrite any prior phase regardless of position. Three
// classes of phases live here:
//
//  1. True terminals — failed, halted_human_needed, coding_halted,
//     planner_needs_input. Pipeline ends; writes always allowed.
//  2. Rearm marker — pr_awaiting_merge. /ship Phase 4 re-emits this after a
//     successful PR-comment-fix round to tell the watcher "look at this PR
//     again." That transition is pr_comments_resolved → pr_awaiting_merge,
//     which is BACKWARDS in phaseOrder (idx 6 → 5). Forward-only would
//     suppress it; allowing overwrite preserves the original direct-wisp-
//     write semantic in skills (.claude/skills/ship/SKILL.md Phase 4).
//
// The pr_merged → pr_awaiting_merge case (a regression from terminal
// success) is unreachable in practice: watcher writes pr_merged AFTER
// GitHub confirms merge, then closes the issue. /ship Phase 4 doesn't
// re-fire after that.
var terminalPhases = map[string]struct{}{
	"failed":              {},
	"halted_human_needed": {},
	"coding_halted":       {},
	"planner_needs_input": {},
	"pr_awaiting_merge":   {},
}

// resolveTargetPhase returns (newPhase, shouldWrite) for a given current phase
// and signal kind. Pure function — no DB access — so the forward-only state
// machine is testable in isolation.
//
// Rules:
//   - Unknown signal → ("", false, ErrUnknownSignal)
//   - Bookkeeping signal (no phase mapping) → ("", false, nil)
//   - Terminal phase → always writes (overrides forward-only).
//   - Otherwise: writes only when newIdx > currentIdx (strictly forward).
//   - Unknown current phase ("" or anything not in phaseOrder) → writes.
func resolveTargetPhase(currentPhase string, kind SignalKind) (string, bool, error) {
	if _, ok := signalKindIndex[kind]; !ok {
		return "", false, fmt.Errorf("unknown signal kind %q", kind)
	}
	target, mapped := signalToPhase[kind]
	if !mapped || target == "" {
		// Bookkeeping signal — caller still records auxiliary state but
		// pipeline_phase is untouched.
		return "", false, nil
	}
	if _, terminal := terminalPhases[target]; terminal {
		return target, true, nil
	}
	newIdx := indexOfPhase(target)
	curIdx := indexOfPhase(currentPhase)
	// newIdx must be a known ordered phase; curIdx unknown (-1) is treated as
	// "before everything" so a fresh issue can transition to any forward phase.
	if newIdx >= 0 && (curIdx < 0 || newIdx > curIdx) {
		return target, true, nil
	}
	return target, false, nil
}

func indexOfPhase(p string) int {
	for i, v := range phaseOrder {
		if v == p {
			return i
		}
	}
	return -1
}

// SignalParams is the input to the signal command.
type SignalParams struct {
	IssueID string
	Kind    SignalKind
	Payload string // free-form context: sha, url, reason, findings path, etc.
	Actor   string
}

// signalLogEvent is the JSON line shape written to .grava/signal-source.jsonl.
// One line per `grava signal` invocation. Phase 6 of the structured-signals
// migration uses this to compare CLI traffic vs hook-fallback traffic over a
// soak window; once the CLI accounts for ≥99% of writes, Phase 8 retires the
// hook regex.
type signalLogEvent struct {
	TS         string `json:"ts"`           // RFC3339Nano UTC
	IssueID    string `json:"issue_id"`
	Kind       string `json:"kind"`
	Phase      string `json:"phase,omitempty"`
	PhaseWrote bool   `json:"phase_wrote"`
	Source     string `json:"source"`       // "cli" here; "hook-fallback" in sync-pipeline-status.sh
	Actor      string `json:"actor,omitempty"`
	Err        string `json:"err,omitempty"`
}

// signalLogPath returns the JSONL log file path. Override with the
// GRAVA_SIGNAL_LOG_PATH env var (set to empty to disable).
func signalLogPath() string {
	if v, ok := os.LookupEnv("GRAVA_SIGNAL_LOG_PATH"); ok {
		return v
	}
	return ".grava/signal-source.jsonl"
}

// logSignalEvent appends a single JSON line to the signal-source log. Best
// effort — any failure (path unwritable, dir missing, encoding error) is
// silently swallowed so observability never breaks the underlying signal.
// Exposed for testing via the package-level emitSignalLog function variable
// so tests can capture events without touching disk.
var emitSignalLog = func(ev signalLogEvent) {
	path := signalLogPath()
	if path == "" {
		return // observability explicitly disabled
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close() //nolint:errcheck
	enc := json.NewEncoder(f)
	_ = enc.Encode(ev) //nolint:errcheck — observability is best-effort
}

// SignalResult is the JSON output for a successful signal.
type SignalResult struct {
	IssueID    string `json:"issue_id"`
	Kind       string `json:"kind"`
	Phase      string `json:"phase,omitempty"`        // resolved pipeline_phase value (blank for bookkeeping signals)
	PhaseWrote bool   `json:"phase_wrote"`             // true if pipeline_phase was actually advanced
	Payload    string `json:"payload,omitempty"`
}

// signalRun executes a signal: writes pipeline_phase (when applicable) and any
// auxiliary wisps documented per signal type. Returns the resolved phase and
// whether it was written so the caller can present accurate stdout output.
//
// Phase 6 telemetry: emits one JSON line to .grava/signal-source.jsonl per
// invocation, including failures. The hook-fallback path emits the same shape
// from sync-pipeline-status.sh with source="hook-fallback", letting operators
// compute the cli-vs-hook ratio over a soak window.
func signalRun(ctx context.Context, store dolt.Store, params SignalParams) (SignalResult, error) {
	logEvent := signalLogEvent{
		TS:      time.Now().UTC().Format(time.RFC3339Nano),
		IssueID: params.IssueID,
		Kind:    string(params.Kind),
		Source:  "cli",
		Actor:   params.Actor,
	}

	if _, ok := signalKindIndex[params.Kind]; !ok {
		err := gravaerrors.New("INVALID_SIGNAL",
			fmt.Sprintf("unknown signal kind %q", params.Kind), nil)
		logEvent.Err = err.Error()
		emitSignalLog(logEvent)
		return SignalResult{}, err
	}

	// Read current phase to apply forward-only logic.
	current := ""
	if entry, err := wispRead(ctx, store, params.IssueID, "pipeline_phase"); err == nil {
		if e, ok := entry.(*WispEntry); ok && e != nil {
			current = e.Value
		}
	} else if gerr, ok := err.(*gravaerrors.GravaError); ok {
		// ISSUE_NOT_FOUND is fatal; WISP_NOT_FOUND just means no prior phase.
		if gerr.Code == "ISSUE_NOT_FOUND" {
			logEvent.Err = err.Error()
			emitSignalLog(logEvent)
			return SignalResult{}, err
		}
	}

	target, writePhase, err := resolveTargetPhase(current, params.Kind)
	if err != nil {
		gerr := gravaerrors.New("INVALID_SIGNAL", err.Error(), err)
		logEvent.Err = gerr.Error()
		emitSignalLog(logEvent)
		return SignalResult{}, gerr
	}
	logEvent.Phase = target
	logEvent.PhaseWrote = writePhase

	if writePhase {
		if _, err := wispWrite(ctx, store, WispWriteParams{
			IssueID: params.IssueID,
			Key:     "pipeline_phase",
			Value:   target,
			Actor:   params.Actor,
		}); err != nil {
			logEvent.Err = err.Error()
			emitSignalLog(logEvent)
			return SignalResult{}, err
		}
	}

	// Record auxiliary wisp for HALT/REJECT signals so downstream triage tools
	// (grava doctor, /ship --retry) can read the reason without parsing logs.
	if aux := auxiliaryKey(params.Kind); aux != "" && params.Payload != "" {
		if _, err := wispWrite(ctx, store, WispWriteParams{
			IssueID: params.IssueID,
			Key:     aux,
			Value:   params.Payload,
			Actor:   params.Actor,
		}); err != nil {
			logEvent.Err = err.Error()
			emitSignalLog(logEvent)
			return SignalResult{}, err
		}
	}

	emitSignalLog(logEvent)
	return SignalResult{
		IssueID:    params.IssueID,
		Kind:       string(params.Kind),
		Phase:      target,
		PhaseWrote: writePhase,
		Payload:    params.Payload,
	}, nil
}

// auxiliaryKey returns the wisp key used to record contextual payload for
// signals that carry triage-relevant detail. Returns "" for signals whose
// payload is purely informational (e.g. CODER_DONE's commit sha is already in
// metadata.last_commit, set by grava-dev-task on commit).
func auxiliaryKey(kind SignalKind) string {
	switch kind {
	case SignalCoderHalted:
		return "coder_halted"
	case SignalReviewerBlocked:
		return "reviewer_findings"
	case SignalPRCreated:
		return "pr_url"
	case SignalPRFailed:
		return "pr_failed_reason"
	case SignalPRClosed:
		return "pr_close_reason"
	case SignalPipelineHalted:
		return "pipeline_halted_reason"
	case SignalPipelineFailed:
		return "pipeline_failed_reason"
	case SignalPlannerNeedsInput:
		return "planner_needs_input_summary"
	}
	return ""
}

// resolveIssueIDFromCwd extracts the issue id from a path matching
// `.worktree/<id>` so agents inside their worktree can call `grava signal X`
// without re-passing --issue. Returns "" when the cwd does not match.
//
// The id character class includes `.` to match dotted subtask IDs like
// `grava-abc.1.2` (subtask-of-subtask), not just top-level `grava-abc`.
var worktreeRE = regexp.MustCompile(`/\.worktree/([a-zA-Z0-9_.-]+)(/|$)`)

func resolveIssueIDFromCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	m := worktreeRE.FindStringSubmatch(cwd)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

func newSignalCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "signal <kind>",
		Short: "Emit a pipeline signal (replaces last-line text parsing)",
		Long: `Emit a typed pipeline signal that updates pipeline_phase atomically.

Kinds:
  ISSUE_CLAIMED,
  CODER_DONE, CODER_HALTED,
  REVIEWER_APPROVED, REVIEWER_BLOCKED,
  PR_CREATED, PR_FAILED, PR_COMMENTS_RESOLVED,
  PR_AWAITING_MERGE, PR_MERGED, PR_CLOSED,
  PIPELINE_COMPLETE, PIPELINE_HALTED, PIPELINE_FAILED,
  PLANNER_NEEDS_INPUT, PLANNER_DONE, BUG_HUNT_COMPLETE

The issue id is auto-resolved from the current working directory when run
from inside a grava worktree (.worktree/<id>/...). Pass --issue to override.

The command also prints the legacy "<KIND>: <payload>" line as the final
line of stdout for backward compatibility with the sync-pipeline-status hook
and any orchestrator that still parses last-line output.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			kind := SignalKind(args[0])

			issueID, _ := cmd.Flags().GetString("issue")
			if issueID == "" {
				issueID = resolveIssueIDFromCwd()
			}
			if issueID == "" {
				return gravaerrors.New("ISSUE_REQUIRED",
					"issue id not given and cwd is not inside a .worktree/<id> directory; pass --issue", nil)
			}

			payload, _ := cmd.Flags().GetString("payload")
			actor, _ := cmd.Flags().GetString("actor")
			if actor == "" {
				actor = *d.Actor
			}

			result, err := signalRun(ctx, *d.Store, SignalParams{
				IssueID: issueID,
				Kind:    kind,
				Payload: payload,
				Actor:   actor,
			})
			if err != nil {
				if *d.OutputJSON {
					return writeJSONError(cmd, err)
				}
				return err
			}

			if *d.OutputJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}

			// Human-readable status line, then the legacy text signal as the
			// final line so existing last-line parsers (orchestrator + hook)
			// continue to work without modification.
			if result.PhaseWrote {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"✅ Signal %s recorded — pipeline_phase=%s\n", result.Kind, result.Phase)
			} else if result.Phase != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"ℹ️  Signal %s — pipeline_phase already at or beyond %s, no change\n",
					result.Kind, result.Phase)
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"ℹ️  Signal %s recorded (bookkeeping only)\n", result.Kind)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), legacyTextLine(result.Kind, result.Payload))

			return nil
		},
	}
	cmd.Flags().String("issue", "", "Issue id (auto-detected from .worktree/<id> cwd when omitted)")
	cmd.Flags().String("payload", "", "Optional payload — sha, url, reason, findings path, etc.")
	cmd.Flags().String("actor", "", "Override the actor identity for this signal")
	return cmd
}

// SignalStats summarizes a soak window of the signal-source.jsonl log.
// Used by `grava signal-stats` to surface the cli-vs-hook ratio Phase 6
// requires before Phase 8 retires the bash hook.
type SignalStats struct {
	WindowStart   string         `json:"window_start"`     // earliest event TS
	WindowEnd     string         `json:"window_end"`       // latest event TS
	Total         int            `json:"total"`
	BySource      map[string]int `json:"by_source"`        // {"cli": N, "hook-fallback": M}
	CLIRatio      float64        `json:"cli_ratio"`        // 0.0..1.0
	ParseErrors   int            `json:"parse_errors"`     // malformed lines (skipped)
	UniqueIssues  int            `json:"unique_issues"`
}

// computeSignalStats reads the signal-source log line-by-line and aggregates
// counts. Returns zero-value SignalStats if the log doesn't exist (no error —
// that just means no traffic yet).
func computeSignalStats(path string) (SignalStats, error) {
	stats := SignalStats{BySource: map[string]int{}}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return stats, nil
		}
		return stats, err
	}
	defer f.Close() //nolint:errcheck

	dec := json.NewDecoder(f)
	dec.UseNumber()
	issues := map[string]struct{}{}
	for dec.More() {
		var ev signalLogEvent
		if err := dec.Decode(&ev); err != nil {
			stats.ParseErrors++
			continue
		}
		stats.Total++
		stats.BySource[ev.Source]++
		if ev.IssueID != "" {
			issues[ev.IssueID] = struct{}{}
		}
		if ev.TS != "" {
			if stats.WindowStart == "" || ev.TS < stats.WindowStart {
				stats.WindowStart = ev.TS
			}
			if ev.TS > stats.WindowEnd {
				stats.WindowEnd = ev.TS
			}
		}
	}
	stats.UniqueIssues = len(issues)
	if stats.Total > 0 {
		stats.CLIRatio = float64(stats.BySource["cli"]) / float64(stats.Total)
	}
	return stats, nil
}

func newSignalStatsCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "signal-stats",
		Short: "Report cli vs hook-fallback ratio from .grava/signal-source.jsonl",
		Long: `Aggregate the signal-source log to compute migration adoption.

Reads .grava/signal-source.jsonl (override with GRAVA_SIGNAL_LOG_PATH) and
prints how many phase-writes came through the typed CLI ("cli") vs the
legacy regex hook ("hook-fallback"). Phase 8 of the structured-signals
migration retires the hook regex once cli_ratio is ≥ 0.99 over a one-week
soak window — this command provides the gate.

Use --json for the raw aggregate, otherwise a human-readable summary.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := signalLogPath()
			stats, err := computeSignalStats(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			if *d.OutputJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(stats)
			}
			w := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(w, "Signal source telemetry — %s\n", path)
			_, _ = fmt.Fprintln(w, "----------------------------------------")
			if stats.Total == 0 {
				_, _ = fmt.Fprintln(w, "No events recorded yet.")
				return nil
			}
			_, _ = fmt.Fprintf(w, "Window:        %s → %s\n", stats.WindowStart, stats.WindowEnd)
			_, _ = fmt.Fprintf(w, "Total events:  %d  (%d unique issues)\n", stats.Total, stats.UniqueIssues)
			_, _ = fmt.Fprintln(w, "By source:")
			for src, n := range stats.BySource {
				pct := float64(n) / float64(stats.Total) * 100
				_, _ = fmt.Fprintf(w, "  %-15s %5d  (%5.1f%%)\n", src, n, pct)
			}
			_, _ = fmt.Fprintf(w, "CLI ratio:     %.4f", stats.CLIRatio)
			switch {
			case stats.CLIRatio >= 0.99:
				_, _ = fmt.Fprintln(w, "  ✅ ≥99% — Phase 8 gate cleared")
			case stats.CLIRatio >= 0.95:
				_, _ = fmt.Fprintln(w, "  🟡 ≥95% — close but not Phase-8-ready yet")
			default:
				_, _ = fmt.Fprintln(w, "  ❌ below 99% — keep the hook fallback live")
			}
			if stats.ParseErrors > 0 {
				_, _ = fmt.Fprintf(w, "Parse errors:  %d (malformed lines skipped)\n", stats.ParseErrors)
			}
			return nil
		},
	}
	return cmd
}

// legacyTextLine renders the canonical last-line signal string used by the
// existing scripts/hooks/sync-pipeline-status.sh and /ship orchestrator.
// Bookkeeping signals (PLANNER_DONE, BUG_HUNT_COMPLETE) never carried payloads
// and are emitted bare.
func legacyTextLine(kind, payload string) string {
	switch SignalKind(kind) {
	case SignalReviewerApproved, SignalPRMerged, SignalPlannerDone, SignalBugHuntComplete:
		return kind
	case SignalReviewerBlocked:
		if payload == "" {
			return kind
		}
		return kind + ": " + payload
	}
	if payload == "" {
		return kind
	}
	return kind + ": " + payload
}
