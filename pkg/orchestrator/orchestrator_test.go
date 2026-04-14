package orchestrator

import (
	"context"
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

var (
	qClaimTask = regexp.QuoteMeta(`UPDATE issues
	SET status = 'in_progress', assignee = ?, started_at = NOW()
	WHERE id = ? AND status = 'open'`)
	qResetTaskOrc    = regexp.QuoteMeta(`UPDATE issues SET status = 'open', assignee = NULL, started_at = NULL WHERE id = ?`)
	qWriteCommentOrc = `INSERT INTO issue_comments`
	qWriteEventOrc   = `INSERT INTO events`
)

func newOrcMock(t *testing.T) (dolt.Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() }) //nolint:errcheck
	return dolt.NewClientFromDB(db), mock
}

// TestOrchestrator_ClaimFirst_SuccessPath verifies that sink() claims the task
// in the DB before dispatching to the agent (claim-first ordering).
func TestOrchestrator_ClaimFirst_SuccessPath(t *testing.T) {
	store, mock := newOrcMock(t)

	var dispatched atomic.Bool
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/task" {
			dispatched.Store(true)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(agent.Close)

	// Expect claim UPDATE then event INSERT (success path — no reset).
	mock.ExpectExec(qClaimTask).
		WithArgs("agent-1", "task-1").
		WillReturnResult(sqlmock.NewResult(1, 1)) // 1 row affected → claimed
	mock.ExpectExec(qWriteEventOrc).WillReturnResult(sqlmock.NewResult(1, 1))

	pool := NewAgentPool([]AgentConfig{makeAgentConfig("agent-1", agent.URL, 3)})
	cfg := &Config{
		PollIntervalSecs:     1,
		HeartbeatTimeoutSecs: 3600,
		TaskTimeoutSecs:      30,
		AgentsConfigPath:     "n/a",
	}
	orc := NewOrchestrator(store, pool, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Manually set dispatchCtx so sink() works without calling Run().
	dispatchCtx, dispatchCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dispatchCancel()
	orc.mu.Lock()
	orc.dispatchCtx = dispatchCtx
	orc.mu.Unlock()

	orc.sink(DispatchableTask{ID: "task-1", Title: "test", Priority: 1})

	// Wait for goroutine to finish.
	done := make(chan struct{})
	go func() {
		orc.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("goroutine did not finish in time")
	}

	assert.True(t, dispatched.Load(), "agent should have received the task after claim")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestOrchestrator_ClaimFirst_RaceLoss verifies that sink() releases the agent
// slot and does NOT dispatch when another orchestrator wins the claim race.
func TestOrchestrator_ClaimFirst_RaceLoss(t *testing.T) {
	store, mock := newOrcMock(t)

	var dispatched atomic.Bool
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/task" {
			dispatched.Store(true)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(agent.Close)

	// 0 rows affected → another orchestrator won.
	mock.ExpectExec(qClaimTask).
		WithArgs("agent-1", "task-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	pool := NewAgentPool([]AgentConfig{makeAgentConfig("agent-1", agent.URL, 3)})
	cfg := &Config{
		PollIntervalSecs:     1,
		HeartbeatTimeoutSecs: 3600,
		TaskTimeoutSecs:      30,
		AgentsConfigPath:     "n/a",
	}
	orc := NewOrchestrator(store, pool, cfg)

	dispatchCtx, dispatchCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dispatchCancel()
	orc.mu.Lock()
	orc.dispatchCtx = dispatchCtx
	orc.mu.Unlock()

	initialActiveTasks := pool.agents[0].activeTasks
	orc.sink(DispatchableTask{ID: "task-1", Title: "test", Priority: 1})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		orc.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("goroutine did not finish in time")
	}

	assert.False(t, dispatched.Load(), "task must NOT be dispatched when claim is lost")

	// Slot must be released: activeTasks back to initial.
	pool.mu.Lock()
	finalActiveTasks := pool.agents[0].activeTasks
	pool.mu.Unlock()
	assert.Equal(t, initialActiveTasks, finalActiveTasks, "agent slot must be released on claim loss")

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestOrchestrator_ClaimFirst_DispatchFailResetsTask verifies that when dispatch
// fails after a successful claim, the task is reset back to open for retry.
func TestOrchestrator_ClaimFirst_DispatchFailResetsTask(t *testing.T) {
	store, mock := newOrcMock(t)

	// Claim succeeds, but HTTP agent is unreachable.
	mock.ExpectExec(qClaimTask).
		WithArgs("agent-1", "task-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(qWriteEventOrc).WillReturnResult(sqlmock.NewResult(1, 1)) // claim event
	mock.ExpectExec(qResetTaskOrc).
		WithArgs("task-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(qWriteCommentOrc).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Use a dead endpoint to force dispatch failure.
	pool := NewAgentPool([]AgentConfig{makeAgentConfig("agent-1", "http://127.0.0.1:19998", 3)})
	cfg := &Config{
		PollIntervalSecs:     1,
		HeartbeatTimeoutSecs: 3600,
		TaskTimeoutSecs:      2, // 2s agent timeout
		AgentsConfigPath:     "n/a",
	}
	orc := NewOrchestrator(store, pool, cfg)

	dispatchCtx, dispatchCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dispatchCancel()
	orc.mu.Lock()
	orc.dispatchCtx = dispatchCtx
	orc.mu.Unlock()

	orc.sink(DispatchableTask{ID: "task-1", Title: "test", Priority: 1})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		orc.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("goroutine did not finish in time")
	}

	require.NoError(t, mock.ExpectationsWereMet(), "task must be reset after dispatch failure")
}
