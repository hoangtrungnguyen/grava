package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show details of an issue",
	Long:  `Display detailed information about a specific issue.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		query := `SELECT title, description, issue_type, priority, status, created_at, updated_at, created_by, updated_by, agent_model 
                  FROM issues WHERE id = ?`

		var title, desc, iType, status string
		var priority int
		var createdAt, updatedAt time.Time
		var createdBy, updatedBy string
		var agentModelStr *string

		err := Store.QueryRow(query, id).Scan(&title, &desc, &iType, &priority, &status, &createdAt, &updatedAt, &createdBy, &updatedBy, &agentModelStr)
		if err != nil {
			return fmt.Errorf("failed to fetch issue %s: %w", id, err)
		}

		cmd.Printf("ID:          %s\n", id)
		cmd.Printf("Title:       %s\n", title)
		cmd.Printf("Type:        %s\n", iType)

		// Map priority back via array or switch
		pStr := "backlog"
		switch priority {
		case 0:
			pStr = "critical"
		case 1:
			pStr = "high"
		case 2:
			pStr = "medium"
		case 3:
			pStr = "low"
		}
		cmd.Printf("Priority:    %s (%d)\n", pStr, priority)
		cmd.Printf("Status:      %s\n", status)
		cmd.Printf("Created:     %s by %s\n", createdAt.Format(time.RFC3339), createdBy)
		cmd.Printf("Updated:     %s by %s\n", updatedAt.Format(time.RFC3339), updatedBy)
		if agentModelStr != nil && *agentModelStr != "" {
			cmd.Printf("Model:       %s\n", *agentModelStr)
		}
		cmd.Printf("\nDescription:\n%s\n", desc)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
