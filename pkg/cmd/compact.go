package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var compactDays int

// compactCmd represents the compact command
var compactCmd = &cobra.Command{
	Use:   "compact",
	Short: "Purge old ephemeral Wisp issues",
	Long: `Compact deletes ephemeral Wisp issues that are older than the specified
number of days and records each deletion in the deletions table to prevent
resurrection during future imports.

Example:
  grava compact --days 7   # delete Wisps older than 7 days (default)
  grava compact --days 0   # delete ALL Wisps regardless of age`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cutoff := time.Now().AddDate(0, 0, -compactDays)

		// 1. Find all ephemeral issues older than the cutoff
		rows, err := Store.Query(
			`SELECT id FROM issues WHERE ephemeral = 1 AND created_at < ?`,
			cutoff,
		)
		if err != nil {
			return fmt.Errorf("failed to query ephemeral issues: %w", err)
		}
		defer rows.Close()

		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("failed to scan row: %w", err)
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("row iteration error: %w", err)
		}

		if len(ids) == 0 {
			cmd.Printf("🧹 No Wisps older than %d day(s) found. Nothing to compact.\n", compactDays)
			return nil
		}

		// 2. For each Wisp: record in deletions, then delete from issues
		purged := 0
		for _, id := range ids {
			// Record tombstone
			_, err := Store.Exec(
				`INSERT INTO deletions (id, deleted_at, reason, actor) VALUES (?, ?, ?, ?)`,
				id, time.Now(), "compact", "grava-compact",
			)
			if err != nil {
				return fmt.Errorf("failed to record deletion for %s: %w", id, err)
			}

			// Delete the issue
			_, err = Store.Exec(`DELETE FROM issues WHERE id = ?`, id)
			if err != nil {
				return fmt.Errorf("failed to delete issue %s: %w", id, err)
			}

			purged++
		}

		cmd.Printf("🧹 Compacted %d Wisp(s) older than %d day(s). Tombstones recorded in deletions table.\n", purged, compactDays)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(compactCmd)
	compactCmd.Flags().IntVar(&compactDays, "days", 7, "Delete Wisps older than this many days")
}
