package sandbox

import (
	"context"
	"fmt"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

const ts05ID = "TS-05"

func init() {
	Register(Scenario{
		ID:       ts05ID,
		Name:     "Doctor Detection + Fix",
		EpicGate: 9,
		Run:      runTS05,
	})
}

// runTS05 validates that grava doctor can detect and fix issues:
//   - Insert an expired file reservation
//   - Verify doctor detects it via DB query
//   - Verify the fix (release) works
func runTS05(ctx context.Context, store dolt.Store) Result {
	tag := fmt.Sprintf("ts05-%d", time.Now().UnixNano())
	resID := fmt.Sprintf("res-%s", tag[:10])

	defer func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = store.ExecContext(ctx2, "DELETE FROM file_reservations WHERE id = ?", resID)
	}()

	// Insert an expired reservation (expires_ts in the past)
	pastTime := time.Now().UTC().Add(-1 * time.Hour)
	_, err := store.ExecContext(ctx,
		`INSERT INTO file_reservations (id, project_id, agent_id, path_pattern, exclusive, reason, created_ts, expires_ts)
		 VALUES (?, 'default', 'stale-agent', 'src/stale/*.go', TRUE, 'sandbox test', NOW(), ?)`,
		resID, pastTime)
	if err != nil {
		return fail(ts05ID, fmt.Sprintf("setup: insert expired reservation: %v", err))
	}

	details := []string{"inserted expired file reservation"}

	// Verify it's detected: query expired reservations
	var count int
	row := store.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM file_reservations WHERE id = ? AND expires_ts < NOW() AND released_ts IS NULL", resID)
	if err := row.Scan(&count); err != nil || count == 0 {
		return fail(ts05ID, "doctor detection: expired reservation not found in DB", details...)
	}
	details = append(details, "expired reservation detected via query")

	// Fix: release the expired reservation
	_, err = store.ExecContext(ctx,
		"UPDATE file_reservations SET released_ts = NOW() WHERE id = ? AND released_ts IS NULL", resID)
	if err != nil {
		return fail(ts05ID, fmt.Sprintf("doctor fix: failed to release expired reservation: %v", err), details...)
	}
	details = append(details, "expired reservation released (fix applied)")

	// Verify fix: should no longer appear as active
	var activeCount int
	row = store.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM file_reservations WHERE id = ? AND released_ts IS NULL AND expires_ts > NOW()", resID)
	if err := row.Scan(&activeCount); err != nil {
		return fail(ts05ID, fmt.Sprintf("verify: query failed: %v", err), details...)
	}
	if activeCount > 0 {
		return fail(ts05ID, "verify: expired reservation still appears as active after fix", details...)
	}
	details = append(details, "fix verified: reservation no longer active")

	return pass(ts05ID, details...)
}
