package cmd

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeStopCmd builds a minimal cobra.Command that runDBStop can use for
// its Printf side-effects. It is NOT the real db-stop cobra binding —
// runDBStop only consumes Printf, so this stand-in keeps the tests
// independent of root cobra wiring.
func makeStopCmd() (*cobra.Command, *bytes.Buffer) {
	c := &cobra.Command{Use: "db-stop"}
	buf := &bytes.Buffer{}
	c.SetOut(buf)
	c.SetErr(buf)
	return c, buf
}

// withStubs swaps the package-level seams (lsof, kill, in-flight count)
// for the duration of a test and restores them on cleanup.
func withStubs(
	t *testing.T,
	pidsByCall [][]string,
	pidsErr error,
	inFlightByCall []struct {
		n  int
		ok bool
	},
	killRecorder *[]string,
	killErr error,
) {
	t.Helper()

	origLsof := lsofPidsFn
	origKill := killPidFn
	origInFlight := inFlightCountFn

	lsofCall := 0
	lsofPidsFn = func(port int) ([]string, error) {
		// Each call returns the next slot, sticking on the last entry
		// once the script runs out (so the post-kill polling loop in
		// runDBStop sees the final state repeatedly).
		idx := lsofCall
		if idx >= len(pidsByCall) {
			idx = len(pidsByCall) - 1
		}
		lsofCall++
		if pidsErr != nil && lsofCall == 1 {
			return nil, pidsErr
		}
		return pidsByCall[idx], nil
	}

	inFlightCall := 0
	inFlightCountFn = func() (int, bool) {
		idx := inFlightCall
		if idx >= len(inFlightByCall) {
			idx = len(inFlightByCall) - 1
		}
		inFlightCall++
		v := inFlightByCall[idx]
		return v.n, v.ok
	}

	killPidFn = func(pid string) error {
		if killRecorder != nil {
			*killRecorder = append(*killRecorder, pid)
		}
		return killErr
	}

	t.Cleanup(func() {
		lsofPidsFn = origLsof
		killPidFn = origKill
		inFlightCountFn = origInFlight
	})
}

// --- TOCTOU re-check tests ---

// TestDBStop_RecheckInflightBeforeKill is the headline TOCTOU test.
// Simulates: entry check sees 0 in-flight (nothing claimed yet), then
// between the lsof call and the kill a /ship run claims an issue, so
// the second check sees 1. runDBStop must abort with the "became
// in_progress during teardown" error and must NOT call kill.
func TestDBStop_RecheckInflightBeforeKill(t *testing.T) {
	cmd, buf := makeStopCmd()
	var killed []string
	withStubs(
		t,
		[][]string{{"12345"}}, // lsof returns one PID
		nil,
		[]struct {
			n  int
			ok bool
		}{
			{0, true}, // entry check: clear
			{1, true}, // re-check just before kill: a claim landed
		},
		&killed,
		nil,
	)

	err := runDBStop(cmd, 3306, false)
	require.Error(t, err, "must refuse to kill when re-check sees a fresh claim")
	assert.Contains(t, err.Error(), "became status=in_progress during teardown")
	assert.Empty(t, killed, "kill must NOT be called when re-check fails")
	// Output should NOT contain the success line.
	assert.NotContains(t, buf.String(), "Dolt server stopped")
}

// TestDBStop_BothChecksClear_ProceedsToKill confirms the happy path:
// when both pre-lsof and post-lsof checks return 0, kill runs.
func TestDBStop_BothChecksClear_ProceedsToKill(t *testing.T) {
	cmd, _ := makeStopCmd()
	var killed []string
	withStubs(
		t,
		[][]string{
			{"12345"}, // first lsof: PID present
			nil,       // post-kill polling: port empty
		},
		nil,
		[]struct {
			n  int
			ok bool
		}{
			{0, true},
			{0, true},
		},
		&killed,
		nil,
	)

	err := runDBStop(cmd, 3306, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"12345"}, killed)
}

