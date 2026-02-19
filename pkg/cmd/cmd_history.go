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

		// Query dolt_history_issues table joined with dolt_log for messages
		query := `
			SELECT h.commit_hash, h.committer, h.commit_date, h.title, h.status, l.message
			FROM dolt_history_issues h
			JOIN dolt_log l ON h.commit_hash = l.commit_hash
			WHERE h.id = ?
			ORDER BY h.commit_date DESC
		`

		rows, err := Store.Query(query, id)
		if err != nil {
			return fmt.Errorf("failed to fetch history for issue %s: %w", id, err)
		}
		defer rows.Close()

		fmt.Fprintf(cmd.OutOrStdout(), "History for Issue %s:\n\n", id)
		fmt.Fprintf(cmd.OutOrStdout(), "%-10s %-20s %-25s %-15s %-20s %s\n", "COMMIT", "AUTHOR", "DATE", "STATUS", "TITLE", "MESSAGE")
		fmt.Fprintln(cmd.OutOrStdout(), "------------------------------------------------------------------------------------------------------------------------")

		count := 0
		for rows.Next() {
			count++
			var hash, committer, title, status, message string
			var date time.Time
			if err := rows.Scan(&hash, &committer, &date, &title, &status, &message); err != nil {
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

			// Truncate title and message if too long
			if len(title) > 20 {
				title = title[:17] + "..."
			}
			if len(message) > 40 {
				message = message[:37] + "..."
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-10s %-20s %-25s %-15s %-20s %s\n", shortHash, committer, date.Format(time.RFC3339), status, title, message)
		}

		if count == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No history found for issue %s (check if it exists or is committed)\n", id)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(historyCmd)
}
