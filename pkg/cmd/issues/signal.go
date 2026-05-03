package issues

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"

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
	SignalCoderDone           SignalKind = "CODER_DONE"
	SignalCoderHalted         SignalKind = "CODER_HALTED"
	SignalReviewerApproved    SignalKind = "REVIEWER_APPROVED"
	SignalReviewerBlocked     SignalKind = "REVIEWER_BLOCKED"
	SignalPRCreated           SignalKind = "PR_CREATED"
	SignalPRFailed            SignalKind = "PR_FAILED"
	SignalPRCommentsResolved  SignalKind = "PR_COMMENTS_RESOLVED"
	SignalPRMerged            SignalKind = "PR_MERGED"
	SignalPipelineComplete    SignalKind = "PIPELINE_COMPLETE"
	SignalPipelineHalted      SignalKind = "PIPELINE_HALTED"
	SignalPipelineFailed      SignalKind = "PIPELINE_FAILED"
	SignalPlannerNeedsInput   SignalKind = "PLANNER_NEEDS_INPUT"
	SignalPlannerDone         SignalKind = "PLANNER_DONE"
	SignalBugHuntComplete     SignalKind = "BUG_HUNT_COMPLETE"
)

// signalKindIndex is built once at init for O(1) validation.
var signalKindIndex = map[SignalKind]struct{}{
	SignalCoderDone:          {},
	SignalCoderHalted:        {},
	SignalReviewerApproved:   {},
	SignalReviewerBlocked:    {},
	SignalPRCreated:          {},
	SignalPRFailed:           {},
	SignalPRCommentsResolved: {},
	SignalPRMerged:           {},
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
	SignalCoderDone:          "coding_complete",
	SignalCoderHalted:        "coding_halted",
	SignalReviewerApproved:   "review_approved",
	SignalReviewerBlocked:    "review_blocked",
	SignalPRCreated:          "pr_created",
	SignalPRFailed:           "failed",
	SignalPRCommentsResolved: "pr_comments_resolved",
	SignalPRMerged:           "pr_merged",
	SignalPipelineComplete:   "complete",
	SignalPipelineHalted:     "halted_human_needed",
	SignalPipelineFailed:     "failed",
	SignalPlannerNeedsInput:  "planner_needs_input",
}

// terminalPhases overwrite any prior phase regardless of position. They mirror
// the bash script's case `failed|halted_human_needed|coding_halted|planner_needs_input`.
var terminalPhases = map[string]struct{}{
	"failed":                {},
	"halted_human_needed":   {},
	"coding_halted":         {},
	"planner_needs_input":   {},
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
func signalRun(ctx context.Context, store dolt.Store, params SignalParams) (SignalResult, error) {
	if _, ok := signalKindIndex[params.Kind]; !ok {
		return SignalResult{}, gravaerrors.New("INVALID_SIGNAL",
			fmt.Sprintf("unknown signal kind %q", params.Kind), nil)
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
			return SignalResult{}, err
		}
	}

	target, writePhase, err := resolveTargetPhase(current, params.Kind)
	if err != nil {
		return SignalResult{}, gravaerrors.New("INVALID_SIGNAL", err.Error(), err)
	}

	if writePhase {
		if _, err := wispWrite(ctx, store, WispWriteParams{
			IssueID: params.IssueID,
			Key:     "pipeline_phase",
			Value:   target,
			Actor:   params.Actor,
		}); err != nil {
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
			return SignalResult{}, err
		}
	}

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
var worktreeRE = regexp.MustCompile(`/\.worktree/([a-zA-Z0-9_-]+)(/|$)`)

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
  CODER_DONE, CODER_HALTED,
  REVIEWER_APPROVED, REVIEWER_BLOCKED,
  PR_CREATED, PR_FAILED, PR_COMMENTS_RESOLVED, PR_MERGED,
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
