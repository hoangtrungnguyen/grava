package issues

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/spf13/cobra"
)

// ClaimResult is returned by claimIssue on success.
type ClaimResult struct {
	IssueID string `json:"id"`
	Status  string `json:"status"`
	Actor   string `json:"actor"`
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

		if (currentStatus == "in_progress" && !isStale) || (currentAssignee.Valid && currentAssignee.String != "" && !isStale) {
			return gravaerrors.New("ALREADY_CLAIMED",
				fmt.Sprintf("Issue %s is already claimed by %s (last heartbeat: %v)",
					issueID, currentAssignee.String, heartbeat.Time.Format(time.RFC3339)), nil)
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
	return &cobra.Command{
		Use:   "claim <issue-id>",
		Short: "Claim an issue (set status to in_progress)",
		Long:  `Claim an issue by atomically setting its status to in_progress and assigning it to the current actor.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			issueID := args[0]
			result, err := claimIssue(ctx, *d.Store, issueID, *d.Actor, *d.AgentModel)
			if err != nil {
				return err
			}
			if *d.OutputJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Claimed %s (status: in_progress, actor: %s)\n",
				result.IssueID, result.Actor)
			return nil
		},
	}
}
