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
	"github.com/hoangtrungnguyen/grava/pkg/graph"
	"github.com/hoangtrungnguyen/grava/pkg/validation"
	"github.com/spf13/cobra"
)

// UpdateParams holds all inputs for the updateIssue named function.
// ChangedFields lists which fields to update (mirrors cobra Flags().Changed() logic).
type UpdateParams struct {
	ID            string
	Title         string
	Description   string
	IssueType     string
	Priority      string
	Status        string
	AffectedFiles []string
	Actor         string
	Model         string
	ChangedFields []string // e.g., ["title", "priority"] — only these fields are written
}

// UpdateResult is the JSON output model for the update command (NFR5).
type UpdateResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// updateIssue updates an existing issue's fields atomically via WithAuditedTx.
// Status changes are routed through the graph layer (dag.SetNodeStatus) which handles
// propagation and its own audit logging — they must NOT be nested inside WithAuditedTx.
// All user-facing errors are returned as *gravaerrors.GravaError.
func updateIssue(ctx context.Context, store dolt.Store, params UpdateParams) (UpdateResult, error) {
	if len(params.ChangedFields) == 0 {
		return UpdateResult{}, gravaerrors.New("MISSING_REQUIRED_FIELD",
			"at least one field must be specified to update", nil)
	}

	// Validate changed fields up-front (before any DB access)
	changedSet := make(map[string]bool, len(params.ChangedFields))
	for _, f := range params.ChangedFields {
		changedSet[f] = true
	}

	var pInt int
	if changedSet["priority"] {
		var err error
		pInt, err = validation.ValidatePriority(params.Priority)
		if err != nil {
			return UpdateResult{}, gravaerrors.New("INVALID_PRIORITY",
				fmt.Sprintf("invalid priority: '%s'. Allowed: critical, high, medium, low, backlog", params.Priority), err)
		}
	}

	if changedSet["type"] {
		if err := validation.ValidateIssueType(params.IssueType); err != nil {
			return UpdateResult{}, gravaerrors.New("INVALID_ISSUE_TYPE",
				fmt.Sprintf("invalid issue type: '%s'", params.IssueType), err)
		}
	}

	if changedSet["status"] {
		if err := validation.ValidateStatus(params.Status); err != nil {
			return UpdateResult{}, gravaerrors.New("INVALID_STATUS",
				fmt.Sprintf("invalid status: '%s'. Allowed: open, in_progress, closed, blocked", params.Status), err)
		}
	}

	// Pre-read current row for old values (used in audit events).
	// This follows the same pre-read-before-WithAuditedTx pattern as Story 2.2.
	// TOCTOU: concurrent modification between this read and the UPDATE is acceptable in Phase 1.
	type currentRow struct {
		title    string
		desc     string
		iType    string
		priority int
		status   string
		assignee string
	}
	var cur currentRow
	row := store.QueryRow(
		"SELECT title, description, issue_type, priority, status, COALESCE(assignee, '') FROM issues WHERE id = ?",
		params.ID,
	)
	if err := row.Scan(&cur.title, &cur.desc, &cur.iType, &cur.priority, &cur.status, &cur.assignee); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UpdateResult{}, gravaerrors.New("ISSUE_NOT_FOUND",
				fmt.Sprintf("Issue %s not found", params.ID), nil)
		}
		return UpdateResult{}, gravaerrors.New("DB_UNREACHABLE", "failed to read issue", err)
	}

	// Handle status update via graph layer — dag.SetNodeStatus manages its own tx + audit.
	if changedSet["status"] {
		dag, err := graph.LoadGraphFromDB(store)
		if err != nil {
			return UpdateResult{}, gravaerrors.New("DB_UNREACHABLE", "failed to load graph for status propagation", err)
		}
		dag.SetSession(params.Actor, params.Model)
		if err := dag.SetNodeStatus(params.ID, graph.IssueStatus(params.Status)); err != nil {
			return UpdateResult{}, gravaerrors.New("DB_UNREACHABLE", "failed to update status via graph", err)
		}
	}

	// Collect non-status fields to update.
	nonStatusFields := make([]string, 0, len(params.ChangedFields))
	for _, f := range params.ChangedFields {
		if f != "status" {
			nonStatusFields = append(nonStatusFields, f)
		}
	}

	// If only status changed, skip WithAuditedTx entirely.
	if len(nonStatusFields) == 0 {
		return UpdateResult{ID: params.ID, Status: "updated"}, nil
	}

	// Build audit events for non-status fields (one per changed field).
	auditEvents := make([]dolt.AuditEvent, 0, len(nonStatusFields))
	for _, field := range nonStatusFields {
		var oldVal, newVal any
		switch field {
		case "title":
			oldVal = cur.title
			newVal = params.Title
		case "desc":
			oldVal = cur.desc
			newVal = params.Description
		case "type":
			oldVal = cur.iType
			newVal = params.IssueType
		case "priority":
			oldVal = cur.priority
			newVal = pInt
		case "files":
			oldVal = nil
			newVal = params.AffectedFiles
		}
		auditEvents = append(auditEvents, dolt.AuditEvent{
			IssueID:   params.ID,
			EventType: dolt.EventUpdate,
			Actor:     params.Actor,
			Model:     params.Model,
			OldValue:  map[string]any{"field": field, "value": oldVal},
			NewValue:  map[string]any{"field": field, "value": newVal},
		})
	}

	now := time.Now()

	err := dolt.WithAuditedTx(ctx, store, auditEvents, func(tx *sql.Tx) error {
		query := "UPDATE issues SET updated_at = ?, updated_by = ?, agent_model = ?"
		queryParams := []any{now, params.Actor, params.Model}

		for _, field := range nonStatusFields {
			switch field {
			case "title":
				query += ", title = ?"
				queryParams = append(queryParams, params.Title)
			case "desc":
				query += ", description = ?"
				queryParams = append(queryParams, params.Description)
			case "type":
				query += ", issue_type = ?"
				queryParams = append(queryParams, params.IssueType)
			case "priority":
				query += ", priority = ?"
				queryParams = append(queryParams, pInt)
			case "files":
				query += ", affected_files = ?"
				b, _ := json.Marshal(params.AffectedFiles) //nolint:errcheck // []string is always serializable
				queryParams = append(queryParams, string(b))
			}
		}

		query += " WHERE id = ?"
		queryParams = append(queryParams, params.ID)

		if _, err := tx.ExecContext(ctx, query, queryParams...); err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to update issue", err)
		}
		return nil
	})
	if err != nil {
		return UpdateResult{}, err
	}

	return UpdateResult{ID: params.ID, Status: "updated"}, nil
}

