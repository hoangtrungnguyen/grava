package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// assignCmd represents the assign command
var assignCmd = &cobra.Command{
	Use:   "assign <id> <user>",
	Short: "Assign an issue to a user or agent",
	Long: `Set the assignee field on an existing issue.

The assignee can be a human username or an agent identity string.
Passing an empty string ("") clears the assignee.

Example:
  grava assign grava-abc alice
  grava assign grava-abc "agent:planner-v2"
  grava assign grava-abc ""   # unassign`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		user := args[1]

		result, err := Store.Exec(
			`UPDATE issues SET assignee = ?, updated_at = ? WHERE id = ?`,
			user, time.Now(), id,
		)
		if err != nil {
			return fmt.Errorf("failed to assign issue %s: %w", id, err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}
		if rowsAffected == 0 {
			return fmt.Errorf("issue %s not found", id)
		}

		if user == "" {
			cmd.Printf("👤 Assignee cleared on %s\n", id)
		} else {
			cmd.Printf("👤 Assigned %s to %s\n", id, user)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(assignCmd)
}
