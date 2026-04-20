package issues

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/hoangtrungnguyen/grava/pkg/utils"
	"github.com/spf13/cobra"
)

// ClaimResult is returned by claimIssue on success.
type ClaimResult struct {
	IssueID string `json:"id"`
	Status  string `json:"status"`
	Actor   string `json:"actor"`
}

// ClaimIssue is the exported entry point for claiming an issue (used by sandbox scenarios).
// See claimIssue for implementation details.
func ClaimIssue(ctx context.Context, store dolt.Store, issueID, actor, model string) (ClaimResult, error) {
	return claimIssue(ctx, store, issueID, actor, model)
}

// claimIssue atomically claims an issue by setting its status to in_progress.
// Uses WithAuditedTx for atomic write + audit log.
// Retries on serialization failure (DB_COMMIT_FAILED) to ensure clean error codes in races.
func claimIssue(ctx context.Context, store dolt.Store, issueID, actor, model string) (ClaimResult, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		result, err := tryClaimIssue(ctx, store, issueID, actor, model)
		if err == nil {
			return result, nil
		}

		// If we encounter a serialization conflict during commit, retry.
		// On the next attempt, query will find the updated state and return ALREADY_CLAIMED.
		if gerr, ok := err.(*gravaerrors.GravaError); ok && gerr.Code == "DB_COMMIT_FAILED" {
			lastErr = err
			time.Sleep(50 * time.Millisecond)
			continue
		}
		return ClaimResult{}, err
	}
	return ClaimResult{}, lastErr
}

func tryClaimIssue(ctx context.Context, store dolt.Store, issueID, actor, model string) (ClaimResult, error) {
	// Enforce a 5s timeout if the caller didn't set one (NFR2 defensive guard).
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	var currentStatus string
	var currentAssignee sql.NullString

	err := dolt.WithAuditedTx(ctx, store, []dolt.AuditEvent{
		{
			IssueID:   issueID,
			EventType: dolt.EventClaim,
			Actor:     actor,
			Model:     model,
			OldValue:  map[string]any{"status": "open"},
			NewValue:  map[string]any{"status": "in_progress", "actor": actor},
		},
	}, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx,
			"SELECT status, assignee, wisp_heartbeat_at FROM issues WHERE id = ? FOR UPDATE", issueID)
		var heartbeat sql.NullTime
		if err := row.Scan(&currentStatus, &currentAssignee, &heartbeat); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return gravaerrors.New("ISSUE_NOT_FOUND",
					fmt.Sprintf("issue %s not found", issueID), err)
			}
			return gravaerrors.New("DB_UNREACHABLE",
				fmt.Sprintf("failed to read issue %s", issueID), err)
		}
		// Handle claiming if stale/crashed
		isStale := false
		if currentStatus == "in_progress" && heartbeat.Valid {
			// TTL = 1 hour (NFR1 Resilience)
			if time.Since(heartbeat.Time) > 1*time.Hour {
				isStale = true
			}
		}

		if currentStatus == "in_progress" && !isStale {
			claimedBy := currentAssignee.String
			if claimedBy == "" {
				claimedBy = "unknown"
			}
			return gravaerrors.New("ALREADY_CLAIMED",
				fmt.Sprintf("Issue %s is already in progress (claimed by %s, last heartbeat: %v)",
					issueID, claimedBy, heartbeat.Time.Format(time.RFC3339)), nil)
		}
		if currentStatus != "open" && !isStale {
			return gravaerrors.New("INVALID_STATUS_TRANSITION",
				fmt.Sprintf("cannot claim issue %s: status is %q (must be \"open\")", issueID, currentStatus), nil)
		}
		_, err := tx.ExecContext(ctx,
			"UPDATE issues SET status='in_progress', assignee=?, agent_model=?, updated_at=NOW(), updated_by=? WHERE id=?",
			actor, model, actor, issueID)
		if err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to update issue status", err)
		}
		return nil
	})
	if err != nil {
		return ClaimResult{}, err
	}
	return ClaimResult{IssueID: issueID, Status: "in_progress", Actor: actor}, nil
}

