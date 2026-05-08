package cmd

import (
	"bytes"
	"strings"
	"testing"

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
