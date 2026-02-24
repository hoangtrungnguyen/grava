package graph

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
)

func TestSetNodeStatus_Persistence(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	store := dolt.NewClientFromDB(db)
	dag := NewAdjacencyDAG(true)
	dag.store = store
	dag.SetSession("test-actor", "test-model")

	dag.AddNode(&Node{ID: "grava-1", Status: StatusOpen})

	// Expect UPDATE
	mock.ExpectExec(regexp.QuoteMeta("UPDATE issues SET status = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?")).
		WithArgs(string(StatusClosed), sqlmock.AnyArg(), "test-actor", "test-model", "grava-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Expect LogEvent (Audit)
	// LogEvent uses INSERT INTO events
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).
		WithArgs("grava-1", "status_change", "test-actor", "\"open\"", "\"closed\"", "test-actor", "test-actor", "test-model", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = dag.SetNodeStatus("grava-1", StatusClosed)
	assert.NoError(t, err)

	node, _ := dag.GetNode("grava-1")
	assert.Equal(t, StatusClosed, node.Status)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetNodePriority_Persistence(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	store := dolt.NewClientFromDB(db)
	dag := NewAdjacencyDAG(true)
	dag.store = store
	dag.SetSession("test-actor", "test-model")

	dag.AddNode(&Node{ID: "grava-1", Priority: PriorityMedium})

	// Expect UPDATE
	mock.ExpectExec(regexp.QuoteMeta("UPDATE issues SET priority = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?")).
		WithArgs(int(PriorityHigh), sqlmock.AnyArg(), "test-actor", "test-model", "grava-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Expect LogEvent (Audit)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).
		WithArgs("grava-1", "priority_change", "test-actor", "2", "1", "test-actor", "test-actor", "test-model", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = dag.SetNodePriority("grava-1", PriorityHigh)
	assert.NoError(t, err)

	node, _ := dag.GetNode("grava-1")
	assert.Equal(t, PriorityHigh, node.Priority)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_CacheConsistency(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	store := dolt.NewClientFromDB(db)
	dag := NewAdjacencyDAG(true) // Enable cache
	dag.store = store
	dag.SetSession("test-actor", "test-model")

	dag.AddNode(&Node{ID: "A", Status: StatusOpen, Priority: PriorityMedium, CreatedAt: time.Now()})
	dag.AddNode(&Node{ID: "B", Status: StatusOpen, Priority: PriorityHigh, CreatedAt: time.Now()})
	dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})

	re := NewReadyEngine(dag, nil)

	// Initially, only A is ready (B is blocked by A)
	ready, _ := re.ComputeReady(0)
	assert.Len(t, ready, 1)
	assert.Equal(t, "A", ready[0].Node.ID)

	// Now close A. DB update expected.
	mock.ExpectExec(regexp.QuoteMeta("UPDATE issues SET status")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).WillReturnResult(sqlmock.NewResult(1, 1))

	err = dag.SetNodeStatus("A", StatusClosed)
	assert.NoError(t, err)

	// ComputeReady should now show B as ready because A is closed.
	// This verifies that SetNodeStatus correctly invalidated the cache.
	ready, _ = re.ComputeReady(0)
	assert.Len(t, ready, 1)
	assert.Equal(t, "B", ready[0].Node.ID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRemoveNode_Persistence(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	store := dolt.NewClientFromDB(db)
	dag := NewAdjacencyDAG(true)
	dag.store = store
	dag.SetSession("test-actor", "test-model")

	dag.AddNode(&Node{ID: "grava-1", Status: StatusOpen})

	// Expect INSERT INTO deletions
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO deletions")).
		WithArgs("grava-1", "remove_node", "test-actor", "test-actor", "test-actor", "test-model").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Expect tombstone UPDATE
	mock.ExpectExec(regexp.QuoteMeta("UPDATE issues SET status = 'tombstone'")).
		WithArgs(sqlmock.AnyArg(), "test-actor", "test-model", "grava-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Expect DELETE dependencies
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM dependencies WHERE from_id = ? OR to_id = ?")).
		WithArgs("grava-1", "grava-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Expect audit log
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = dag.RemoveNode("grava-1")
	assert.NoError(t, err)
	assert.False(t, dag.HasNode("grava-1"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRemoveEdge_Persistence(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	store := dolt.NewClientFromDB(db)
	dag := NewAdjacencyDAG(true)
	dag.store = store
	dag.SetSession("test-actor", "test-model")

	dag.AddNode(&Node{ID: "A", Status: StatusOpen})
	dag.AddNode(&Node{ID: "B", Status: StatusOpen})
	dag.AddEdge(&Edge{FromID: "A", ToID: "B", Type: DependencyBlocks})

	// Expect DELETE FROM dependencies with type filter
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM dependencies WHERE from_id = ? AND to_id = ? AND type = ?")).
		WithArgs("A", "B", "blocks").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect audit log
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = dag.RemoveEdge("A", "B", DependencyBlocks)
	assert.NoError(t, err)
	assert.Equal(t, 0, dag.EdgeCount())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStatusBubbling_Persistence(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	store := dolt.NewClientFromDB(db)
	dag := NewAdjacencyDAG(true)
	dag.store = store
	dag.SetSession("test-actor", "test-model")

	// Setup hierarchy: Parent -> Child 1, Parent -> Child 2
	dag.AddNode(&Node{ID: "parent", Status: StatusOpen})
	dag.AddNode(&Node{ID: "child1", Status: StatusOpen})
	dag.AddNode(&Node{ID: "child2", Status: StatusOpen})
	// child1 --subtask-of--> parent
	dag.AddEdge(&Edge{FromID: "child1", ToID: "parent", Type: DependencySubtaskOf})
	// child2 --subtask-of--> parent
	dag.AddEdge(&Edge{FromID: "child2", ToID: "parent", Type: DependencySubtaskOf})

	// 1. Mark child1 as In Progress -> Parent should become In Progress
	mock.ExpectExec(regexp.QuoteMeta("UPDATE issues SET status = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?")).
		WithArgs(string(StatusInProgress), sqlmock.AnyArg(), "test-actor", "test-model", "child1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).
		WithArgs("child1", "status_change", "test-actor", "\"open\"", "\"in_progress\"", "test-actor", "test-actor", "test-model", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Automated update for parent (actor='system')
	mock.ExpectExec(regexp.QuoteMeta("UPDATE issues SET status = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?")).
		WithArgs(string(StatusInProgress), sqlmock.AnyArg(), "system", "test-model", "parent").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).
		WithArgs("parent", "status_change", "system", "\"open\"", "\"in_progress\"", "system", "system", "test-model", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = dag.SetNodeStatus("child1", StatusInProgress)
	assert.NoError(t, err)

	pNode, _ := dag.GetNode("parent")
	assert.Equal(t, StatusInProgress, pNode.Status)

	// 2. Mark child1 closed, child2 still open -> Parent stays In Progress
	mock.ExpectExec(regexp.QuoteMeta("UPDATE issues SET status")).
		WithArgs(string(StatusClosed), sqlmock.AnyArg(), "test-actor", "test-model", "child1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).
		WithArgs("child1", "status_change", "test-actor", "\"in_progress\"", "\"closed\"", "test-actor", "test-actor", "test-model", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = dag.SetNodeStatus("child1", StatusClosed)
	assert.NoError(t, err)

	pNode, _ = dag.GetNode("parent")
	assert.Equal(t, StatusInProgress, pNode.Status)

	// 3. Mark child2 closed -> Parent becomes Closed
	mock.ExpectExec(regexp.QuoteMeta("UPDATE issues SET status")).
		WithArgs(string(StatusClosed), sqlmock.AnyArg(), "test-actor", "test-model", "child2").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).
		WithArgs("child2", "status_change", "test-actor", "\"open\"", "\"closed\"", "test-actor", "test-actor", "test-model", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Automated update for parent to Closed
	mock.ExpectExec(regexp.QuoteMeta("UPDATE issues SET status")).
		WithArgs(string(StatusClosed), sqlmock.AnyArg(), "system", "test-model", "parent").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).
		WithArgs("parent", "status_change", "system", "\"in_progress\"", "\"closed\"", "system", "system", "test-model", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = dag.SetNodeStatus("child2", StatusClosed)
	assert.NoError(t, err)

	pNode, _ = dag.GetNode("parent")
	assert.Equal(t, StatusClosed, pNode.Status)

	assert.NoError(t, mock.ExpectationsWereMet())
}
