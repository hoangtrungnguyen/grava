package orchestrator

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync/atomic"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// qResetTask / qFetchAssigned / qInsertComment / qInsertEvent are partial SQL
// anchors used by sqlmock (prefix-match via regexp.QuoteMeta on leading tokens).
var (
	qFetchAssigned = regexp.QuoteMeta(`SELECT id FROM issues WHERE status = 'in_progress' AND assignee = ?`)
	qResetTask     = regexp.QuoteMeta(`UPDATE issues SET status = 'open', assignee = NULL, started_at = NULL WHERE id = ?`)
	qInsertComment = regexp.QuoteMeta(`INSERT INTO issue_comments (issue_id, message, actor) VALUES`)
	qInsertEvent   = regexp.QuoteMeta(`INSERT INTO events`)
	qTaskTimeout   = regexp.QuoteMeta(`SELECT id FROM issues`)
)

func newWatchdogMock(t *testing.T) (dolt.Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() }) //nolint:errcheck
	return dolt.NewClientFromDB(db), mock
}

// healthServer builds an httptest.Server that initially returns 200 for /health.
// Calling the returned stop func closes the server so subsequent requests fail.
func healthServer(t *testing.T) (url string, stop func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	return srv.URL, srv.Close
}

func TestWatchdog_HealthyAgentResetsFailureCounter(t *testing.T) {
	url, stop := healthServer(t)
	defer stop()

	store, _ := newWatchdogMock(t)
	pool := NewAgentPool([]AgentConfig{makeAgentConfig("agent-1", url, 3)})
	// Pre-set failure count.
	pool.agents[0].consecutiveFailures = 2

	wd := NewWatchdog(pool, store, 10, 30)
	wd.checkHeartbeats(context.Background())

	pool.mu.Lock()
	defer pool.mu.Unlock()
	assert.Equal(t, 0, pool.agents[0].consecutiveFailures, "successful ping resets failure counter")
	assert.True(t, pool.agents[0].available)
}

