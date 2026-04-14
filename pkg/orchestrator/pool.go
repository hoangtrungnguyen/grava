package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Agent tracks runtime state for a single registered agent.
type Agent struct {
	cfg                 AgentConfig
	client              *http.Client
	activeTasks         int
	available           bool
	consecutiveFailures int // incremented by Watchdog on /health miss, reset on success
}

// AgentPool manages a pool of agents, routing tasks to the least-loaded one.
type AgentPool struct {
	agents []*Agent
	mu     sync.Mutex
}

// NewAgentPool creates an AgentPool from loaded agent configs. All agents start
// as available with zero active tasks. Each agent gets its own HTTP client
// honouring the per-agent TimeoutSecs setting.
func NewAgentPool(configs []AgentConfig) *AgentPool {
	agents := make([]*Agent, len(configs))
	for i, c := range configs {
		timeout := time.Duration(c.TimeoutSecs) * time.Second
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		agents[i] = &Agent{
			cfg:       c,
			client:    &http.Client{Timeout: timeout},
			available: true,
		}
	}
	return &AgentPool{agents: agents}
}

// Pick atomically selects and reserves a slot on the least-loaded available
// agent whose active_tasks count is below its MaxConcurrentTasks limit. Ties
// are broken by agent list order. The returned agent's active_tasks counter is
// already incremented; if the subsequent Dispatch call fails, Dispatch
// decrements it. Returns nil if no eligible agent exists.
func (p *AgentPool) Pick() *Agent {
	p.mu.Lock()
	defer p.mu.Unlock()

	var best *Agent
	for _, a := range p.agents {
		if !a.available {
			continue
		}
		if a.activeTasks >= a.cfg.MaxConcurrentTasks {
			continue
		}
		if best == nil || a.activeTasks < best.activeTasks {
			best = a
		}
	}
	if best != nil {
		best.activeTasks++ // reserve slot optimistically
	}
	return best
}

// taskRequest is the JSON payload sent to an agent's /task endpoint.
type taskRequest struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Priority    int                    `json:"priority"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// Dispatch POSTs the task to the agent's /task endpoint. The agent's
// active_tasks slot was already reserved by Pick(); Dispatch decrements it
// on any failure so the slot is released.
//
//   - On a 2xx response, the reservation stands (counter stays incremented).
//   - On a network error, the agent is marked UNAVAILABLE, the slot is
//     released, and an error is returned so the caller can retry with Pick().
//   - On a non-2xx HTTP status, the slot is released and an error is returned.
//     The agent's availability is not changed (it is reachable).
func (p *AgentPool) Dispatch(ctx context.Context, agent *Agent, task DispatchableTask) error {
	body, err := json.Marshal(taskRequest{
		ID:          task.ID,
		Title:       task.Title,
		Description: task.Description,
		Priority:    task.Priority,
		Metadata:    map[string]interface{}{},
	})
	if err != nil {
		p.releaseSlot(agent, false)
		return fmt.Errorf("pool: marshal task: %w", err)
	}

	url := agent.cfg.Endpoint + "/task"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		p.releaseSlot(agent, false)
		return fmt.Errorf("pool: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := agent.client.Do(req)
	if err != nil {
		// Network-level failure: release slot and mark agent unavailable.
		p.releaseSlot(agent, true)
		return fmt.Errorf("pool: dispatch to %s: %w", agent.cfg.ID, err)
	}
	defer resp.Body.Close() //nolint:errcheck
	// Drain to enable HTTP keep-alive connection reuse.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		p.releaseSlot(agent, false)
		return fmt.Errorf("pool: agent %s returned HTTP %d", agent.cfg.ID, resp.StatusCode)
	}

	return nil
}

// releaseSlot decrements the agent's active_tasks counter. If markUnavailable
// is true, the agent is also marked unavailable (used on network errors).
func (p *AgentPool) releaseSlot(agent *Agent, markUnavailable bool) {
	p.mu.Lock()
	if agent.activeTasks > 0 {
		agent.activeTasks--
	}
	if markUnavailable {
		agent.available = false
	}
	p.mu.Unlock()
}

// Complete decrements the active_tasks counter for the given agent when a task
// finishes (successfully or otherwise). Call this when the agent reports task
// completion, not when Dispatch returns.
func (p *AgentPool) Complete(agent *Agent) {
	p.mu.Lock()
	if agent.activeTasks > 0 {
		agent.activeTasks--
	}
	p.mu.Unlock()
}

// MarkAvailable re-enables an agent that was previously marked unavailable.
// Useful for periodic recovery checks.
func (p *AgentPool) MarkAvailable(agent *Agent) {
	p.mu.Lock()
	agent.available = true
	p.mu.Unlock()
}

// Stats returns a snapshot of each agent's current state for observability.
func (p *AgentPool) Stats() []AgentStat {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats := make([]AgentStat, len(p.agents))
	for i, a := range p.agents {
		stats[i] = AgentStat{
			ID:          a.cfg.ID,
			Endpoint:    a.cfg.Endpoint,
			ActiveTasks: a.activeTasks,
			Available:   a.available,
		}
	}
	return stats
}

// AgentStat is a point-in-time snapshot of a single agent's state.
type AgentStat struct {
	ID          string
	Endpoint    string
	ActiveTasks int
	Available   bool
}
