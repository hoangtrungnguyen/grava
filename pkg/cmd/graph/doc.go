// Package cmdgraph implements the dependency-graph and query CLI commands:
// dep, graph, ready, blocked, search, and stats.
//
// The package name is cmdgraph (not graph) to avoid collision with the
// underlying domain package github.com/hoangtrungnguyen/grava/pkg/graph,
// which provides the AdjacencyDAG, ReadyEngine, and gate evaluator. This
// package is the thin cobra wrapper that loads the graph from Dolt
// (graph.LoadGraphFromDB), performs cycle-checked edits in audited
// transactions, and renders results as tables or JSON.
//
// Commands registered:
//   - dep <from> <to>           add/remove a typed dependency edge
//   - dep batch / clear / tree / path / impact
//   - graph stats / cycle / health / visualize
//   - ready                     compute the unblocked task queue
//   - blocked [id]              show blockers globally or per-issue
//   - search                    text/label search over issues
//   - stats                     workspace KPIs
//
// AddCommands(root, deps) wires every command into the root cobra.Command
// using shared cmddeps.Deps for Store, Actor, AgentModel, and OutputJSON.
// ReadyQueue is also exported for sandbox scenarios that need the same
// computation outside the CLI.
package cmdgraph
