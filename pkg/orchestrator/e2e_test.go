package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openTestDB returns a dolt.Store connected to the test database.
// Tests that call this function skip themselves if the DB is unavailable.
func openTestDB(t *testing.T) dolt.Store {
	t.Helper()
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		dsn = "root@tcp(127.0.0.1:3306)/dolt?parseTime=true"
	}
	store, err := dolt.NewClient(dsn)
	if err != nil {
		t.Skipf("skip E2E: DB unavailable: %v", err)
	}
	t.Cleanup(func() { store.Close() }) //nolint:errcheck
	return store
}

// createTestTasks inserts n open tasks with the given prefix and returns their IDs.
func createTestTasks(t *testing.T, store dolt.Store, prefix string, n int) []string {
	t.Helper()
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = fmt.Sprintf("%s-%d", prefix, i)
		_, err := store.ExecContext(context.Background(),
			`INSERT INTO issues (id, title, status, priority, issue_type, ephemeral)
			 VALUES (?, ?, 'open', 3, 'task', 0)`,
			ids[i], fmt.Sprintf("E2E task %d", i))
		require.NoError(t, err, "insert test task %s", ids[i])
	}
	return ids
}

// cleanupTasks deletes all tasks created by the test.
func cleanupTasks(t *testing.T, store dolt.Store, prefix string) {
	t.Helper()
	_, _ = store.ExecContext(context.Background(),
		`DELETE FROM issues WHERE id LIKE ?`, prefix+"%")
	_, _ = store.ExecContext(context.Background(),
		`DELETE FROM issue_comments WHERE issue_id LIKE ?`, prefix+"%")
}

// agentServer returns an httptest.Server whose /task handler marks the
// received task as 'closed' in the DB, and whose /health handler returns 200
// so the Watchdog heartbeat check does not declare the agent dead during tests.
func agentServer(t *testing.T, store dolt.Store) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			return
		case "/task":
			// continue below
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// Use context.Background() so the DB write completes even if the HTTP
		// connection resets during orchestrator shutdown.
		_, _ = store.ExecContext(context.Background(),
			`UPDATE issues SET status='closed', stopped_at=NOW() WHERE id=?`, req.ID)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// waitAllClosed polls the DB until all taskIDs reach 'closed' or timeout.
