package orchestrator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
