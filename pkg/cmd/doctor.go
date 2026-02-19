package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// doctorCheck holds the result of a single diagnostic check.
type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "PASS", "WARN", "FAIL"
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

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose and report system health",
	Long: `Doctor runs a series of read-only checks against the Grava database
and reports the health of each component.

Checks performed:
  1. Database connectivity
  2. Required tables present (issues, dependencies, deletions, child_counters)
  3. Data integrity — orphaned dependency edges
  4. Data integrity — issues missing a title
  5. Wisp count (informational)

Example:
  grava doctor`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var checks []doctorCheck
		hasFailure := false

		// ── Check 1: DB connectivity ─────────────────────────────────────────
		var dbVersion string
		err := Store.QueryRow("SELECT VERSION()").Scan(&dbVersion)
		if err != nil {
			checks = append(checks, doctorCheck{"DB connectivity", "FAIL",
				fmt.Sprintf("cannot query database: %v", err)})
			hasFailure = true
		} else {
			checks = append(checks, doctorCheck{"DB connectivity", "PASS",
				fmt.Sprintf("connected (server %s)", dbVersion)})
		}

		// ── Check 2: Required tables ─────────────────────────────────────────
		requiredTables := []string{"issues", "dependencies", "deletions", "child_counters"}
		for _, table := range requiredTables {
			var count int
			err := Store.QueryRow(
				"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
				table,
			).Scan(&count)
			if err != nil {
				checks = append(checks, doctorCheck{
					fmt.Sprintf("Table: %s", table), "FAIL",
					fmt.Sprintf("query error: %v", err),
				})
				hasFailure = true
			} else if count == 0 {
				checks = append(checks, doctorCheck{
					fmt.Sprintf("Table: %s", table), "FAIL",
					"table does not exist — run `grava init`",
				})
				hasFailure = true
			} else {
				checks = append(checks, doctorCheck{
					fmt.Sprintf("Table: %s", table), "PASS", "exists",
				})
			}
		}

		// ── Check 3: Orphaned dependency edges ───────────────────────────────
		var orphanCount int
		err = Store.QueryRow(`
			SELECT COUNT(*) FROM dependencies d
			WHERE NOT EXISTS (SELECT 1 FROM issues i WHERE i.id = d.from_id)
			   OR NOT EXISTS (SELECT 1 FROM issues i WHERE i.id = d.to_id)
		`).Scan(&orphanCount)
		if err != nil {
			checks = append(checks, doctorCheck{"Orphaned dependencies", "WARN",
				fmt.Sprintf("could not check: %v", err)})
		} else if orphanCount > 0 {
			checks = append(checks, doctorCheck{"Orphaned dependencies", "WARN",
				fmt.Sprintf("%d edge(s) reference non-existent issues", orphanCount)})
		} else {
			checks = append(checks, doctorCheck{"Orphaned dependencies", "PASS", "none found"})
		}

		// ── Check 4: Issues missing a title ──────────────────────────────────
		var untitledCount int
		err = Store.QueryRow(
			`SELECT COUNT(*) FROM issues WHERE title IS NULL OR title = ''`,
		).Scan(&untitledCount)
		if err != nil {
			checks = append(checks, doctorCheck{"Untitled issues", "WARN",
				fmt.Sprintf("could not check: %v", err)})
		} else if untitledCount > 0 {
			checks = append(checks, doctorCheck{"Untitled issues", "WARN",
				fmt.Sprintf("%d issue(s) have no title", untitledCount)})
		} else {
			checks = append(checks, doctorCheck{"Untitled issues", "PASS", "none found"})
		}

		// ── Check 5: Wisp count (informational) ──────────────────────────────
		var wispCount int
		err = Store.QueryRow(`SELECT COUNT(*) FROM issues WHERE ephemeral = 1`).Scan(&wispCount)
		if err != nil {
			checks = append(checks, doctorCheck{"Wisp count", "WARN",
				fmt.Sprintf("could not check: %v", err)})
		} else {
			status := "PASS"
			detail := fmt.Sprintf("%d Wisp(s) in database", wispCount)
			if wispCount > 100 {
				status = "WARN"
				detail += " — consider running `grava compact`"
			}
			checks = append(checks, doctorCheck{"Wisp count", status, detail})
		}

		// ── Handle Output ───────────────────────────────────────────────────
		if outputJSON {
			resp := map[string]any{
				"status": "healthy",
				"checks": checks,
			}
			if hasFailure {
				resp["status"] = "unhealthy"
			}
			b, _ := json.MarshalIndent(resp, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			if hasFailure {
				return fmt.Errorf("doctor found critical issues")
			}
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), "🩺 Grava Doctor Report")
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("─", 50))
		for _, c := range checks {
			fmt.Fprintf(cmd.OutOrStdout(), "%s  %-30s %s\n", c.icon(), c.Name, c.Detail)
		}
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("─", 50))

		if hasFailure {
			fmt.Fprintln(cmd.OutOrStdout(), "❌ Some checks FAILED. Please review the issues above.")
			return fmt.Errorf("doctor found critical issues")
		}

		fmt.Fprintln(cmd.OutOrStdout(), "✅ All critical checks passed.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
