// Package graph implements the directed acyclic graph (DAG) of grava issues
// and the algorithms grava uses to reason about it.
//
// The core type is AdjacencyDAG, an in-memory adjacency-list DAG with a
// read/write mutex, an optional GraphCache for incremental indegree and
// effective-priority recomputation, and a back-link to a dolt.Store so node
// status, priority, and edge mutations are persisted and audit-logged.
// LoadGraphFromDB hydrates a DAG from the issues and dependencies tables.
// Nodes carry issue metadata (status, priority, await gate, ephemeral flag)
// and edges carry typed dependencies (DependencyType: blocks, blocked-by,
// subtask-of, duplicates, fixes, follows, …).
//
// On top of the storage layer the package provides DAG algorithms used by
// the rest of grava: cycle detection, topological sort (Kahn's algorithm),
// transitive reduction, BFS/DFS traversal, blocking-path queries, status
// bubbling along subtask-of edges, priority inheritance, and the Ready
// Engine (ready_engine.go + priority_queue.go) that produces the prioritised
// list of actionable tasks driven by GateEvaluator. Render helpers emit
// JSON (render.go) and Mermaid.js (mermaid.go) for visualisation.
//
// In grava this package is the substrate for `grava graph`, `grava ready`,
// and the dependency-aware claim/merge logic. Callers typically obtain a
// DAG via LoadGraphFromDB and either query it read-only or mutate through
// the persistence-aware setters (SetNodeStatus, SetNodePriority,
// AddEdgeWithCycleCheck, RemoveNode), which keep the in-memory graph and
// the Dolt database in sync.
package graph
