package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history <id>",
	Short: "Show modification history of an issue",
	Long:  `Display the modification history of a specific issue, including commit hashes, authors, and dates.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		// Query dolt_history_issues table
		// We select key metadata columns
		query := `
			SELECT commit_hash, committer, commit_date, title, status
			FROM dolt_history_issues
			WHERE id = ?
			ORDER BY commit_date DESC
		`

		rows, err := Store.Query(query, id)
		if err != nil {
			return fmt.Errorf("failed to fetch history for issue %s: %w", id, err)
		}
		defer rows.Close()

		fmt.Printf("History for Issue %s:\n\n", id)
		fmt.Printf("%-10s %-20s %-25s %-15s %s\n", "COMMIT", "AUTHOR", "DATE", "STATUS", "TITLE")
		fmt.Println("------------------------------------------------------------------------------------------------")

		count := 0
		for rows.Next() {
			count++
			var hash, committer, title, status string
			var date time.Time
			if err := rows.Scan(&hash, &committer, &date, &title, &status); err != nil {
				return fmt.Errorf("failed to scan history row: %w", err)
			}
			var shortHash string
			if len(hash) >= 8 {
				shortHash = hash[:8]
			} else {
				shortHash = hash
			}

			if status == "tombstone" {
				status = "🗑️ DELETED"
			}

			fmt.Printf("%-10s %-20s %-25s %-15s %s\n", shortHash, committer, date.Format(time.RFC3339), status, title)
		}

		if count == 0 {
			fmt.Printf("No history found for issue %s (check if it exists or is committed)\n", id)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(historyCmd)
}