func waitAllClosed(t *testing.T, store dolt.Store, taskIDs []string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		allClosed := true
		for _, id := range taskIDs {
			row := store.QueryRowContext(context.Background(),
				`SELECT status FROM issues WHERE id = ?`, id)
			var status string
			if err := row.Scan(&status); err != nil || status != "closed" {
				allClosed = false
				break
			}
		}
		if allClosed {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func TestOrchestrator_E2E(t *testing.T) {
	store := openTestDB(t)

	prefix := fmt.Sprintf("e2e-%d", time.Now().UnixNano()%9999999)
	taskIDs := createTestTasks(t, store, prefix, 5)
	t.Cleanup(func() { cleanupTasks(t, store, prefix) })

	// Two in-process agents that mark tasks closed on receipt.
	agent1 := agentServer(t, store)
	agent2 := agentServer(t, store)

	pool := NewAgentPool([]AgentConfig{
		{ID: "a1", Endpoint: agent1.URL, MaxConcurrentTasks: 3, TimeoutSecs: 5},
		{ID: "a2", Endpoint: agent2.URL, MaxConcurrentTasks: 3, TimeoutSecs: 5},
	})
	cfg := &Config{
		PollIntervalSecs:    1,
		HeartbeatTimeoutSecs: 5,
		TaskTimeoutSecs:     30,
		AgentsConfigPath:    "n/a",
	}
	orc := NewOrchestrator(store, pool, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go orc.Run(ctx)

	assert.True(t,
		waitAllClosed(t, store, taskIDs, 25*time.Second),
		"all 5 tasks should reach closed status within 25s")
}

func TestOrchestrator_E2E_AgentCrash(t *testing.T) {
	store := openTestDB(t)

	prefix := fmt.Sprintf("e2ec-%d", time.Now().UnixNano()%9999999)
	taskIDs := createTestTasks(t, store, prefix, 4)
	t.Cleanup(func() { cleanupTasks(t, store, prefix) })

	// Healthy agent closes tasks.
	healthy := agentServer(t, store)

	// crashSrv serves one task successfully, then is closed to simulate a
	// mid-run crash. We track how many /task requests it handled.
	var crashHandled int
	crashSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/task" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		crashHandled++
		var req struct{ ID string `json:"id"` }
		_ = json.NewDecoder(r.Body).Decode(&req)
		_, _ = store.ExecContext(context.Background(),
			`UPDATE issues SET status='closed', stopped_at=NOW() WHERE id=?`, req.ID)
		w.WriteHeader(http.StatusOK)
	}))

	pool := NewAgentPool([]AgentConfig{
		{ID: "crash", Endpoint: crashSrv.URL, MaxConcurrentTasks: 2, TimeoutSecs: 2},
		{ID: "healthy", Endpoint: healthy.URL, MaxConcurrentTasks: 4, TimeoutSecs: 5},
	})
	cfg := &Config{
		PollIntervalSecs:    1,
		HeartbeatTimeoutSecs: 5,
		TaskTimeoutSecs:     30,
		AgentsConfigPath:    "n/a",
	}
	orc := NewOrchestrator(store, pool, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go orc.Run(ctx)

	// Let the orchestrator process at least one task via crashSrv, then kill it.
	time.Sleep(1500 * time.Millisecond)
	crashSrv.Close() // crash mid-run — remaining tasks must fall over to healthy

	// All 4 tasks should still reach closed — healthy agent handles the rest.
	assert.True(t,
		waitAllClosed(t, store, taskIDs, 25*time.Second),
		"all tasks should complete even when one agent crashes mid-run")
	_ = crashHandled // crashHandled >= 0; healthy picked up the rest
}

func TestOrchestrator_GracefulShutdown_NoDataCorruption(t *testing.T) {
	store := openTestDB(t)

	prefix := fmt.Sprintf("e2es-%d", time.Now().UnixNano()%9999999)
	taskIDs := createTestTasks(t, store, prefix, 3)
	t.Cleanup(func() { cleanupTasks(t, store, prefix) })

	agent := agentServer(t, store)
	pool := NewAgentPool([]AgentConfig{
		{ID: "a1", Endpoint: agent.URL, MaxConcurrentTasks: 3, TimeoutSecs: 5},
	})
	cfg := &Config{
		PollIntervalSecs:    1,
		HeartbeatTimeoutSecs: 5,
		TaskTimeoutSecs:     30,
		AgentsConfigPath:    "n/a",
	}
	orc := NewOrchestrator(store, pool, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		orc.Run(ctx)
		close(done)
	}()

	// Let the orchestrator process some tasks, then cancel.
	time.Sleep(2 * time.Second)
	cancel()

	// Run() must exit within 5s after cancel.
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Orchestrator.Run did not exit within 5s of context cancel")
	}

	// After clean shutdown there must be no task stuck in_progress.
	for _, id := range taskIDs {
		row := store.QueryRowContext(context.Background(),
			`SELECT status FROM issues WHERE id = ?`, id)
		var status string
		require.NoError(t, row.Scan(&status))
		assert.NotEqual(t, "in_progress", status,
			"task %s must not be stuck in_progress after shutdown", id)
	}
}

