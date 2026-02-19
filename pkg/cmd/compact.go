package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var compactDays int

// compactCmd represents the compact command
var compactCmd = &cobra.Command{
	Use:   "compact",
	Short: "Purge old ephemeral Wisp issues (soft delete)",
	Long: `Compact soft-deletes ephemeral Wisp issues that are older than the specified
	number of days. Issues are marked with 'tombstone' and recorded in the deletions table.

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
			if outputJSON {
				resp := map[string]any{
					"status":  "unchanged",
					"count":   0,
					"message": fmt.Sprintf("No Wisps older than %d day(s) found", compactDays),
				}
				b, _ := json.MarshalIndent(resp, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			cmd.Printf("🧹 No Wisps older than %d day(s) found. Nothing to compact.\n", compactDays)
			return nil
		}

		// 2. Perform soft deletions in a transaction
		ctx := context.Background()
		tx, err := Store.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}
		defer tx.Rollback()

		for _, id := range ids {
			// 1. Record tombstone
			_, err := tx.ExecContext(ctx,
				`INSERT INTO deletions (id, deleted_at, reason, actor, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				id, time.Now(), "compact", "grava-compact", actor, actor, agentModel,
			)
			if err != nil {
				return fmt.Errorf("failed to record deletion for %s: %w", id, err)
			}

			// 2. Soft delete the issue
			_, err = tx.ExecContext(ctx, "UPDATE issues SET status = 'tombstone', updated_at = NOW(), updated_by = ?, agent_model = ? WHERE id = ?", actor, agentModel, id)
			if err != nil {
				return fmt.Errorf("failed to soft delete issue %s: %w", id, err)
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		purged := len(ids)

		if outputJSON {
			resp := map[string]any{
				"status": "compacted",
				"count":  purged,
				"ids":    ids,
			}
			b, _ := json.MarshalIndent(resp, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		}

		cmd.Printf("🧹 Compacted %d Wisp(s) older than %d day(s). Tombstones recorded in deletions table.\n", purged, compactDays)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(compactCmd)
	compactCmd.Flags().IntVar(&compactDays, "days", 7, "Delete Wisps older than this many days")
}
