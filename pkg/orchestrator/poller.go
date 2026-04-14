package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

// DispatchableTask is a task ready for dispatch from the Grava queue.
type DispatchableTask struct {
	ID          string
	Title       string
	Description string
	Priority    int
}

// TaskSink is a function that receives tasks discovered by the Poller.
type TaskSink func(task DispatchableTask)

// Poller queries Grava for open, unblocked tasks on each tick and delivers
// them to a TaskSink. It runs until the provided context is cancelled.
type Poller struct {
	store    dolt.Store
	interval time.Duration
	sink     TaskSink
}

// NewPoller creates a Poller that fires every interval, querying the given
// store and delivering tasks to sink.
func NewPoller(store dolt.Store, interval time.Duration, sink TaskSink) *Poller {
	return &Poller{
		store:    store,
		interval: interval,
		sink:     sink,
	}
}

// Run starts the poll loop. It ticks immediately on entry, then every
// p.interval until ctx is cancelled. Errors from individual poll ticks are
// silently swallowed so a transient DB failure does not kill the loop.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Fire immediately so callers don't wait one full interval.
	p.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

// poll fetches open tasks not blocked by unstarted (open) dependencies and
// sends each one to the sink in priority order (lowest priority number =
// highest urgency).
func (p *Poller) poll(ctx context.Context) {
	tasks, err := p.fetchTasks(ctx)
	if err != nil {
		// Transient error — log nothing (callers can wrap with a logger if needed).
		return
	}
	for _, t := range tasks {
		p.sink(t)
	}
}

// fetchTasks returns open issues that have no unstarted (open) blocking
// dependencies, ordered by priority ascending (1 = highest urgency).
func (p *Poller) fetchTasks(ctx context.Context) ([]DispatchableTask, error) {
	const query = `
SELECT i.id, i.title, COALESCE(i.description, ''), i.priority
FROM issues i
WHERE i.status = 'open'
  AND i.ephemeral = 0
  AND NOT EXISTS (
      SELECT 1
      FROM dependencies d
      JOIN issues b ON b.id = d.from_id
      WHERE d.to_id = i.id
        AND b.status = 'open'
  )
ORDER BY i.priority ASC, i.created_at ASC`

	rows, err := p.store.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("poller: failed to query tasks: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var tasks []DispatchableTask
	for rows.Next() {
		var t DispatchableTask
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Priority); err != nil {
			return nil, fmt.Errorf("poller: failed to scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("poller: row iteration error: %w", err)
	}
	return tasks, nil
}
