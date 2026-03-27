package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/hoangtrungnguyen/grava/pkg/idgen"
	"github.com/hoangtrungnguyen/grava/pkg/validation"
	"github.com/spf13/cobra"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new issue",
	Long: `Create a new issue in the Grava tracker.
You can specify title, description, type, and priority.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Fetch flag values locally to avoid state leakage between test runs
		title, _ := cmd.Flags().GetString("title")
		desc, _ := cmd.Flags().GetString("desc")
		issueType, _ := cmd.Flags().GetString("type")
		priority, _ := cmd.Flags().GetString("priority")
		parentID, _ := cmd.Flags().GetString("parent")
		ephemeral, _ := cmd.Flags().GetBool("ephemeral")

		// Use global slice var (createAffectedFiles) to avoid accumulation issues with pflag StringSlice
		// Reset slices manually in tests.
		affectedFiles := createAffectedFiles

		// 1. Initialize Generator
		generator := idgen.NewStandardGenerator(Store)

		// Validate inputs
		if err := validation.ValidateIssueType(issueType); err != nil {
			return err
		}

		pInt, err := validation.ValidatePriority(priority)
		if err != nil {
			return err
		}

		// 2. Generate ID
		var id string
		if parentID != "" {
			id, err = generator.GenerateChildID(parentID)
			if err != nil {
				return fmt.Errorf("failed to generate child ID: %w", err)
			}
		} else {
			id = generator.GenerateBaseID()
		}

		// 3. Insert into DB (with Transaction and Audit Log)
		ephemeralVal := 0
		if ephemeral {
			ephemeralVal = 1
		}

		affectedFilesJSON := "[]"
		if len(affectedFiles) > 0 {
			b, _ := json.Marshal(affectedFiles)
			affectedFilesJSON = string(b)
		}

		ctx := context.Background()

		// 4. Pre-build audit events (all values are known before the transaction)
		auditEvents := []dolt.AuditEvent{
			{
				IssueID:   id,
				EventType: dolt.EventCreate,
				Actor:     actor,
				Model:     agentModel,
				OldValue:  nil,
				NewValue: map[string]interface{}{
					"title":    title,
					"type":     issueType,
					"priority": pInt,
					"status":   "open",
				},
			},
		}
		if parentID != "" {
			auditEvents = append(auditEvents, dolt.AuditEvent{
				IssueID:   id,
				EventType: dolt.EventDependencyAdd,
				Actor:     actor,
				Model:     agentModel,
				OldValue:  nil,
				NewValue:  map[string]interface{}{"to_id": parentID, "type": "subtask-of"},
			})
		}

		if err := dolt.WithAuditedTx(ctx, Store, auditEvents, func(tx *sql.Tx) error {
			query := `INSERT INTO issues (id, title, description, issue_type, priority, status, ephemeral, created_at, updated_at, created_by, updated_by, agent_model, affected_files)
                  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
			if _, err := tx.ExecContext(ctx, query, id, title, desc, issueType, pInt, "open", ephemeralVal, time.Now(), time.Now(), actor, actor, agentModel, affectedFilesJSON); err != nil {
				return fmt.Errorf("failed to insert issue: %w", err)
			}

			// Add subtask-of dependency if parent is specified
			if parentID != "" {
				var exists int
				if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM issues WHERE id = ?", parentID).Scan(&exists); err != nil {
					return fmt.Errorf("failed to check parent existence: %w", err)
				}
				if exists == 0 {
					return fmt.Errorf("parent issue %s not found", parentID)
				}

				// Epic 2.4 specifies 'subtask-of' for hierarchical breakdown; direction: child --subtask-of--> parent
				depQuery := `INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)`
				if _, err := tx.ExecContext(ctx, depQuery, id, parentID, "subtask-of", actor, actor, agentModel); err != nil {
					return fmt.Errorf("failed to create subtask-of dependency: %w", err)
				}
			}
			return nil
		}); err != nil {
			return err
		}

		if outputJSON {
			resp := map[string]string{
				"id":     id,
				"status": "created",
			}
			if ephemeral {
				resp["ephemeral"] = "true"
			}
			b, _ := json.MarshalIndent(resp, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
			return nil
		}

		if ephemeral {
			cmd.Printf("👻 Created ephemeral issue (Wisp): %s\n", id)
		} else {
			cmd.Printf("✅ Created issue: %s\n", id)
		}

		return nil
	},
}

var createAffectedFiles []string

