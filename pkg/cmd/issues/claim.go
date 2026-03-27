package issues

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

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
// DO NOT wrap in WithDeadlockRetry — would duplicate audit logs on retry.
func claimIssue(ctx context.Context, store dolt.Store, issueID, actor, model string) (ClaimResult, error) {
	var currentStatus string

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
			"SELECT status FROM issues WHERE id = ? FOR UPDATE", issueID)
		if err := row.Scan(&currentStatus); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return gravaerrors.New("ISSUE_NOT_FOUND",
					fmt.Sprintf("issue %s not found", issueID), err)
			}
			return gravaerrors.New("DB_UNREACHABLE",
				fmt.Sprintf("failed to read issue %s", issueID), err)
		}
		if currentStatus == "in_progress" {
			return gravaerrors.New("ALREADY_CLAIMED",
				fmt.Sprintf("issue %s is already claimed", issueID), nil)
		}
		if currentStatus != "open" {
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
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Claimed %s (status: in_progress, actor: %s)\n",
				result.IssueID, result.Actor)
			return nil
		},
	}
}
