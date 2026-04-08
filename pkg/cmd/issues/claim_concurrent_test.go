//go:build integration

package issues

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dsn() string {
	if d := os.Getenv("GRAVA_TEST_DSN"); d != "" {
		return d
	}
	return "root@tcp(127.0.0.1:3311)/?parseTime=true"
}

func setupIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", dsn())
	require.NoError(t, err, "failed to connect to Dolt")
	t.Cleanup(func() { db.Close() })

	// Verify connection
	err = db.Ping()
	require.NoError(t, err, "failed to ping Dolt — is it running?")
	return db
}

func createTestIssue(t *testing.T, db *sql.DB) string {
	t.Helper()
	id := fmt.Sprintf("test-claim-%d", time.Now().UnixNano())
	_, err := db.Exec(`INSERT INTO issues (id, title, status, priority, issue_type) VALUES (?, 'concurrent claim test', 'open', 4, 'task')`, id)
	require.NoError(t, err)
	return id
}

func cleanupTestIssue(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	db.Exec(`DELETE FROM events WHERE issue_id = ?`, id)
	db.Exec(`DELETE FROM issues WHERE id = ?`, id)
}

func TestConcurrentClaim_ExactlyOneSucceeds(t *testing.T) {
	db := setupIntegrationDB(t)
	issueID := createTestIssue(t, db)
	defer cleanupTestIssue(t, db, issueID)

	store := dolt.NewClientFromDB(db)
	ctx := context.Background()

	var wg sync.WaitGroup
	results := make(chan error, 2)
	start := make(chan struct{})

	for _, actor := range []string{"agent-int-a", "agent-int-b"} {
		wg.Add(1)
		go func(actor string) {
			defer wg.Done()
			<-start // synchronize start
			_, err := claimIssue(ctx, store, issueID, actor, "test-model")
			results <- err
		}(actor)
	}

	close(start) // fire both goroutines simultaneously
	wg.Wait()
	close(results)

	var successes, failures int
	var failErr error
	for err := range results {
		if err == nil {
			successes++
		} else {
			failures++
			failErr = err
		}
	}

	assert.Equal(t, 1, successes, "exactly one claim should succeed")
	assert.Equal(t, 1, failures, "exactly one claim should fail")

	// Verify the failing error is ALREADY_CLAIMED
	require.NotNil(t, failErr, "expected a non-nil error from the failing claim")
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(failErr, &gravaErr), "error should be a GravaError")
	assert.Contains(t, []string{"ALREADY_CLAIMED", "DB_COMMIT_FAILED"}, gravaErr.Code)

	// Verify DB state: exactly one assignee, status is in_progress
	var assignee string
	var status string
	err := db.QueryRow(`SELECT assignee, status FROM issues WHERE id = ?`, issueID).Scan(&assignee, &status)
	require.NoError(t, err)
	assert.Equal(t, "in_progress", status)
	assert.NotEmpty(t, assignee, "assignee should be set")
}

// TestConcurrentClaim_FiveAgents_ExactlyOneSucceeds validates that SELECT FOR UPDATE
// serializes correctly under higher contention (5 concurrent claimers).
func TestConcurrentClaim_FiveAgents_ExactlyOneSucceeds(t *testing.T) {
	db := setupIntegrationDB(t)
	issueID := createTestIssue(t, db)
	defer cleanupTestIssue(t, db, issueID)

	store := dolt.NewClientFromDB(db)
	ctx := context.Background()

	const numAgents = 5
	var wg sync.WaitGroup
	results := make(chan error, numAgents)
	start := make(chan struct{})

	for i := 0; i < numAgents; i++ {
		actor := fmt.Sprintf("agent-int-%d", i)
		wg.Add(1)
		go func(actor string) {
			defer wg.Done()
			<-start
			_, err := claimIssue(ctx, store, issueID, actor, "test-model")
			results <- err
		}(actor)
	}

	close(start)
	wg.Wait()
	close(results)

	var successes, failures int
	for err := range results {
		if err == nil {
			successes++
		} else {
			failures++
			var gravaErr *gravaerrors.GravaError
			require.True(t, errors.As(err, &gravaErr), "error should be a GravaError")
			assert.Contains(t, []string{"ALREADY_CLAIMED", "DB_COMMIT_FAILED"}, gravaErr.Code)
		}
	}

	assert.Equal(t, 1, successes, "exactly one claim should succeed out of %d agents", numAgents)
	assert.Equal(t, numAgents-1, failures, "%d claims should fail", numAgents-1)

	// Verify DB state
	var assignee string
	var status string
	err := db.QueryRow(`SELECT assignee, status FROM issues WHERE id = ?`, issueID).Scan(&assignee, &status)
	require.NoError(t, err)
	assert.Equal(t, "in_progress", status)
	assert.NotEmpty(t, assignee, "assignee should be set")
}

// BenchmarkClaimIssue_Latency verifies NFR2: claim operation must complete in <15ms.
func BenchmarkClaimIssue_Latency(b *testing.B) {
	db, err := sql.Open("mysql", dsn())
	if err != nil {
		b.Fatalf("failed to connect to Dolt: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		b.Skip("Dolt not available — skipping benchmark")
	}

	store := dolt.NewClientFromDB(db)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		id := fmt.Sprintf("bench-claim-%d-%d", time.Now().UnixNano(), i)
		_, _ = db.Exec(`INSERT INTO issues (id, title, status, priority, issue_type) VALUES (?, 'bench claim', 'open', 4, 'task')`, id)
		b.StartTimer()

		_, err := claimIssue(ctx, store, id, "bench-actor", "bench-model")
		if err != nil {
			b.Fatalf("claim failed: %v", err)
		}

		b.StopTimer()
		db.Exec(`DELETE FROM events WHERE issue_id = ?`, id)
		db.Exec(`DELETE FROM issues WHERE id = ?`, id)
		b.StartTimer()
	}
}
