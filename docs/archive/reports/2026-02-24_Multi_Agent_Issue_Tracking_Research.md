# Multi-Agent Issue Tracking: Data Structures & Beads Analysis

**Date:** 2026-02-24  
**Status:** Research Complete  
**Author:** Research Session  
**Scope:** Data structures for multi-agent coordination, Beads (`steveyegge/beads`) architecture review, and improvement recommendations for Grava.

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Part 1: PageRank Applicability Analysis](#part-1-pagerank-applicability-analysis)
3. [Part 2: Data Structures for Multi-Agent Issue Tracking](#part-2-data-structures-for-multi-agent-issue-tracking)
4. [Part 3: Beads Architecture Deep Dive](#part-3-beads-architecture-deep-dive)
5. [Part 4: Grava vs Beads Comparison](#part-4-grava-vs-beads-comparison)
6. [Part 5: Actionable Recommendations](#part-5-actionable-recommendations)
7. [References](#references)

---

## Executive Summary

This report investigates two questions:

1. **What data structure best supports tracking issues across multiple AI agents?**
2. **What can Grava learn from the Beads issue tracker (`steveyegge/beads`)?**

### Key Findings

- **PageRank is not recommended** for Grava's bottleneck detection. Grava's existing priority inheritance and blocking indegree provide more precise, actionable signals than PageRank's fuzzy importance scores.
- **Claim-based DAG with work-stealing** is the recommended data structure pattern for multi-agent coordination. It extends Grava's existing `AdjacencyDAG` with agent ownership, heartbeats, and auto-release.
- **Beads** (Steve Yegge's distributed graph issue tracker) is the closest comparable system to Grava. It shares the same tech stack (Go + Dolt), similar graph model, and targets the same use case (AI agent memory). Beads has stronger multi-agent primitives; Grava has a stronger graph engine.
- **Four high-priority improvements** are recommended: `grava prime` (context generation), claim semantics, ephemeral issues (wisps), and content-addressable IDs.

---

## Part 1: PageRank Applicability Analysis

### What PageRank Does

PageRank computes a steady-state probability for a random walk on a graph. A node is "important" if many important nodes link to it. It was designed for the web graph where:
- Edges are **homogeneous** (all links are "votes")
- There's no inherent **direction of work**
- Importance must be **discovered emergently** in a massive, sparse graph

### Why PageRank Doesn't Fit Grava

| Property | PageRank (Web Graph) | Grava (Issue DAG) |
|---|---|---|
| Edge semantics | Homogeneous ("link") | 19 heterogeneous types (`blocks`, `waits-for`, `subtask-of`, etc.) |
| Graph type | Cyclic, arbitrary | DAG (acyclic, enforced by Tarjan's SCC) |
| What "important" means | Many important pages link to it | It **blocks** the most other work |
| Scale | Billions of nodes | Hundreds to low thousands |
| Priority source | Emergent from structure | Explicitly assigned (P0–P4) + inherited |

### What Grava Already Has (Better Alternatives)

1. **Priority Inheritance** (`calculateInheritedPriority`) — A node inherits the highest priority of all tasks it transitively blocks. This is a semantically correct version of what PageRank approximates.
2. **Blocking Indegree** (`getBlockingIndegree`) — Only counts edges of blocking types from open nodes. PageRank treats all edges equally.
3. **Impact Analysis** (planned in Epic 2.2, `grava dep impact <id>`) — Shows the downstream "blast radius" with full causal paths, not just a scalar score.

### Alternative: Bottleneck Score

If additional structural analysis is desired, a simple **bottleneck score** outperforms PageRank for DAGs:

```
BottleneckScore(node) = descendant_count × (max_priority_of_descendants + 1)
```

- **Computable in O(V+E)** via reverse topological traversal
- **Directly actionable** — unlike a PageRank probability
- **Composable** with the existing priority system

### When PageRank Would Make Sense

Only if Grava evolves to support:
- Multi-project graphs with cross-team dependencies creating a web (not DAG)
- 10k+ node scale where local metrics miss global structure
- Discovering emergent importance across organizational boundaries

---

## Part 2: Data Structures for Multi-Agent Issue Tracking

### Pattern 1: Claim-Based DAG with Work-Stealing Queue ⭐ Recommended

This pattern extends Grava's existing `AdjacencyDAG` with multi-agent awareness.

#### Architecture

```
┌─────────────────────────────────────────────────────┐
│                   Shared DAG                        │
│                                                     │
│   ┌───────┐ blocks  ┌───────┐ blocks  ┌───────┐    │
│   │ A (P0)│────────▶│ B (P1)│────────▶│ C (P2)│    │
│   │ agent1│         │ FREE  │         │ agent2│    │
│   └───────┘         └───────┘         └───────┘    │
│                                                     │
│   Ready Queue (sorted by EffectivePriority):        │
│   ┌────────────────────────────────────────────┐    │
│   │  B(P0 inherited) → D(P1) → E(P2) → ...    │    │
│   └────────────────────────────────────────────┘    │
│                                                     │
│   Per-Agent Local Deque:                            │
│   Agent1: [A]         Agent2: [C]                   │
│   Agent3: [] ← steals B from Ready Queue            │
└─────────────────────────────────────────────────────┘
```

#### Required Node Extensions

```go
type Node struct {
    // ... existing fields ...
    ClaimedBy   string    `json:"claimed_by,omitempty"`   // Agent ID
    ClaimedAt   time.Time `json:"claimed_at,omitempty"`   // When claimed
    HeartbeatAt time.Time `json:"heartbeat_at,omitempty"` // Liveness proof
}
```

#### How It Works

1. Agents call `grava ready` → returns unclaimed tasks sorted by effective priority.
2. Agent atomically claims a task via `grava update <id> --claim <agent-id>`.
3. If an agent dies (no heartbeat within TTL), the task is auto-released.
4. Idle agents "steal" from the ready queue (highest-priority unclaimed task).

#### Fit with Grava

- ✅ `ReadyEngine.ComputeReady()` already computes the ready queue.
- ✅ `SetNodeStatus()` can be extended with `ClaimedBy`.
- ✅ Priority inheritance ensures agents naturally get the most impactful work first.
- ❌ Missing: claim/release semantics, heartbeat, agent identity.

### Pattern 2: Blackboard Pattern

A shared state space where agents post observations and consume tasks.

```
┌─────────────────────────────────────────┐
│              Blackboard                 │
│                                         │
│  ┌─────────┐  ┌──────────┐  ┌────────┐ │
│  │ DAG     │  │ Claims   │  │ Events │ │
│  │ (graph) │  │ (locks)  │  │ (log)  │ │
│  └─────────┘  └──────────┘  └────────┘ │
│        ▲           ▲            ▲       │
│  ┌─────┴─┐  ┌─────┴──┐  ┌─────┴──┐    │
│  │Agent 1│  │Agent 2 │  │Agent 3 │    │
│  └───────┘  └────────┘  └────────┘    │
└─────────────────────────────────────────┘
```

- **Pro:** Good for highly collaborative agents that need to share observations.
- **Con:** Adds significant complexity (shared memory management, conflict resolution).
- **When to use:** When agents need to communicate findings to each other, not just claim tasks.

### Pattern 3: Coordination Graph (Meta-Graph)

A meta-graph where agents themselves are nodes and edges represent communication channels.

- **Pro:** Enables sophisticated negotiation and capability-based task routing.
- **Con:** Significant complexity overhead. Overkill for <10 agents.
- **When to use:** Large-scale multi-agent systems (50+ agents) with heterogeneous capabilities.

### Recommendation

**Pattern 1 (Claim-Based DAG)** is the clear winner for Grava because:
- It extends, rather than replaces, the existing architecture.
- The `ReadyEngine` already provides the ready queue.
- Dolt provides ACID transactions for atomic claims.
- Low implementation overhead (3 new fields on `Node`, filter logic in `ComputeReady`).

---

## Part 3: Beads Architecture Deep Dive

### Overview

[Beads](https://github.com/steveyegge/beads) (`bd`) is a distributed, Git-backed graph issue tracker for AI agents, created by Steve Yegge. It provides persistent, structured memory for coding agents, replacing unstructured markdown plans with a dependency-aware graph.

**Tech Stack:** Go, Dolt (sole backend since v0.51.0), SQLite (ephemeral store only), MCP server (Python).

### Core Design Principles

| Principle | Implementation |
|---|---|
| **Agent-first** | JSON output (`--json`), atomic claim (`bd update --claim`), context generation (`bd prime`) |
| **Zero conflict** | Content-addressable IDs: `sha256(prefix + title + description)[:6]` |
| **Distributed** | Git-backed, Dolt remotes for federation, branch operations |
| **Ephemeral-aware** | Wisps (temporary issues) separated from version history |
| **Token-efficient** | `BriefIssue` and `BriefDep` models achieve 97% token reduction |

### Storage Architecture

#### DoltStore (Primary)
- SQL operations via `database/sql` with Dolt-specific functions (`DOLT_COMMIT`, `DOLT_MERGE`).
- Version control: `Commit()`, `Push()`, `Pull()`, `Merge()`.
- Federation with encrypted credential storage (AES-GCM).
- **Dual-mode operation:**
  - **Server mode** (recommended): multi-writer via MySQL protocol on port 3307.
  - **Embedded mode**: direct file access via `dolthub/driver` (requires CGO).

#### EphemeralStore (Isolated)
- SQLite for temporary issues that shouldn't clutter version history.
- **Wisps:** Ephemeral work tracking (heartbeats, patrols, recovery). Hash IDs but not in Git.
- **Molecules:** Multi-step workflow definitions. An epic with child tasks = a "molecule".
- TTL-based compaction via `issue.Ephemeral` check.

### Database Schema

```
issues          → id, title, status, priority, type
dependencies    → issue_id, depends_on, type
labels          → issue_id, label
comments        → issue_id, content, created_at
events          → issue_id, event_type, actor, timestamp
config          → key, value
metadata        → key, value
export_hashes   → issue_id, content_hash
blocked_issues_cache → issue_id
```

### Key Features for Multi-Agent

#### 1. `bd prime` — Context Generation
Generates a concise summary (~1-2k tokens) at session start:
- Open issues by priority
- Blocked/unblocked work
- Dependencies and relationships
- Current workflow state

Solves the **"50 First Dates" problem** — agents don't lose context between sessions.

#### 2. `bd update --claim` — Atomic Claim
Sets assignee + `in_progress` status in a single atomic operation. Prevents two agents from working on the same task.

#### 3. Content-Addressable IDs
```
ID = prefix + "-" + sha256(prefix + title + description)[:6]
```
- Prevents merge conflicts across branches/clones.
- Two agents creating the same issue independently get the same ID (idempotent).

#### 4. Wisps (Ephemeral Issues)
- Temporary orchestration entities (heartbeats, patrols, scratch tasks).
- Stored in SQLite, never committed to Dolt history.
- Prevents version history bloat from transient agent activity.

#### 5. 3-Way Merge Driver
- Custom `.gitattributes` merge driver for `issues.jsonl`.
- **Field-level merging**, not line-level — two agents can update different fields of the same issue without conflict.
- Dolt provides **cell-level merge** (individual columns), even more granular.

### AI Agent Integration Paths

| Path | Mechanism |
|---|---|
| **MCP Protocol** | `beads-mcp` server exposes tools: `create_issue`, `update_issue`, `list_issues`, etc. |
| **Claude Code Plugin** | SessionStart hook injects `bd prime` output automatically. |
| **Direct CLI** | `bd <command> --json` for structured output. |

### Synchronization Strategies

| Mode | Description |
|---|---|
| `dolt-native` | Push directly to Dolt remote. No JSONL export. |
| `dolt-in-git` | Export to JSONL for human-readable Git history. |
| `belt-and-suspenders` | Both modes simultaneously. |

---

## Part 4: Grava vs Beads Comparison

### Feature Matrix

| Feature | Grava | Beads | Winner |
|---|---|---|---|
| **Storage** | Dolt (embedded) | Dolt (embedded + server mode) | Beads (server mode enables multi-writer) |
| **Ephemeral Layer** | ❌ None | ✅ SQLite wisps | Beads |
| **Graph Engine** | ✅ Full `AdjacencyDAG` | Implicit DAG (SQL-based) | **Grava** (richer algorithms) |
| **Ready Engine** | ✅ Full (indegree + gates + aging) | ✅ `bd ready` | **Grava** (more features) |
| **Priority System** | ✅ Inheritance + aging + propagation | Basic priority | **Grava** |
| **Cache** | ✅ `GraphCache` (indegree, priority, ready list, dirty tracking) | `blocked_issues_cache` | **Grava** |
| **Multi-Agent** | ❌ Single agent | ✅ Claim + heartbeat | Beads |
| **Context Generation** | ❌ None | ✅ `bd prime` (1-2k tokens) | Beads |
| **Token Efficiency** | ❌ Full serialization | ✅ `BriefIssue` (97% reduction) | Beads |
| **Merge Safety** | Basic | ✅ 3-way field-level merge | Beads |
| **Edge Types** | ✅ 19 semantic types | 6 types | **Grava** |
| **Cycle Detection** | ✅ Tarjan's SCC + Kahn's toposort | Basic | **Grava** |
| **Gate System** | ✅ `gh:pr`, `timer`, `human` | ❌ None | **Grava** |
| **Visualization** | ✅ Mermaid (planned) | ❌ None | **Grava** |
| **Impact Analysis** | ✅ Reverse tree (planned) | ❌ None | **Grava** |
| **ID System** | Sequential/hash-based | Content-addressable (merge-safe) | Beads |
| **Events/Audit** | ❌ None | ✅ `events` table | Beads |

### Summary

**Grava excels at graph mechanics:** richer type system, stronger algorithms, caching, and analytics.  
**Beads excels at multi-agent operations:** claim semantics, context generation, ephemeral storage, merge safety.

The two systems are highly complementary. Grava should adopt Beads' multi-agent primitives while preserving its superior graph engine.

---

## Part 5: Actionable Recommendations

### Priority 1 — Critical (Multi-Agent Foundation)

#### 1.1 `grava prime` Command
**Effort:** Medium  
**Impact:** High  

Generate a concise context dump (~1-2k tokens) for agent session start:

```
# Example output
## Ready Work (3 tasks)
- [grava-abc123] P0: Fix authentication bug (age: 3d)
- [grava-def456] P1: Add rate limiting (age: 1d, priority-boosted)
- [grava-ghi789] P2: Update README

## Blocked (2 tasks)
- [grava-jkl012] ← blocked by [grava-abc123]
- [grava-mno345] ← blocked by [grava-def456], [grava-ghi789]

## In Progress (1 task, agent: agent-1)
- [grava-pqr678] P1: Implement caching
```

#### 1.2 Claim Semantics
**Effort:** Low  
**Impact:** High  

Add `ClaimedBy`, `ClaimedAt`, `HeartbeatAt` to `Node`. Extend `ReadyEngine.ComputeReady()`:

```go
// Filter out actively claimed tasks
if node.ClaimedBy != "" && time.Since(node.HeartbeatAt) < claimTTL {
    continue
}
```

Add `grava update <id> --claim <agent-id>` for atomic claim.

### Priority 2 — Medium (Operational Robustness)

#### 2.1 Ephemeral Issues (Wisps)
**Effort:** Medium  
**Impact:** Medium  

Separate SQLite store for transient tasks (heartbeats, scratch plans, patrol tasks). Prevents Dolt history bloat.

#### 2.2 Token-Optimized Serialization (`BriefNode`)
**Effort:** Low  
**Impact:** Medium  

```go
type BriefNode struct {
    ID       string      `json:"id"`
    Title    string      `json:"t"`
    Status   IssueStatus `json:"s"`
    Priority Priority    `json:"p"`
    BlockedBy []string   `json:"bb,omitempty"`
}
```

#### 2.3 Content-Addressable IDs
**Effort:** Low  
**Impact:** Medium  

```go
func GenerateID(prefix, title, description string) string {
    hash := sha256.Sum256([]byte(prefix + title + description))
    return prefix + "-" + hex.EncodeToString(hash[:])[:6]
}
```

Prevents merge conflicts when multiple agents create issues simultaneously.

#### 2.4 Events/Audit Trail
**Effort:** Low  
**Impact:** Medium  

Add an `events` table to track all state changes:

```sql
CREATE TABLE events (
    id INTEGER PRIMARY KEY,
    issue_id TEXT NOT NULL,
    event_type TEXT NOT NULL,  -- 'created', 'claimed', 'status_changed', 'priority_changed'
    actor TEXT NOT NULL,        -- Agent ID or 'human'
    old_value TEXT,
    new_value TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Priority 3 — Later (Scale & Robustness)

#### 3.1 Dolt Server Mode
Enable multi-writer concurrent access via MySQL protocol. Required when running 3+ agents simultaneously.

#### 3.2 3-Way Field-Level Merge Driver
Custom `.gitattributes` merge driver for issue data. Prevents conflicts when two agents modify different fields of the same issue.

#### 3.3 Bottleneck Score
Add a cached bottleneck metric to `GraphCache`:

```go
func (c *GraphCache) GetBottleneckScore(nodeID string) float64 {
    descendantCount := c.getDescendantCount(nodeID)
    maxDescPriority := c.getMaxDescendantPriority(nodeID)
    return float64(descendantCount) * float64(maxDescPriority + 1)
}
```

---

## References

### Beads Project
- **Repository:** https://github.com/steveyegge/beads
- **DeepWiki:** https://deepwiki.com/steveyegge/beads
- **Key Source Files:**
  - Storage: `internal/storage/dolt/store.go`
  - Ephemeral: `internal/storage/ephemeral/store.go`
  - Prime Command: `cmd/bd/prime.go`
  - Ready Command: `cmd/bd/ready.go`
  - ID Generation: `internal/storage/hash_ids.go`
  - Merge Driver: `cmd/bd/merge.go`
  - MCP Server: `integrations/beads-mcp/src/beads_mcp/server.py`

### Grava Source Files Referenced
- `pkg/graph/types.go` — Node, Edge, DependencyType definitions
- `pkg/graph/dag.go` — AdjacencyDAG implementation
- `pkg/graph/ready_engine.go` — ReadyEngine with priority inheritance
- `pkg/graph/cache.go` — GraphCache with priority propagation
- `pkg/graph/topology.go` — Topological sort and cycle detection
- `pkg/graph/priority_queue.go` — Heap-based priority queue
- `pkg/graph/traversal.go` — BFS/DFS traversal

### Research Sources
- Multi-agent coordination patterns: Blackboard, Pub/Sub, Auction-based allocation
- Work-stealing queues: Cilk, Go runtime scheduler, Java ForkJoinPool
- Coordination graphs: Sparse cooperative Q-learning (Guestrin et al., CMU)
- DAG scheduling: Token-based decentralized scheduling with dependency awareness

### Related Grava Epics
- `docs/epics/Epic_2_Graph_Mechanics.md` — Graph engine foundation
- `docs/epics/Epic_2.2_Graph_Lifecycle_and_Propagation.md` — Priority propagation and visualization
