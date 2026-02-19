package cmd

import (
	"encoding/json"
	"fmt"
	"time"

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
		} else {
			id = generator.GenerateBaseID()
		}

		// 3. Insert into DB
		ephemeralVal := 0
		if ephemeral {
			ephemeralVal = 1
		}

		affectedFilesJSON := "[]"
		if len(affectedFiles) > 0 {
			b, _ := json.Marshal(affectedFiles)
			affectedFilesJSON = string(b)
		}

		query := `INSERT INTO issues (id, title, description, issue_type, priority, status, ephemeral, created_at, updated_at, created_by, updated_by, agent_model, affected_files)
                  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

		_, err = Store.Exec(query, id, title, desc, issueType, pInt, "open", ephemeralVal, time.Now(), time.Now(), actor, actor, agentModel, affectedFilesJSON)
		if err != nil {
			return fmt.Errorf("failed to insert issue: %w", err)
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
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
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

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringP("title", "t", "", "Issue title (required)")
	createCmd.Flags().StringP("desc", "d", "", "Issue description")
	createCmd.Flags().String("type", "task", "Issue type (task, bug, epic, story)")
	createCmd.Flags().StringP("priority", "p", "medium", "Issue priority (low, medium, high, critical)")
	createCmd.Flags().String("parent", "", "Parent Issue ID for sub-tasks")
	createCmd.Flags().Bool("ephemeral", false, "Mark issue as ephemeral (Wisp) — excluded from normal queries")
	createCmd.Flags().StringSliceVar(&createAffectedFiles, "files", []string{}, "Affected files (comma separated)")

	createCmd.MarkFlagRequired("title")
}
