// Package maintenance contains the maintenance commands (compact, doctor, clear).
package maintenance

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
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
	root.AddCommand(newCmdHistoryCmd(d))
	// issue-level history is in pkg/cmd/issues (Story 3.3: events-based audit trail)
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
	var (
		flagFix    bool
		flagDryRun bool
	)

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose and report system health",
		Long: `Doctor runs diagnostic checks against the Grava database and reports
the health of each component.

With --fix, doctor also repairs detected issues (e.g., releases expired file leases).
With --dry-run, doctor shows what --fix would change without executing any writes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			store := *d.Store
			outputJSON := *d.OutputJSON

			var checks []doctorCheck
			hasFailure := false

			var dbVersion string
			err := store.QueryRow("SELECT VERSION()").Scan(&dbVersion)
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
				err := store.QueryRow(
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
			err = store.QueryRow(`
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
			err = store.QueryRow(`SELECT COUNT(*) FROM issues WHERE title IS NULL OR title = ''`).Scan(&untitledCount)
			if err != nil {
				checks = append(checks, doctorCheck{"Untitled issues", "WARN", fmt.Sprintf("could not check: %v", err)})
			} else if untitledCount > 0 {
				checks = append(checks, doctorCheck{"Untitled issues", "WARN", fmt.Sprintf("%d issue(s) have no title", untitledCount)})
			} else {
				checks = append(checks, doctorCheck{"Untitled issues", "PASS", "none found"})
			}

			var wispCount int
			err = store.QueryRow(`SELECT COUNT(*) FROM issues WHERE ephemeral = 1`).Scan(&wispCount)
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

			var fixResults []map[string]any // non-nil entries when --fix ran

			// Check: ghost worktrees — issues marked in_progress whose .worktree/<id>
			// directory has been removed (typical after a reaped container).
			cwd, cwdErr := os.Getwd()
			if cwdErr != nil {
				checks = append(checks, doctorCheck{"Ghost worktrees", "WARN",
					fmt.Sprintf("could not resolve working directory: %v", cwdErr)})
			} else {
				ghostIDs, ghostErr := queryGhostWorktrees(ctx, store, cwd)
				switch {
				case ghostErr != nil:
					checks = append(checks, doctorCheck{"Ghost worktrees", "WARN",
						fmt.Sprintf("could not check: %v", ghostErr)})
				case len(ghostIDs) == 0:
					checks = append(checks, doctorCheck{"Ghost worktrees", "PASS", "none found"})
				case flagDryRun:
					detail := fmt.Sprintf("%d ghost claim(s) would be released: %s",
						len(ghostIDs), strings.Join(ghostIDs, ", "))
					checks = append(checks, doctorCheck{"Ghost worktrees", "WARN", detail})
				case flagFix:
					healed, healErr := healGhostWorktrees(ctx, store, *d.Actor, *d.AgentModel, ghostIDs)
					if healErr != nil {
						checks = append(checks, doctorCheck{"Ghost worktrees", "FAIL",
							fmt.Sprintf("auto-heal failed: %v", healErr)})
						hasFailure = true
						fixResults = append(fixResults, map[string]any{
							"check":        "ghost_worktrees",
							"status":       "error",
							"action_taken": fmt.Sprintf("error during heal: %v", healErr),
						})
					} else {
						checks = append(checks, doctorCheck{"Ghost worktrees", "PASS",
							fmt.Sprintf("released %d ghost claim(s) (auto-fix)", healed)})
						fixResults = append(fixResults, map[string]any{
							"check":        "ghost_worktrees",
							"status":       "fixed",
							"action_taken": fmt.Sprintf("released %d ghost claim(s)", healed),
							"ids":          ghostIDs,
						})
					}
				default:
					detail := fmt.Sprintf("%d ghost claim(s) detected — run `grava doctor --fix` to release",
						len(ghostIDs))
					checks = append(checks, doctorCheck{"Ghost worktrees", "WARN", detail})
				}
			}

			// Check #12: expired file reservations (FR-ECS-1c).
			expiredLeases, expiredCheckErr := queryExpiredLeases(ctx, store)
			if expiredCheckErr != nil {
				checks = append(checks, doctorCheck{"Expired file leases", "WARN",
					fmt.Sprintf("could not check: %v", expiredCheckErr)})
			} else if len(expiredLeases) > 0 {
				switch {
				case flagDryRun:
					// List exact IDs so the user knows what would be released.
					detail := fmt.Sprintf("%d expired lease(s) would be released: %s",
						len(expiredLeases), strings.Join(expiredLeases, ", "))
					checks = append(checks, doctorCheck{"Expired file leases", "WARN", detail})
				case flagFix:
					// Release expired leases and fold result into the check.
					released, fixErr := releaseExpiredLeases(ctx, store, expiredLeases)
					if fixErr != nil {
						checks = append(checks, doctorCheck{"Expired file leases", "FAIL",
							fmt.Sprintf("auto-release failed: %v", fixErr)})
						hasFailure = true
						fixResults = append(fixResults, map[string]any{
							"check":        "expired_file_reservations",
							"status":       "error",
							"action_taken": fmt.Sprintf("error during release: %v", fixErr),
						})
					} else {
						checks = append(checks, doctorCheck{"Expired file leases", "PASS",
							fmt.Sprintf("released %d expired lease(s) (auto-fix)", released)})
						fixResults = append(fixResults, map[string]any{
							"check":        "expired_file_reservations",
							"status":       "fixed",
							"action_taken": fmt.Sprintf("released %d expired lease(s)", released),
						})
					}
				default:
					detail := fmt.Sprintf("%d expired lease(s) not yet released — run `grava doctor --fix` to auto-release",
						len(expiredLeases))
					checks = append(checks, doctorCheck{"Expired file leases", "WARN", detail})
				}
			} else {
				checks = append(checks, doctorCheck{"Expired file leases", "PASS", "none found"})
			}

			if outputJSON {
				resp := map[string]any{"status": "healthy", "checks": checks}
				if hasFailure {
					resp["status"] = "unhealthy"
				}
				if len(fixResults) > 0 {
					resp["fix_results"] = fixResults
					// Preserve legacy single-result key for expired-lease callers.
					for _, fr := range fixResults {
						if fr["check"] == "expired_file_reservations" {
							resp["fix_result"] = fr
							break
						}
					}
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

	cmd.Flags().BoolVar(&flagFix, "fix", false, "Auto-repair detected issues (e.g., release expired file leases)")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Show what --fix would change without executing any writes")
	return cmd
}

// queryExpiredLeases returns IDs of file_reservations rows where expires_ts < NOW() and released_ts IS NULL.
func queryExpiredLeases(ctx context.Context, store dolt.Store) ([]string, error) {
	rows, err := store.QueryContext(ctx,
		`SELECT id FROM file_reservations WHERE expires_ts < NOW() AND released_ts IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("query expired leases: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan expired lease id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// releaseExpiredLeases sets released_ts=NOW() for each given reservation ID.
// Returns the number of rows updated.
func releaseExpiredLeases(ctx context.Context, store dolt.Store, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	released := 0
	for _, id := range ids {
		result, err := store.ExecContext(ctx,
			`UPDATE file_reservations SET released_ts = NOW() WHERE id = ? AND released_ts IS NULL`, id)
		if err != nil {
			return released, fmt.Errorf("release lease %s: %w", id, err)
		}
		n, _ := result.RowsAffected() // RowsAffected never errors for MySQL/Dolt drivers
		released += int(n)
	}
	return released, nil
}

// queryGhostWorktrees returns IDs of issues whose status is 'in_progress' but
// whose on-disk worktree directory (<cwd>/.worktree/<id>) no longer exists.
// A ghost is a claim that outlived its worktree — typically after a container
// was reaped mid-flight. The DB still points at a phantom workspace.
func queryGhostWorktrees(ctx context.Context, store dolt.Store, cwd string) ([]string, error) {
	rows, err := store.QueryContext(ctx,
		`SELECT id FROM issues WHERE status = 'in_progress'`)
	if err != nil {
		return nil, fmt.Errorf("query in_progress issues: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var ghosts []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan in_progress issue id: %w", err)
		}
		wt := filepath.Join(cwd, ".worktree", id)
		if _, statErr := os.Stat(wt); os.IsNotExist(statErr) {
			ghosts = append(ghosts, id)
		}
	}
	return ghosts, rows.Err()
}

// healGhostWorktrees resets each ghost issue's status back to 'open' and
// clears its assignee, emitting a "release" audit event per issue inside a
// single transaction. Returns the number of rows actually reset.
//
// The Git branch grava/<id> is preserved — it may still hold partial work
// that a future claim can revisit. Only the DB ownership record is cleared.
func healGhostWorktrees(ctx context.Context, store dolt.Store, actor, model string, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	events := make([]dolt.AuditEvent, 0, len(ids))
	for _, id := range ids {
		events = append(events, dolt.AuditEvent{
			IssueID:   id,
			EventType: dolt.EventRelease,
			Actor:     actor,
			Model:     model,
			OldValue:  map[string]any{"status": "in_progress"},
			NewValue:  map[string]any{"status": "open", "reason": "ghost_worktree"},
		})
	}

	healed := 0
	err := dolt.WithAuditedTx(ctx, store, events, func(tx *sql.Tx) error {
		for _, id := range ids {
			result, execErr := tx.ExecContext(ctx,
				`UPDATE issues SET status='open', assignee=NULL, agent_model=NULL, updated_at=NOW(), updated_by=? WHERE id=? AND status='in_progress'`,
				actor, id)
			if execErr != nil {
				return fmt.Errorf("heal ghost %s: %w", id, execErr)
			}
			n, _ := result.RowsAffected()
			healed += int(n)
		}
		return nil
	})
	if err != nil {
		return healed, err
	}
	return healed, nil
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
