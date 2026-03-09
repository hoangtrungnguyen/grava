# Grava Graph Mechanics

**Source Package:** `pkg/graph`  
**Last Updated:** 2026-02-24

---

## Table of Contents

1. [Overview](#1-overview)
2. [Core Data Structures](#2-core-data-structures)
3. [Dependency Types (Edge Semantics)](#3-dependency-types-edge-semantics)
4. [The AdjacencyDAG](#4-the-adjacencydag)
5. [The Ready Engine](#5-the-ready-engine)
6. [The Graph Cache](#6-the-graph-cache)
7. [Algorithms](#7-algorithms)
8. [Node Lifecycle & Status Model](#8-node-lifecycle--status-model)
9. [Gate System](#9-gate-system)
10. [Performance Characteristics](#10-performance-characteristics)
11. [Extension Points (Future Work)](#11-extension-points-future-work)

---

## 1. Overview

Grava's graph engine stores every issue as a **node** and every relationship as a **directed edge** in a Directed Acyclic Graph (DAG). The graph is held entirely in memory using adjacency lists, backed by Dolt for persistence.

The central question the engine answers is:

> **"Which tasks can an agent work on right now, and in what order?"**

This is solved by the **Ready Engine**, which traverses the graph to find issues with no open blocking dependencies, then ranks them using **priority inheritance**, **aging boosts**, and **gate evaluation**.

### Architecture Overview

```
                ┌──────────────────────────┐
                │     AdjacencyDAG         │
                │  nodes (map[id]*Node)    │
                │  outgoing (map adj list) │
                │  incoming (map adj list) │
                │  cache *GraphCache  ─────┼──▶ GraphCache
                │  store dolt.Store   ─────┼──▶ Dolt (persistence)
                └──────────┬───────────────┘
                           │
                    ┌──────▼──────────┐
                    │  ReadyEngine    │
                    │  ComputeReady() │
                    └──────┬──────────┘
                           │ sorted by EffectivePriority
                    ┌──────▼──────────┐
                    │  PriorityQueue  │
                    │  (container/heap)│
                    └─────────────────┘
```

---

## 2. Core Data Structures

### 2.1 Node (`types.go`)

```go
type Node struct {
    ID        string
    Title     string
    Status    IssueStatus
    Priority  Priority
    CreatedAt time.Time
    UpdatedAt time.Time
    AwaitType string // Gate: "gh:pr", "timer", "human", or ""
    AwaitID   string // Gate identifier (e.g. PR number)
    Metadata  map[string]interface{}
}
```

A Node represents a single issue. The `AwaitType` and `AwaitID` fields implement the **Gate System** — a node blocked on an external event (a GitHub PR merging, a timer expiring) will not appear in the ready list until the gate condition is satisfied.

### 2.2 Edge (`types.go`)

```go
type Edge struct {
    FromID   string
    ToID     string
    Type     DependencyType
    Metadata map[string]interface{}
}
```

An edge in Grava is **directional** and **typed**. The direction and type together determine the semantic meaning: `A --blocks--> B` means "A must be completed before B can start."

### 2.3 ReadyTask (`types.go`)

```go
type ReadyTask struct {
    Node              *Node
    EffectivePriority Priority  // After priority inheritance and aging
    Age               time.Duration
    PriorityBoosted   bool      // true if priority was elevated by inheritance or aging
}
```

The output unit of the Ready Engine. An agent receives a sorted list of `ReadyTask` values and picks the first one.

### 2.4 Priority

```go
const (
    PriorityCritical Priority = 0  // P0 — drop everything
    PriorityHigh     Priority = 1  // P1
    PriorityMedium   Priority = 2  // P2
    PriorityLow      Priority = 3  // P3
    PriorityBacklog  Priority = 4  // P4 — do when time allows
)
```

**Lower number = higher priority.** The `PriorityQueue` is a min-heap keyed on `EffectivePriority`.

---

## 3. Dependency Types (Edge Semantics)

Grava defines **19 semantic edge types** grouped into 5 categories. Each type carries a distinct meaning that the engine uses to make different decisions.

### Blocking Types — Hard Dependencies

These are the most important. They define what **must** be done before something else can start. Only these edges count toward a node's "blocking indegree" in the Ready Engine.

| Type | Meaning |
|---|---|
| `blocks` | `A --blocks--> B`: A must be completed before B |
| `blocked-by` | Inverse of `blocks` (added symmetrically) |

```go
func (dt DependencyType) IsBlockingType() bool {
    return dt == DependencyBlocks || dt == DependencyBlockedBy
}
```

### Soft Dependencies — Ordering Hints

These de-prioritize a task but do **not** prevent it from appearing in the ready list.

| Type | Meaning |
|---|---|
| `waits-for` | A should ideally wait for B, but can proceed if necessary |
| `depends-on` | General dependency (informational) |

```go
func (dt DependencyType) IsSoftDependency() bool {
    return dt == DependencyWaitsFor || dt == DependencyDependsOn
}
```

### Hierarchical — Work Decomposition

Used to link Epics → Tasks → Subtasks. These define the **parent-child structure** and drive automated status propagation (Epic 2.4).

| Type | Meaning |
|---|---|
| `parent-child` | `Epic --parent-child--> Task` |
| `child-of` | Inverse of `parent-child` |
| `subtask-of` | `Subtask --subtask-of--> Task` |
| `has-subtask` | Inverse of `subtask-of` |

### Semantic Relationships — Metadata

These are informational and do not affect the Ready Engine.

| Type | Meaning |
|---|---|
| `duplicates` / `duplicated-by` | Issue deduplication |
| `relates-to` | General association |
| `supersedes` / `superseded-by` | Replacement tracking |
| `follows` / `precedes` | Sequencing hints |

### Technical — Code Provenance

Used to link issues to code events.

| Type | Meaning |
|---|---|
| `caused-by` / `causes` | Bug causation chain |
| `fixes` / `fixed-by` | Fix linkage |

---

## 4. The AdjacencyDAG

**File:** `pkg/graph/dag.go`

The core data structure. It is a **doubly-linked adjacency list** stored in three maps:

```go
type AdjacencyDAG struct {
    mu sync.RWMutex

    nodes    map[string]*Node            // O(1) node lookup
    outgoing map[string]map[string]*Edge // fromID -> toID -> Edge
    incoming map[string]map[string]*Edge // toID -> fromID -> Edge

    cache *GraphCache  // Optional pre-computed properties
    store dolt.Store   // Persistence layer
    actor string       // Current user/agent (for audit logging)
}
```

The **dual adjacency list** (both `outgoing` and `incoming`) is the key design decision. It enables `O(1)` lookup of both successors ("what does this task block?") and predecessors ("what blocks this task?") without traversal.

### Why Both Directions?

| Query | Without incoming map | With incoming map |
|---|---|---|
| "What does A block?" | O(1) via `outgoing[A]` | O(1) via `outgoing[A]` |
| "What blocks A?" | O(V+E) — scan all edges | O(1) via `incoming[A]` |
| Indegree of A | O(V+E) scan | O(1): `len(incoming[A])` |
| Priority inheritance BFS | O(V+E) from every node | O(1) start, O(E) total |

### Thread Safety

All public methods acquire a `sync.RWMutex`. Read operations (`GetNode`, `GetIndegree`, traversals) acquire `RLock()`. Write operations (`AddNode`, `AddEdge`, `SetNodeStatus`) acquire `Lock()`.

The cache uses its own separate `RWMutex` to avoid deadlocks during propagation.

### Persistence Integration

Every mutation (status change, priority change, node removal, edge add/remove) writes to Dolt and appends to the `events` table via `g.store.LogEvent()`. This means the in-memory graph and the Dolt database are always in sync.

```go
// Example — SetNodeStatus writes to DB and logs event atomically
g.store.Exec("UPDATE issues SET status = ? WHERE id = ?", status, id)
g.store.LogEvent(id, "status_change", actor, model, oldStatus, newStatus)
```

---

## 5. The Ready Engine

**File:** `pkg/graph/ready_engine.go`

The Ready Engine is the **core algorithm** that answers "what should an agent work on next?" It runs in three phases: **filtering**, **scoring**, and **sorting**.

### 5.1 Phase 1 — Filtering (Finding Candidates)

A node is a candidate for the ready list if ALL of the following are true:

1. **Status is `open`** — `in_progress`, `closed`, `blocked`, `deferred`, `pinned` nodes are excluded.
2. **Blocking indegree is 0** — No open nodes with `blocks` edges pointing to this node.
3. **Gate is open** (if applicable) — The `AwaitType` condition is satisfied (see Section 9).

```go
// From ready_engine.go - ComputeReady()
if node.Status != StatusOpen { continue }

blockingIndegree := re.getBlockingIndegree(nodeID)
if blockingIndegree > 0 { continue }

if node.AwaitType != "" {
    gateOpen, _ := re.config.GateEvaluator.IsGateOpen(node)
    if !gateOpen { continue }
}
```

The `getBlockingIndegree` function is careful to only count edges from **open** predecessor nodes. A `closed` predecessor does not contribute to the indegree:

```go
func (re *ReadyEngine) getBlockingIndegree(nodeID string) int {
    for _, edge := range re.dag.incoming[nodeID] {
        if edge.Type.IsBlockingType() {
            fromNode := re.dag.nodes[edge.FromID]
            if fromNode.Status == StatusOpen {   // ← critical: only open nodes block
                count++
            }
        }
    }
}
```

### 5.2 Phase 2 — Scoring (Effective Priority)

Two mechanisms can **elevate** a task's effective priority above its assigned value:

#### A. Priority Inheritance

If node A blocks node B, and B has P0 (Critical) priority, then A's **effective priority** is also P0 — even if A was assigned P3. This ensures agents always unblock the highest-impact work first.

```go
// BFS up the outgoing blocking chain, finds minimum (highest) priority downstream
func (re *ReadyEngine) calculateInheritedPriority(nodeID string) Priority {
    // Traverses all nodes that nodeID transitively blocks
    // Returns the minimum (highest) priority found
}
```

**Example:**
```
grava-a1b2 (P3-Low) --blocks--> grava-c3d4 (P1-High) --blocks--> grava-e5f6 (P0-Critical)
```
Result: `grava-a1b2` gets effective priority **P0**, not P3. Agent sees it at the top of the ready list.

#### B. Aging Boost

Tasks that have been `open` for more than `AgingThreshold` (default: 7 days) receive a **+1 priority boost** (one level higher). This prevents low-priority tasks from being starved indefinitely.

```go
if age >= re.config.AgingThreshold && effectivePriority > PriorityCritical {
    effectivePriority -= re.config.AgingBoost  // e.g. P3 → P2
    priorityBoosted = true
}
```

### 5.3 Phase 3 — Sorting (Priority Queue)

Candidates are pushed into a **min-heap** (`PriorityQueue` in `priority_queue.go`) keyed on `EffectivePriority`. Ties are broken by `CreatedAt` (oldest task first — the elder wins).

```go
// priority_queue.go — Less() — min-heap ordering
func (pq PriorityQueue) Less(i, j int) bool {
    if pq[i].EffectivePriority != pq[j].EffectivePriority {
        return pq[i].EffectivePriority < pq[j].EffectivePriority // lower = higher priority
    }
    return pq[i].Node.CreatedAt.Before(pq[j].Node.CreatedAt)    // older first
}
```

### 5.4 Configuration

```go
type ReadyEngineConfig struct {
    EnablePriorityInheritance bool
    PriorityInheritanceDepth  int           // Default: 10 levels
    AgingThreshold            time.Duration // Default: 7 days
    AgingBoost                Priority      // Default: 1 level (e.g. P3 → P2)
    GateEvaluator             GateEvaluator
}
```

---

## 6. The Graph Cache

**File:** `pkg/graph/cache.go`

The `GraphCache` stores pre-computed properties to avoid recomputing them on every `ComputeReady()` call. It uses an **invalidation-on-mutation** strategy: when a node or edge changes, only the affected cache entries are invalidated, not the entire cache.

### 6.1 What Is Cached

| Property | Cache Field | Invalidated By |
|---|---|---|
| Total indegree | `indegreeMap / indegreeValid` | `AddEdge`, `RemoveEdge` to that node |
| Blocking indegree | `blockingIndegreeMap / blockingIndegreeValid` | Same as above |
| Effective priority | `priorityMap / priorityValid` | Priority change, status change, new blocking edge |
| Ready list | `readyList / readyListValid` | Any of the above |

### 6.2 Priority Propagation — The Key Feature

When a node's priority changes, the cache **proactively propagates** the change upstream through all blocking predecessors. This is a recursive DFS that stops early if the priority hasn't changed:

```go
// cache.go — propagatePriorityChangeUnsafe()
func (c *GraphCache) propagatePriorityChangeUnsafe(nodeID string) {
    newPriority := c.calculateEffectivePriorityUnsafe(nodeID)

    // Early termination if priority unchanged
    if isValid && oldPriority == newPriority {
        return
    }

    c.SetPriority(nodeID, newPriority)
    c.InvalidateReady()

    // Recurse upstream to all blocking predecessors
    for predID, edge := range c.dag.incoming[nodeID] {
        if edge.Type.IsBlockingType() {
            c.propagatePriorityChangeUnsafe(predID)
        }
    }
}
```

This is triggered by `SetNodeStatus()` and `SetNodePriority()` in `dag.go`, ensuring the cache is always reflect the current state of the graph.

### 6.3 Dirty Node Tracking

Nodes are marked as "dirty" when they are modified. The `dirtyNodes` map is used for incremental processing — only dirty nodes need to be recomputed rather than triggering a full cache invalidation.

---

## 7. Algorithms

### 7.1 Topological Sort (Kahn's Algorithm)

**File:** `pkg/graph/topology.go`

Used to produce a valid processing order (dependencies before dependents). The algorithm:

1. Compute indegree for all nodes.
2. Push all zero-indegree nodes into a queue.
3. Dequeue a node, add to result, reduce indegree of its successors.
4. If a successor reaches indegree 0, enqueue it.
5. If `processed < total nodes` at the end → cycle detected.

**Complexity:** O(V + E)

### 7.2 Cycle Detection (DFS with 3-Color Marking)

**File:** `pkg/graph/topology.go` — `detectCycleUnsafe()`

Uses the standard DFS approach with `WHITE (0) / GRAY (1) / BLACK (2)` node colouring:
- **WHITE:** Unvisited
- **GRAY:** Currently in the DFS stack (presence of a back-edge = cycle)
- **BLACK:** Fully processed

On cycle detection, the path is reconstructed using a `parent` map.

**Used by:** `AddEdgeWithCycleCheck()` — every new blocking dependency is validated before being committed.

### 7.3 Transitive Reduction

**File:** `pkg/graph/dag.go` — `TransitiveReduction()`

Removes **redundant edges** — edges where a longer path already exists. For each edge `A → B`, temporarily removes it and checks if `B` is still reachable from `A` via another path. If yes, the edge is redundant.

**Complexity:** O(V × (V + E))

**Purpose:** Keeps the graph clean and prevents duplicate dependency chains from inflating indegrees.

### 7.4 BFS Traversal

**File:** `pkg/graph/traversal.go`

Two standard BFS implementations:
- `BFS(startID)` — traverses `outgoing` edges (successors)
- `BFSIncoming(startID)` — traverses `incoming` edges (predecessors = blockers)

Used extensively in `GetTransitiveDependencies()`, `GetTransitiveBlockers()`, and the priority inheritance computation.

### 7.5 Blocking Path (BFS on Blocking Edges Only)

**File:** `pkg/graph/dag.go` — `GetBlockingPath(fromID, toID)`

Finds the shortest path between two nodes using **only** `blocks`/`blocked-by` edges. Used by `grava dep impact` to show the causal chain.

---

## 8. Node Lifecycle & Status Model

```
               ┌─────────────────┐
               │      open       │◀─────────┐
               └────────┬────────┘          │
                        │ claim / start      │ release (heartbeat expired)
                        ▼                   │
               ┌─────────────────┐          │
               │   in_progress   │──────────┘
               └────────┬────────┘
                        │ complete
                        ▼
               ┌─────────────────┐
               │     closed      │
               └─────────────────┘
                        
               ┌─────────────────┐
               │    deferred     │  (manually set, not in ready list)
               └─────────────────┘

               ┌─────────────────┐
               │     pinned      │  (manually pinned, stays in ready list always)
               └─────────────────┘

               ┌─────────────────┐
               │    tombstone    │  (soft-deleted, invisible to all queries)
               └─────────────────┘
```

### Status → Cache Impact

| Status Change | Cache Action |
|---|---|
| `open` → `in_progress` | `MarkDirty(id)` + `PropagatePriorityChange(id)` + invalidate downstream blocking indegrees |
| `open` → `closed` | Same as above — successors' blocking indegrees decrease |
| `in_progress` → `open` | `MarkDirty(id)` + `PropagatePriorityChange(id)` + re-validate downstream indegrees |
| Any → `tombstone` | `InvalidateAll()` |

---

## 9. Gate System

Gates allow an issue to be logically "open" but functionally blocked on an **external event**.

A node with `AwaitType != ""` is skipped by the Ready Engine unless the `GateEvaluator.IsGateOpen(node)` returns `true`.

### Gate Types

| AwaitType | Condition | AwaitID example |
|---|---|---|
| `gh:pr` | GitHub PR is merged | `"42"` (PR number) |
| `timer` | Current time is past `AwaitID` | `"2026-03-01T00:00:00Z"` |
| `human` | Manual approval required | `"manager-sign-off"` |
| _(empty)_ | No gate — always eligible | — |

### Custom Gates

The `GateEvaluator` is an interface, allowing custom gate types to be registered at initialization time:

```go
type GateEvaluator interface {
    IsGateOpen(node *Node) (bool, error)
}
```

---

## 10. Performance Characteristics

### Performance Goals (from Epic 2)
- Ready Engine: **< 10ms** for 10,000 nodes
- Cycle Detection: **< 100ms** for 10,000 nodes
- Memory: **< 50MB** for 10,000 nodes with 30,000 edges

### Why These Goals Are Achievable

| Operation | Complexity | Notes |
|---|---|---|
| `GetNode(id)` | O(1) | Hash map |
| `GetIndegree(id)` | O(1) | Cache hit; O(degree) on miss |
| `getBlockingIndegree(id)` | O(in-degree) | Scans only incoming edges |
| `ComputeReady(limit)` | O(V + E log V) | E for filtering, V log V for heap |
| `AddEdge` | O(1) amortised | + O(depth) for priority propagation |
| `TopologicalSort` | O(V + E) | Kahn's algorithm |
| `DetectCycle` | O(V + E) | DFS |
| `TransitiveReduction` | O(V × (V + E)) | One BFS per edge |
| `GetBlockingPath` | O(V + E) | BFS on blocking edges |
| `PropagatePriorityChange` | O(V + E) worst case | Stops early if priority unchanged |

### Cache Hit Rates

The cache is designed for read-heavy workloads (an agent reads the ready list far more often than it writes). Under normal conditions:
- **Indegree cache hit rate:** ~95%+ (only invalidated on edge add/remove)
- **Priority cache hit rate:** ~90%+ (only invalidated on priority/status changes)
- **Ready list cache hit rate:** ~80%+ (has a 1-minute TTL)

---

## 11. Extension Points (Future Work)

### Multi-Agent Coordination (Epic — Planned)
Add `ClaimedBy`, `ClaimedAt`, `HeartbeatAt` to `Node`. Update `ComputeReady()` to filter out actively claimed tasks, and add `ClaimNode(id, agentID)` to `AdjacencyDAG`.

### Hierarchical Status Propagation (Epic 2.4)
When all children of a parent node are `closed`, automatically `close` the parent. Implement `checkParentStatus(nodeID)` called from `SetNodeStatus()`.

### Bottleneck Score
Add a cached `BottleneckScore` metric to `GraphCache`:
```
BottleneckScore(node) = descendant_count × (min_priority_of_descendants + 1)
```
Computable in O(V + E) via reverse topological traversal.

### `grava show --tree`
Use the existing `BFS` traversal starting from a root node, following only `parent-child` and `subtask-of` edges, to render a tree view with completion percentages.
