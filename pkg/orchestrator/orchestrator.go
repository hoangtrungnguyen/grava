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
	store    dolt.Store
	pool     *AgentPool
	watchdog *Watchdog
	cfg      *Config

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
// It picks an agent, dispatches the task asynchronously, and marks the task
// in_progress on success.
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

		if err := o.pool.Dispatch(dispatchCtx, agent, task); err != nil {
			// Dispatch failed (network error or non-2xx). Pool already handled
			// slot release and agent availability. Task stays open for retry.
			return
		}
		// Mark task in_progress so the Poller does not re-dispatch it.
		// Uses AND status='open' guard to prevent overwriting a concurrent close.
		if err := o.claimTask(dispatchCtx, task.ID, agent.cfg.ID); err != nil {
			slog.Error("orchestrator: failed to claim task after dispatch",
				"task_id", task.ID, "agent", agent.cfg.ID, "error", err)
		} else {
			slog.Info("orchestrator: task claimed", "task_id", task.ID, "agent", agent.cfg.ID)
		}
	}()
}

// claimTask atomically transitions a task from open to in_progress in the DB,
// recording which agent was assigned and when work started.
func (o *Orchestrator) claimTask(ctx context.Context, taskID, agentID string) error {
	const q = `UPDATE issues
	SET status = 'in_progress', assignee = ?, started_at = NOW()
	WHERE id = ? AND status = 'open'`
	_, err := o.store.ExecContext(ctx, q, agentID, taskID)
	if err != nil {
		return fmt.Errorf("orchestrator: claim task %s: %w", taskID, err)
	}
	return nil
}
