package cmd

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	listStatus string
	listType   string
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all issues",
	Long: `List all issues in the Grava tracker.
You can filter by status or type.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		query := "SELECT id, title, issue_type, priority, status, created_at FROM issues"
		var params []any

		whereClause := ""
		if listStatus != "" {
			whereClause += " status = ?"
			params = append(params, listStatus)
		}
		if listType != "" {
			if whereClause != "" {
				whereClause += " AND"
			} else {
				// Bug fix: if listStatus is empty, we don't need AND, but we need empty check
				// simpler: construct where clauses list and join with AND
				// But let's stick to simple logic for now
			}
			// wait, above logic is slightly flawed if I just append.
			// Let's rewrite query construction
		}

		// Reset logic
		whereParts := []string{}
		params = []any{}

		if listStatus != "" {
			whereParts = append(whereParts, "status = ?")
			params = append(params, listStatus)
		}
		if listType != "" {
			whereParts = append(whereParts, "issue_type = ?")
			params = append(params, listType)
		}

		if len(whereParts) > 0 {
			query += " WHERE "
			for i, part := range whereParts {
				if i > 0 {
					query += " AND "
				}
				query += part
			}
		}

		query += " ORDER BY priority ASC, created_at DESC"

		rows, err := Store.Query(query, params...)
		if err != nil {
			return fmt.Errorf("failed to query issues: %w", err)
		}
		defer rows.Close()

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTitle\tType\tPriority\tStatus\tCreated")

		for rows.Next() {
			var id, title, iType, status string
			var priority int
			var createdAt time.Time
			if err := rows.Scan(&id, &title, &iType, &priority, &status, &createdAt); err != nil {
				return fmt.Errorf("failed to scan row: %w", err)
			}
			// Truncate title if too long?
			if len(title) > 50 {
				title = title[:47] + "..."
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
				id, title, iType, priority, status, createdAt.Format("2006-01-02"))
		}
		w.Flush()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&listStatus, "status", "s", "", "Filter by status")
	listCmd.Flags().StringVarP(&listType, "type", "t", "", "Filter by type")
}
