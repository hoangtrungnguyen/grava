// Package maintenance contains the maintenance commands (compact, doctor, clear).
package maintenance

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/validation"
	"github.com/spf13/cobra"
)

var (
	compactDays      int
	clearFrom        string
	clearTo          string
	clearForce       bool
	clearIncludeWisp bool
)

// ClearStdinReader is overridable in tests to simulate interactive input for the clear command.
var ClearStdinReader io.Reader = os.Stdin

// AddCommands registers all maintenance commands with the root cobra.Command.
func AddCommands(root *cobra.Command, d *cmddeps.Deps) {
	root.AddCommand(newCompactCmd(d))
	root.AddCommand(newDoctorCmd(d))
	root.AddCommand(newClearCmd(d))
	// history command moved to pkg/cmd/issues (Story 3.3: events-based audit trail)
}

func newCompactCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compact",
		Short: "Purge old ephemeral Wisp issues (soft delete)",
		Long: `Compact soft-deletes ephemeral Wisp issues that are older than the specified
	number of days. Issues are marked with 'tombstone' and recorded in the deletions table.

Example:
  grava compact --days 7   # delete Wisps older than 7 days (default)
  grava compact --days 0   # delete ALL Wisps regardless of age`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cutoff := time.Now().AddDate(0, 0, -compactDays)

			rows, err := (*d.Store).Query(
				`SELECT id FROM issues WHERE ephemeral = 1 AND created_at < ?`,
				cutoff,
			)
			if err != nil {
				return fmt.Errorf("failed to query ephemeral issues: %w", err)
			}
			defer rows.Close() //nolint:errcheck

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
				if *d.OutputJSON {
					resp := map[string]any{
						"status":  "unchanged",
						"count":   0,
						"message": fmt.Sprintf("No Wisps older than %d day(s) found", compactDays),
					}
					b, _ := json.MarshalIndent(resp, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
					return nil
				}
				cmd.Printf("🧹 No Wisps older than %d day(s) found. Nothing to compact.\n", compactDays)
				return nil
			}

			ctx := context.Background()
			tx, err := (*d.Store).BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("failed to start transaction: %w", err)
			}
			defer tx.Rollback() //nolint:errcheck

			for _, id := range ids {
				_, err := tx.ExecContext(ctx,
					`INSERT INTO deletions (id, deleted_at, reason, actor, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?, ?)`,
					id, time.Now(), "compact", "grava-compact", *d.Actor, *d.Actor, *d.AgentModel,
				)
				if err != nil {
					return fmt.Errorf("failed to record deletion for %s: %w", id, err)
				}

				_, err = tx.ExecContext(ctx, "UPDATE issues SET status = 'tombstone', updated_at = NOW(), updated_by = ?, agent_model = ? WHERE id = ?", *d.Actor, *d.AgentModel, id)
				if err != nil {
					return fmt.Errorf("failed to soft delete issue %s: %w", id, err)
				}
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit transaction: %w", err)
			}

			purged := len(ids)

			if *d.OutputJSON {
				resp := map[string]any{
					"status": "compacted",
					"count":  purged,
					"ids":    ids,
				}
				b, _ := json.MarshalIndent(resp, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
				return nil
			}

			cmd.Printf("🧹 Compacted %d Wisp(s) older than %d day(s). Tombstones recorded in deletions table.\n", purged, compactDays)
			return nil
		},
	}

	cmd.Flags().IntVar(&compactDays, "days", 7, "Delete Wisps older than this many days")
	return cmd
}

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

func (c doctorCheck) icon() string {
	switch c.Status {
	case "PASS":
		return "✅"
	case "WARN":
		return "⚠️ "
	default:
		return "❌"
	}
}