// newUpdateCmd builds the `grava update` cobra command.
// It delegates all logic to the updateIssue named function.
func newUpdateCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an existing issue",
		Long: `Update specific fields of an existing issue.
Only the flags provided will be updated.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			// Collect which fields were explicitly set by the caller
			var changedFields []string
			if cmd.Flags().Changed("title") {
				changedFields = append(changedFields, "title")
			}
			if cmd.Flags().Changed("desc") {
				changedFields = append(changedFields, "desc")
			}
			if cmd.Flags().Changed("type") {
				changedFields = append(changedFields, "type")
			}
			if cmd.Flags().Changed("priority") {
				changedFields = append(changedFields, "priority")
			}
			if cmd.Flags().Changed("status") {
				changedFields = append(changedFields, "status")
			}
			if cmd.Flags().Changed("files") {
				changedFields = append(changedFields, "files")
			}

			title, _ := cmd.Flags().GetString("title")
			desc, _ := cmd.Flags().GetString("desc")
			issueType, _ := cmd.Flags().GetString("type")
			priority, _ := cmd.Flags().GetString("priority")
			status, _ := cmd.Flags().GetString("status")

			result, err := updateIssue(cmd.Context(), *d.Store, UpdateParams{
				ID:            id,
				Title:         title,
				Description:   desc,
				IssueType:     issueType,
				Priority:      priority,
				Status:        status,
				AffectedFiles: UpdateAffectedFiles,
				Actor:         *d.Actor,
				Model:         *d.AgentModel,
				ChangedFields: changedFields,
			})
			if err != nil {
				if *d.OutputJSON {
					return writeJSONError(cmd, err)
				}
				return err
			}

			// Handle --last-commit separately (metadata-only, no audit event needed)
			if cmd.Flags().Changed("last-commit") {
				val, _ := cmd.Flags().GetString("last-commit")
				if err := setLastCommit(d, id, val); err != nil {
					return err
				}
			}

			if *d.OutputJSON {
				b, _ := json.MarshalIndent(result, "", "  ") //nolint:errcheck // UpdateResult is always serializable
				fmt.Fprintln(cmd.OutOrStdout(), string(b))  //nolint:errcheck
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✅ Updated issue %s\n", result.ID) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringP("title", "t", "", "Update title")
	cmd.Flags().StringP("desc", "d", "", "Update description")
	cmd.Flags().String("type", "", "Update type")
	cmd.Flags().StringP("priority", "p", "", "Update priority")
	cmd.Flags().StringP("status", "s", "", "Update status")
	cmd.Flags().StringSliceVar(&UpdateAffectedFiles, "files", []string{}, "Update affected files")
	cmd.Flags().String("last-commit", "", "Store the last session's commit hash")
	return cmd
}
