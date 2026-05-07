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

const ts10ID = "TS-10"

func init() {
	Register(Scenario{
		ID:       ts10ID,
		Name:     "Rapid Swarm Claims",
		EpicGate: 5,
		Run:      runTS10,
	})
}

// runTS10 validates claim atomicity under high contention:
//   - Create 20 issues
//   - 50 concurrent agents each try to claim a random issue
//   - Verify: no double claims, all claims return valid results
func runTS10(ctx context.Context, store dolt.Store) Result {
	const issueCount = 20
	const agentCount = 50

	tag := fmt.Sprintf("ts10-%d", time.Now().UnixNano())
	var ids []string

	defer func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		for _, id := range ids {
			_, _ = store.ExecContext(ctx2, "DELETE FROM issues WHERE id = ?", id)
			_, _ = store.ExecContext(ctx2, "DELETE FROM events WHERE issue_id = ?", id)
		}
	}()

	// Create issues
	for i := 0; i < issueCount; i++ {
		created, err := issuesapi.CreateIssue(ctx, store, issuesapi.CreateParams{
			Title:     fmt.Sprintf("%s-issue-%d", tag, i),
			IssueType: "task",
			Priority:  "low",
			Actor:     "sandbox",
		})
		if err != nil {
			return fail(ts10ID, fmt.Sprintf("setup: create issue %d: %v", i, err))
		}
		ids = append(ids, created.ID)
	}

	details := []string{fmt.Sprintf("created %d issues for %d agents", issueCount, agentCount)}

	// Swarm: each agent tries to claim issue[i % issueCount]
	// Track per-issue claim winners to detect double-claims.
	type claimAttempt struct {
		targetIdx int
		agent     string
		err       error
	}
	results := make([]claimAttempt, agentCount)
	var successCount int64
	var failCount int64
	var wg sync.WaitGroup

	start := time.Now()
	for i := 0; i < agentCount; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			targetIdx := i % issueCount
			agent := fmt.Sprintf("ts10-agent-%02d", i)
			claimCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			_, cerr := issuesapi.ClaimIssue(claimCtx, store, ids[targetIdx], agent, "")
			results[i] = claimAttempt{targetIdx: targetIdx, agent: agent, err: cerr}
			if cerr == nil {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&failCount, 1)
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	if elapsed > 10*time.Second {
		return fail(ts10ID, fmt.Sprintf("swarm took %dms > 10000ms", elapsed.Milliseconds()), details...)
	}
	details = append(details, fmt.Sprintf("swarm completed in %dms", elapsed.Milliseconds()))

	sc := atomic.LoadInt64(&successCount)
	fc := atomic.LoadInt64(&failCount)
	details = append(details, fmt.Sprintf("claims: %d succeeded, %d failed", sc, fc))

	// Per-issue uniqueness check: verify no issue was claimed by more than one agent.
	claimedBy := make(map[int]string) // targetIdx -> winning agent
	doubleClaims := 0
	for _, r := range results {
		if r.err != nil {
			continue
		}
		if prev, exists := claimedBy[r.targetIdx]; exists {
			doubleClaims++
			details = append(details, fmt.Sprintf("DOUBLE CLAIM: issue[%d] claimed by both %s and %s", r.targetIdx, prev, r.agent))
		} else {
			claimedBy[r.targetIdx] = r.agent
		}
	}
	if doubleClaims > 0 {
		return fail(ts10ID,
			fmt.Sprintf("%d per-issue double claim(s) detected", doubleClaims),
			details...)
	}
	details = append(details, "no per-issue double claims detected")

	// At least some claims should succeed
	if sc == 0 {
		return fail(ts10ID, "no claims succeeded — possible deadlock or error", details...)
	}

	return pass(ts10ID, details...)
}
