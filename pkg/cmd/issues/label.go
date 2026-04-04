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

// LabelParams holds all inputs for the labelIssue named function.
type LabelParams struct {
	ID           string
	AddLabels    []string
	RemoveLabels []string
	Actor        string
	Model        string
}

// LabelResult is the JSON output model for the label command (NFR5).
type LabelResult struct {
	ID            string   `json:"id"`
	LabelsAdded   []string `json:"labels_added"`
	LabelsRemoved []string `json:"labels_removed"`
	CurrentLabels []string `json:"current_labels"`
}

// labelIssue adds and/or removes labels on an issue using the issue_labels table.
// All mutations are wrapped in WithAuditedTx. Adding an existing label is idempotent;
// removing a non-existent label is a graceful no-op.
func labelIssue(ctx context.Context, store dolt.Store, params LabelParams) (LabelResult, error) {
	if len(params.AddLabels) == 0 && len(params.RemoveLabels) == 0 {
		return LabelResult{}, gravaerrors.New("MISSING_REQUIRED_FIELD",
			"at least one --add or --remove flag is required", nil)
	}

	var currentLabels []string

	err := dolt.WithAuditedTx(ctx, store, []dolt.AuditEvent{
		{
			IssueID:   params.ID,
			EventType: dolt.EventLabel,
			Actor:     params.Actor,
			Model:     params.Model,
			OldValue:  map[string]any{"labels_removed": params.RemoveLabels},
			NewValue:  map[string]any{"labels_added": params.AddLabels},
		},
	}, func(tx *sql.Tx) error {
		// Pre-read: validate issue existence
		var exists string
		row := tx.QueryRowContext(ctx, "SELECT id FROM issues WHERE id = ?", params.ID)
		if err := row.Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return gravaerrors.New("ISSUE_NOT_FOUND",
					fmt.Sprintf("Issue %s not found", params.ID), nil)
			}
			return gravaerrors.New("DB_UNREACHABLE", "failed to read issue", err)
		}

		// Add labels (idempotent via INSERT IGNORE)
		for _, label := range params.AddLabels {
			_, err := tx.ExecContext(ctx,
				"INSERT IGNORE INTO issue_labels (issue_id, label, created_by) VALUES (?, ?, ?)",
				params.ID, label, params.Actor,
			)
			if err != nil {
				return gravaerrors.New("DB_UNREACHABLE", "failed to add label", err)
			}
		}

		// Remove labels (graceful no-op if not present)
		for _, label := range params.RemoveLabels {
			_, err := tx.ExecContext(ctx,
				"DELETE FROM issue_labels WHERE issue_id = ? AND label = ?",
				params.ID, label,
			)
			if err != nil {
				return gravaerrors.New("DB_UNREACHABLE", "failed to remove label", err)
			}
		}

		// Query final labels
		rows, err := tx.QueryContext(ctx,
			"SELECT label FROM issue_labels WHERE issue_id = ? ORDER BY label",
			params.ID,
		)
		if err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to query labels", err)
		}
		defer rows.Close() //nolint:errcheck

		for rows.Next() {
			var label string
			if err := rows.Scan(&label); err != nil {
				return gravaerrors.New("DB_UNREACHABLE", "failed to scan label", err)
			}
			currentLabels = append(currentLabels, label)
		}
		if err := rows.Err(); err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to read label rows", err)
		}

		return nil
	})
	if err != nil {
		return LabelResult{}, err
	}

	added := params.AddLabels
	if added == nil {
		added = []string{}
	}
	removed := params.RemoveLabels
	if removed == nil {
		removed = []string{}
	}
	if currentLabels == nil {
		currentLabels = []string{}
	}

	return LabelResult{
		ID:            params.ID,
		LabelsAdded:   added,
		LabelsRemoved: removed,
		CurrentLabels: currentLabels,
	}, nil
}

// LabelAddFlags is the StringSliceVar target for --add on the label command.
// Tests may reset this to nil between runs.
var LabelAddFlags []string

// LabelRemoveFlags is the StringSliceVar target for --remove on the label command.
// Tests may reset this to nil between runs.
var LabelRemoveFlags []string

// newLabelCmd builds the `grava label` cobra command.
// Interface: grava label <id> --add <label>... --remove <label>...
func newLabelCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label <id>",
		Short: "Add or remove labels on an issue",
		Long: `Add or remove labels on an issue.

Labels are stored in the issue_labels table. Adding an existing label
is idempotent. Removing a non-existent label is a graceful no-op.

Examples:
  grava label grava-abc --add bug --add critical
  grava label grava-abc --remove bug
  grava label grava-abc --add urgent --remove low-priority`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			result, err := labelIssue(cmd.Context(), *d.Store, LabelParams{
				ID:           id,
				AddLabels:    LabelAddFlags,
				RemoveLabels: LabelRemoveFlags,
				Actor:        *d.Actor,
				Model:        *d.AgentModel,
			})
			if err != nil {
				if *d.OutputJSON {
					return writeJSONError(cmd, err)
				}
				return err
			}

			if *d.OutputJSON {
				b, _ := json.MarshalIndent(result, "", "  ") //nolint:errcheck
				fmt.Fprintln(cmd.OutOrStdout(), string(b))   //nolint:errcheck
				return nil
			}

			if len(result.LabelsAdded) > 0 {
				cmd.Printf("🏷️  Labels added to %s: %v\n", id, result.LabelsAdded)
			}
			if len(result.LabelsRemoved) > 0 {
				cmd.Printf("🏷️  Labels removed from %s: %v\n", id, result.LabelsRemoved)
			}
			cmd.Printf("🏷️  Current labels: %v\n", result.CurrentLabels)
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&LabelAddFlags, "add", nil, "Label(s) to add (repeatable)")
	cmd.Flags().StringSliceVar(&LabelRemoveFlags, "remove", nil, "Label(s) to remove (repeatable)")

	return cmd
}
