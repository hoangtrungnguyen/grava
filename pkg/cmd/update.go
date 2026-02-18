package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var (
	updateTitle    string
	updateDesc     string
	updateType     string
	updatePriority string
	updateStatus   string
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an existing issue",
	Long: `Update specific fields of an existing issue.
Only the flags provided will be updated.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		// Build dynamic query
		query := "UPDATE issues SET updated_at = ?"
		queryParams := []any{time.Now()}

		if cmd.Flags().Changed("title") {
			query += ", title = ?"
			queryParams = append(queryParams, updateTitle)
		}
		if cmd.Flags().Changed("desc") {
			query += ", description = ?"
			queryParams = append(queryParams, updateDesc)
		}
		if cmd.Flags().Changed("type") {
			query += ", issue_type = ?"
			queryParams = append(queryParams, updateType)
		}
		if cmd.Flags().Changed("priority") {
			query += ", priority = ?"
			var pInt int
			switch updatePriority {
			case "critical":
				pInt = 0
			case "high":
				pInt = 1
			case "medium":
				pInt = 2
			case "low":
				pInt = 3
			default:
				pInt = 4 // backlog
			}
			queryParams = append(queryParams, pInt)
		}
		if cmd.Flags().Changed("status") {
			query += ", status = ?"
			queryParams = append(queryParams, updateStatus)
		}

		query += " WHERE id = ?"
		queryParams = append(queryParams, id)

		result, err := Store.Exec(query, queryParams...)
		if err != nil {
			return fmt.Errorf("failed to update issue %s: %w", id, err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}

		if rowsAffected == 0 {
			return fmt.Errorf("issue %s not found or no changes made", id)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "✅ Updated issue %s\n", id)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().StringVarP(&updateTitle, "title", "t", "", "Update title")
	updateCmd.Flags().StringVarP(&updateDesc, "desc", "d", "", "Update description")
	updateCmd.Flags().StringVar(&updateType, "type", "", "Update type")
	updateCmd.Flags().StringVarP(&updatePriority, "priority", "p", "", "Update priority")
	updateCmd.Flags().StringVarP(&updateStatus, "status", "s", "", "Update status")
}
