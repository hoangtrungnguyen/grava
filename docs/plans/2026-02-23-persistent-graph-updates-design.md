# Design Doc: Persistent Graph Updates and Audit Logging

**Date:** 2026-02-23
**Status:** Approved
**Issue:** [grava-0637.6](grava-0637.6)

## Overview
Currently, the `pkg/graph` data structure is purely in-memory. While it handles complex logic like priority inheritance and cache invalidation, it is disconnected from the underlying Dolt database during updates. This design bridges that gap by making the `AdjacencyDAG` "store-aware," ensuring that every status or priority update is persisted and audited.

## Goals
1.  **Atomicity (Soft)**: Ensure DB update and memory update are consistent (DB first).
2.  **Persistence**: `SetNodeStatus` and `SetNodePriority` must update the `issues` table.
3.  **Auditing**: Every update must trigger a `LogEvent` entry in the `events` table.
4.  **Cache Integrity**: Ensure `GraphCache` is correctly invalidated or updated after a persistent change.

## Architecture

### 1. Store Integration
The `AdjacencyDAG` struct will be updated to include an optional `dolt.Store` and session metadata (`actor`, `agentModel`).

```go
type AdjacencyDAG struct {
    mu sync.RWMutex
    store dolt.Store
    actor string
    model string
    // ... existing fields ...
}
```

### 2. Method Signatures
The `DAG` interface in `pkg/graph/graph.go` will be enhanced:

```go
type DAG interface {
    Graph
    // ...
    SetNodeStatus(id string, status IssueStatus) error
    SetNodePriority(id string, priority Priority) error
    SetSession(actor, model string) // New: Binds metadata for auditing
}
```

### 3. Update Flow (e.g., SetNodeStatus)
1.  **Locking**: Acquire `g.mu.Lock()`.
2.  **Validation**: Verify node existence and check if the value actually changed.
3.  **Persistence**:
    - If `g.store != nil`:
        - Execute `UPDATE issues SET status = ?, updated_at = ?, updated_by = ? WHERE id = ?`.
        - Call `g.store.LogEvent(...)` to record the change.
4.  **Memory Sync**: Update `node.Status` and `node.UpdatedAt`.
5.  **Cache Invalidation**: Call `g.cache.MarkDirty(id)` and propagate changes (e.g., `PropagatePriorityChange`).
6.  **Unlock**: Release `g.mu.Unlock()`.

## Error Handling
- If the DB update fails, the method returns an error and the in-memory state is **not** updated.
- Since the DB is local, failures are expected to be rare (e.g., disk full, schema mismatch).

## Testing Strategy
1.  **Unit Tests (Mocked)**: Use `sqlmock` to verify that `SetNodeStatus` calls the correct SQL and `LogEvent`.
2.  **Cache Integrity Tests**: Verify that `ComputeReady` (which uses the cache) reflects changes immediately after `SetNodeStatus`.
3.  **Integration Tests**: Use `scripts/setup_test_env.sh` to perform a real end-to-end update and verify DB state.
