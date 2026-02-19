package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hoangtrungnguyen/grava/pkg/validation"
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
	Short: "Soft-delete issues within a date range",
	Long: `Soft-delete issues (and related data) created within a specified date range.
	Issues are marked with 'tombstone' status and recorded in the deletions table.

Example:
  grava clear --from 2026-01-01 --to 2026-01-31
  grava clear --from 2026-02-18 --to 2026-02-18 --force --include-wisps`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fromDate, toDate, err := validation.ValidateDateRange(clearFrom, clearTo)
		if err != nil {
			return err
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
			if outputJSON {
				resp := map[string]any{
					"status":  "unchanged",
					"count":   0,
					"message": fmt.Sprintf("No issues found between %s and %s", clearFrom, clearTo),
				}
				b, _ := json.MarshalIndent(resp, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			cmd.Printf("No issues found between %s and %s.\n", clearFrom, clearTo)
			return nil
		}

		if !clearForce && !outputJSON {
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

			// 2. Soft delete issue
			_, err = tx.ExecContext(ctx, "UPDATE issues SET status = 'tombstone', updated_at = NOW(), updated_by = ?, agent_model = ? WHERE id = ?", actor, agentModel, id)
			if err != nil {
				return fmt.Errorf("failed to soft delete issue %s: %w", id, err)
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		if outputJSON {
			resp := map[string]any{
				"status": "cleared",
				"count":  len(ids),
				"ids":    ids,
				"from":   clearFrom,
				"to":     clearTo,
			}
			b, _ := json.MarshalIndent(resp, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
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
