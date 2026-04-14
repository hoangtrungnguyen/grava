package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetMergeFlags resets merge-slot flag vars to their defaults between tests.
func resetMergeFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		mergeAncestor = ""
		mergeCurrent = ""
		mergeOther = ""
	})
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "merge-*.jsonl")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func TestMergeSlotCmd_NoConflict(t *testing.T) {
	resetMergeFlags(t)
	ancestor := writeTempFile(t, `{"id":"1","title":"A","status":"open"}`+"\n")
	current := writeTempFile(t, `{"id":"1","title":"A","status":"closed"}`+"\n")
	other := writeTempFile(t, `{"id":"1","title":"B","status":"open"}`+"\n")

	rootCmd.SetArgs([]string{
		"merge-slot",
		"--ancestor", ancestor,
		"--current", current,
		"--other", other,
	})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	result, err := os.ReadFile(current)
	require.NoError(t, err)
	body := string(result)
	assert.Contains(t, body, `"title":"B"`)
	assert.Contains(t, body, `"status":"closed"`)
}

func TestMergeSlotCmd_MissingFlags(t *testing.T) {
	resetMergeFlags(t)
	rootCmd.SetArgs([]string{"merge-slot"})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestMergeSlotCmd_ResultWrittenToCurrentFile(t *testing.T) {
	resetMergeFlags(t)
	// Verifies that the merged output is written back to the --current path
	ancestor := writeTempFile(t, `{"id":"1","title":"Old"}`+"\n")
	current := writeTempFile(t, `{"id":"1","title":"Old","priority":"high"}`+"\n")
	other := writeTempFile(t, `{"id":"1","title":"New"}`+"\n")

	origCurrent, err := os.ReadFile(current)
	require.NoError(t, err)

	rootCmd.SetArgs([]string{
		"merge-slot",
		"--ancestor", ancestor,
		"--current", current,
		"--other", other,
	})
	err = rootCmd.Execute()
	assert.NoError(t, err)

	merged, err := os.ReadFile(current)
	require.NoError(t, err)

	// File must have been overwritten with merged content
	assert.NotEqual(t, string(origCurrent), string(merged))
	assert.Contains(t, string(merged), `"title":"New"`)
	assert.Contains(t, string(merged), `"priority":"high"`)
}

func TestMergeSlotCmd_ConflictWritesConflictMarkers(t *testing.T) {
	resetMergeFlags(t)
	// When conflicts exist, the %A file must contain conflict markers
	// (the command exits 1, but we test the file content separately
	// since os.Exit cannot be intercepted in unit tests).
	ancestor := writeTempFile(t, `{"id":"1","title":"A"}`+"\n")
	current := writeTempFile(t, `{"id":"1","title":"X"}`+"\n")
	other := writeTempFile(t, `{"id":"1","title":"Y"}`+"\n")

	// Swap in a test-friendly version of the conflict exit so we can assert
	// file content without the process terminating.
	origExit := conflictExitFn
	t.Cleanup(func() { conflictExitFn = origExit })
	exited := false
	conflictExitFn = func(code int) { exited = true }

	rootCmd.SetArgs([]string{
		"merge-slot",
		"--ancestor", ancestor,
		"--current", current,
		"--other", other,
	})
	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.True(t, exited, "conflict exit should have been called")

	result, err := os.ReadFile(current)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(result), `"_conflict":true`),
		"merged file should contain conflict markers")
}