// TestDBStop_EntryCheckBlocks confirms the existing concurrency-matrix
// #4 baseline still holds: when the entry check sees in-flight > 0,
// we refuse before even invoking lsof.
func TestDBStop_EntryCheckBlocks(t *testing.T) {
	cmd, _ := makeStopCmd()
	var killed []string
	withStubs(
		t,
		[][]string{{"12345"}},
		nil,
		[]struct {
			n  int
			ok bool
		}{
			{2, true}, // entry: 2 in-flight
		},
		&killed,
		nil,
	)

	err := runDBStop(cmd, 3306, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing to stop: 2 issue(s) are status=in_progress")
	assert.Empty(t, killed, "kill must not run when entry check blocks")
}

// TestDBStop_ForceBypassesBothChecks confirms --force still works:
// even when in-flight > 0 on both calls, we proceed to kill.
func TestDBStop_ForceBypassesBothChecks(t *testing.T) {
	cmd, _ := makeStopCmd()
	var killed []string
	withStubs(
		t,
		[][]string{
			{"12345"},
			nil,
		},
		nil,
		[]struct {
			n  int
			ok bool
		}{
			{5, true},
			{5, true},
		},
		&killed,
		nil,
	)

	err := runDBStop(cmd, 3306, true) // force = true
	require.NoError(t, err)
	assert.Equal(t, []string{"12345"}, killed, "--force must skip both guard checks")
}

// TestDBStop_DBUnreachableSkipsGuard confirms best-effort behavior: if
// the DB is already down (inFlightCountFn returns ok=false), we proceed
// without blocking. This preserves the documented "if the DB is
// unreachable, skip the check" semantics.
func TestDBStop_DBUnreachableSkipsGuard(t *testing.T) {
	cmd, _ := makeStopCmd()
	var killed []string
	withStubs(
		t,
		[][]string{
			{"12345"},
			nil,
		},
		nil,
		[]struct {
			n  int
			ok bool
		}{
			{0, false}, // DB down
			{0, false},
		},
		&killed,
		nil,
	)

	err := runDBStop(cmd, 3306, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"12345"}, killed)
}

// --- Multi-PID tests ---

// TestDBStop_MultiplePIDs is the headline multi-PID test. lsof returns
// two PIDs (the bug case where TrimSpace alone left an embedded
// newline); runDBStop must call kill for each one, not pass the
// concatenated string.
func TestDBStop_MultiplePIDs(t *testing.T) {
	cmd, _ := makeStopCmd()
	var killed []string
	withStubs(
		t,
		[][]string{
			{"12345", "67890"}, // two PIDs on the same port
			nil,                // both gone after kill
		},
		nil,
		[]struct {
			n  int
			ok bool
		}{
			{0, true},
			{0, true},
		},
		&killed,
		nil,
	)

	err := runDBStop(cmd, 3306, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"12345", "67890"}, killed,
		"every PID returned by lsof must be killed individually")
}

// TestDBStop_MultiplePIDsOneKillFails confirms partial-failure
// behavior: one PID failing to receive SIGTERM does not prevent the
// kill loop from attempting the others. The function still reports an
// error if the port never clears.
func TestDBStop_MultiplePIDsOneKillFails(t *testing.T) {
	cmd, buf := makeStopCmd()
	var killed []string

	origLsof := lsofPidsFn
	origKill := killPidFn
	origInFlight := inFlightCountFn
	t.Cleanup(func() {
		lsofPidsFn = origLsof
		killPidFn = origKill
		inFlightCountFn = origInFlight
	})

	lsofCall := 0
	lsofPidsFn = func(port int) ([]string, error) {
		lsofCall++
		if lsofCall <= 2 {
			// First call: list PIDs. Second call (re-check happens via
			// inFlightCountFn, not lsof — but the post-kill polling
			// loop also calls lsof). Return a stuck PID for one
			// polling iteration to force the timeout path.
			return []string{"12345", "67890"}, nil
		}
		return []string{"12345"}, nil // 67890 died, 12345 stuck
	}

	inFlightCountFn = func() (int, bool) { return 0, true }

	killPidFn = func(pid string) error {
		killed = append(killed, pid)
		if pid == "12345" {
			return assert.AnError // simulated kill failure
		}
		return nil
	}

	err := runDBStop(cmd, 3306, false)
	// Stuck PID 12345 means port never clears → timeout error.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stop server within timeout")
	assert.Contains(t, err.Error(), "12345")
	// Both PIDs should have been attempted regardless of the first failure.
	assert.Equal(t, []string{"12345", "67890"}, killed,
		"a single failed kill must not abort the loop")
	// Warning should be emitted for the failed kill.
	assert.Contains(t, buf.String(), "failed to kill pid 12345")
}

// TestDBStop_NoPIDs is the no-op case: lsof returns nothing, runDBStop
// prints the friendly message and returns nil without calling kill.
func TestDBStop_NoPIDs(t *testing.T) {
	cmd, buf := makeStopCmd()
	var killed []string
	withStubs(
		t,
		[][]string{nil}, // lsof: nothing listening
		nil,
		[]struct {
			n  int
			ok bool
		}{
			{0, true},
		},
		&killed,
		nil,
	)

	err := runDBStop(cmd, 3306, false)
	require.NoError(t, err)
	assert.Empty(t, killed)
	assert.True(t, strings.Contains(buf.String(), "No process found"),
		"should print the no-process message")
}

// --- inFlightCountFn SQL filter tests (grava-ad3b) ---
//
// These tests pin the SQL the live inFlightCountFn issues against the
// database. They verify the heartbeat-freshness filter is part of the
// query so that crashed agents (status=in_progress with stale heartbeat)
// no longer block db-stop. The earlier withStubs-based tests stub the
// function variable wholesale and therefore can't observe the SQL — that's
// why we go through connectDBFn here, the same pattern used in
// sync_status_test.go.
//
// The SQL must mirror the stale-detection semantics in claim.go (1h TTL)
// and the existing graph.go stale-in_progress count. Both use
// COALESCE(wisp_heartbeat_at, updated_at) so a row with NULL heartbeat
// (newly claimed, no wisp written yet) falls back to updated_at — which
// is set to NOW() by the claim — keeping the conservative behavior of
// blocking db-stop while a fresh claim hasn't yet emitted its first
// heartbeat.

