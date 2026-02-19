package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	listStatus string
	listType   string
	listWisp   bool
	listSort   string
)

var sortColumnMap = map[string]string{
	"id":       "id",
	"title":    "title",
	"type":     "issue_type",
	"status":   "status",
	"priority": "priority",
	"created":  "created_at",
	"updated":  "updated_at",
	"assignee": "assignee",
}

func parseSortFlag(sortStr string) (string, error) {
	if sortStr == "" {
		return "priority ASC, created_at DESC, id ASC", nil
	}

	parts := strings.Split(sortStr, ",")
	var segments []string

	for _, p := range parts {
		subparts := strings.Split(p, ":")
		field := strings.ToLower(strings.TrimSpace(subparts[0]))
		col, ok := sortColumnMap[field]
		if !ok {
			return "", fmt.Errorf("invalid sort field %q", field)
		}

		order := "ASC"
		if len(subparts) > 1 {
			o := strings.ToUpper(strings.TrimSpace(subparts[1]))
			if o != "ASC" && o != "DESC" {
				return "", fmt.Errorf("invalid order %q for field %q", subparts[1], field)
			}
			order = o
		}
		segments = append(segments, fmt.Sprintf("%s %s", col, order))
	}

	// Always add ID for stable sorting
	segments = append(segments, "id ASC")
	return strings.Join(segments, ", "), nil
}

type IssueListItem struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Type      string    `json:"type"`
	Priority  int       `json:"priority"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all issues",
	Long: `List all issues in the Grava tracker.
You can filter by status or type.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		query := "SELECT id, title, issue_type, priority, status, created_at FROM issues"
		var params []any

		// Build WHERE clauses
		whereParts := []string{}
		params = []any{}

		// Ephemeral filter: by default exclude Wisps; --wisp shows only Wisps
		if listWisp {
			whereParts = append(whereParts, "ephemeral = 1")
		} else {
			whereParts = append(whereParts, "ephemeral = 0")
		}

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

		sortClause, err := parseSortFlag(listSort)
		if err != nil {
			return err
		}
		query += " ORDER BY " + sortClause

		rows, err := Store.Query(query, params...)
		if err != nil {
			return fmt.Errorf("failed to query issues: %w", err)
		}
		defer rows.Close()

		var results []IssueListItem

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		if !outputJSON {
			fmt.Fprintln(w, "ID\tTitle\tType\tPriority\tStatus\tCreated")
		}

		for rows.Next() {
			var id, title, iType, status string
			var priority int
			var createdAt time.Time
			if err := rows.Scan(&id, &title, &iType, &priority, &status, &createdAt); err != nil {
				return fmt.Errorf("failed to scan row: %w", err)
			}

			if outputJSON {
				results = append(results, IssueListItem{
					ID:        id,
					Title:     title,
					Type:      iType,
					Priority:  priority,
					Status:    status,
					CreatedAt: createdAt,
				})
			} else {
				// Truncate title if too long?
				if len(title) > 50 {
					title = title[:47] + "..."
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
					id, title, iType, priority, status, createdAt.Format("2006-01-02"))
			}
		}

		if outputJSON {
			b, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
		} else {
			w.Flush()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&listStatus, "status", "s", "", "Filter by status")
	listCmd.Flags().StringVarP(&listType, "type", "t", "", "Filter by type")
	listCmd.Flags().BoolVar(&listWisp, "wisp", false, "Show only ephemeral Wisp issues")
	listCmd.Flags().StringVar(&listSort, "sort", "", "Sort by fields (e.g. priority:asc,created:desc)")
}
