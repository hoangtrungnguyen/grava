package cmd

import (
	"fmt"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/idgen"
	"github.com/spf13/cobra"
)

var (
	title     string
	desc      string
	issueType string
	priority  string
	parentID  string
	ephemeral bool
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new issue",
	Long: `Create a new issue in the Grava tracker.
You can specify title, description, type, and priority.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Initialize Generator
		generator := idgen.NewStandardGenerator(Store)

		// 2. Generate ID
		var id string
		var err error
		if parentID != "" {
			id, err = generator.GenerateChildID(parentID)
		} else {
			id = generator.GenerateBaseID()
		}

		// Map priority
		var pInt int
		switch priority {
		case "critical":
			pInt = 0
		case "high":
			pInt = 1
		case "medium":
			pInt = 2
		case "low":
			pInt = 3
		default:
			pInt = 4 // backlog/default
		}

		// 3. Insert into DB
		// Note: status 'todo' is NOT allowed by schema check, use 'open'.
		// Note: column is 'issue_type', not 'type'.
		ephemeralVal := 0
		if ephemeral {
			ephemeralVal = 1
		}

		query := `INSERT INTO issues (id, title, description, issue_type, priority, status, ephemeral, created_at, updated_at, created_by, updated_by, agent_model)
                  VALUES (?, ?, ?, ?, ?, 'open', ?, ?, ?, ?, ?, ?)`

		_, err = Store.Exec(query, id, title, desc, issueType, pInt, ephemeralVal, time.Now(), time.Now(), actor, actor, agentModel)
		if err != nil {
			return fmt.Errorf("failed to insert issue: %w", err)
		}

		if ephemeral {
			cmd.Printf("👻 Created ephemeral issue (Wisp): %s\n", id)
		} else {
			cmd.Printf("✅ Created issue: %s\n", id)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringVarP(&title, "title", "t", "", "Issue title (required)")
	createCmd.Flags().StringVarP(&desc, "desc", "d", "", "Issue description")
	createCmd.Flags().StringVar(&issueType, "type", "task", "Issue type (task, bug, epic, story)")
	createCmd.Flags().StringVarP(&priority, "priority", "p", "medium", "Issue priority (low, medium, high, critical)")
	createCmd.Flags().StringVar(&parentID, "parent", "", "Parent Issue ID for sub-tasks")
	createCmd.Flags().BoolVar(&ephemeral, "ephemeral", false, "Mark issue as ephemeral (Wisp) — excluded from normal queries")

	createCmd.MarkFlagRequired("title")
}
