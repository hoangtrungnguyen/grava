package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	statsDays int
)

type StatsResult struct {
	Total         int            `json:"total_issues"`
	Open          int            `json:"open_issues"`
	Closed        int            `json:"closed_issues"`
	ByStatus      map[string]int `json:"by_status"`
	ByPriority    map[int]int    `json:"by_priority"`
	ByAuthor      map[string]int `json:"by_author"`
	ByAssignee    map[string]int `json:"by_assignee"`
	CreatedByDate map[string]int `json:"created_by_date"`
	ClosedByDate  map[string]int `json:"closed_by_date"`
}

// statsCmd represents the stats command
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show usage statistics",
	Long: `Display statistics about issues in the database.
Includes counts by status, priority, author, assignee, and daily activity.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		stats := StatsResult{
			ByStatus:      make(map[string]int),
			ByPriority:    make(map[int]int),
			ByAuthor:      make(map[string]int),
			ByAssignee:    make(map[string]int),
			CreatedByDate: make(map[string]int),
			ClosedByDate:  make(map[string]int),
		}

		// 1. By Status
		rows, err := Store.Query("SELECT status, COUNT(*) FROM issues WHERE ephemeral = 0 GROUP BY status")
		if err != nil {
			return fmt.Errorf("query by status failed: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var status string
			var count int
			if err := rows.Scan(&status, &count); err != nil {
				return err
			}
			stats.ByStatus[status] = count

			// Simple aggregation for summary
			if status == "closed" || status == "tombstone" {
				stats.Closed += count
			} else {
				stats.Open += count
			}
			stats.Total += count
		}

		// 2. By Priority
		rows, err = Store.Query("SELECT priority, COUNT(*) FROM issues WHERE ephemeral = 0 GROUP BY priority")
		if err != nil {
			return fmt.Errorf("query by priority failed: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var priority int
			var count int
			if err := rows.Scan(&priority, &count); err != nil {
				return err
			}
			stats.ByPriority[priority] = count
		}

		// 3. By Author (Top 10)
		rows, err = Store.Query("SELECT created_by, COUNT(*) FROM issues WHERE ephemeral = 0 GROUP BY created_by ORDER BY COUNT(*) DESC LIMIT 10")
		if err != nil {
			return fmt.Errorf("query by author failed: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var author string
			var count int
			if err := rows.Scan(&author, &count); err != nil {
				return err
			}
			if author == "" {
				author = "unknown"
			}
			stats.ByAuthor[author] = count
		}

		// 4. By Assignee (Top 10)
		rows, err = Store.Query("SELECT assignee, COUNT(*) FROM issues WHERE ephemeral = 0 AND assignee IS NOT NULL AND assignee != '' GROUP BY assignee ORDER BY COUNT(*) DESC LIMIT 10")
		if err != nil {
			return fmt.Errorf("query by assignee failed: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var assignee string
			var count int
			if err := rows.Scan(&assignee, &count); err != nil {
				return err
			}
			stats.ByAssignee[assignee] = count
		}

		// 5. Created By Date (Last N days)
		// Note: using string formatting for interval is safe here as statsDays is an int
		queryDate := fmt.Sprintf("SELECT DATE_FORMAT(created_at, '%%Y-%%m-%%d') as day, COUNT(*) FROM issues WHERE ephemeral = 0 AND created_at >= DATE_SUB(NOW(), INTERVAL %d DAY) GROUP BY day ORDER BY day DESC", statsDays)
		rows, err = Store.Query(queryDate)
		if err != nil {
			return fmt.Errorf("query created by date failed: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var day string
			var count int
			if err := rows.Scan(&day, &count); err != nil {
				return err
			}
			stats.CreatedByDate[day] = count
		}

		// 6. Closed By Date (Last N days) - using updated_at as proxy for now
		queryClosed := fmt.Sprintf("SELECT DATE_FORMAT(updated_at, '%%Y-%%m-%%d') as day, COUNT(*) FROM issues WHERE ephemeral = 0 AND status = 'closed' AND updated_at >= DATE_SUB(NOW(), INTERVAL %d DAY) GROUP BY day ORDER BY day DESC", statsDays)
		rows, err = Store.Query(queryClosed)
		if err != nil {
			return fmt.Errorf("query closed by date failed: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var day string
			var count int
			if err := rows.Scan(&day, &count); err != nil {
				return err
			}
			stats.ClosedByDate[day] = count
		}

		if outputJSON {
			bytes, err := json.MarshalIndent(stats, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(bytes))
			return nil
		}

		// Text Output
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Total Issues:\t%d\n", stats.Total)
		fmt.Fprintf(w, "Open Issues:\t%d\n", stats.Open)
		fmt.Fprintf(w, "Closed Issues:\t%d\n", stats.Closed)
		fmt.Fprintln(w, "")

		fmt.Fprintln(w, "By Status:")
		for status, count := range stats.ByStatus {
			fmt.Fprintf(w, "  %s:\t%d\n", status, count)
		}
		fmt.Fprintln(w, "")

		fmt.Fprintln(w, "By Priority:")
		// Sort priorities
		var priorities []int
		for p := range stats.ByPriority {
			priorities = append(priorities, p)
		}
		sort.Ints(priorities)
		for _, p := range priorities {
			fmt.Fprintf(w, "  P%d:\t%d\n", p, stats.ByPriority[p])
		}
		fmt.Fprintln(w, "")

		fmt.Fprintln(w, "Top Authors:")
		// Sort authors by count desc
		type kv struct {
			Key   string
			Value int
		}
		var authors []kv
		for k, v := range stats.ByAuthor {
			authors = append(authors, kv{k, v})
		}
		sort.Slice(authors, func(i, j int) bool {
			return authors[i].Value > authors[j].Value
		})
		for _, a := range authors {
			fmt.Fprintf(w, "  %s:\t%d\n", a.Key, a.Value)
		}
		fmt.Fprintln(w, "")

		fmt.Fprintln(w, "Top Assignees:")
		var assignees []kv
		for k, v := range stats.ByAssignee {
			assignees = append(assignees, kv{k, v})
		}
		sort.Slice(assignees, func(i, j int) bool {
			return assignees[i].Value > assignees[j].Value
		})
		for _, a := range assignees {
			fmt.Fprintf(w, "  %s:\t%d\n", a.Key, a.Value)
		}
		fmt.Fprintln(w, "")

		fmt.Fprintf(w, "Activity (Last %d Days):\n", statsDays)
		fmt.Fprintln(w, "  Date\t\tCreated\tClosed")

		// Generate dates for the last N days
		now := time.Now()
		for i := 0; i < statsDays; i++ {
			d := now.AddDate(0, 0, -i).Format("2006-01-02")
			created := stats.CreatedByDate[d]
			closed := stats.ClosedByDate[d]

			if created > 0 || closed > 0 {
				fmt.Fprintf(w, "  %s\t%d\t%d\n", d, created, closed)
			}
		}

		w.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
	statsCmd.Flags().IntVar(&statsDays, "days", 7, "Number of days to show activity for")
}
