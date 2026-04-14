package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// agentState tracks runtime state for a single agent.
type agentState struct {
	cfg         AgentConfig
	activeTasks int
	available   bool
}

// AgentPool manages a pool of agents, routing tasks to the least-loaded one.
type AgentPool struct {
	agents []*agentState
	client *http.Client
	mu     sync.Mutex
}

// NewAgentPool creates an AgentPool from loaded agent configs. All agents start
// as available with zero active tasks.
func NewAgentPool(configs []AgentConfig) *AgentPool {
	states := make([]*agentState, len(configs))
	for i, c := range configs {
		states[i] = &agentState{
			cfg:       c,
			available: true,
		}
	}
	return &AgentPool{
		agents: states,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Pick returns the least-loaded available agent whose active_tasks count is
// below its MaxConcurrentTasks limit. Ties are broken by agent list order.
// Returns nil if no eligible agent exists.
func (p *AgentPool) Pick() *agentState {
	p.mu.Lock()
	defer p.mu.Unlock()

	var best *agentState
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

// Dispatch POSTs the task to the agent's /task endpoint.
//   - On a 2xx response the agent's active_tasks counter is incremented.
//   - On a network error the agent is marked UNAVAILABLE and an error is
//     returned so the caller can try the next agent.
//   - On a non-2xx HTTP status an error is returned without changing availability
//     (the agent is reachable but rejected the task).
func (p *AgentPool) Dispatch(ctx context.Context, agent *agentState, task DispatchableTask) error {
	body, err := json.Marshal(taskRequest{
		ID:          task.ID,
		Title:       task.Title,
		Description: task.Description,
		Priority:    task.Priority,
		Metadata:    map[string]interface{}{},
	})
	if err != nil {
		return fmt.Errorf("pool: marshal task: %w", err)
	}

	url := agent.cfg.Endpoint + "/task"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("pool: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		// Network-level failure: mark agent unavailable.
		p.mu.Lock()
		agent.available = false
		p.mu.Unlock()
		return fmt.Errorf("pool: dispatch to %s: %w", agent.cfg.ID, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pool: agent %s returned HTTP %d", agent.cfg.ID, resp.StatusCode)
	}

	p.mu.Lock()
	agent.activeTasks++
	p.mu.Unlock()
	return nil
}

// Complete decrements the active_tasks counter for the given agent when a task
// finishes (successfully or otherwise).
func (p *AgentPool) Complete(agent *agentState) {
	p.mu.Lock()
	if agent.activeTasks > 0 {
		agent.activeTasks--
	}
	p.mu.Unlock()
}

// MarkAvailable re-enables an agent that was previously marked unavailable.
// Useful for periodic recovery checks.
func (p *AgentPool) MarkAvailable(agent *agentState) {
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