func newDoctorCmd(d *cmddeps.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose and report system health",
		Long: `Doctor runs a series of read-only checks against the Grava database
and reports the health of each component.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var checks []doctorCheck
			hasFailure := false

			var dbVersion string
			err := (*d.Store).QueryRow("SELECT VERSION()").Scan(&dbVersion)
			if err != nil {
				checks = append(checks, doctorCheck{"DB connectivity", "FAIL",
					fmt.Sprintf("cannot query database: %v", err)})
				hasFailure = true
			} else {
				checks = append(checks, doctorCheck{"DB connectivity", "PASS",
					fmt.Sprintf("connected (server %s)", dbVersion)})
			}

			for _, table := range []string{"issues", "dependencies", "deletions", "child_counters"} {
				var count int
				err := (*d.Store).QueryRow(
					"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
					table,
				).Scan(&count)
				if err != nil {
					checks = append(checks, doctorCheck{fmt.Sprintf("Table: %s", table), "FAIL", fmt.Sprintf("query error: %v", err)})
					hasFailure = true
				} else if count == 0 {
					checks = append(checks, doctorCheck{fmt.Sprintf("Table: %s", table), "FAIL", "table does not exist — run `grava init`"})
					hasFailure = true
				} else {
					checks = append(checks, doctorCheck{fmt.Sprintf("Table: %s", table), "PASS", "exists"})
				}
			}

			var orphanCount int
			err = (*d.Store).QueryRow(`
			SELECT COUNT(*) FROM dependencies d
			WHERE NOT EXISTS (SELECT 1 FROM issues i WHERE i.id = d.from_id)
			   OR NOT EXISTS (SELECT 1 FROM issues i WHERE i.id = d.to_id)
		`).Scan(&orphanCount)
			if err != nil {
				checks = append(checks, doctorCheck{"Orphaned dependencies", "WARN", fmt.Sprintf("could not check: %v", err)})
			} else if orphanCount > 0 {
				checks = append(checks, doctorCheck{"Orphaned dependencies", "WARN", fmt.Sprintf("%d edge(s) reference non-existent issues", orphanCount)})
			} else {
				checks = append(checks, doctorCheck{"Orphaned dependencies", "PASS", "none found"})
			}

			var untitledCount int
			err = (*d.Store).QueryRow(`SELECT COUNT(*) FROM issues WHERE title IS NULL OR title = ''`).Scan(&untitledCount)
			if err != nil {
				checks = append(checks, doctorCheck{"Untitled issues", "WARN", fmt.Sprintf("could not check: %v", err)})
			} else if untitledCount > 0 {
				checks = append(checks, doctorCheck{"Untitled issues", "WARN", fmt.Sprintf("%d issue(s) have no title", untitledCount)})
			} else {
				checks = append(checks, doctorCheck{"Untitled issues", "PASS", "none found"})
			}

			var wispCount int
			err = (*d.Store).QueryRow(`SELECT COUNT(*) FROM issues WHERE ephemeral = 1`).Scan(&wispCount)
			if err != nil {
				checks = append(checks, doctorCheck{"Wisp count", "WARN", fmt.Sprintf("could not check: %v", err)})
			} else {
				status := "PASS"
				detail := fmt.Sprintf("%d Wisp(s) in database", wispCount)
				if wispCount > 100 {
					status = "WARN"
					detail += " — consider running `grava compact`"
				}
				checks = append(checks, doctorCheck{"Wisp count", status, detail})
			}

			if *d.OutputJSON {
				resp := map[string]any{"status": "healthy", "checks": checks}
				if hasFailure {
					resp["status"] = "unhealthy"
				}
				b, _ := json.MarshalIndent(resp, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
				if hasFailure {
					return fmt.Errorf("doctor found critical issues")
				}
				return nil
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "🩺 Grava Doctor Report")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("─", 50))
			for _, c := range checks {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s  %-30s %s\n", c.icon(), c.Name, c.Detail)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("─", 50))

			if hasFailure {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "❌ Some checks FAILED. Please review the issues above.")
				return fmt.Errorf("doctor found critical issues")
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ All critical checks passed.")
			return nil
		},
	}
}


func newClearCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Purge archived issues, or soft-delete issues within a date range",
		Long: `Clear purges all archived issues permanently, or soft-deletes issues
within a specified date range.

Purge archived issues (no flags):
  grava clear            # permanently delete all archived issues

Date-range soft-delete:
  grava clear --from 2026-01-01 --to 2026-01-31
  grava clear --from 2026-02-18 --to 2026-02-18 --force --include-wisps`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no date flags provided, purge archived issues (Story 2.6)
			if clearFrom == "" && clearTo == "" {
				return clearArchivedIssues(cmd, d)
			}

			fromDate, toDate, err := validation.ValidateDateRange(clearFrom, clearTo)
			if err != nil {
				return err
			}

			query := "SELECT id FROM issues WHERE created_at >= ? AND created_at < ?"
			nextDay := toDate.AddDate(0, 0, 1)

			if !clearIncludeWisp {
				query += " AND ephemeral = FALSE"
			}

			rows, err := (*d.Store).Query(query, fromDate.Format("2006-01-02"), nextDay.Format("2006-01-02"))
			if err != nil {
				return fmt.Errorf("failed to query issues: %w", err)
			}
			defer rows.Close() //nolint:errcheck

			var ids []string
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					return fmt.Errorf("failed to scan issue ID: %w", err)
				}
				ids = append(ids, id)
			}

			if len(ids) == 0 {
				if *d.OutputJSON {
					resp := map[string]any{
						"status":  "unchanged",
						"count":   0,
						"message": fmt.Sprintf("No issues found between %s and %s", clearFrom, clearTo),
					}
					b, _ := json.MarshalIndent(resp, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
					return nil
				}
				cmd.Printf("No issues found between %s and %s.\n", clearFrom, clearTo)
				return nil
			}

			if !clearForce && !*d.OutputJSON {
				cmd.Printf("⚠️  Found %d issue(s) created between %s and %s.\nType \"yes\" to delete them: ", len(ids), clearFrom, clearTo)

				scanner := bufio.NewScanner(ClearStdinReader)
				scanner.Scan()
				answer := strings.TrimSpace(scanner.Text())

				if answer != "yes" {
					cmd.Println("Aborted. No data was deleted.")
					return nil
				}
			}

			ctx := context.Background()
			tx, err := (*d.Store).BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("failed to start transaction: %w", err)
			}
			defer tx.Rollback() //nolint:errcheck

			for _, id := range ids {
				_, err = tx.ExecContext(ctx,
					"INSERT INTO deletions (id, reason, actor, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)",
					id, "clear", "grava-clear", *d.Actor, *d.Actor, *d.AgentModel,
				)
				if err != nil {
					return fmt.Errorf("failed to record tombstone for %s: %w", id, err)
				}

				_, err = tx.ExecContext(ctx, "UPDATE issues SET status = 'tombstone', updated_at = NOW(), updated_by = ?, agent_model = ? WHERE id = ?", *d.Actor, *d.AgentModel, id)
				if err != nil {
					return fmt.Errorf("failed to soft delete issue %s: %w", id, err)
				}
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit transaction: %w", err)
			}

			if *d.OutputJSON {
				resp := map[string]any{
					"status": "cleared",
					"count":  len(ids),
					"ids":    ids,
					"from":   clearFrom,
					"to":     clearTo,
				}
				b, _ := json.MarshalIndent(resp, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
				return nil
			}

			cmd.Printf("🗑️  Cleared %d issue(s) from %s to %s. Tombstones recorded.\n", len(ids), clearFrom, clearTo)
			return nil
		},
	}

	cmd.Flags().StringVar(&clearFrom, "from", "", "Start date (inclusive), format YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&clearTo, "to", "", "End date (inclusive), format YYYY-MM-DD (required)")
	cmd.Flags().BoolVar(&clearForce, "force", false, "Skip interactive confirmation prompt")
	cmd.Flags().BoolVar(&clearIncludeWisp, "include-wisps", false, "Also delete ephemeral Wisp issues")
	return cmd
}
