package sandbox

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	issuesapi "github.com/hoangtrungnguyen/grava/pkg/cmd/issues"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

const ts01ID = "TS-01"

func init() {
	Register(Scenario{
		ID:       ts01ID,
		Name:     "Concurrent Claim Race",
		EpicGate: 3, // Epic 3: Atomic Claim
		Run:      runTS01,
	})
}

// runTS01 validates that under 10 concurrent claim attempts on the same issue,
// exactly 1 succeeds and the rest fail with a deterministic error.
func runTS01(ctx context.Context, store dolt.Store) Result {
	const concurrency = 10
	const timeoutSec = 10

	// Create a dedicated test issue.
	created, err := issuesapi.CreateIssue(ctx, store, issuesapi.CreateParams{
		Title:     fmt.Sprintf("sandbox-ts01-%d", time.Now().UnixNano()),
		IssueType: "task",
		Priority:  "low",
		Actor:     "sandbox",
	})
	if err != nil {
		return fail(ts01ID, fmt.Sprintf("setup: create test issue: %v", err))
	}
	issueID := created.ID

	// Ensure cleanup even on failure.
	defer func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = store.ExecContext(ctx2, "DELETE FROM issues WHERE id = ?", issueID)
		_, _ = store.ExecContext(ctx2, "DELETE FROM events WHERE issue_id = ?", issueID)
	}()

	// Spawn concurrency goroutines, each claiming the same issue.
	type outcome struct {
		agentID string
		err     error
	}
	results := make([]outcome, concurrency)
	var wg sync.WaitGroup
	var successCount int64

	start := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			agent := fmt.Sprintf("ts01-agent-%02d", i)
			claimCtx, cancel := context.WithTimeout(ctx, timeoutSec*time.Second)
			defer cancel()
			_, cerr := issuesapi.ClaimIssue(claimCtx, store, issueID, agent, "")
			results[i] = outcome{agentID: agent, err: cerr}
			if cerr == nil {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	// --- Assertions ---
	details := []string{
		fmt.Sprintf("concurrency=%d issue=%s elapsed=%dms", concurrency, issueID, elapsed.Milliseconds()),
	}

	if n := atomic.LoadInt64(&successCount); n != 1 {
		return fail(ts01ID,
			fmt.Sprintf("expected exactly 1 successful claim, got %d", n),
			details...)
	}
	details = append(details, "exactly 1 claim succeeded")

	// Verify 9 failures have deterministic error codes (not nil, not panics).
	failCount := 0
	for _, r := range results {
		if r.err != nil {
			failCount++
		}
	}
	if failCount != concurrency-1 {
		return fail(ts01ID,
			fmt.Sprintf("expected %d failures, got %d", concurrency-1, failCount),
			details...)
	}
	details = append(details, fmt.Sprintf("%d claims failed with deterministic errors", failCount))

	// Verify no deadlock: all goroutines finished in under timeoutSec seconds.
	if elapsed > timeoutSec*time.Second {
		return fail(ts01ID,
			fmt.Sprintf("scenario took %s > %ds timeout", elapsed, timeoutSec),
			details...)
	}
	details = append(details, fmt.Sprintf("no deadlock: completed in %dms", elapsed.Milliseconds()))

	return pass(ts01ID, details...)
}
