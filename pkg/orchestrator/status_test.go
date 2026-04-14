package orchestrator

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusServer_HandlerReturnsJSON(t *testing.T) {
	pool := NewAgentPool([]AgentConfig{
		makeAgentConfig("agent-1", "http://localhost:8001", 3),
		makeAgentConfig("agent-2", "http://localhost:8002", 2),
	})
	srv := NewStatusServer(pool)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp statusResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "ok", resp.Status)
	assert.Len(t, resp.Agents, 2)
	assert.Equal(t, "agent-1", resp.Agents[0].ID)
	assert.True(t, resp.Agents[0].Available)
	assert.Equal(t, 0, resp.Agents[0].ActiveTasks)
	assert.Equal(t, 0, resp.Agents[0].ConsecutiveFailures)
}

func TestStatusServer_DegradedWhenEmptyPool(t *testing.T) {
	pool := NewAgentPool([]AgentConfig{}) // no agents
	srv := NewStatusServer(pool)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp statusResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "degraded", resp.Status)
}

func TestStatusServer_DegradedWhenAgentUnavailable(t *testing.T) {
	pool := NewAgentPool([]AgentConfig{
		makeAgentConfig("agent-1", "http://localhost:8001", 3),
		makeAgentConfig("agent-2", "http://localhost:8002", 3),
	})
	// Mark one agent unavailable.
	pool.mu.Lock()
	pool.agents[0].available = false
	pool.mu.Unlock()

	srv := NewStatusServer(pool)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp statusResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "degraded", resp.Status)
}

func TestStatusServer_DegradedWhenAllAgentsUnavailable(t *testing.T) {
	pool := NewAgentPool([]AgentConfig{
		makeAgentConfig("agent-1", "http://localhost:8001", 3),
	})
	pool.mu.Lock()
	pool.agents[0].available = false
	pool.mu.Unlock()

	srv := NewStatusServer(pool)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp statusResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "degraded", resp.Status)
}

func TestStatusServer_TaskCounters(t *testing.T) {
	pool := NewAgentPool([]AgentConfig{makeAgentConfig("a1", "http://localhost", 3)})
	srv := NewStatusServer(pool)

	srv.IncrDispatched()
	srv.IncrDispatched()
	srv.IncrDispatched()
	srv.IncrFailed()

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp statusResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, int64(3), resp.TasksDispatched)
	assert.Equal(t, int64(1), resp.TasksFailed)
}

func TestStatusServer_MethodNotAllowed(t *testing.T) {
	pool := NewAgentPool([]AgentConfig{makeAgentConfig("a1", "http://localhost", 3)})
	srv := NewStatusServer(pool)

	req := httptest.NewRequest(http.MethodPost, "/status", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestStatusServer_NotFoundForUnknownPath(t *testing.T) {
	pool := NewAgentPool([]AgentConfig{makeAgentConfig("a1", "http://localhost", 3)})
	srv := NewStatusServer(pool)

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestStatusServer_LiveHTTP verifies that the /status handler works over a real
// TCP connection — exercises the full HTTP wiring that orchestrate.go uses.
func TestStatusServer_LiveHTTP(t *testing.T) {
	pool := NewAgentPool([]AgentConfig{makeAgentConfig("a1", "http://localhost:9001", 3)})
	srv := NewStatusServer(pool)
	srv.IncrDispatched()

	// Use OS-assigned port so parallel tests don't collide.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()

	httpSrv := &http.Server{
		Handler:      srv.Handler(),
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	}
	go func() { _ = httpSrv.Serve(ln) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
	})

	url := "http://" + addr + "/status"
	resp, err := http.Get(url) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result statusResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "ok", result.Status)
	assert.Len(t, result.Agents, 1)
	assert.Equal(t, int64(1), result.TasksDispatched)
}

func TestStatusServer_UptimeSecsGrowsOverTime(t *testing.T) {
	pool := NewAgentPool([]AgentConfig{makeAgentConfig("a1", "http://localhost", 3)})
	srv := NewStatusServer(pool)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp statusResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.GreaterOrEqual(t, resp.UptimeSecs, int64(0))
}
