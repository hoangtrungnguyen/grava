package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/idgen"
	"github.com/hoangtrungnguyen/grava/pkg/validation"
	"github.com/spf13/cobra"
)

var (
	subtaskTitle         string
	subtaskDesc          string
	subtaskType          string
	subtaskPriority      string
	subtaskEphemeral     bool
	subtaskAffectedFiles []string
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

		// 0. Verify Parent Exists
		var exists int
		err := Store.QueryRow("SELECT 1 FROM issues WHERE id = ?", parentID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("parent issue %s not found: %w", parentID, err)
		}

		// Validate inputs
		if err := validation.ValidateIssueType(subtaskType); err != nil {
			return err
		}

		pInt, err := validation.ValidatePriority(subtaskPriority)
		if err != nil {
			return err
		}

		// 1. Initialize Generator
		generator := idgen.NewStandardGenerator(Store)

		// 2. Generate Subtask ID
		id, err := generator.GenerateChildID(parentID)
		if err != nil {
			return fmt.Errorf("failed to generate subtask ID: %w", err)
		}

		// 3. Insert into DB
		ephemeralVal := 0
		if subtaskEphemeral {
			ephemeralVal = 1
		}

		affectedFilesJSON := "[]"
		if len(subtaskAffectedFiles) > 0 {
			b, _ := json.Marshal(subtaskAffectedFiles)
			affectedFilesJSON = string(b)
		}

		// Arguments: 1=id, 2=title, 3=desc, 4=type, 5=priority, 6=status, 7=ephemeral, 8=created_at, 9=updated_at, 10=created_by, 11=updated_by, 12=agent_model, 13=affected_files
		query := `INSERT INTO issues (id, title, description, issue_type, priority, status, ephemeral, created_at, updated_at, created_by, updated_by, agent_model, affected_files) 
                  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

		_, err = Store.Exec(query, id, subtaskTitle, subtaskDesc, subtaskType, pInt, "open", ephemeralVal, time.Now(), time.Now(), actor, actor, agentModel, affectedFilesJSON)
		if err != nil {
			return fmt.Errorf("failed to insert subtask: %w", err)
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

func init() {
	rootCmd.AddCommand(subtaskCmd)

	subtaskCmd.Flags().StringVarP(&subtaskTitle, "title", "t", "", "Subtask title (required)")
	subtaskCmd.Flags().StringVarP(&subtaskDesc, "desc", "d", "", "Subtask description")
	subtaskCmd.Flags().StringVar(&subtaskType, "type", "task", "Subtask type (task, bug, epic, story)")
	subtaskCmd.Flags().StringVarP(&subtaskPriority, "priority", "p", "medium", "Subtask priority (low, medium, high, critical)")
	subtaskCmd.Flags().BoolVar(&subtaskEphemeral, "ephemeral", false, "Mark subtask as ephemeral (Wisp)")
	subtaskCmd.Flags().StringSliceVar(&subtaskAffectedFiles, "files", []string{}, "Affected files (comma separated)")

	subtaskCmd.MarkFlagRequired("title")
}
