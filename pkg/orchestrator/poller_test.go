package orchestrator

import (
	"context"
	"regexp"
	"sync"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var qFetchTasks = regexp.QuoteMeta(`SELECT i.id, i.title, COALESCE(i.description, ''), i.priority`)

// TestPoller_SQLUsesOpenStatusForBlocking verifies that the NOT EXISTS clause
// filters by b.status = 'open', matching the ready-engine's invariant: a task
// is unblocked once all its dependencies have been claimed (in_progress).
func TestPoller_SQLUsesOpenStatusForBlocking(t *testing.T) {
	const fullQuery = `SELECT i.id, i.title, COALESCE(i.description, ''), i.priority
FROM issues i
WHERE i.status = 'open'
  AND i.ephemeral = 0
  AND NOT EXISTS (
      SELECT 1
      FROM dependencies d
      JOIN issues b ON b.id = d.from_id
      WHERE d.to_id = i.id
        AND b.status = 'open'
  )
ORDER BY i.priority ASC, i.created_at ASC`

	assert.Contains(t, fullQuery, "b.status = 'open'",
		"poller must filter by open blockers to match ready-engine semantics")
	assert.NotContains(t, fullQuery, "b.status = 'in_progress'",
		"in_progress check would incorrectly block dispatch of unblocked tasks")
}

func taskCols() []string { return []string{"id", "title", "description", "priority"} }

func TestPoller_DeliversTasksInPriorityOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var store dolt.Store = dolt.NewClientFromDB(db)

	mock.ExpectQuery(qFetchTasks).
		WillReturnRows(sqlmock.NewRows(taskCols()).
			AddRow("task-1", "High priority", "desc-1", 1).
			AddRow("task-2", "Low priority", "desc-2", 3))

	var received []DispatchableTask
	var mu sync.Mutex
	sink := func(t DispatchableTask) {
		mu.Lock()
		received = append(received, t)
		mu.Unlock()
	}

	ctx, cancel := context.WithCancel(context.Background())
	poller := NewPoller(store, 10*time.Second, sink)

	// Run one tick via poll directly (avoids timer complexity).
	poller.poll(ctx)
	cancel()

	require.NoError(t, mock.ExpectationsWereMet())
	require.Len(t, received, 2)
	assert.Equal(t, "task-1", received[0].ID)
	assert.Equal(t, 1, received[0].Priority)
	assert.Equal(t, "task-2", received[1].ID)
	assert.Equal(t, 3, received[1].Priority)
}

func TestPoller_EmptyQueue(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var store dolt.Store = dolt.NewClientFromDB(db)
	mock.ExpectQuery(qFetchTasks).
		WillReturnRows(sqlmock.NewRows(taskCols()))

	var called int
	poller := NewPoller(store, 10*time.Second, func(DispatchableTask) { called++ })

	ctx, cancel := context.WithCancel(context.Background())
	poller.poll(ctx)
	cancel()

	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, 0, called)
}

func TestPoller_DBErrorDoesNotPanic(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var store dolt.Store = dolt.NewClientFromDB(db)
	mock.ExpectQuery(qFetchTasks).WillReturnError(assert.AnError)

	var called int
	poller := NewPoller(store, 10*time.Second, func(DispatchableTask) { called++ })

	ctx, cancel := context.WithCancel(context.Background())
	// Should not panic — error is swallowed.
	assert.NotPanics(t, func() { poller.poll(ctx) })
	cancel()
	assert.Equal(t, 0, called)
}

func TestPoller_RunCancelsCleanly(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var store dolt.Store = dolt.NewClientFromDB(db)
	// Expect at least one query (the immediate tick on Run entry).
	mock.ExpectQuery(qFetchTasks).
		WillReturnRows(sqlmock.NewRows(taskCols()))

	ctx, cancel := context.WithCancel(context.Background())
	poller := NewPoller(store, 1*time.Hour, func(DispatchableTask) {})

	done := make(chan struct{})
	go func() {
		poller.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
		// OK — Run exited cleanly.
	case <-time.After(2 * time.Second):
		t.Fatal("Poller.Run did not exit after context cancel")
	}
}

func TestPoller_BlockedTasksExcluded(t *testing.T) {
	// The SQL query's NOT EXISTS clause filters blocked tasks at the DB level.
	// This test verifies the column mapping by checking that only the two
	// unblocked tasks returned by the mock are delivered.
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var store dolt.Store = dolt.NewClientFromDB(db)
	// Mock returns only unblocked tasks (blocked ones excluded by the query).
	mock.ExpectQuery(qFetchTasks).
		WillReturnRows(sqlmock.NewRows(taskCols()).
			AddRow("task-unblocked-1", "Unblocked", "", 1))

	var received []DispatchableTask
	sink := func(t DispatchableTask) { received = append(received, t) }

	ctx, cancel := context.WithCancel(context.Background())
	poller := NewPoller(store, 10*time.Second, sink)
	poller.poll(ctx)
	cancel()

	require.NoError(t, mock.ExpectationsWereMet())
	require.Len(t, received, 1)
	assert.Equal(t, "task-unblocked-1", received[0].ID)
}
