package sandbox

import (
	"context"
	"fmt"
	"sync"
	"time"

	issuesapi "github.com/hoangtrungnguyen/grava/pkg/cmd/issues"
	cmdgraph "github.com/hoangtrungnguyen/grava/pkg/cmd/graph"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/hoangtrungnguyen/grava/pkg/graph"
)

const ts02ID = "TS-02"

func init() {
	Register(Scenario{
		ID:       ts02ID,
		Name:     "Ready Queue Under Swarm Load",
		EpicGate: 4,
		Run:      runTS02,
	})
}

// runTS02 validates that under 30 concurrent ReadyQueue calls:
//   - No in_progress issue ever appears in any ready queue response.
//   - All responses complete within 2 seconds (no deadlock / lock starvation).
func runTS02(ctx context.Context, store dolt.Store) Result {
	const (
		totalIssues   = 50
		openCount     = 20
		swarmSize     = 30
		timeoutSec    = 10
		readyLimit    = 100
	)

	// --- Setup: seed test issues ---
	seededIDs := make([]string, 0, totalIssues)

	defer func() {
		// Cleanup all seeded issues regardless of test outcome.
		ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, id := range seededIDs {
			_, _ = store.ExecContext(ctx2, "DELETE FROM issues WHERE id = ?", id)
			_, _ = store.ExecContext(ctx2, "DELETE FROM events WHERE issue_id = ?", id)
		}
	}()

	tag := fmt.Sprintf("ts02-%d", time.Now().UnixNano())

	// Seed openCount open issues (visible to ready queue).
	for i := 0; i < openCount; i++ {
		created, err := issuesapi.CreateIssue(ctx, store, issuesapi.CreateParams{
			Title:     fmt.Sprintf("%s-open-%02d", tag, i),
			IssueType: "task",
			Priority:  "low",
			Actor:     "sandbox",
		})
		if err != nil {
			return fail(ts02ID, fmt.Sprintf("setup: create open issue %d: %v", i, err))
		}
		seededIDs = append(seededIDs, created.ID)
	}

	// Seed in_progress issues (must NOT appear in ready queue).
	inProgressIDs := make([]string, 0, totalIssues-openCount)
	for i := openCount; i < totalIssues; i++ {
		created, err := issuesapi.CreateIssue(ctx, store, issuesapi.CreateParams{
			Title:     fmt.Sprintf("%s-inprogress-%02d", tag, i),
			IssueType: "task",
			Priority:  "low",
			Actor:     "sandbox",
		})
		if err != nil {
			return fail(ts02ID, fmt.Sprintf("setup: create in_progress issue %d: %v", i, err))
		}
		seededIDs = append(seededIDs, created.ID)
		// Claim it so it becomes in_progress.
		_, err = issuesapi.ClaimIssue(ctx, store, created.ID, fmt.Sprintf("ts02-agent-%02d", i), "")
		if err != nil {
			return fail(ts02ID, fmt.Sprintf("setup: claim issue %s: %v", created.ID, err))
		}
		inProgressIDs = append(inProgressIDs, created.ID)
	}

	// Build a fast-lookup set of in_progress IDs.
	inProgressSet := make(map[string]struct{}, len(inProgressIDs))
	for _, id := range inProgressIDs {
		inProgressSet[id] = struct{}{}
	}

	details := []string{
		fmt.Sprintf("seeded %d open + %d in_progress issues", openCount, totalIssues-openCount),
	}

	// --- Swarm: 30 concurrent ReadyQueue calls ---
	type callResult struct {
		tasks []*graph.ReadyTask
		err   error
	}
	results := make([]callResult, swarmSize)
	var wg sync.WaitGroup

	start := time.Now()
	for i := 0; i < swarmSize; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			callCtx, cancel := context.WithTimeout(ctx, timeoutSec*time.Second)
			defer cancel()
			tasks, err := cmdgraph.ReadyQueue(callCtx, store, readyLimit)
			results[i] = callResult{tasks: tasks, err: err}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	// --- Assert: no deadlock ---
	if elapsed > 2*time.Second {
		return fail(ts02ID,
			fmt.Sprintf("swarm took %dms > 2000ms — possible lock starvation", elapsed.Milliseconds()),
			details...)
	}
	details = append(details, fmt.Sprintf("swarm completed in %dms", elapsed.Milliseconds()))

	// --- Assert: no call returned an error ---
	errorCount := 0
	for _, r := range results {
		if r.err != nil {
			errorCount++
		}
	}
	if errorCount > 0 {
		return fail(ts02ID,
			fmt.Sprintf("%d/%d ReadyQueue calls returned errors", errorCount, swarmSize),
			details...)
	}
	details = append(details, fmt.Sprintf("all %d ReadyQueue calls succeeded", swarmSize))

	// --- Assert: no in_progress issue appears in any ready queue response ---
	contaminated := 0
	for _, r := range results {
		for _, t := range r.tasks {
			if t.Node == nil {
				continue
			}
			if _, bad := inProgressSet[t.Node.ID]; bad {
				contaminated++
			}
		}
	}
	if contaminated > 0 {
		return fail(ts02ID,
			fmt.Sprintf("%d in_progress issue(s) leaked into ready queue responses", contaminated),
			details...)
	}
	details = append(details, "no in_progress issues appeared in any ready queue response")

	return pass(ts02ID, details...)
}
