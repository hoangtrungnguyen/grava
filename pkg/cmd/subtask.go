package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/idgen"
	"github.com/hoangtrungnguyen/grava/pkg/validation"
	"github.com/spf13/cobra"
)

// subtaskCmd represents the subtask command
var subtaskCmd = &cobra.Command{
	Use:   "subtask <parent_id>",
	Short: "Create a subtask",
	Long: `Create a new subtask for an existing issue.
The subtask ID will be hierarchical (e.g., parent_id.1).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		parentID := args[0]
		subtaskTitle, _ := cmd.Flags().GetString("title")
		subtaskDesc, _ := cmd.Flags().GetString("desc")
		subtaskType, _ := cmd.Flags().GetString("type")
		subtaskPriority, _ := cmd.Flags().GetString("priority")
		subtaskEphemeral, _ := cmd.Flags().GetBool("ephemeral")
		// Use global slice var
		// subtaskAffectedFiles, _ := cmd.Flags().GetStringSlice("files")

		// 1. Initialize Generator and start Transaction
		generator := idgen.NewStandardGenerator(Store)
		ctx := context.Background()

		// Validate inputs before starting transaction
		if err := validation.ValidateIssueType(subtaskType); err != nil {
			return err
		}

		pInt, err := validation.ValidatePriority(subtaskPriority)
		if err != nil {
			return err
		}

		tx, err := Store.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}
		defer tx.Rollback()

		// 2. Verify Parent Exists (within transaction)
		var exists int
		err = tx.QueryRowContext(ctx, "SELECT 1 FROM issues WHERE id = ?", parentID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("parent issue %s not found: %w", parentID, err)
		}

		// 3. Generate Subtask ID
		id, err := generator.GenerateChildID(parentID)
		if err != nil {
			return fmt.Errorf("failed to generate subtask ID: %w", err)
		}

		// 4. Insert into DB (with Audit Log)
		ephemeralVal := 0
		if subtaskEphemeral {
			ephemeralVal = 1
		}

		affectedFilesJSON := "[]"
		if len(subtaskAffectedFiles) > 0 {
			b, _ := json.Marshal(subtaskAffectedFiles)
			affectedFilesJSON = string(b)
		}

		query := `INSERT INTO issues (id, title, description, issue_type, priority, status, ephemeral, created_at, updated_at, created_by, updated_by, agent_model, affected_files) 
                  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

		_, err = tx.ExecContext(ctx, query, id, subtaskTitle, subtaskDesc, subtaskType, pInt, "open", ephemeralVal, time.Now(), time.Now(), actor, actor, agentModel, affectedFilesJSON)
		if err != nil {
			return fmt.Errorf("failed to insert subtask: %w", err)
		}

		// 5. Add parent-child dependency
		depQuery := `INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)`
		_, err = tx.ExecContext(ctx, depQuery, parentID, id, "parent-child", actor, actor, agentModel)
		if err != nil {
			return fmt.Errorf("failed to create parent-child dependency: %w", err)
		}

		// Audit Log
		err = Store.LogEventTx(ctx, tx, id, "create", actor, agentModel, nil, map[string]interface{}{
			"title":     subtaskTitle,
			"type":      subtaskType,
			"priority":  pInt,
			"parent_id": parentID,
		})
		if err != nil {
			return fmt.Errorf("failed to log event: %w", err)
		}

		// Audit Log for the edge
		err = Store.LogEventTx(ctx, tx, parentID, "dependency_add", actor, agentModel, nil, map[string]interface{}{
			"to_id": id,
			"type":  "parent-child",
		})
		if err != nil {
			return fmt.Errorf("failed to log dependency event: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		if outputJSON {
			resp := map[string]string{
				"id":     id,
				"status": "created",
			}
			if subtaskEphemeral {
				resp["ephemeral"] = "true"
			}
			b, _ := json.MarshalIndent(resp, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		}

		if subtaskEphemeral {
			cmd.Printf("👻 Created ephemeral subtask (Wisp): %s\n", id)
		} else {
			cmd.Printf("✅ Created subtask: %s\n", id)
		}

		return nil
	},
}

var subtaskAffectedFiles []string

func init() {
	rootCmd.AddCommand(subtaskCmd)

	subtaskCmd.Flags().StringP("title", "t", "", "Subtask title (required)")
	subtaskCmd.Flags().StringP("desc", "d", "", "Subtask description")
	subtaskCmd.Flags().String("type", "task", "Subtask type (task, bug, epic, story)")
	subtaskCmd.Flags().StringP("priority", "p", "medium", "Subtask priority (low, medium, high, critical)")
	subtaskCmd.Flags().Bool("ephemeral", false, "Mark subtask as ephemeral (Wisp)")
	subtaskCmd.Flags().StringSliceVar(&subtaskAffectedFiles, "files", []string{}, "Affected files (comma separated)")

	subtaskCmd.MarkFlagRequired("title")
}
