package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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
// received task as 'closed' in the DB and writes a comment.
func agentServer(t *testing.T, store dolt.Store) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		// Simulate work: mark task closed.
		_, _ = store.ExecContext(r.Context(),
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

	prefix := fmt.Sprintf("e2e-%d", time.Now().Unix()%99999)
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

	prefix := fmt.Sprintf("e2ec-%d", time.Now().Unix()%99999)
	taskIDs := createTestTasks(t, store, prefix, 4)
	t.Cleanup(func() { cleanupTasks(t, store, prefix) })

	// Healthy agent closes tasks.
	healthy := agentServer(t, store)

	// Crash agent: close it immediately to simulate a dead server.
	crash := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // won't actually be served
	}))
	crash.Close() // closed immediately — all connections will fail

	pool := NewAgentPool([]AgentConfig{
		{ID: "crash", Endpoint: crash.URL, MaxConcurrentTasks: 2, TimeoutSecs: 2},
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

	// All 4 tasks should still reach closed — healthy agent handles everything
	// after crash agent is marked unavailable.
	assert.True(t,
		waitAllClosed(t, store, taskIDs, 25*time.Second),
		"all tasks should complete even when one agent crashes")
}

func TestOrchestrator_GracefulShutdown_NoDataCorruption(t *testing.T) {
	store := openTestDB(t)

	prefix := fmt.Sprintf("e2es-%d", time.Now().Unix()%99999)
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
