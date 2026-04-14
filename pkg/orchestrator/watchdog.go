package orchestrator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

const defaultMaxConsecutiveFailures = 3

// Watchdog monitors agent health via periodic /health pings and resets tasks
// when agents are declared DEAD or when tasks exceed their timeout.
type Watchdog struct {
	pool            *AgentPool
	store           dolt.Store
	heartbeatSecs   int
	taskTimeoutSecs int
	maxFailures     int // consecutive /health misses before agent declared DEAD
}

// NewWatchdog creates a Watchdog. heartbeatSecs controls the polling interval;
// taskTimeoutSecs is how long an in-progress task may sit before being reset.
func NewWatchdog(pool *AgentPool, store dolt.Store, heartbeatSecs, taskTimeoutSecs int) *Watchdog {
	if heartbeatSecs <= 0 {
		heartbeatSecs = 5
	}
	if taskTimeoutSecs <= 0 {
		taskTimeoutSecs = 30
	}
	return &Watchdog{
		pool:            pool,
		store:           store,
		heartbeatSecs:   heartbeatSecs,
		taskTimeoutSecs: taskTimeoutSecs,
		maxFailures:     defaultMaxConsecutiveFailures,
	}
}

// Run starts the watchdog loop. It fires immediately then every heartbeatSecs
// until ctx is cancelled. Each tick checks heartbeats and task timeouts.
func (w *Watchdog) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(w.heartbeatSecs) * time.Second)
	defer ticker.Stop()

	w.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *Watchdog) tick(ctx context.Context) {
	w.checkHeartbeats(ctx)
	w.checkTaskTimeouts(ctx)
}

// checkHeartbeats pings /health on each agent. Consecutive failures increment
// a counter; once maxFailures is reached the agent is declared DEAD and all
// its in-flight tasks are reset to open.
func (w *Watchdog) checkHeartbeats(ctx context.Context) {
	w.pool.mu.Lock()
	agents := make([]*Agent, len(w.pool.agents))
	copy(agents, w.pool.agents)
	w.pool.mu.Unlock()

	for _, agent := range agents {
		if err := w.ping(ctx, agent); err != nil {
			w.pool.mu.Lock()
			agent.consecutiveFailures++
			failures := agent.consecutiveFailures
			alreadyDead := agent.dead
			w.pool.mu.Unlock()

			slog.Warn("watchdog: agent heartbeat failed",
				"agent", agent.cfg.ID,
				"consecutive_failures", failures,
				"error", err)

			// Only fire markAgentDead on the exact tick that crosses the threshold,
			// not on every subsequent tick while the agent remains unresponsive.
			if failures == w.maxFailures && !alreadyDead {
				w.pool.mu.Lock()
				agent.dead = true
				w.pool.mu.Unlock()
				slog.Error("watchdog: agent declared dead", "agent", agent.cfg.ID)
				w.markAgentDead(ctx, agent)
			}
		} else {
			// Successful heartbeat: reset failure counter and restore availability.
			w.pool.mu.Lock()
			wasUnavailable := !agent.available
			agent.consecutiveFailures = 0
			agent.available = true
			agent.dead = false
			w.pool.mu.Unlock()

			if wasUnavailable {
				slog.Info("watchdog: agent recovered", "agent", agent.cfg.ID)
			} else {
				slog.Debug("watchdog: agent heartbeat ok", "agent", agent.cfg.ID)
			}
		}
	}
}

// ping issues a GET request to the agent's /health endpoint using the agent's
// own HTTP client (so per-agent timeout is honoured).
func (w *Watchdog) ping(ctx context.Context, agent *Agent) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, agent.cfg.Endpoint+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := agent.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	// Drain to enable HTTP keep-alive connection reuse.
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("watchdog: agent %s /health returned %d", agent.cfg.ID, resp.StatusCode)
	}
	return nil
}

// markAgentDead marks the agent unavailable and resets all in-progress tasks
// assigned to it, writing a grava comment on each.
func (w *Watchdog) markAgentDead(ctx context.Context, agent *Agent) {
	w.pool.mu.Lock()
	agent.available = false
	w.pool.mu.Unlock()

	taskIDs, err := w.fetchAssignedTasks(ctx, agent.cfg.ID)
	if err != nil {
		return // transient DB error; will retry on next tick
	}

	for _, id := range taskIDs {
		_ = w.resetTask(ctx, id)
		_ = w.writeComment(ctx, id,
			fmt.Sprintf("Agent %s timeout/crash. Task reassigned.", agent.cfg.ID))
	}
}

// fetchAssignedTasks returns IDs of in-progress issues assigned to agentID.
func (w *Watchdog) fetchAssignedTasks(ctx context.Context, agentID string) ([]string, error) {
	const q = `SELECT id FROM issues WHERE status = 'in_progress' AND assignee = ?`
	rows, err := w.store.QueryContext(ctx, q, agentID)
	if err != nil {
		return nil, fmt.Errorf("watchdog: query assigned tasks: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("watchdog: scan task id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// checkTaskTimeouts finds tasks that have been in_progress longer than
// taskTimeoutSecs and resets them to open.
func (w *Watchdog) checkTaskTimeouts(ctx context.Context) {
	const q = `
SELECT id FROM issues
WHERE status = 'in_progress'
  AND started_at IS NOT NULL
  AND started_at < NOW() - INTERVAL ? SECOND`

	rows, err := w.store.QueryContext(ctx, q, w.taskTimeoutSecs)
	if err != nil {
		return // transient error
	}
	defer rows.Close() //nolint:errcheck

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		return
	}

	for _, id := range ids {
		slog.Warn("watchdog: task timeout exceeded, resetting to open", "task_id", id, "timeout_secs", w.taskTimeoutSecs)
		_ = w.resetTask(ctx, id)
		_ = w.writeComment(ctx, id, "Task timeout exceeded. Task reassigned.")
	}
}

// resetTask updates a single issue back to open status.
func (w *Watchdog) resetTask(ctx context.Context, taskID string) error {
	const q = `UPDATE issues SET status = 'open', assignee = NULL, started_at = NULL WHERE id = ?`
	_, err := w.store.ExecContext(ctx, q, taskID)
	return err
}

// writeComment inserts a comment on the given issue.
func (w *Watchdog) writeComment(ctx context.Context, issueID, message string) error {
	const q = `INSERT INTO issue_comments (issue_id, message, actor) VALUES (?, ?, 'orchestrator')`
	_, err := w.store.ExecContext(ctx, q, issueID, message)
	return err
}
