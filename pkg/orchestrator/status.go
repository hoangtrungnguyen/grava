package orchestrator

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// StatusServer exposes a /status HTTP endpoint reporting orchestrator health
// and per-agent runtime state. Create with NewStatusServer and attach to an
// Orchestrator via Orchestrator.WithStatusServer before calling Run.
type StatusServer struct {
	pool            *AgentPool
	startTime       time.Time
	tasksDispatched atomic.Int64
	tasksFailed     atomic.Int64
}

// NewStatusServer creates a StatusServer backed by the given pool.
func NewStatusServer(pool *AgentPool) *StatusServer {
	return &StatusServer{
		pool:      pool,
		startTime: time.Now(),
	}
}

// IncrDispatched increments the total tasks-dispatched counter.
func (s *StatusServer) IncrDispatched() {
	s.tasksDispatched.Add(1)
}

// IncrFailed increments the total tasks-failed counter.
func (s *StatusServer) IncrFailed() {
	s.tasksFailed.Add(1)
}

// statusResponse is the JSON shape of the /status response.
type statusResponse struct {
	Status          string        `json:"status"`
	Agents          []agentStatus `json:"agents"`
	UptimeSecs      int64         `json:"uptime_secs"`
	TasksDispatched int64         `json:"tasks_dispatched"`
	TasksFailed     int64         `json:"tasks_failed"`
}

type agentStatus struct {
	ID                  string `json:"id"`
	Available           bool   `json:"available"`
	ActiveTasks         int    `json:"active_tasks"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
}

// Handler returns an http.Handler that serves GET /status.
// All other paths return 404.
func (s *StatusServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.handleStatus)
	return mux
}

func (s *StatusServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := s.pool.Stats()
	agents := make([]agentStatus, len(stats))
	availableCount := 0
	for i, st := range stats {
		agents[i] = agentStatus{
			ID:                  st.ID,
			Available:           st.Available,
			ActiveTasks:         st.ActiveTasks,
			ConsecutiveFailures: st.ConsecutiveFailures,
		}
		if st.Available {
			availableCount++
		}
	}

	// degraded if: no agents registered, any agent unavailable, or all unavailable.
	status := "ok"
	if len(stats) == 0 || availableCount < len(stats) {
		status = "degraded"
	}

	resp := statusResponse{
		Status:          status,
		Agents:          agents,
		UptimeSecs:      int64(time.Since(s.startTime).Seconds()),
		TasksDispatched: s.tasksDispatched.Load(),
		TasksFailed:     s.tasksFailed.Load(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
