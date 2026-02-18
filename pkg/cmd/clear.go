package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	clearFrom        string
	clearTo          string
	clearForce       bool
	clearIncludeWisp bool

	// clearStdinReader is the reader used for interactive confirmation prompts.
	// Override in tests to inject a fake reader.
	clearStdinReader io.Reader = os.Stdin
)

// clearCmd represents the clear command
var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete issues within a date range",
	Long: `Delete issues (and related data) created within a specified date range.
Tombstones are recorded in the deletions table for all purged IDs.

Example:
  grava clear --from 2026-01-01 --to 2026-01-31
  grava clear --from 2026-02-18 --to 2026-02-18 --force --include-wisps`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if clearFrom == "" || clearTo == "" {
			return fmt.Errorf("--from and --to flags are required")
		}

		fromDate, err := time.Parse("2006-01-02", clearFrom)
		if err != nil {
			return fmt.Errorf("invalid --from date format (use YYYY-MM-DD): %w", err)
		}

		toDate, err := time.Parse("2006-01-02", clearTo)
		if err != nil {
			return fmt.Errorf("invalid --to date format (use YYYY-MM-DD): %w", err)
		}

		if fromDate.After(toDate) {
			return fmt.Errorf("--from date must be before or equal to --to date")
		}

		// Prepare query for matching issues
		// Note: Dolt/MySQL BETWEEN is inclusive.
		// We want to include the entire 'to' day, so we should either use < toDate + 24h
		// or use the date format in the query.
		query := "SELECT id FROM issues WHERE created_at >= ? AND created_at < ?"
		// End of 'to' day is start of next day
		nextDay := toDate.AddDate(0, 0, 1)

		if !clearIncludeWisp {
			query += " AND ephemeral = FALSE"
		}

		rows, err := Store.Query(query, fromDate.Format("2006-01-02"), nextDay.Format("2006-01-02"))
		if err != nil {
			return fmt.Errorf("failed to query issues: %w", err)
		}
		defer rows.Close()

		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("failed to scan issue ID: %w", err)
			}
			ids = append(ids, id)
		}

		if len(ids) == 0 {
			cmd.Printf("No issues found between %s and %s.\n", clearFrom, clearTo)
			return nil
		}

		if !clearForce {
			cmd.Printf("⚠️  Found %d issue(s) created between %s and %s.\nType \"yes\" to delete them: ", len(ids), clearFrom, clearTo)

			scanner := bufio.NewScanner(clearStdinReader)
			scanner.Scan()
			answer := strings.TrimSpace(scanner.Text())

			if answer != "yes" {
				cmd.Println("Aborted. No data was deleted.")
				return nil
			}
		}

		// Perform deletions in a transaction
		ctx := context.Background()
		tx, err := Store.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}
		defer tx.Rollback()

		for _, id := range ids {
			// 1. Record tombstone
			_, err = tx.ExecContext(ctx,
				"INSERT INTO deletions (id, reason, actor, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)",
				id, "clear", "grava-clear", actor, actor, agentModel,
			)
			if err != nil {
				return fmt.Errorf("failed to record tombstone for %s: %w", id, err)
			}

			// 2. Delete issue (cascading FKs handle dependencies and events)
			_, err = tx.ExecContext(ctx, "DELETE FROM issues WHERE id = ?", id)
			if err != nil {
				return fmt.Errorf("failed to delete issue %s: %w", id, err)
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		cmd.Printf("🗑️  Cleared %d issue(s) from %s to %s. Tombstones recorded.\n", len(ids), clearFrom, clearTo)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(clearCmd)

	clearCmd.Flags().StringVar(&clearFrom, "from", "", "Start date (inclusive), format YYYY-MM-DD (required)")
	clearCmd.Flags().StringVar(&clearTo, "to", "", "End date (inclusive), format YYYY-MM-DD (required)")
	clearCmd.Flags().BoolVar(&clearForce, "force", false, "Skip interactive confirmation prompt")
	clearCmd.Flags().BoolVar(&clearIncludeWisp, "include-wisps", false, "Also delete ephemeral Wisp issues")
}