// TestOrchestrator_E2E_3Agents_100Tasks verifies that 3 agents fully drain a
// 100-task queue with zero data corruption (all tasks reach 'closed').
func TestOrchestrator_E2E_3Agents_100Tasks(t *testing.T) {
	store := openTestDB(t)

	prefix := fmt.Sprintf("e2e3a-%d", time.Now().UnixNano()%9999999)
	taskIDs := createTestTasks(t, store, prefix, 100)
	t.Cleanup(func() { cleanupTasks(t, store, prefix) })

	agent1 := agentServer(t, store)
	agent2 := agentServer(t, store)
	agent3 := agentServer(t, store)

	pool := NewAgentPool([]AgentConfig{
		{ID: "a1", Endpoint: agent1.URL, MaxConcurrentTasks: 40, TimeoutSecs: 10},
		{ID: "a2", Endpoint: agent2.URL, MaxConcurrentTasks: 40, TimeoutSecs: 10},
		{ID: "a3", Endpoint: agent3.URL, MaxConcurrentTasks: 40, TimeoutSecs: 10},
	})
	cfg := &Config{
		PollIntervalSecs:     1,
		HeartbeatTimeoutSecs: 5,
		TaskTimeoutSecs:      60,
		AgentsConfigPath:     "n/a",
	}
	orc := NewOrchestrator(store, pool, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	go orc.Run(ctx)

	// waitAllClosed polls until every task is 'closed'; failure here indicates
	// data corruption (tasks stuck in open/in_progress) or a timeout.
	assert.True(t,
		waitAllClosed(t, store, taskIDs, 80*time.Second),
		"all 100 tasks should reach closed status within 80s")
}

// TestOrchestrator_E2E_ConcurrentClaim verifies that 3 parallel orchestrators
// sharing the same DB do not produce double-claims: each task is dispatched at
// most once and all tasks eventually reach 'closed'.
func TestOrchestrator_E2E_ConcurrentClaim(t *testing.T) {
	store := openTestDB(t)

	prefix := fmt.Sprintf("e2ecc-%d", time.Now().UnixNano()%9999999)
	taskIDs := createTestTasks(t, store, prefix, 15)
	t.Cleanup(func() { cleanupTasks(t, store, prefix) })

	// trackingSrv records how many times each task ID is dispatched, then
	// marks the task closed. Using context.Background() so DB writes complete
	// even if the HTTP connection resets during shutdown.
	var mu sync.Mutex
	dispatchCount := make(map[string]int)

	trackingSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/task" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		dispatchCount[req.ID]++
		mu.Unlock()

		_, _ = store.ExecContext(context.Background(),
			`UPDATE issues SET status='closed', stopped_at=NOW() WHERE id=?`, req.ID)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(trackingSrv.Close)

	cfg := &Config{
		PollIntervalSecs:     1,
		HeartbeatTimeoutSecs: 5,
		TaskTimeoutSecs:      30,
		AgentsConfigPath:     "n/a",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Spawn 3 independent orchestrators, each with its own agent slot pointing
	// to the same tracking server.
	for i := 0; i < 3; i++ {
		agentID := fmt.Sprintf("orc%d-agent", i)
		pool := NewAgentPool([]AgentConfig{
			{ID: agentID, Endpoint: trackingSrv.URL, MaxConcurrentTasks: 5, TimeoutSecs: 5},
		})
		orc := NewOrchestrator(store, pool, cfg)
		go orc.Run(ctx)
	}

	require.True(t,
		waitAllClosed(t, store, taskIDs, 25*time.Second),
		"all 15 tasks should reach closed status with 3 concurrent orchestrators")

	// Assert liveness (all tasks closed) and at-most-once dispatch.
	// Note: the orchestrator uses dispatch-then-claim ordering. The Poller
	// filters on status='open', but there is a short window between sink()
	// picking up a task and claimTask() setting it to 'in_progress'. Two
	// orchestrators polling concurrently could both see the same task as
	// 'open' and both dispatch it (count == 2). The atomic claimTask guard
	// (AND status='open') prevents double in_progress state, but cannot
	// prevent double HTTP dispatch. We assert at-most-once here; strictly
	// exactly-once would require claim-before-dispatch ordering.
	mu.Lock()
	defer mu.Unlock()
	for _, id := range taskIDs {
		count := dispatchCount[id]
		assert.LessOrEqual(t, count, 1,
			"task %s dispatched %d times; at-most-once expected", id, count)
	}
}
