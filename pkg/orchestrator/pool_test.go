package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeAgentConfig(id, endpoint string, maxTasks int) AgentConfig {
	return AgentConfig{
		ID:                 id,
		Endpoint:           endpoint,
		MaxConcurrentTasks: maxTasks,
	}
}

func TestAgentPool_Pick_LeastLoaded(t *testing.T) {
	pool := NewAgentPool([]AgentConfig{
		makeAgentConfig("agent-1", "http://localhost:8001", 3),
		makeAgentConfig("agent-2", "http://localhost:8002", 3),
	})

	// Artificially load agent-1.
	pool.mu.Lock()
	pool.agents[0].activeTasks = 2
	pool.mu.Unlock()

	got := pool.Pick()
	require.NotNil(t, got)
	assert.Equal(t, "agent-2", got.cfg.ID, "should pick least-loaded agent")
}

func TestAgentPool_Pick_RespectsMaxConcurrentTasks(t *testing.T) {
	pool := NewAgentPool([]AgentConfig{
		makeAgentConfig("agent-1", "http://localhost:8001", 1),
	})

	// Saturate the only agent.
	pool.mu.Lock()
	pool.agents[0].activeTasks = 1
	pool.mu.Unlock()

	got := pool.Pick()
	assert.Nil(t, got, "no agent should be returned when all are at capacity")
}

func TestAgentPool_Pick_SkipsUnavailableAgents(t *testing.T) {
	pool := NewAgentPool([]AgentConfig{
		makeAgentConfig("agent-1", "http://localhost:8001", 3),
		makeAgentConfig("agent-2", "http://localhost:8002", 3),
	})

	pool.mu.Lock()
	pool.agents[0].available = false
	pool.mu.Unlock()

	got := pool.Pick()
	require.NotNil(t, got)
	assert.Equal(t, "agent-2", got.cfg.ID)
}

func TestAgentPool_Dispatch_SuccessIncrementsCounter(t *testing.T) {
	var receivedBody taskRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/task", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &receivedBody))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pool := NewAgentPool([]AgentConfig{
		makeAgentConfig("agent-1", srv.URL, 3),
	})
	agent := pool.agents[0]

	task := DispatchableTask{ID: "task-1", Title: "Do something", Description: "desc", Priority: 2}
	err := pool.Dispatch(context.Background(), agent, task)
	require.NoError(t, err)

	assert.Equal(t, 1, agent.activeTasks, "active_tasks must be incremented on 2xx")
	assert.Equal(t, "task-1", receivedBody.ID)
	assert.Equal(t, "Do something", receivedBody.Title)
	assert.Equal(t, "desc", receivedBody.Description)
	assert.Equal(t, 2, receivedBody.Priority)
	assert.NotNil(t, receivedBody.Metadata)
}

func TestAgentPool_Dispatch_NetworkErrorMarksUnavailable(t *testing.T) {
	// Point at a non-listening port to simulate network failure.
	pool := NewAgentPool([]AgentConfig{
		makeAgentConfig("agent-dead", "http://127.0.0.1:19999", 3),
	})
	agent := pool.agents[0]

	err := pool.Dispatch(context.Background(), agent, DispatchableTask{ID: "t1"})
	require.Error(t, err)
	assert.False(t, agent.available, "agent must be marked unavailable on network error")
	assert.Equal(t, 0, agent.activeTasks, "active_tasks must not be incremented on failure")
}

func TestAgentPool_Dispatch_Non2xxNoAvailabilityChange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	pool := NewAgentPool([]AgentConfig{
		makeAgentConfig("agent-1", srv.URL, 3),
	})
	agent := pool.agents[0]

	err := pool.Dispatch(context.Background(), agent, DispatchableTask{ID: "t1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
	assert.True(t, agent.available, "agent remains available on non-2xx — it is reachable")
	assert.Equal(t, 0, agent.activeTasks)
}

func TestAgentPool_Complete_DecrementsCounter(t *testing.T) {
	pool := NewAgentPool([]AgentConfig{makeAgentConfig("agent-1", "http://localhost", 3)})
	agent := pool.agents[0]

	pool.mu.Lock()
	agent.activeTasks = 2
	pool.mu.Unlock()

	pool.Complete(agent)
	assert.Equal(t, 1, agent.activeTasks)
}

func TestAgentPool_FallbackToNextAgent(t *testing.T) {
	// First agent dead, second healthy.
	var dispatched bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dispatched = true
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	pool := NewAgentPool([]AgentConfig{
		makeAgentConfig("agent-dead", "http://127.0.0.1:19999", 3),
		makeAgentConfig("agent-live", srv.URL, 3),
	})

	task := DispatchableTask{ID: "task-1", Title: "T", Priority: 1}

	// Simulate caller retry: pick → dispatch fails → mark unavailable → pick again.
	agent := pool.Pick()
	require.NotNil(t, agent)
	err := pool.Dispatch(context.Background(), agent, task)
	require.Error(t, err)
	assert.False(t, agent.available)

	// Second pick should return the live agent.
	agent2 := pool.Pick()
	require.NotNil(t, agent2)
	assert.Equal(t, "agent-live", agent2.cfg.ID)

	err = pool.Dispatch(context.Background(), agent2, task)
	require.NoError(t, err)
	assert.True(t, dispatched)
	assert.Equal(t, 1, agent2.activeTasks)
}

func TestAgentPool_Stats(t *testing.T) {
	pool := NewAgentPool([]AgentConfig{
		makeAgentConfig("agent-1", "http://localhost:8001", 3),
		makeAgentConfig("agent-2", "http://localhost:8002", 3),
	})

	pool.mu.Lock()
	pool.agents[0].activeTasks = 1
	pool.agents[1].available = false
	pool.mu.Unlock()

	stats := pool.Stats()
	require.Len(t, stats, 2)
	assert.Equal(t, "agent-1", stats[0].ID)
	assert.Equal(t, 1, stats[0].ActiveTasks)
	assert.True(t, stats[0].Available)
	assert.False(t, stats[1].Available)
}
