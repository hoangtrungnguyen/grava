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

// StartParams holds all inputs for the startIssue named function.
type StartParams struct {
	ID    string
	Actor string
	Model string
}

// StartResult is the JSON output model for the start command (NFR5).
type StartResult struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	StartedAt string `json:"started_at"`
}

// startIssue marks work as started on an issue, transitioning status from open → in_progress.
// It validates issue existence, checks for conflicts (already in progress), and records the timestamp.
// All user-facing errors are returned as *gravaerrors.GravaError.
func startIssue(ctx context.Context, store dolt.Store, params StartParams) (StartResult, error) {
	startTime := time.Now()
	var currentAssignee string

	// All reads and writes happen inside one transaction with SELECT FOR UPDATE to prevent
	// concurrent agents from simultaneously claiming the same issue (TOCTOU fix).
	err := dolt.WithAuditedTx(ctx, store, []dolt.AuditEvent{
		{
			IssueID:   params.ID,
			EventType: dolt.EventStart,
			Actor:     params.Actor,
			Model:     params.Model,
			OldValue:  map[string]any{"status": "open"},
			NewValue:  map[string]any{"status": "in_progress", "started_at": startTime.Format(time.RFC3339)},
		},
	}, func(tx *sql.Tx) error {
		var currentStatus string
		row := tx.QueryRowContext(ctx,
			"SELECT status, COALESCE(assignee, '') FROM issues WHERE id = ? FOR UPDATE",
			params.ID,
		)
		if err := row.Scan(&currentStatus, &currentAssignee); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return gravaerrors.New("ISSUE_NOT_FOUND",
					fmt.Sprintf("Issue %s not found", params.ID), nil)
			}
			return gravaerrors.New("DB_UNREACHABLE", "failed to read issue", err)
		}

		if currentStatus == "in_progress" {
			msg := "Issue is already being worked on"
			if currentAssignee != "" {
				msg = fmt.Sprintf("Issue is already being worked on by %s", currentAssignee)
			}
			return gravaerrors.New("ALREADY_IN_PROGRESS", msg, nil)
		}

		if _, err := tx.ExecContext(ctx,
			"UPDATE issues SET status = ?, started_at = ?, stopped_at = NULL, assignee = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?",
			"in_progress", startTime, params.Actor, startTime, params.Actor, params.Model, params.ID,
		); err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to start work on issue", err)
		}
		return nil
	})
	if err != nil {
		return StartResult{}, err
	}

	return StartResult{
		ID:        params.ID,
		Status:    "in_progress",
		StartedAt: startTime.Format(time.RFC3339),
	}, nil
}

// newStartCmd builds the `grava start` cobra command.
// It delegates all logic to the startIssue named function.
// Interface: grava start <id>
func newStartCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <id>",
		Short: "Mark work as started on an issue",
		Long: `Mark work as started on an issue, transitioning status to in_progress.

This records the timestamp when work began for cycle time measurement.

Example:
  grava start grava-abc`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			result, err := startIssue(cmd.Context(), *d.Store, StartParams{
				ID:    id,
				Actor: *d.Actor,
				Model: *d.AgentModel,
			})
			if err != nil {
				if *d.OutputJSON {
					return writeJSONError(cmd, err)
				}
				return err
			}

			if *d.OutputJSON {
				b, _ := json.MarshalIndent(result, "", "  ") //nolint:errcheck // StartResult is always serializable
				fmt.Fprintln(cmd.OutOrStdout(), string(b))  //nolint:errcheck
				return nil
			}

			cmd.Printf("▶️  Started work on %s\n", result.ID)
			return nil
		},
	}

	return cmd
}
