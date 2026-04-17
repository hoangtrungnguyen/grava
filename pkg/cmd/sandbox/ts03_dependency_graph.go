package sandbox

import (
	"context"
	"fmt"
	"sync"
	"time"

	cmdgraph "github.com/hoangtrungnguyen/grava/pkg/cmd/graph"
	issuesapi "github.com/hoangtrungnguyen/grava/pkg/cmd/issues"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

const ts03ID = "TS-03"

func init() {
	Register(Scenario{
		ID:       ts03ID,
		Name:     "Dependency Graph Traversal Under Load",
		EpicGate: 4,
		Run:      runTS03,
	})
}

// runTS03 validates dependency graph traversal under concurrent load:
//   - Create a chain of 10 issues with sequential dependencies
//   - 20 concurrent ReadyQueue calls
//   - Only the leaf (unblocked) issue should appear in ready results
func runTS03(ctx context.Context, store dolt.Store) Result {
	const chainLen = 10
	const swarmSize = 20

	tag := fmt.Sprintf("ts03-%d", time.Now().UnixNano())
	var ids []string

	defer func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, id := range ids {
			_, _ = store.ExecContext(ctx2, "DELETE FROM issues WHERE id = ?", id)
			_, _ = store.ExecContext(ctx2, "DELETE FROM events WHERE issue_id = ?", id)
		}
		for i := 0; i < len(ids)-1; i++ {
			_, _ = store.ExecContext(ctx2, "DELETE FROM dependencies WHERE from_id = ? AND to_id = ?", ids[i+1], ids[i])
		}
	}()

	// Create chain: issue[0] ← issue[1] ← ... ← issue[9]
	// issue[i+1] blocks issue[i], so only issue[chainLen-1] is ready (no blockers)
	for i := 0; i < chainLen; i++ {
		created, err := issuesapi.CreateIssue(ctx, store, issuesapi.CreateParams{
			Title:     fmt.Sprintf("%s-chain-%d", tag, i),
			IssueType: "task",
			Priority:  "medium",
			Actor:     "sandbox",
		})
		if err != nil {
			return fail(ts03ID, fmt.Sprintf("setup: create issue %d: %v", i, err))
		}
		ids = append(ids, created.ID)
	}

	// Add dependencies: issue[i+1] blocks issue[i]
	for i := 0; i < chainLen-1; i++ {
		_, err := store.ExecContext(ctx,
			"INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by) VALUES (?, ?, 'blocks', 'sandbox', 'sandbox')",
			ids[i+1], ids[i])
		if err != nil {
			return fail(ts03ID, fmt.Sprintf("setup: add dependency %d→%d: %v", i+1, i, err))
		}
	}

	details := []string{fmt.Sprintf("created chain of %d issues with sequential deps", chainLen)}

	// Build set of blocked IDs (all except the leaf)
	blockedSet := make(map[string]struct{}, chainLen-1)
	for i := 0; i < chainLen-1; i++ {
		blockedSet[ids[i]] = struct{}{}
	}

	// Swarm: concurrent ReadyQueue calls
	type callResult struct {
		count  int
		taskIDs []string
		err    error
	}
	results := make([]callResult, swarmSize)
	var wg sync.WaitGroup

	start := time.Now()
	for i := 0; i < swarmSize; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			tasks, err := cmdgraph.ReadyQueue(callCtx, store, 200)
			var taskIDs []string
			for _, t := range tasks {
				if t.Node != nil {
					taskIDs = append(taskIDs, t.Node.ID)
				}
			}
			results[i] = callResult{count: len(tasks), taskIDs: taskIDs, err: err}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	if elapsed > 3*time.Second {
		return fail(ts03ID, fmt.Sprintf("swarm took %dms > 3000ms", elapsed.Milliseconds()), details...)
	}
	details = append(details, fmt.Sprintf("swarm completed in %dms", elapsed.Milliseconds()))

	// All calls should succeed
	errorCount := 0
	for _, r := range results {
		if r.err != nil {
			errorCount++
		}
	}
	if errorCount > 0 {
		return fail(ts03ID, fmt.Sprintf("%d/%d ReadyQueue calls failed", errorCount, swarmSize), details...)
	}
	details = append(details, fmt.Sprintf("all %d ReadyQueue calls succeeded", swarmSize))

	// Verify blocked issues never appear in ready queue results
	leaked := 0
	for _, r := range results {
		for _, id := range r.taskIDs {
			if _, blocked := blockedSet[id]; blocked {
				leaked++
			}
		}
	}
	if leaked > 0 {
		return fail(ts03ID,
			fmt.Sprintf("%d blocked issue(s) leaked into ReadyQueue responses", leaked),
			details...)
	}
	details = append(details, "no blocked issues appeared in ready queue results")

	return pass(ts03ID, details...)
}
