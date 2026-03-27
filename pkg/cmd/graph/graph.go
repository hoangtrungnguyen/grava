// Package cmdgraph contains the graph analysis, dependency, and query commands
// (dep, graph, ready, blocked, search, stats).
//
// Note: package name is cmdgraph (not graph) to avoid collision with pkg/graph.
package cmdgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/hoangtrungnguyen/grava/pkg/graph"
	"github.com/spf13/cobra"
)

var (
	depType      string
	batchFile    string
	blockedDepth int
	searchWisp   bool
	statsDays    int
	readyLimit    int
	readyPriority int
	showInherited bool
)

// SearchWisp is exported for test access.
var SearchWisp = &searchWisp

// QuickVars exposes quick command vars for test resets.
var StatsDays = &statsDays

// IssueListItem is used by search command output.
type IssueListItem struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Type      string    `json:"type"`
	Priority  int       `json:"priority"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// AddCommands registers all graph/dep/query commands with the root cobra.Command.
func AddCommands(root *cobra.Command, d *cmddeps.Deps) {
	depCmd := newDepCmd(d)
	depCmd.AddCommand(newDepBatchCmd(d))
	depCmd.AddCommand(newDepClearCmd(d))
	depCmd.AddCommand(newDepTreeCmd(d))
	depCmd.AddCommand(newDepPathCmd(d))
	depCmd.AddCommand(newDepImpactCmd(d))
	root.AddCommand(depCmd)

	graphCmd := &cobra.Command{
		Use:   "graph",
		Short: "Graph analysis and visualization",
		Long:  `Subcommands for analyzing the task dependency graph.`,
	}
	graphCmd.AddCommand(newGraphStatsCmd(d))
	graphCmd.AddCommand(newGraphCycleCmd(d))
	graphCmd.AddCommand(newGraphHealthCmd(d))
	graphCmd.AddCommand(newGraphVisualizeCmd(d))
	root.AddCommand(graphCmd)

	root.AddCommand(newReadyCmd(d))
	root.AddCommand(newBlockedCmd(d))
	root.AddCommand(newSearchCmd(d))
	root.AddCommand(newStatsCmd(d))
}

func newDepCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dep",
		Short: "Manage task dependencies",
		Long: `Create, list, or batch manage directed dependency edges between issues.

The default usage 'grava dep <from> <to>' creates a "blocks" dependency.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) == 2 {
				return addDependency(cmd, d, args[0], args[1])
			}
			return fmt.Errorf("requires exactly 2 arguments for adding a dependency, or use a subcommand")
		},
	}

	cmd.PersistentFlags().StringVar(&depType, "type", "blocks", "Dependency type (blocks, relates-to, duplicates, parent-child, subtask-of)")
	return cmd
}

func addDependency(cmd *cobra.Command, d *cmddeps.Deps, fromID, toID string) error {
	if fromID == toID {
		return fmt.Errorf("from_id and to_id must be different issues")
	}

	dag, err := graph.LoadGraphFromDB(*d.Store)
	if err != nil {
		return fmt.Errorf("failed to load graph for validation: %w", err)
	}

	dt := graph.DependencyType(depType)
	edge := &graph.Edge{FromID: fromID, ToID: toID, Type: dt}

	if dt.IsBlockingType() {
		if err := dag.AddEdgeWithCycleCheck(edge); err != nil {
			return fmt.Errorf("invalid dependency: %w", err)
		}
	} else {
		if err := dag.AddEdge(edge); err != nil {
			return fmt.Errorf("invalid dependency: %w", err)
		}
	}

	_, err = (*d.Store).Exec(
		`INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)`,
		fromID, toID, depType, *d.Actor, *d.Actor, *d.AgentModel,
	)
	if err != nil {
		return fmt.Errorf("failed to commit dependency to database: %w", err)
	}

	_ = (*d.Store).LogEvent(fromID, "dependency_add", *d.Actor, *d.AgentModel, nil, map[string]interface{}{
		"to_id": toID,
		"type":  depType,
	})

	fmt.Fprintf(cmd.OutOrStdout(), "🔗 Dependency created: %s -[%s]-> %s\n", fromID, depType, toID) //nolint:errcheck
	return nil
}

func newDepBatchCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Batch create dependencies from a JSON file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if batchFile == "" {
				return fmt.Errorf("--file is required")
			}
			f, err := os.Open(batchFile)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close() //nolint:errcheck

			var deps []struct {
				From string `json:"from"`
				To   string `json:"to"`
				Type string `json:"type"`
			}
			if err := json.NewDecoder(f).Decode(&deps); err != nil {
				return fmt.Errorf("failed to decode JSON: %w", err)
			}

			dag, err := graph.LoadGraphFromDB(*d.Store)
			if err != nil {
				return fmt.Errorf("failed to load graph for validation: %w", err)
			}

			for _, dep := range deps {
				if dep.Type == "" {
					dep.Type = "blocks"
				}
				dt := graph.DependencyType(dep.Type)
				edge := &graph.Edge{FromID: dep.From, ToID: dep.To, Type: dt}

				var valErr error
				if dt.IsBlockingType() {
					valErr = dag.AddEdgeWithCycleCheck(edge)
				} else {
					valErr = dag.AddEdge(edge)
				}

				if valErr != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "⚠️ Skipping %s -> %s: %v\n", dep.From, dep.To, valErr) //nolint:errcheck
					continue
				}

				_, err := (*d.Store).Exec(
					`INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)`,
					dep.From, dep.To, dep.Type, *d.Actor, *d.Actor, *d.AgentModel,
				)
				if err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "⚠️ Database failure for %s -> %s: %v\n", dep.From, dep.To, err) //nolint:errcheck
				} else {
					_ = (*d.Store).LogEvent(dep.From, "dependency_add", *d.Actor, *d.AgentModel, nil, map[string]interface{}{
						"to_id": dep.To,
						"type":  dep.Type,
					})
					fmt.Fprintf(cmd.OutOrStdout(), "🔗 Created: %s -[%s]-> %s\n", dep.From, dep.Type, dep.To) //nolint:errcheck
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&batchFile, "file", "f", "", "JSON file containing dependencies")
	return cmd
}

func newDepClearCmd(d *cmddeps.Deps) *cobra.Command {
	return &cobra.Command{
		Use:  "clear <id>",
		Short: "Remove all dependencies for an issue",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			_, err := (*d.Store).Exec(`DELETE FROM dependencies WHERE from_id = ? OR to_id = ?`, id, id)
			if err != nil {
				return fmt.Errorf("failed to clear dependencies: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "🧹 All dependencies for %s cleared.\n", id) //nolint:errcheck
			return nil
		},
	}
}

func newDepTreeCmd(d *cmddeps.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "tree <id>",
		Short: "Show dependency tree (ancestry) for an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			dag, err := graph.LoadGraphFromDB(*d.Store)
			if err != nil {
				return err
			}
			fmt.Printf("Dependency ancestry for %s:\n", id)
			printTree(dag, id, "", true, true, make(map[string]bool))
			return nil
		},
	}
}

func newDepPathCmd(d *cmddeps.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "path <from> <to>",
		Short: "Show the blocking path between two issues",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			from, to := args[0], args[1]
			dag, err := graph.LoadGraphFromDB(*d.Store)
			if err != nil {
				return err
			}
			path, err := dag.GetBlockingPath(from, to)
			if err != nil {
				return err
			}
			if path == nil {
				fmt.Printf("No blocking path found between %s and %s\n", from, to)
				return nil
			}
			fmt.Printf("Blocking path: %s\n", strings.Join(path, " -> "))
			return nil
		},
	}
}

func newDepImpactCmd(d *cmddeps.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "impact <id>",
		Short: "Show downstream impact (successors) for an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			dag, err := graph.LoadGraphFromDB(*d.Store)
			if err != nil {
				return err
			}
			fmt.Printf("Downstream impact of %s:\n", id)
			printImpactTree(dag, id, "", true, true, make(map[string]bool))
			return nil
		},
	}
}

func newGraphStatsCmd(d *cmddeps.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show graph statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			dag, err := graph.LoadGraphFromDB(*d.Store)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Nodes: %d\n", dag.NodeCount())
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Edges: %d\n", dag.EdgeCount())
			if dag.NodeCount() > 1 {
				density := float64(dag.EdgeCount()) / float64(dag.NodeCount()*(dag.NodeCount()-1))
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Density: %.4f\n", density)
			}
			return nil
		},
	}
}

func newGraphCycleCmd(d *cmddeps.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "cycle",
		Short: "Check for cycles in the graph",
		RunE: func(cmd *cobra.Command, args []string) error {
			dag, err := graph.LoadGraphFromDB(*d.Store)
			if err != nil {
				return err
			}
			cycle, err := dag.DetectCycle()
			if err != nil {
				return err
			}
			if len(cycle) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "❌ Cycle detected: %v\n", cycle)
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ No cycles detected.")
			}
			return nil
		},
	}
}