// expectedInFlightSQL is the exact statement inFlightCountFn must
// execute, written out in full (not bound to inFlightCountQuery) so this
// test acts as an independent contract pin: a refactor that silently
// changes the live SQL — e.g. dropping the ephemeral=0 filter or swapping
// the COALESCE order — is caught here, not just in code review. Update
// this literal in lock step with db_server.go *and* claim.go's 1h TTL
// (see comment on staleClaimTTLSQL).
const expectedInFlightSQL = `SELECT COUNT(*) FROM issues WHERE ephemeral = 0 AND status = 'in_progress' AND COALESCE(wisp_heartbeat_at, updated_at) > DATE_SUB(NOW(), INTERVAL 1 HOUR)`

// withSQLMockedInflight points connectDBFn at a sqlmock-backed Store and
// restores it on cleanup. Returns the mock so the caller can program
// expected queries / responses.
func withSQLMockedInflight(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() }) //nolint:errcheck

	orig := connectDBFn
	connectDBFn = func() (dolt.Store, error) { return dolt.NewClientFromDB(db), nil }
	t.Cleanup(func() { connectDBFn = orig })
	return mock
}

// TestInFlightCount_StaleClaimDoesNotBlock asserts the freshness filter
// applies: when the only in_progress row has a heartbeat older than 1h,
// the SQL filter excludes it and inFlightCountFn returns 0. db-stop will
// then proceed without --force, which is the whole point of grava-ad3b.
func TestInFlightCount_StaleClaimDoesNotBlock(t *testing.T) {
	mock := withSQLMockedInflight(t)

	// Mock returns 0: the SQL filter excluded the stale row.
	// The SQL itself does the time math via DATE_SUB(NOW(), INTERVAL 1 HOUR);
	// we can't fake "now" inside sqlmock so we trust the SQL filter and
	// just observe what the query returns. The query string assertion is
	// the load-bearing check.
	mock.ExpectQuery(regexp.QuoteMeta(expectedInFlightSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	n, ok := inFlightCountFn()
	require.True(t, ok, "ok=true expected when DB query succeeds")
	assert.Equal(t, 0, n, "stale in_progress rows must be excluded by the SQL filter")
	assert.NoError(t, mock.ExpectationsWereMet(),
		"inFlightCountFn must execute the heartbeat-filtered SQL exactly")
}

// TestInFlightCount_FreshClaimBlocks: an active /ship run keeps wisp
// heartbeats fresh, so the SQL filter includes that row and the count is
// non-zero. db-stop then refuses (without --force).
func TestInFlightCount_FreshClaimBlocks(t *testing.T) {
	mock := withSQLMockedInflight(t)

	mock.ExpectQuery(regexp.QuoteMeta(expectedInFlightSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	n, ok := inFlightCountFn()
	require.True(t, ok)
	assert.Equal(t, 1, n, "fresh in_progress claim must be counted, blocking db-stop")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestInFlightCount_NullHeartbeatBlocks pins the conservative semantics
// for newly-claimed issues that haven't yet written their first
// orchestrator_heartbeat wisp. claim.go sets updated_at=NOW(), so the
// COALESCE branch falls through to updated_at and the row is still inside
// the 1h window. The mock simulates that "row matches the filter" outcome
// (count=1). False-positive blocks are preferable to false-negative
// unblocks here — losing a freshly-claimed run mid-bootstrap is worse
// than waiting an hour to db-stop.
func TestInFlightCount_NullHeartbeatBlocks(t *testing.T) {
	mock := withSQLMockedInflight(t)

	mock.ExpectQuery(regexp.QuoteMeta(expectedInFlightSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	n, ok := inFlightCountFn()
	require.True(t, ok)
	assert.Equal(t, 1, n,
		"row with NULL heartbeat but recent updated_at must still be counted "+
			"(COALESCE fallback keeps the conservative behavior)")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestInFlightCount_DBUnreachable preserves the best-effort contract:
// when the DB is down (connectDBFn errors), inFlightCountFn returns
// (0,false) and the caller skips the guard. This test guards against a
// regression where the SQL filter rewrite accidentally changes the
// failure-mode return tuple.
func TestInFlightCount_DBUnreachable(t *testing.T) {
	orig := connectDBFn
	connectDBFn = func() (dolt.Store, error) { return nil, assert.AnError }
	t.Cleanup(func() { connectDBFn = orig })

	n, ok := inFlightCountFn()
	assert.False(t, ok, "ok=false when DB unreachable, so guard is skipped")
	assert.Equal(t, 0, n)
}
