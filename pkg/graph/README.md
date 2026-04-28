# Package: graph

Path: `github.com/hoangtrungnguyen/grava/pkg/graph`

## Purpose

In-memory DAG of grava issues plus the algorithms (cycle detection,
topological sort, transitive reduction, ready-task scheduling, priority
inheritance, status bubbling, rendering) that operate on it. Backed by a
`dolt.Store` for persistence and audit logging.

## Key Types & Functions

- `Graph`, `DAG` — interfaces (graph.go).
- `Node`, `Edge`, `ReadyTask`, `IssueStatus`, `Priority`, `DependencyType`
  — domain types (types.go). DependencyType has helpers `IsBlockingType`,
  `IsSoftDependency`.
- `AdjacencyDAG` — mutex-guarded adjacency-list DAG with optional cache and
  store back-link (dag.go). Implements DAG plus `GetTreeChildren`,
  `GetTransitiveBlockers`, `GetBlockingPath`, `TransitiveReduction`.
- `NewAdjacencyDAG(enableCache bool)` — constructor.
- `LoadGraphFromDB(store dolt.Store) (*AdjacencyDAG, error)` — hydrate from
  `issues` + `dependencies` tables (loader.go).
- `GraphCache` — incremental indegree + effective-priority cache
  (cache.go).
- `TopologicalSort` (topology.go), `DetectCycle` + `CycleError` (cycle.go,
  errors.go), `BFS`/`DFS` (traversal.go).
- `ReadyEngine`, `ReadyEngineConfig`, `GateEvaluator`,
  `DefaultGateEvaluator` — actionable-task selection (ready_engine.go,
  gates.go, priority_queue.go).
- `ToMermaid(g DAG) string` (mermaid.go), `GraphJSON` /
  `RenderJSON` family (render.go).
- Errors: `ErrNilNode`, `ErrNilEdge`, `ErrNodeNotFound`, `ErrNodeExists`,
  `ErrSelfLoop`, `ErrCycleDetected`.

## Dependencies

- `github.com/hoangtrungnguyen/grava/pkg/dolt` for persistence.
- Standard library `sync`, `container/heap`, `encoding/json`, `time`,
  `fmt`, `sort`, `strings`.

## How It Fits

Substrate for `grava graph`, `grava ready`, dependency-aware claim/merge,
and any command that needs to reason about issue relationships. Mutations
go through persistence-aware setters so the DAG and Dolt stay in sync.

## Usage

```go
dag, err := graph.LoadGraphFromDB(store)
if err != nil {
    return err
}
dag.SetSession("alice", "claude-opus-4-7")
if err := dag.AddEdgeWithCycleCheck(&graph.Edge{
    FromID: "grava-a1b2", ToID: "grava-c3d4",
    Type:   graph.DependencyBlocks,
}); err != nil {
    return err
}
order, err := dag.TopologicalSort()
```