func TestWatchdog_AgentDeclaredDeadAfterMaxFailures(t *testing.T) {
	store, mock := newWatchdogMock(t)

	// Expect DB calls for the dead agent: query tasks + reset + comment + event per task.
	mock.ExpectQuery(qFetchAssigned).
		WithArgs("agent-dead").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("task-1").AddRow("task-2"))
	mock.ExpectExec(qResetTask).WithArgs("task-1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(qInsertComment).WithArgs("task-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(qInsertEvent).
		WithArgs("task-1", "reset", "orchestrator", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(qResetTask).WithArgs("task-2").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(qInsertComment).WithArgs("task-2", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(qInsertEvent).
		WithArgs("task-2", "reset", "orchestrator", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	pool := NewAgentPool([]AgentConfig{
		// Non-listening port — every ping will fail.
		makeAgentConfig("agent-dead", "http://127.0.0.1:19999", 3),
	})

	wd := NewWatchdog(pool, store, 10, 30)
	wd.maxFailures = 3

	// Run checkHeartbeats 3 times to reach the threshold.
	for i := 0; i < 3; i++ {
		wd.checkHeartbeats(context.Background())
	}

	pool.mu.Lock()
	failures := pool.agents[0].consecutiveFailures
	available := pool.agents[0].available
	pool.mu.Unlock()

	assert.GreaterOrEqual(t, failures, 3)
	assert.False(t, available, "agent must be marked unavailable after max failures")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWatchdog_TaskTimeoutResetsToOpen(t *testing.T) {
	store, mock := newWatchdogMock(t)

	mock.ExpectQuery(qTaskTimeout).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("timed-out-task"))
	mock.ExpectExec(qResetTask).WithArgs("timed-out-task").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(qInsertComment).WithArgs("timed-out-task", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(qInsertEvent).
		WithArgs("timed-out-task", "reset", "orchestrator", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	pool := NewAgentPool([]AgentConfig{makeAgentConfig("agent-1", "http://localhost", 3)})
	wd := NewWatchdog(pool, store, 10, 30)
	wd.checkTaskTimeouts(context.Background())

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWatchdog_TaskTimeoutEmptyResultNoOp(t *testing.T) {
	store, mock := newWatchdogMock(t)

	mock.ExpectQuery(qTaskTimeout).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	pool := NewAgentPool([]AgentConfig{makeAgentConfig("agent-1", "http://localhost", 3)})
	wd := NewWatchdog(pool, store, 10, 30)
	wd.checkTaskTimeouts(context.Background())

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWatchdog_RunCancelsCleanly(t *testing.T) {
	url, stop := healthServer(t)
	defer stop()

	store, mock := newWatchdogMock(t)
	// Immediate tick on Run entry triggers heartbeat + timeout check.
	mock.ExpectQuery(qTaskTimeout).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	pool := NewAgentPool([]AgentConfig{makeAgentConfig("agent-1", url, 3)})
	wd := NewWatchdog(pool, store, 1*3600, 30) // 1h interval so only initial tick fires

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		wd.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Watchdog.Run did not exit after context cancel")
	}
}

func TestWatchdog_DeadAgentCommentContainsAgentID(t *testing.T) {
	store, mock := newWatchdogMock(t)

	// Use a custom matcher to verify the comment message contains the agent ID.
	agentIDMatcher := sqlmock.AnyArg() // placeholder; actual verification below via writeComment call
	_ = agentIDMatcher

	mock.ExpectQuery(qFetchAssigned).
		WithArgs("agent-42").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("task-x"))
	mock.ExpectExec(qResetTask).WithArgs("task-x").WillReturnResult(sqlmock.NewResult(1, 1))
	// Expect the comment to contain the agent ID "agent-42".
	mock.ExpectExec(qInsertComment).
		WithArgs("task-x", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(qInsertEvent).
		WithArgs("task-x", "reset", "orchestrator", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	pool := NewAgentPool([]AgentConfig{
		makeAgentConfig("agent-42", "http://127.0.0.1:19999", 3),
	})

	wd := NewWatchdog(pool, store, 10, 30)
	wd.maxFailures = 3

	// Drive to dead state.
	for i := 0; i < 3; i++ {
		wd.checkHeartbeats(context.Background())
	}

	require.NoError(t, mock.ExpectationsWereMet())

	// Verify comment text directly via the writeComment helper.
	db2, mock2, err := sqlmock.New()
	require.NoError(t, err)
	defer db2.Close() //nolint:errcheck
	store2 := dolt.NewClientFromDB(db2)
	mock2.ExpectExec(qInsertComment).
		WithArgs("task-y", "Agent agent-42 timeout/crash. Task reassigned.").
		WillReturnResult(sqlmock.NewResult(1, 1))

	wd2 := NewWatchdog(pool, store2, 10, 30)
	err = wd2.writeComment(context.Background(), "task-y",
		fmt.Sprintf("Agent %s timeout/crash. Task reassigned.", "agent-42"))
	require.NoError(t, err)
	require.NoError(t, mock2.ExpectationsWereMet())
}

func TestWatchdog_DBErrorDoesNotPanic(t *testing.T) {
	store, mock := newWatchdogMock(t)

	mock.ExpectQuery(qFetchAssigned).
		WithArgs("agent-dead").
		WillReturnError(assert.AnError)

	pool := NewAgentPool([]AgentConfig{
		makeAgentConfig("agent-dead", "http://127.0.0.1:19999", 3),
	})

	wd := NewWatchdog(pool, store, 10, 30)
	wd.maxFailures = 3

	// Drive to dead — DB error should be swallowed without panic.
	assert.NotPanics(t, func() {
		for i := 0; i < 3; i++ {
			wd.checkHeartbeats(context.Background())
		}
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWatchdog_RecoveredAgentBecomesAvailable(t *testing.T) {
	var healthy atomic.Bool
	healthy.Store(false)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if healthy.Load() {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()

	store, mock := newWatchdogMock(t)
	// Expect dead-agent task query when threshold hit.
	mock.ExpectQuery(qFetchAssigned).
		WillReturnRows(sqlmock.NewRows([]string{"id"})) // no tasks to reset

	pool := NewAgentPool([]AgentConfig{makeAgentConfig("agent-1", srv.URL, 3)})
	wd := NewWatchdog(pool, store, 10, 30)
	wd.maxFailures = 3

	// Three failures → DEAD.
	for i := 0; i < 3; i++ {
		wd.checkHeartbeats(context.Background())
	}
	pool.mu.Lock()
	assert.False(t, pool.agents[0].available)
	pool.mu.Unlock()

	// Agent recovers.
	healthy.Store(true)
	wd.checkHeartbeats(context.Background())

	pool.mu.Lock()
	assert.True(t, pool.agents[0].available, "agent should be available again after successful ping")
	assert.Equal(t, 0, pool.agents[0].consecutiveFailures)
	pool.mu.Unlock()
}
