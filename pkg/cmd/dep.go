package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hoangtrungnguyen/grava/pkg/graph"
	"github.com/spf13/cobra"
)

var (
	depType   string
	batchFile string
)

// depCmd represents the dep command
var depCmd = &cobra.Command{
	Use:   "dep",
	Short: "Manage task dependencies",
	Long: `Create, list, or batch manage directed dependency edges between issues.

The default usage 'grava dep <from> <to>' creates a "blocks" dependency.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		if len(args) == 2 {
			return addDependency(cmd, args[0], args[1])
		}
		return fmt.Errorf("requires exactly 2 arguments for adding a dependency, or use a subcommand")
	},
}

func addDependency(cmd *cobra.Command, fromID, toID string) error {
	if fromID == toID {
		return fmt.Errorf("from_id and to_id must be different issues")
	}

	// Load graph to validate and check for cycles
	dag, err := graph.LoadGraphFromDB(Store)
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
		// Even non-blocking edges should be validated for existence and self-loops
		if err := dag.AddEdge(edge); err != nil {
			return fmt.Errorf("invalid dependency: %w", err)
		}
	}

	_, err = Store.Exec(
		`INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)`,
		fromID, toID, depType, actor, actor, agentModel,
	)
	if err != nil {
		return fmt.Errorf("failed to commit dependency to database: %w", err)
	}

	// Audit Log
	_ = Store.LogEvent(fromID, "dependency_add", actor, agentModel, nil, map[string]interface{}{
		"to_id": toID,
		"type":  depType,
	})

	fmt.Fprintf(cmd.OutOrStdout(), "🔗 Dependency created: %s -[%s]-> %s\n", fromID, depType, toID) //nolint:errcheck
	return nil
}

var depBatchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Batch create dependencies from a JSON file",
	Long: `Provide a JSON file with an array of dependency objects.
Example JSON:
[
  {"from": "grava-123", "to": "grava-456", "type": "blocks"},
  {"from": "grava-789", "to": "grava-abc"}
]`,
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

		dag, err := graph.LoadGraphFromDB(Store)
		if err != nil {
			return fmt.Errorf("failed to load graph for validation: %w", err)
		}

		for _, d := range deps {
			if d.Type == "" {
				d.Type = "blocks"
			}
			dt := graph.DependencyType(d.Type)
			edge := &graph.Edge{FromID: d.From, ToID: d.To, Type: dt}

			// Validate against graph state (cycle check, existence)
			var valErr error
			if dt.IsBlockingType() {
				valErr = dag.AddEdgeWithCycleCheck(edge)
			} else {
				valErr = dag.AddEdge(edge)
			}

			if valErr != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "⚠️ Skipping %s -> %s: %v\n", d.From, d.To, valErr) //nolint:errcheck
				continue
			}

			_, err := Store.Exec(
				`INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)`,
				d.From, d.To, d.Type, actor, actor, agentModel,
			)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "⚠️ Database failure for %s -> %s: %v\n", d.From, d.To, err) //nolint:errcheck
			} else {
				// Audit Log
				_ = Store.LogEvent(d.From, "dependency_add", actor, agentModel, nil, map[string]interface{}{
					"to_id": d.To,
					"type":  d.Type,
				})
				fmt.Fprintf(cmd.OutOrStdout(), "🔗 Created: %s -[%s]-> %s\n", d.From, d.Type, d.To) //nolint:errcheck
			}
		}

		return nil
	},
}

var depClearCmd = &cobra.Command{
	Use:   "clear <id>",
	Short: "Remove all dependencies for an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		_, err := Store.Exec(`DELETE FROM dependencies WHERE from_id = ? OR to_id = ?`, id, id)
		if err != nil {
			return fmt.Errorf("failed to clear dependencies: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "🧹 All dependencies for %s cleared.\n", id) //nolint:errcheck
		return nil
	},
}

var depTreeCmd = &cobra.Command{
	Use:   "tree <id>",
	Short: "Show dependency tree (ancestry) for an issue",
	Long:  `Displays a tree-based visualization of all tasks that the given issue depends on.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		dag, err := graph.LoadGraphFromDB(Store)
		if err != nil {
			return err
		}
		cmd.Printf("Dependency ancestry for %s:\n", id)
		printTree(cmd.OutOrStdout(), dag, id, "", true, true, make(map[string]bool))
		return nil
	},
}

var depPathCmd = &cobra.Command{
	Use:   "path <from> <to>",
	Short: "Show the blocking path between two issues",
	Long:  `Finds and displays the specific chain of dependencies blocking a task.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		from, to := args[0], args[1]
		dag, err := graph.LoadGraphFromDB(Store)
		if err != nil {
			return err
		}
		path, err := dag.GetBlockingPath(from, to)
		if err != nil {
			return err
		}
		if path == nil {
			cmd.Printf("No blocking path found between %s and %s\n", from, to)
			return nil
		}
		cmd.Printf("Blocking path: %s\n", strings.Join(path, " -> "))
		return nil
	},
}

var depImpactCmd = &cobra.Command{
	Use:   "impact <id>",
	Short: "Show downstream impact (successors) for an issue",
	Long:  `Displays a tree-based visualization of all tasks that depend on the given issue (the "blast radius" of a delay).`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		dag, err := graph.LoadGraphFromDB(Store)
		if err != nil {
			return err
		}
		cmd.Printf("Downstream impact of %s:\n", id)
		printImpactTree(cmd.OutOrStdout(), dag, id, "", true, true, make(map[string]bool))
		return nil
	},
}

func printTree(w io.Writer, dag *graph.AdjacencyDAG, id string, indent string, isLast bool, isRoot bool, visited map[string]bool) {
	node, err := dag.GetNode(id)
	if err != nil {
		_, _ = fmt.Fprintf(w, "%s%s %s [Missing]\n", indent, getMarker(isLast, isRoot), id)
		return
	}

	_, _ = fmt.Fprintf(w, "%s%s %s: %s [%s]\n", indent, getMarker(isLast, isRoot), node.ID, node.Title, node.Status)

	if visited[id] {
		_, _ = fmt.Fprintf(w, "%s    (cycle/already shown)\n", indent+getFill(isLast, isRoot))
		return
	}
	visited[id] = true

	preds, _ := dag.GetPredecessors(id)
	for i, predID := range preds {
		printTree(w, dag, predID, indent+getFill(isLast, isRoot), i == len(preds)-1, false, visited)
	}
}

func printImpactTree(w io.Writer, dag *graph.AdjacencyDAG, id string, indent string, isLast bool, isRoot bool, visited map[string]bool) {
	node, err := dag.GetNode(id)
	if err != nil {
		_, _ = fmt.Fprintf(w, "%s%s %s [Missing]\n", indent, getMarker(isLast, isRoot), id)
		return
	}

	_, _ = fmt.Fprintf(w, "%s%s %s: %s [%s]\n", indent, getMarker(isLast, isRoot), node.ID, node.Title, node.Status)

	if visited[id] {
		_, _ = fmt.Fprintf(w, "%s    (cycle/already shown)\n", indent+getFill(isLast, isRoot))
		return
	}
	visited[id] = true

	successors, _ := dag.GetSuccessors(id)
	for i, succID := range successors {
		printImpactTree(w, dag, succID, indent+getFill(isLast, isRoot), i == len(successors)-1, false, visited)
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