func newGraphHealthCmd(d *cmddeps.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Perform a full graph health check",
		RunE: func(cmd *cobra.Command, args []string) error {
			dag, err := graph.LoadGraphFromDB(*d.Store)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Performing health check...")

			cycle, _ := dag.DetectCycle()
			if len(cycle) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "- Cycles: ❌ Found (%v)\n", cycle)
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "- Cycles: ✅ None")
			}

			orphans := 0
			for _, node := range dag.GetAllNodes() {
				out, _ := dag.GetOutgoingEdges(node.ID)
				in, _ := dag.GetIncomingEdges(node.ID)
				if len(out) == 0 && len(in) == 0 {
					orphans++
				}
			}
			if orphans > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "- Orphans: ⚠️ %d nodes have no dependencies\n", orphans)
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "- Orphans: ✅ None")
			}
			return nil
		},
	}
}

func newGraphVisualizeCmd(d *cmddeps.Deps) *cobra.Command {
	var graphFormat string
	cmd := &cobra.Command{
		Use:   "visualize",
		Short: "Export graph to DOT or Mermaid format",
		RunE: func(cmd *cobra.Command, args []string) error {
			dag, err := graph.LoadGraphFromDB(*d.Store)
			if err != nil {
				return err
			}

			if graphFormat == "mermaid" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), graph.ToMermaid(dag))
				return nil
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "digraph G {")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  rankdir=LR;")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  node [shape=box, style=rounded];")

			for _, node := range dag.GetAllNodes() {
				color := "white"
				switch node.Status {
				case graph.StatusClosed:
					color = "gray"
				case graph.StatusInProgress:
					color = "lightblue"
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  \"%s\" [label=\"%s\", fillcolor=\"%s\", style=\"filled,rounded\"];\n", node.ID, node.Title, color)
			}

			for _, edge := range dag.GetAllEdges() {
				style := "solid"
				if edge.Type == graph.DependencyWaitsFor || edge.Type == graph.DependencyRelatesTo {
					style = "dashed"
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  \"%s\" -> \"%s\" [label=\"%s\", style=\"%s\"];\n", edge.FromID, edge.ToID, edge.Type, style)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "}")
			return nil
		},
	}
	cmd.Flags().StringVarP(&graphFormat, "format", "f", "dot", "Output format (dot, mermaid)")
	return cmd
}

// readyQueue loads the dependency graph from the database and computes the set
// of tasks that are ready to be worked on (not blocked by any open dependency).
// ctx is accepted for future compatibility — graph.LoadGraphFromDB does not currently use it.
func readyQueue(ctx context.Context, store dolt.Store, limit int) ([]*graph.ReadyTask, error) {
	_ = ctx // reserved for future context propagation into LoadGraphFromDB
	dag, err := graph.LoadGraphFromDB(store)
	if err != nil {
		return nil, gravaerrors.New("DB_UNREACHABLE", "failed to load graph", err)
	}
	engine := graph.NewReadyEngine(dag, graph.DefaultReadyEngineConfig())
	tasks, err := engine.ComputeReady(limit)
	if err != nil {
		return nil, gravaerrors.New("DB_UNREACHABLE", "failed to compute ready queue", err)
	}
	return tasks, nil
}

func newReadyCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ready",
		Short: "Show tasks that are ready to be worked on",
		Long: `Ready computes tasks that are not blocked by any open dependencies or gates.
Tasks are sorted by their effective priority (highest first) and age.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			tasks, err := readyQueue(cmd.Context(), *d.Store, readyLimit)
			if err != nil {
				return err
			}

			if readyPriority != -1 {
				filtered := []*graph.ReadyTask{}
				for _, t := range tasks {
					if int(t.EffectivePriority) == readyPriority {
						filtered = append(filtered, t)
					}
				}
				tasks = filtered
			}

			if *d.OutputJSON {
				b, err := json.MarshalIndent(tasks, "", "  ")
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tTitle\tPriority\tAge\tStatus")

			for _, task := range tasks {
				prioStr := fmt.Sprintf("%d", task.EffectivePriority)
				if task.PriorityBoosted && showInherited {
					prioStr = fmt.Sprintf("%d*", task.EffectivePriority)
				}

				title := task.Node.Title
				if task.Node.Ephemeral {
					title = "👻 " + title
				}
				if len(title) > 50 {
					title = title[:47] + "..."
				}

				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					task.Node.ID, title, prioStr, formatAge(task.Age), task.Node.Status)
			}
			w.Flush() //nolint:errcheck

			if len(tasks) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No ready tasks found.")
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&readyLimit, "limit", "l", 20, "Limit number of results")
	cmd.Flags().IntVarP(&readyPriority, "priority", "p", -1, "Filter by priority level")
	cmd.Flags().BoolVar(&showInherited, "show-inherited", false, "Show if priority was inherited or boosted (indicated by *)")
	return cmd
}

func newBlockedCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blocked",
		Short: "Show tasks that are currently blocked",
		RunE: func(cmd *cobra.Command, args []string) error {
			dag, err := graph.LoadGraphFromDB(*d.Store)
			if err != nil {
				return fmt.Errorf("failed to load graph: %w", err)
			}

			type blockedInfo struct {
				ID          string   `json:"id"`
				Title       string   `json:"title"`
				Blockers    []string `json:"blockers"`
				GateBlocked bool     `json:"gate_blocked"`
				AwaitType   string   `json:"await_type,omitempty"`
				Ephemeral   bool     `json:"ephemeral"`
			}

			blockedResults := []blockedInfo{}

			for _, node := range dag.GetAllNodes() {
				if node.Status != graph.StatusOpen {
					continue
				}

				blockers, _ := dag.GetTransitiveBlockers(node.ID, blockedDepth)

				gateBlocked := false
				if node.AwaitType != "" {
					ge := graph.NewDefaultGateEvaluator()
					open, _ := ge.IsGateOpen(node)
					if !open {
						gateBlocked = true
					}
				}

				if len(blockers) > 0 || gateBlocked {
					blockedResults = append(blockedResults, blockedInfo{
						ID:          node.ID,
						Title:       node.Title,
						Blockers:    blockers,
						GateBlocked: gateBlocked,
						AwaitType:   node.AwaitType,
						Ephemeral:   node.Ephemeral,
					})
				}
			}

			if *d.OutputJSON {
				b, _ := json.MarshalIndent(blockedResults, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTitle\tBlocked By\tGate") //nolint:errcheck

			for _, info := range blockedResults {
				blockerStr := "-"
				if len(info.Blockers) > 0 {
					blockerStr = fmt.Sprintf("%v", info.Blockers)
				}
				gateStr := "-"
				if info.GateBlocked {
					gateStr = info.AwaitType
				}
				title := info.Title
				if info.Ephemeral {
					title = "👻 " + title
				}
				if len(title) > 40 {
					title = title[:37] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", info.ID, title, blockerStr, gateStr) //nolint:errcheck
			}
			w.Flush() //nolint:errcheck

			if len(blockedResults) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No blocked tasks found.") //nolint:errcheck
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&blockedDepth, "depth", "d", 1, "Depth of transitive blockers to show")
	return cmd
}

func newSearchCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search for issues matching a text query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			if strings.TrimSpace(query) == "" {
				return fmt.Errorf("search query must not be empty")
			}

			pattern := "%" + query + "%"

			sql := `SELECT id, title, issue_type, priority, status, created_at
		        FROM issues
		        WHERE ephemeral = ?
		          AND status != 'tombstone'
		          AND (title LIKE ? OR description LIKE ? OR COALESCE(metadata,'') LIKE ?)
		        ORDER BY priority ASC, created_at DESC`

			ephemeralVal := 0
			if searchWisp {
				ephemeralVal = 1
			}

			rows, err := (*d.Store).Query(sql, ephemeralVal, pattern, pattern, pattern)
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}
			defer rows.Close() //nolint:errcheck

			var results []IssueListItem
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			if !*d.OutputJSON {
				_, _ = fmt.Fprintln(w, "ID\tTitle\tType\tPriority\tStatus\tCreated")
			}

			found := 0
			for rows.Next() {
				var id, title, iType, status string
				var priority int
				var createdAt time.Time
				if err := rows.Scan(&id, &title, &iType, &priority, &status, &createdAt); err != nil {
					return fmt.Errorf("failed to scan row: %w", err)
				}

				if *d.OutputJSON {
					results = append(results, IssueListItem{
						ID:        id,
						Title:     title,
						Type:      iType,
						Priority:  priority,
						Status:    status,
						CreatedAt: createdAt,
					})
				} else {
					if len(title) > 50 {
						title = title[:47] + "..."
					}
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
						id, title, iType, priority, status, createdAt.Format("2006-01-02"))
				}
				found++
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("row iteration error: %w", err)
			}

			if *d.OutputJSON {
				b, _ := json.MarshalIndent(results, "", "  ")
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
			} else {
				w.Flush() //nolint:errcheck
				if found == 0 {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No issues found matching %q\n", query)
				} else {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n🔍 %d result(s) for %q\n", found, query)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&searchWisp, "wisp", false, "Include ephemeral Wisp issues in results")
	return cmd
}

// StatsResult holds the statistics response data.
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

func newStatsCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show usage statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			stats := StatsResult{
				ByStatus:      make(map[string]int),
				ByPriority:    make(map[int]int),
				ByAuthor:      make(map[string]int),
				ByAssignee:    make(map[string]int),
				CreatedByDate: make(map[string]int),
				ClosedByDate:  make(map[string]int),
			}

			rows, err := (*d.Store).Query("SELECT status, COUNT(*) FROM issues WHERE ephemeral = 0 GROUP BY status")
			if err != nil {
				return fmt.Errorf("query by status failed: %w", err)
			}
			defer rows.Close() //nolint:errcheck
			for rows.Next() {
				var status string
				var count int
				if err := rows.Scan(&status, &count); err != nil {
					return err
				}
				stats.ByStatus[status] = count
				if status == "closed" || status == "tombstone" {
					stats.Closed += count
				} else {
					stats.Open += count
				}
				stats.Total += count
			}

			rows, err = (*d.Store).Query("SELECT priority, COUNT(*) FROM issues WHERE ephemeral = 0 GROUP BY priority")
			if err != nil {
				return fmt.Errorf("query by priority failed: %w", err)
			}
			defer rows.Close() //nolint:errcheck
			for rows.Next() {
				var priority, count int
				if err := rows.Scan(&priority, &count); err != nil {
					return err
				}
				stats.ByPriority[priority] = count
			}

			rows, err = (*d.Store).Query("SELECT created_by, COUNT(*) FROM issues WHERE ephemeral = 0 GROUP BY created_by ORDER BY COUNT(*) DESC LIMIT 10")
			if err != nil {
				return fmt.Errorf("query by author failed: %w", err)
			}
			defer rows.Close() //nolint:errcheck
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

			rows, err = (*d.Store).Query("SELECT assignee, COUNT(*) FROM issues WHERE ephemeral = 0 AND assignee IS NOT NULL AND assignee != '' GROUP BY assignee ORDER BY COUNT(*) DESC LIMIT 10")
			if err != nil {
				return fmt.Errorf("query by assignee failed: %w", err)
			}
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var assignee string
				var count int
				if err := rows.Scan(&assignee, &count); err != nil {
					return err
				}
				stats.ByAssignee[assignee] = count
			}

			queryDate := fmt.Sprintf("SELECT DATE_FORMAT(created_at, '%%Y-%%m-%%d') as day, COUNT(*) FROM issues WHERE ephemeral = 0 AND created_at >= DATE_SUB(NOW(), INTERVAL %d DAY) GROUP BY day ORDER BY day DESC", statsDays)
			rows, err = (*d.Store).Query(queryDate)
			if err != nil {
				return fmt.Errorf("query created by date failed: %w", err)
			}
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var day string
				var count int
				if err := rows.Scan(&day, &count); err != nil {
					return err
				}
				stats.CreatedByDate[day] = count
			}

			queryClosed := fmt.Sprintf("SELECT DATE_FORMAT(updated_at, '%%Y-%%m-%%d') as day, COUNT(*) FROM issues WHERE ephemeral = 0 AND status = 'closed' AND updated_at >= DATE_SUB(NOW(), INTERVAL %d DAY) GROUP BY day ORDER BY day DESC", statsDays)
			rows, err = (*d.Store).Query(queryClosed)
			if err != nil {
				return fmt.Errorf("query closed by date failed: %w", err)
			}
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var day string
				var count int
				if err := rows.Scan(&day, &count); err != nil {
					return err
				}
				stats.ClosedByDate[day] = count
			}

			if *d.OutputJSON {
				bytes, err := json.MarshalIndent(stats, "", "  ")
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(bytes))
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintf(w, "Total Issues:\t%d\n", stats.Total)
			_, _ = fmt.Fprintf(w, "Open Issues:\t%d\n", stats.Open)
			_, _ = fmt.Fprintf(w, "Closed Issues:\t%d\n", stats.Closed)
			_, _ = fmt.Fprintln(w, "")

			_, _ = fmt.Fprintln(w, "By Status:")
			for status, count := range stats.ByStatus {
				_, _ = fmt.Fprintf(w, "  %s:\t%d\n", status, count)
			}
			_, _ = fmt.Fprintln(w, "")

			_, _ = fmt.Fprintln(w, "By Priority:")
			var priorities []int
			for p := range stats.ByPriority {
				priorities = append(priorities, p)
			}
			sort.Ints(priorities)
			for _, p := range priorities {
				_, _ = fmt.Fprintf(w, "  P%d:\t%d\n", p, stats.ByPriority[p])
			}
			_, _ = fmt.Fprintln(w, "")

			_, _ = fmt.Fprintln(w, "Top Authors:")
			type kv struct{ Key string; Value int }
			var authors []kv
			for k, v := range stats.ByAuthor {
				authors = append(authors, kv{k, v})
			}
			sort.Slice(authors, func(i, j int) bool { return authors[i].Value > authors[j].Value })
			for _, a := range authors {
				_, _ = fmt.Fprintf(w, "  %s:\t%d\n", a.Key, a.Value)
			}
			_, _ = fmt.Fprintln(w, "")

			_, _ = fmt.Fprintln(w, "Top Assignees:")
			var assignees []kv
			for k, v := range stats.ByAssignee {
				assignees = append(assignees, kv{k, v})
			}
			sort.Slice(assignees, func(i, j int) bool { return assignees[i].Value > assignees[j].Value })
			for _, a := range assignees {
				_, _ = fmt.Fprintf(w, "  %s:\t%d\n", a.Key, a.Value)
			}
			_, _ = fmt.Fprintln(w, "")

			_, _ = fmt.Fprintf(w, "Activity (Last %d Days):\n", statsDays)
			_, _ = fmt.Fprintln(w, "  Date\t\tCreated\tClosed")
			now := time.Now()
			for i := 0; i < statsDays; i++ {
				d := now.AddDate(0, 0, -i).Format("2006-01-02")
				created := stats.CreatedByDate[d]
				closed := stats.ClosedByDate[d]
				if created > 0 || closed > 0 {
					_, _ = fmt.Fprintf(w, "  %s\t%d\t%d\n", d, created, closed)
				}
			}
			w.Flush() //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().IntVar(&statsDays, "days", 7, "Number of days to show activity for")
	return cmd
}

func formatAge(d time.Duration) string {
	if d < 24*time.Hour {
		return d.Round(time.Minute).String()
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%dd", days)
}

func printTree(dag *graph.AdjacencyDAG, id string, indent string, isLast bool, isRoot bool, visited map[string]bool) {
	node, err := dag.GetNode(id)
	if err != nil {
		fmt.Printf("%s%s %s [Missing]\n", indent, getMarker(isLast, isRoot), id)
		return
	}
	fmt.Printf("%s%s %s: %s [%s]\n", indent, getMarker(isLast, isRoot), node.ID, node.Title, node.Status)
	if visited[id] {
		fmt.Printf("%s    (cycle/already shown)\n", indent+getFill(isLast, isRoot))
		return
	}
	visited[id] = true
	preds, _ := dag.GetPredecessors(id)
	for i, predID := range preds {
		printTree(dag, predID, indent+getFill(isLast, isRoot), i == len(preds)-1, false, visited)
	}
}

func printImpactTree(dag *graph.AdjacencyDAG, id string, indent string, isLast bool, isRoot bool, visited map[string]bool) {
	node, err := dag.GetNode(id)
	if err != nil {
		fmt.Printf("%s%s %s [Missing]\n", indent, getMarker(isLast, isRoot), id)
		return
	}
	fmt.Printf("%s%s %s: %s [%s]\n", indent, getMarker(isLast, isRoot), node.ID, node.Title, node.Status)
	if visited[id] {
		fmt.Printf("%s    (cycle/already shown)\n", indent+getFill(isLast, isRoot))
		return
	}
	visited[id] = true
	successors, _ := dag.GetSuccessors(id)
	for i, succID := range successors {
		printImpactTree(dag, succID, indent+getFill(isLast, isRoot), i == len(successors)-1, false, visited)
	}
}

func getMarker(isLast bool, isRoot bool) string {
	if isRoot {
		return ""
	}
	if isLast {
		return "└──"
	}
	return "├──"
}

func getFill(isLast bool, isRoot bool) string {
	if isRoot {
		return ""
	}
	if isLast {
		return "    "
	}
	return "│   "
}