func newClaimCmd(d *cmddeps.Deps) *cobra.Command {
	var launch bool

	cmd := &cobra.Command{
		Use:   "claim <issue-id>",
		Short: "Claim an issue and provision a Git worktree",
		Long:  `Claim an issue by setting its status to in_progress and provisioning a Git worktree at .worktree/<issue-id> with branch grava/<issue-id>. Use --launch to also create a Claude-compatible symlink.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			issueID := args[0]

			// Get current working directory for worktree operations
			cwd, err := os.Getwd()
			if err != nil {
				return gravaerrors.New("CWD_UNREACHABLE", "failed to get working directory", err)
			}

			// Step 1: Check for worktree conflicts BEFORE attempting DB claim
			if err := utils.CheckWorktreeConflict(cwd, issueID); err != nil {
				return gravaerrors.New("WORKTREE_CONFLICT", err.Error(), err)
			}

			// Step 2: Attempt DB claim
			result, err := claimIssue(ctx, *d.Store, issueID, *d.Actor, *d.AgentModel)
			if err != nil {
				return err
			}

			// Step 3: DB claim succeeded, now provision worktree
			if provErr := utils.ProvisionWorktree(cwd, issueID); provErr != nil {
				// Worktree provisioning failed, rollback DB state
				rollbackErr := rollbackClaimDB(ctx, *d.Store, issueID)
				if rollbackErr != nil {
					return gravaerrors.New("ATOMIC_FAILURE",
						fmt.Sprintf("worktree provisioning failed (%v) and rollback failed (%v)", provErr, rollbackErr),
						provErr)
				}
				return gravaerrors.New("WORKTREE_PROVISION_FAILED",
					fmt.Sprintf("failed to provision worktree: %v", provErr), provErr)
			}

			// Step 3b: Sync Claude settings and git user config into the new worktree.
			// Both calls are non-fatal: on error, warn and continue.
			worktreeDir := utils.WorktreePath(cwd, issueID)
			if err := utils.SyncClaudeSettings(cwd, worktreeDir); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  Claude settings sync failed: %v\n", err)
			}
			if err := utils.ConfigureGitUser(cwd, worktreeDir); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  git user config failed: %v\n", err)
			}

			// Step 4: If --launch, create Claude symlink
			if launch {
				if linkErr := utils.LinkClaudeWorktree(cwd, issueID); linkErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  Claude symlink failed: %v\n", linkErr)
				}
			}

			// Success
			if *d.OutputJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Claimed %s (status: in_progress, actor: %s)\n✅ Provisioned worktree: .worktree/%s (branch: grava/%s)\n",
				result.IssueID, result.Actor, issueID, issueID)
			if launch {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Claude symlink: .claude/worktrees/%s → .worktree/%s\n"+
					"   Launch Claude in worktree: cd .worktree/%s && claude\n",
					issueID, issueID, issueID)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&launch, "launch", false, "Create Claude-compatible symlink at .claude/worktrees/<id>")
	return cmd
}

// rollbackClaimDB reverts the issue status from in_progress back to open.
// Called when worktree provisioning fails after DB claim succeeds.
func rollbackClaimDB(ctx context.Context, store dolt.Store, issueID string) error {
	// Enforce a 5s timeout if the caller didn't set one
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	err := dolt.WithAuditedTx(ctx, store, []dolt.AuditEvent{
		{
			IssueID:   issueID,
			EventType: dolt.EventUpdate,
			OldValue:  map[string]any{"status": "in_progress"},
			NewValue:  map[string]any{"status": "open"},
		},
	}, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			"UPDATE issues SET status='open', assignee=NULL, agent_model=NULL, updated_at=NOW() WHERE id=?",
			issueID)
		if err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to rollback issue status", err)
		}
		return nil
	})
	return err
}
