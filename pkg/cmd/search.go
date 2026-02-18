package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	searchWisp bool
)

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for issues matching a text query",
	Long: `Search scans issue titles, descriptions, and metadata for the given text.

By default ephemeral Wisp issues are excluded from results.
Use --wisp to include them.

Examples:
  grava search "login bug"
  grava search "auth" --wisp`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		if strings.TrimSpace(query) == "" {
			return fmt.Errorf("search query must not be empty")
		}

		pattern := "%" + query + "%"

		sql := `SELECT id, title, issue_type, priority, status, created_at
		        FROM issues
		        WHERE ephemeral = ?
		          AND (title LIKE ? OR description LIKE ? OR COALESCE(metadata,'') LIKE ?)
		        ORDER BY priority ASC, created_at DESC`

		ephemeralVal := 0
		if searchWisp {
			ephemeralVal = 1
		}

		rows, err := Store.Query(sql, ephemeralVal, pattern, pattern, pattern)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}
		defer rows.Close()

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTitle\tType\tPriority\tStatus\tCreated")

		found := 0
		for rows.Next() {
			var id, title, iType, status string
			var priority int
			var createdAt time.Time
			if err := rows.Scan(&id, &title, &iType, &priority, &status, &createdAt); err != nil {
				return fmt.Errorf("failed to scan row: %w", err)
			}
			if len(title) > 50 {
				title = title[:47] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
				id, title, iType, priority, status, createdAt.Format("2006-01-02"))
			found++
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("row iteration error: %w", err)
		}
		w.Flush()

		if found == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No issues found matching %q\n", query)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "\n🔍 %d result(s) for %q\n", found, query)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().BoolVar(&searchWisp, "wisp", false, "Include ephemeral Wisp issues in results")
}
