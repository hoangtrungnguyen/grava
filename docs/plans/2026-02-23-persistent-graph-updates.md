# Persistent Graph Updates Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Bridge the `AdjacencyDAG` with the `dolt.Store` to ensure status and priority updates are persisted and audited.

**Architecture:** Add `store`, `actor`, and `agent_model` fields to `AdjacencyDAG`. Update `SetNodeStatus` and `SetNodePriority` to perform SQL updates and log audit events before updating the in-memory state and cache.

**Tech Stack:** Go (Golang), Dolt (MySQL compatible), `github.com/DATA-DOG/go-sqlmock`.

---

### Task 1: Enhance AdjacencyDAG Struct and Interface

**Files:**
- Modify: `pkg/graph/graph.go`
- Modify: `pkg/graph/dag.go`

**Step 1: Update DAG interface in graph.go**
Add `SetSession(actor, model string)` to the interface.

**Step 2: Update AdjacencyDAG struct and NewAdjacencyDAG**
Add `store dolt.Store`, `actor string`, and `agentModel string` to the struct.

**Step 3: Implement SetSession method**
```go
func (g *AdjacencyDAG) SetSession(actor, model string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.actor = actor
	g.agentModel = model
}
```

**Step 4: Update LoadGraphFromDB to pass store**
Modify `pkg/graph/loader.go` to set the store in the newly created DAG.

**Step 5: Commit**
```bash
git add pkg/graph/graph.go pkg/graph/dag.go pkg/graph/loader.go
git commit -m "feat(graph): add store and session metadata to AdjacencyDAG"
```

---

### Task 2: Implement Persistent SetNodeStatus

**Files:**
- Modify: `pkg/graph/dag.go`
- Test: `pkg/graph/persistence_test.go` (Create)

**Step 1: Write a failing test for persistent status update**
```go
func TestSetNodeStatus_Persistence(t *testing.T) {
	db, mock, _ := sqlmock.New()
	store := dolt.NewClientFromDB(db)
	dag := NewAdjacencyDAG(true)
	dag.store = store
	dag.SetSession("test-actor", "test-model")
	
	dag.AddNode(&Node{ID: "A", Status: StatusOpen})
	
	mock.ExpectExec(regexp.QuoteMeta("UPDATE issues SET status = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?")).
		WithArgs(string(StatusClosed), sqlmock.AnyArg(), "test-actor", "test-model", "A").
		WillReturnResult(sqlmock.NewResult(1, 1))
	
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := dag.SetNodeStatus("A", StatusClosed)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
```

**Step 2: Implement persistent logic in SetNodeStatus**
```go
func (g *AdjacencyDAG) SetNodeStatus(id string, status IssueStatus) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, exists := g.nodes[id]
	if !exists { return ErrNodeNotFound }
	if node.Status == status { return nil }

	if g.store != nil {
		actor := g.actor
		if actor == "" { actor = "unknown" }
		
		query := "UPDATE issues SET status = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?"
		_, err := g.store.Exec(query, string(status), time.Now(), actor, g.agentModel, id)
		if err != nil { return err }
		
		_ = g.store.LogEvent(id, "status_change", actor, g.agentModel, node.Status, status)
	}

	node.Status = status
	// ... existing cache invalidation ...
	return nil
}
```

**Step 3: Run tests**
Run: `go test ./pkg/graph/...`

**Step 4: Commit**
```bash
git add pkg/graph/dag.go pkg/graph/persistence_test.go
git commit -m "feat(graph): implement persistent SetNodeStatus"
```

---

### Task 3: Implement Persistent SetNodePriority

**Files:**
- Modify: `pkg/graph/dag.go`
- Modify: `pkg/graph/persistence_test.go`

**Step 1: Write a failing test for persistent priority update**
Verify SQL update for priority.

**Step 2: Implement persistent logic in SetNodePriority**
Similar pattern to Task 2.

**Step 3: Run tests**
Run: `go test ./pkg/graph/...`

**Step 4: Commit**
```bash
git add pkg/graph/dag.go pkg/graph/persistence_test.go
git commit -m "feat(graph): implement persistent SetNodePriority"
```

---

### Task 4: Verify Cache Invalidation after Persistent Update

**Files:**
- Modify: `pkg/graph/persistence_test.go`

**Step 1: Test Cache vs DB**
Ensure that `ComputeReady` (using cache) correctly reflects a status change performed via `SetNodeStatus`.

**Step 2: Test Priority Inheritance Invalidation**
Ensure `PropagatePriorityChange` is called after the DB update.

**Step 3: Run all tests**
Run: `go test ./pkg/graph/...`

**Step 4: Commit**
```bash
git add pkg/graph/persistence_test.go
git commit -m "test(graph): verify cache consistency after persistent updates"
```
