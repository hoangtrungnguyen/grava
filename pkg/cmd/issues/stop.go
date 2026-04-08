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

// StopParams holds all inputs for the stopIssue named function.
type StopParams struct {
	ID    string
	Actor string
	Model string
}

// StopResult is the JSON output model for the stop command (NFR5).
type StopResult struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	StoppedAt string `json:"stopped_at"`
}

// stopIssue marks work as stopped on an issue, transitioning status from in_progress → open.
// It validates issue existence and status (must be in_progress), and records the timestamp.
// All user-facing errors are returned as *gravaerrors.GravaError.
func stopIssue(ctx context.Context, store dolt.Store, params StopParams) (StopResult, error) {
	stopTime := time.Now()

	// All reads and writes happen inside one transaction with SELECT FOR UPDATE to prevent
	// concurrent agents from racing on status checks (TOCTOU fix).
	err := dolt.WithAuditedTx(ctx, store, []dolt.AuditEvent{
		{
			IssueID:   params.ID,
			EventType: dolt.EventStop,
			Actor:     params.Actor,
			Model:     params.Model,
			OldValue:  map[string]any{"status": "in_progress"},
			NewValue:  map[string]any{"status": "open", "stopped_at": stopTime.Format(time.RFC3339)},
		},
	}, func(tx *sql.Tx) error {
		var currentStatus string
		row := tx.QueryRowContext(ctx,
			"SELECT status FROM issues WHERE id = ? FOR UPDATE",
			params.ID,
		)
		if err := row.Scan(&currentStatus); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return gravaerrors.New("ISSUE_NOT_FOUND",
					fmt.Sprintf("Issue %s not found", params.ID), nil)
			}
			return gravaerrors.New("DB_UNREACHABLE", "failed to read issue", err)
		}

		if currentStatus != "in_progress" {
			return gravaerrors.New("NOT_IN_PROGRESS",
				"Cannot stop work on an issue not in progress", nil)
		}

		if _, err := tx.ExecContext(ctx,
			"UPDATE issues SET status = ?, stopped_at = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?",
			"open", stopTime, stopTime, params.Actor, params.Model, params.ID,
		); err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to stop work on issue", err)
		}
		return nil
	})
	if err != nil {
		return StopResult{}, err
	}

	return StopResult{
		ID:        params.ID,
		Status:    "open",
		StoppedAt: stopTime.Format(time.RFC3339),
	}, nil
}

// newStopCmd builds the `grava stop` cobra command.
// It delegates all logic to the stopIssue named function.
// Interface: grava stop <id>
func newStopCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <id>",
		Short: "Mark work as stopped on an issue",
		Long: `Mark work as stopped on an issue, transitioning status back to open (ready queue).

This records the timestamp when work ended for cycle time measurement.

Example:
  grava stop grava-abc`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			result, err := stopIssue(cmd.Context(), *d.Store, StopParams{
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
				b, _ := json.MarshalIndent(result, "", "  ") //nolint:errcheck // StopResult is always serializable
				fmt.Fprintln(cmd.OutOrStdout(), string(b))   //nolint:errcheck
				return nil
			}

			cmd.Printf("⏹️  Stopped work on %s\n", result.ID)
			return nil
		},
	}

	return cmd
}
