package cmd

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	quickPriority int
	quickLimit    int
)

// quickCmd represents the quick command
var quickCmd = &cobra.Command{
	Use:   "quick",
	Short: "List high-priority or quick tasks",
	Long: `Quick shows open issues at or above the given priority threshold.

Priority levels:
  0 = critical
  1 = high      (default threshold)
  2 = medium
  3 = low
  4 = backlog

Examples:
  grava quick                  # show critical + high priority open issues
  grava quick --priority 2     # include medium priority as well
  grava quick --limit 5        # cap results at 5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sql := `SELECT id, title, issue_type, priority, status, created_at
		        FROM issues
		        WHERE ephemeral = 0
		          AND status = 'open'
		          AND priority <= ?
		        ORDER BY priority ASC, created_at DESC
		        LIMIT ?`

		rows, err := Store.Query(sql, quickPriority, quickLimit)
		if err != nil {
			return fmt.Errorf("failed to query quick issues: %w", err)
		}
		defer rows.Close()

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTitle\tType\tPriority\tStatus\tCreated")

		found := 0
		for rows.Next() {
			var id, title, iType, status string
			var prio int
			var createdAt time.Time
			if err := rows.Scan(&id, &title, &iType, &prio, &status, &createdAt); err != nil {
				return fmt.Errorf("failed to scan row: %w", err)
			}
			if len(title) > 50 {
				title = title[:47] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
				id, title, iType, prio, status, createdAt.Format("2006-01-02"))
			found++
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("row iteration error: %w", err)
		}
		w.Flush()

		if found == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "🎉 No high-priority open issues. You're all caught up!")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "\n⚡ %d high-priority issue(s) need attention.\n", found)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(quickCmd)
	quickCmd.Flags().IntVar(&quickPriority, "priority", 1, "Show issues at or above this priority level (0=critical, 1=high, 2=medium, 3=low)")
	quickCmd.Flags().IntVar(&quickLimit, "limit", 20, "Maximum number of results to return")
}
