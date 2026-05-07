package issues

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/hoangtrungnguyen/grava/pkg/idgen"
	"github.com/hoangtrungnguyen/grava/pkg/validation"
	"github.com/spf13/cobra"
)

// CreateParams holds all inputs for the createIssue named function.
type CreateParams struct {
	Title         string
	Description   string
	IssueType     string
	Priority      string
	ParentID      string
	Ephemeral     bool
	AffectedFiles []string
	Actor         string
	Model         string
}

// CreateResult is the JSON output model for the create and quick commands (NFR5).
type CreateResult struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Priority  string `json:"priority"`
	Ephemeral bool   `json:"ephemeral,omitempty"`
}

// priorityToString maps the integer DB priority to its string label.
var priorityToString = map[int]string{
	0: "critical",
	1: "high",
	2: "medium",
	3: "low",
	4: "backlog",
}

// CreateIssue is the exported entry point for creating an issue (used by sandbox scenarios).
func CreateIssue(ctx context.Context, store dolt.Store, params CreateParams) (CreateResult, error) {
	return createIssue(ctx, store, params)
}

// createIssue inserts a new issue into the database.
// It validates all inputs, generates a unique ID, and wraps the mutation in WithAuditedTx.
// All user-facing errors are returned as *gravaerrors.GravaError.
func createIssue(ctx context.Context, store dolt.Store, params CreateParams) (CreateResult, error) {
	// Validate required fields
	if params.Title == "" {
		return CreateResult{}, gravaerrors.New("MISSING_REQUIRED_FIELD", "title is required", nil)
	}

	// Normalize and validate issue type
	params.IssueType = strings.ToLower(strings.TrimSpace(params.IssueType))
	if err := validation.ValidateIssueType(params.IssueType); err != nil {
		return CreateResult{}, gravaerrors.New("INVALID_ISSUE_TYPE",
			fmt.Sprintf("invalid issue type: '%s'", params.IssueType), err)
	}

	// Validate priority and convert to int
	pInt, err := validation.ValidatePriority(params.Priority)
	if err != nil {
		return CreateResult{}, gravaerrors.New("INVALID_PRIORITY",
			fmt.Sprintf("invalid priority: '%s'", params.Priority), err)
	}

	// Generate ID
	generator := idgen.NewStandardGenerator(store)
	var id string
	if params.ParentID != "" {
		id, err = generator.GenerateChildID(params.ParentID)
		if err != nil {
			return CreateResult{}, gravaerrors.New("DB_UNREACHABLE", "failed to generate child ID", err)
		}
	} else {
		id = generator.GenerateBaseID()
	}

	ephemeralVal := 0
	if params.Ephemeral {
		ephemeralVal = 1
	}

	affectedFilesJSON := "[]"
	if len(params.AffectedFiles) > 0 {
		b, _ := json.Marshal(params.AffectedFiles)
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
	}

	if params.ParentID != "" {
		auditEvents = append(auditEvents, dolt.AuditEvent{
			IssueID:   id,
			EventType: dolt.EventDependencyAdd,
			Actor:     params.Actor,
			Model:     params.Model,
			OldValue:  nil,
			NewValue:  map[string]any{"to_id": params.ParentID, "type": "subtask-of"},
		})
	}

	err = dolt.WithAuditedTx(ctx, store, auditEvents, func(tx *sql.Tx) error {
		insertQuery := `INSERT INTO issues (id, title, description, issue_type, priority, status, ephemeral, created_at, updated_at, created_by, updated_by, agent_model, affected_files)
		                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		_, err := tx.ExecContext(ctx, insertQuery,
			id, params.Title, params.Description, params.IssueType, pInt, "open",
			ephemeralVal, now, now, params.Actor, params.Actor, params.Model, affectedFilesJSON,
		)
		if err != nil {
			return gravaerrors.New("DB_UNREACHABLE", fmt.Sprintf("failed to insert issue: %v", err), err)
		}

		if params.ParentID != "" {
			var exists int
			if err := tx.QueryRowContext(ctx,
				"SELECT COUNT(*) FROM issues WHERE id = ?", params.ParentID).Scan(&exists); err != nil {
				return gravaerrors.New("DB_UNREACHABLE", "failed to check parent existence", err)
			}
			if exists == 0 {
				return gravaerrors.New("PARENT_NOT_FOUND",
					fmt.Sprintf("parent issue %s not found", params.ParentID), nil)
			}

			depQuery := `INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)`
			if _, err := tx.ExecContext(ctx, depQuery,
				id, params.ParentID, "subtask-of", params.Actor, params.Actor, params.Model); err != nil {
				return gravaerrors.New("DB_UNREACHABLE", "failed to create subtask-of dependency", err)
			}
		}

		return nil
	})
	if err != nil {
		return CreateResult{}, err
	}

	return CreateResult{
		ID:        id,
		Title:     params.Title,
		Status:    "open",
		Priority:  priorityToString[pInt],
		Ephemeral: params.Ephemeral,
	}, nil
}

// newCreateCmd builds the `grava create` cobra command.
// It delegates all logic to the createIssue named function.
func newCreateCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new issue",
		Long: `Create a new issue in the Grava tracker.
You can specify title, description, type, and priority.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			title, _ := cmd.Flags().GetString("title")
			desc, _ := cmd.Flags().GetString("desc")
			issueType, _ := cmd.Flags().GetString("type")
			priority, _ := cmd.Flags().GetString("priority")
			parentID, _ := cmd.Flags().GetString("parent")
			ephemeral, _ := cmd.Flags().GetBool("ephemeral")

			result, err := createIssue(cmd.Context(), *d.Store, CreateParams{
				Title:         title,
				Description:   desc,
				IssueType:     issueType,
				Priority:      priority,
				ParentID:      parentID,
				Ephemeral:     ephemeral,
				AffectedFiles: CreateAffectedFiles,
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
				b, _ := json.MarshalIndent(result, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
				return nil
			}

			if result.Ephemeral {
				cmd.Printf("👻 Created ephemeral issue (Wisp): %s\n", result.ID)
			} else {
				cmd.Printf("✅ Created issue: %s\n", result.ID)
			}
			return nil
		},
	}

	cmd.Flags().StringP("title", "t", "", "Issue title (required)")
	cmd.Flags().StringP("desc", "d", "", "Issue description")
	cmd.Flags().String("type", "task", "Issue type (task, bug, epic, story)")
	cmd.Flags().StringP("priority", "p", "medium", "Issue priority (low, medium, high, critical)")
	cmd.Flags().String("parent", "", "Parent Issue ID for sub-tasks")
	cmd.Flags().Bool("ephemeral", false, "Mark issue as ephemeral (Wisp) — excluded from normal queries")
	cmd.Flags().StringSliceVar(&CreateAffectedFiles, "files", []string{}, "Affected files (comma separated)")

	return cmd
}

// writeJSONError writes a structured JSON error envelope to stderr and returns nil
// so that cobra does not double-print the error.
func writeJSONError(cmd *cobra.Command, err error) error {
	type jsonErrorDetail struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	type jsonErrorEnvelope struct {
		Error jsonErrorDetail `json:"error"`
	}

	code := "INTERNAL_ERROR"
	message := err.Error()

	var gravaErr *gravaerrors.GravaError
	if errors.As(err, &gravaErr) {
		code = gravaErr.Code
		message = gravaErr.Message
	}

	envelope := jsonErrorEnvelope{Error: jsonErrorDetail{Code: code, Message: message}}
	b, _ := json.Marshal(envelope)
	fmt.Fprintln(cmd.ErrOrStderr(), string(b)) //nolint:errcheck
	return nil
}
