package cmd

import (
	"fmt"

	"github.com/hoangtrungnguyen/grava/pkg/graph"
	"github.com/spf13/cobra"
)

// graphCmd represents the graph command
var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Graph analysis and visualization",
	Long:  `Subcommands for analyzing the task dependency graph.`,
}

var graphStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show graph statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		dag, err := graph.LoadGraphFromDB(Store)
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

var graphCycleCmd = &cobra.Command{
	Use:   "cycle",
	Short: "Check for cycles in the graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		dag, err := graph.LoadGraphFromDB(Store)
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

var graphHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Perform a full graph health check",
	RunE: func(cmd *cobra.Command, args []string) error {
		dag, err := graph.LoadGraphFromDB(Store)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Performing health check...")

		// Check for cycles
		cycle, _ := dag.DetectCycle()
		if len(cycle) > 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "- Cycles: ❌ Found (%v)\n", cycle)
		} else {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "- Cycles: ✅ None")
		}

		// Check for orphan nodes (no incoming, no outgoing)
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

var graphFormat string

var graphVisualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Export graph to DOT or Mermaid format",
	RunE: func(cmd *cobra.Command, args []string) error {
		dag, err := graph.LoadGraphFromDB(Store)
		if err != nil {
			return err
		}

		if graphFormat == "mermaid" {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), graph.ToMermaid(dag))
			return nil
		}

		// Default: DOT format
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

