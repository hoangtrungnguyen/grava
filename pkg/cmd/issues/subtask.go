package issues

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/hoangtrungnguyen/grava/pkg/idgen"
	"github.com/hoangtrungnguyen/grava/pkg/validation"
	"github.com/spf13/cobra"
)

// SubtaskParams holds all inputs for the subtaskIssue named function.
type SubtaskParams struct {
	ParentID      string
	Title         string
	Description   string
	IssueType     string
	Priority      string
	Ephemeral     bool
	AffectedFiles []string
	Actor         string
	Model         string
}

// SubtaskResult is the JSON output model for the subtask command (NFR5).
type SubtaskResult struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Priority  string `json:"priority"`
	Ephemeral bool   `json:"ephemeral,omitempty"`
}

// subtaskIssue creates a new subtask under an existing parent issue.
// It validates inputs, generates a hierarchical child ID, and wraps the mutation in WithAuditedTx.
// All user-facing errors are returned as *gravaerrors.GravaError.
func subtaskIssue(ctx context.Context, store dolt.Store, params SubtaskParams) (SubtaskResult, error) {
	// Validate required fields
	if params.Title == "" {
		return SubtaskResult{}, gravaerrors.New("MISSING_REQUIRED_FIELD", "title is required", nil)
	}

	// Validate issue type
	if err := validation.ValidateIssueType(params.IssueType); err != nil {
		return SubtaskResult{}, gravaerrors.New("INVALID_ISSUE_TYPE",
			fmt.Sprintf("invalid issue type: '%s'", params.IssueType), err)
	}

	// Validate priority and convert to int
	pInt, err := validation.ValidatePriority(params.Priority)
	if err != nil {
		return SubtaskResult{}, gravaerrors.New("INVALID_PRIORITY",
			fmt.Sprintf("invalid priority: '%s'", params.Priority), err)
	}

	// Verify parent exists before generating a child ID. GenerateChildID increments
	// child_counters in its own committed transaction and cannot be rolled back.
	// Checking the parent first avoids wasting sequence numbers on invalid parents.
	// TOCTOU note: concurrent parent deletion between this check and the insert is
	// acceptable — grava does not support concurrent issue deletion in Phase 1.
	var parentCount int
	if err := store.QueryRow("SELECT COUNT(*) FROM issues WHERE id = ?", params.ParentID).Scan(&parentCount); err != nil {
		return SubtaskResult{}, gravaerrors.New("DB_UNREACHABLE", "failed to check parent existence", err)
	}
	if parentCount == 0 {
		return SubtaskResult{}, gravaerrors.New("ISSUE_NOT_FOUND",
			fmt.Sprintf("Issue %s not found", params.ParentID), nil)
	}

	// Generate child ID BEFORE opening the transaction — GenerateChildID opens its own
	// internal transaction; nesting it inside WithAuditedTx causes conflicts with sqlmock.
	generator := idgen.NewStandardGenerator(store)
	id, err := generator.GenerateChildID(params.ParentID)
	if err != nil {
		return SubtaskResult{}, gravaerrors.New("DB_UNREACHABLE", "failed to generate child ID", err)
	}

	ephemeralVal := 0
	if params.Ephemeral {
		ephemeralVal = 1
	}

	affectedFilesJSON := "[]"
	if len(params.AffectedFiles) > 0 {
		b, _ := json.Marshal(params.AffectedFiles) //nolint:errcheck // []string is always serializable
		affectedFilesJSON = string(b)
	}

	now := time.Now()

	auditEvents := []dolt.AuditEvent{
		{
			IssueID:   id,
			EventType: dolt.EventCreate,
			Actor:     params.Actor,
			Model:     params.Model,
			OldValue:  nil,
			NewValue: map[string]any{
				"title":    params.Title,
				"type":     params.IssueType,
				"priority": pInt,
				"status":   "open",
			},
		},
		{
			IssueID:   id,
			EventType: dolt.EventDependencyAdd,
			Actor:     params.Actor,
			Model:     params.Model,
			OldValue:  nil,
			NewValue:  map[string]any{"to_id": params.ParentID, "type": "subtask-of"},
		},
	}

	err = dolt.WithAuditedTx(ctx, store, auditEvents, func(tx *sql.Tx) error {
		insertQuery := `INSERT INTO issues (id, title, description, issue_type, priority, status, ephemeral, created_at, updated_at, created_by, updated_by, agent_model, affected_files)
		                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		if _, err := tx.ExecContext(ctx, insertQuery,
			id, params.Title, params.Description, params.IssueType, pInt, "open",
			ephemeralVal, now, now, params.Actor, params.Actor, params.Model, affectedFilesJSON,
		); err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to insert subtask", err)
		}

		depQuery := `INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)`
		if _, err := tx.ExecContext(ctx, depQuery,
			id, params.ParentID, "subtask-of", params.Actor, params.Actor, params.Model); err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to create subtask-of dependency", err)
		}

		return nil
	})
	if err != nil {
		return SubtaskResult{}, err
	}

	return SubtaskResult{
		ID:        id,
		Title:     params.Title,
		Status:    "open",
		Priority:  priorityToString[pInt],
		Ephemeral: params.Ephemeral,
	}, nil
}

// newSubtaskCmd builds the `grava subtask` cobra command.
// It delegates all logic to the subtaskIssue named function.
func newSubtaskCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subtask <parent_id>",
		Short: "Create a subtask",
		Long: `Create a new subtask for an existing issue.
The subtask ID will be hierarchical (e.g., parent_id.1).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parentID := args[0]
			title, _ := cmd.Flags().GetString("title")
			desc, _ := cmd.Flags().GetString("desc")
			issueType, _ := cmd.Flags().GetString("type")
			priority, _ := cmd.Flags().GetString("priority")
			ephemeral, _ := cmd.Flags().GetBool("ephemeral")

			result, err := subtaskIssue(cmd.Context(), *d.Store, SubtaskParams{
				ParentID:      parentID,
				Title:         title,
				Description:   desc,
				IssueType:     issueType,
				Priority:      priority,
				Ephemeral:     ephemeral,
				AffectedFiles: SubtaskAffectedFiles,
				Actor:         *d.Actor,
				Model:         *d.AgentModel,
			})
			if err != nil {
				if *d.OutputJSON {
					return writeJSONError(cmd, err)
				}
				return err
			}

			if *d.OutputJSON {
				b, _ := json.MarshalIndent(result, "", "  ") //nolint:errcheck // SubtaskResult is always serializable
				fmt.Fprintln(cmd.OutOrStdout(), string(b))   //nolint:errcheck
				return nil
			}

			if result.Ephemeral {
				cmd.Printf("👻 Created ephemeral subtask (Wisp): %s\n", result.ID)
			} else {
				cmd.Printf("✅ Created subtask: %s\n", result.ID)
			}
			return nil
		},
	}

	cmd.Flags().StringP("title", "t", "", "Subtask title (required)")
	cmd.Flags().StringP("desc", "d", "", "Subtask description")
	cmd.Flags().String("type", "task", "Subtask type (task, bug, epic, story)")
	cmd.Flags().StringP("priority", "p", "medium", "Subtask priority (low, medium, high, critical)")
	cmd.Flags().Bool("ephemeral", false, "Mark subtask as ephemeral (Wisp)")
	cmd.Flags().StringSliceVar(&SubtaskAffectedFiles, "files", []string{}, "Affected files (comma separated)")
	cmd.MarkFlagRequired("title") //nolint:errcheck

	return cmd
}
