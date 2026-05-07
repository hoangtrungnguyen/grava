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

// AssignParams holds all inputs for the assignIssue named function.
type AssignParams struct {
	ID       string
	Assignee string // target assignee; empty string when Unassign=true
	Unassign bool   // if true, clears the assignee field
	Actor    string
	Model    string
}

// AssignResult is the JSON output model for the assign command (NFR5).
type AssignResult struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Assignee string `json:"assignee"`
}

// assignIssue sets or clears the assignee field on an existing issue.
// It validates issue existence, wraps the mutation in WithAuditedTx, and emits EventAssign.
// All user-facing errors are returned as *gravaerrors.GravaError.
func assignIssue(ctx context.Context, store dolt.Store, params AssignParams) (AssignResult, error) {
	newAssignee := params.Assignee
	if params.Unassign {
		newAssignee = ""
	}

	if err := guardNotArchived(store, params.ID); err != nil {
		return AssignResult{}, err
	}

	// Pre-read current assignee for old value in audit event.
	var currentAssignee string
	row := store.QueryRow(
		"SELECT COALESCE(assignee, '') FROM issues WHERE id = ?",
		params.ID,
	)
	if err := row.Scan(&currentAssignee); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AssignResult{}, gravaerrors.New("ISSUE_NOT_FOUND",
				fmt.Sprintf("Issue %s not found", params.ID), nil)
		}
		return AssignResult{}, gravaerrors.New("DB_UNREACHABLE", "failed to read issue", err)
	}

	auditEvents := []dolt.AuditEvent{
		{
			IssueID:   params.ID,
			EventType: dolt.EventAssign,
			Actor:     params.Actor,
			Model:     params.Model,
			OldValue:  map[string]any{"assignee": currentAssignee},
			NewValue:  map[string]any{"assignee": newAssignee},
		},
	}

	now := time.Now()

	err := dolt.WithAuditedTx(ctx, store, auditEvents, func(tx *sql.Tx) error {
		var assigneeVal any = newAssignee
		if newAssignee == "" {
			assigneeVal = nil // store NULL in DB when clearing
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE issues SET assignee = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?",
			assigneeVal, now, params.Actor, params.Model, params.ID,
		); err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to assign issue", err)
		}
		return nil
	})
	if err != nil {
		return AssignResult{}, err
	}

	return AssignResult{ID: params.ID, Status: "updated", Assignee: newAssignee}, nil
}

// newAssignCmd builds the `grava assign` cobra command.
// It delegates all logic to the assignIssue named function.
// Interface: grava assign <id> --actor <user> | --unassign
func newAssignCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assign <id>",
		Short: "Assign or unassign an issue",
		Long: `Set or clear the assignee field on an existing issue.

The assignee can be a human username or an agent identity string.
Use --unassign to clear the assignee.

Example:
  grava assign grava-abc --actor alice
  grava assign grava-abc --actor "agent:planner-v2"
  grava assign grava-abc --unassign`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			assignee, _ := cmd.Flags().GetString("actor")
			unassign, _ := cmd.Flags().GetBool("unassign")

			if !unassign && assignee == "" {
				return gravaerrors.New("MISSING_REQUIRED_FIELD",
					"either --actor <user> or --unassign must be specified", nil)
			}

			result, err := assignIssue(cmd.Context(), *d.Store, AssignParams{
				ID:       id,
				Assignee: assignee,
				Unassign: unassign,
				Actor:    *d.Actor,
				Model:    *d.AgentModel,
			})
			if err != nil {
				if *d.OutputJSON {
					return writeJSONError(cmd, err)
				}
				return err
			}

			if *d.OutputJSON {
				b, _ := json.MarshalIndent(result, "", "  ") //nolint:errcheck // AssignResult is always serializable
				fmt.Fprintln(cmd.OutOrStdout(), string(b))   //nolint:errcheck
				return nil
			}

			if result.Assignee == "" {
				cmd.Printf("👤 Assignee cleared on %s\n", result.ID)
			} else {
				cmd.Printf("👤 Assigned %s to %s\n", result.ID, result.Assignee)
			}
			return nil
		},
	}

	cmd.Flags().String("actor", "", "Assignee username or agent identity")
	cmd.Flags().Bool("unassign", false, "Clear the assignee field")
	return cmd
}
