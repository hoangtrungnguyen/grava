package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

// Orchestrator wires together the Poller, AgentPool, and Watchdog into a
// running dispatch loop. Create with NewOrchestrator and call Run.
type Orchestrator struct {
	store     dolt.Store
	pool      *AgentPool
	watchdog  *Watchdog
	cfg       *Config
	statusSrv *StatusServer // optional; nil means no status tracking

	// wg tracks in-flight async dispatches so Run() can drain them on shutdown.
	wg sync.WaitGroup

	// dispatchCtx is set in Run() before the poller starts; sink goroutines use
	// it so they can complete their HTTP call even after the poll context cancels.
	dispatchCtx context.Context
	mu          sync.Mutex
}

// NewOrchestrator creates an Orchestrator from the given config. It wires the
// AgentPool to a Watchdog that monitors /health and resets timed-out tasks.
func NewOrchestrator(store dolt.Store, pool *AgentPool, cfg *Config) *Orchestrator {
	wd := NewWatchdog(pool, store, cfg.HeartbeatTimeoutSecs, cfg.TaskTimeoutSecs)
	return &Orchestrator{
		store:    store,
		pool:     pool,
		watchdog: wd,
		cfg:      cfg,
	}
}

// WithStatusServer attaches a StatusServer to the Orchestrator so that
// dispatch and failure events increment the server's counters. Returns the
// Orchestrator for method chaining.
func (o *Orchestrator) WithStatusServer(srv *StatusServer) *Orchestrator {
	o.statusSrv = srv
	return o
}

// Run starts the orchestrator loop. It polls for open tasks, dispatches them
// to available agents, and monitors agent health. On ctx cancellation (e.g.
// SIGTERM), the poller and watchdog stop; Run blocks until all in-flight
// dispatches complete before returning (graceful drain).
func (o *Orchestrator) Run(ctx context.Context) {
	slog.Info("orchestrator: starting",
		"poll_interval_secs", o.cfg.PollIntervalSecs,
		"heartbeat_timeout_secs", o.cfg.HeartbeatTimeoutSecs,
		"task_timeout_secs", o.cfg.TaskTimeoutSecs)

	// dispatchCtx allows in-flight goroutines to finish after polling stops.
	// A 30s drain timeout ensures Run() is not blocked forever by a hung agent.
	drainTimeout := time.Duration(o.cfg.TaskTimeoutSecs) * time.Second
	if drainTimeout <= 0 {
		drainTimeout = 30 * time.Second
	}
	dispatchCtx, cancelDispatch := context.WithTimeout(context.Background(), drainTimeout)
	o.mu.Lock()
	o.dispatchCtx = dispatchCtx
	o.mu.Unlock()
	defer func() {
		slog.Info("orchestrator: draining in-flight tasks")
		o.wg.Wait() // drain in-flight dispatches (bounded by drainTimeout)
		cancelDispatch()
		slog.Info("orchestrator: shutdown complete")
	}()

	poller := NewPoller(
		o.store,
		time.Duration(o.cfg.PollIntervalSecs)*time.Second,
		o.sink,
	)

	go o.watchdog.Run(ctx)
	poller.Run(ctx) // blocks until ctx cancelled
}

// sink is the TaskSink callback invoked by the Poller for each discovered task.
// It picks an agent, claims the task atomically in the DB (claim-first), then
// dispatches to the agent. If dispatch fails after claim, the task is reset to
// open so the Poller can retry it. This ordering eliminates the race where two
// concurrent pollers could both dispatch the same task before either claims it.
func (o *Orchestrator) sink(task DispatchableTask) {
	o.mu.Lock()
	dispatchCtx := o.dispatchCtx
	o.mu.Unlock()
	if dispatchCtx == nil {
		return
	}

	agent := o.pool.Pick()
	if agent == nil {
		// No capacity — task stays open; next poll tick will retry.
		slog.Debug("orchestrator: no agent available for task", "task_id", task.ID)
		return
	}

	o.wg.Add(1)
	go func() {
		defer o.wg.Done()

		// Claim first: atomically transition task to in_progress.
		// If another orchestrator claimed it (0 rows affected), release the
		// reserved slot and skip — the task is not ours.
		claimed, err := o.claimTask(dispatchCtx, task.ID, agent.cfg.ID)
		if err != nil {
			o.pool.releaseSlot(agent, false)
			slog.Error("orchestrator: failed to claim task",
				"task_id", task.ID, "agent", agent.cfg.ID, "error", err)
			return
		}
		if !claimed {
			o.pool.releaseSlot(agent, false)
			slog.Debug("orchestrator: task claimed by another orchestrator, skipping",
				"task_id", task.ID)
			return
		}
		slog.Info("orchestrator: task claimed", "task_id", task.ID, "agent", agent.cfg.ID)

		// Dispatch after claim. On failure, reset the task to open so the
		// Poller can retry it. Pool.Dispatch handles slot release on error.
		if err := o.pool.Dispatch(dispatchCtx, agent, task); err != nil {
			if resetErr := o.resetTask(dispatchCtx, task.ID); resetErr != nil {
				slog.Error("orchestrator: failed to reset task after dispatch failure",
					"task_id", task.ID, "error", resetErr)
			} else {
				msg := fmt.Sprintf("dispatch failed (agent %s): %v — task reset to open for retry", agent.cfg.ID, err)
				if commentErr := o.writeComment(dispatchCtx, task.ID, msg); commentErr != nil {
					slog.Warn("orchestrator: failed to write dispatch-failure comment",
						"task_id", task.ID, "error", commentErr)
				}
			}
			if o.statusSrv != nil {
				o.statusSrv.IncrFailed()
			}
			return
		}
		if o.statusSrv != nil {
			o.statusSrv.IncrDispatched()
		}
	}()
}

// claimTask atomically transitions a task from open to in_progress in the DB.
// Returns (true, nil) if the task was claimed, (false, nil) if another
// orchestrator already claimed it (0 rows affected), or (false, err) on error.
func (o *Orchestrator) claimTask(ctx context.Context, taskID, agentID string) (bool, error) {
	const q = `UPDATE issues
	SET status = 'in_progress', assignee = ?, started_at = NOW()
	WHERE id = ? AND status = 'open'`
	result, err := o.store.ExecContext(ctx, q, agentID, taskID)
	if err != nil {
		return false, fmt.Errorf("orchestrator: claim task %s: %w", taskID, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("orchestrator: claim task %s rows affected: %w", taskID, err)
	}
	return n > 0, nil
}

// resetTask transitions a task from in_progress back to open, clearing the
// assignee. Called when a dispatch fails after a successful claim.
func (o *Orchestrator) resetTask(ctx context.Context, taskID string) error {
	const q = `UPDATE issues SET status = 'open', assignee = NULL, started_at = NULL WHERE id = ?`
	_, err := o.store.ExecContext(ctx, q, taskID)
	if err != nil {
		return fmt.Errorf("orchestrator: reset task %s: %w", taskID, err)
	}
	return nil
}

// writeComment inserts a comment on the given issue.
func (o *Orchestrator) writeComment(ctx context.Context, issueID, message string) error {
	const q = `INSERT INTO issue_comments (issue_id, message, actor) VALUES (?, ?, 'orchestrator')`
	_, err := o.store.ExecContext(ctx, q, issueID, message)
	if err != nil {
		return fmt.Errorf("orchestrator: write comment %s: %w", issueID, err)
	}
	return nil
}
