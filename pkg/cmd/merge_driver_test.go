package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetMergeDriverFlags resets the merge-driver dry-run flag between tests.
func resetMergeDriverFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { mergeDriverDryRun = false })
}

func TestMergeDriverCmd_NoConflict(t *testing.T) {
	resetMergeDriverFlags(t)
	ancestor := writeTempFile(t, `{"id":"1","title":"A","status":"open"}`+"\n")
	current := writeTempFile(t, `{"id":"1","title":"A","status":"closed"}`+"\n")
	other := writeTempFile(t, `{"id":"1","title":"B","status":"open"}`+"\n")

	rootCmd.SetArgs([]string{"merge-driver", ancestor, current, other})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	result, err := os.ReadFile(current)
	require.NoError(t, err)
	body := string(result)
	assert.Contains(t, body, `"title":"B"`)
	assert.Contains(t, body, `"status":"closed"`)
}

func TestMergeDriverCmd_RequiresExactly3Args(t *testing.T) {
	resetMergeDriverFlags(t)
	rootCmd.SetArgs([]string{"merge-driver"})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 3 arg")
}

func TestMergeDriverCmd_ResultWrittenToCurrentFile(t *testing.T) {
	resetMergeDriverFlags(t)
	ancestor := writeTempFile(t, `{"id":"1","title":"Old"}`+"\n")
	current := writeTempFile(t, `{"id":"1","title":"Old","priority":"high"}`+"\n")
	other := writeTempFile(t, `{"id":"1","title":"New"}`+"\n")

	origCurrent, err := os.ReadFile(current)
	require.NoError(t, err)

	rootCmd.SetArgs([]string{"merge-driver", ancestor, current, other})
	err = rootCmd.Execute()
	assert.NoError(t, err)

	merged, err := os.ReadFile(current)
	require.NoError(t, err)

	assert.NotEqual(t, string(origCurrent), string(merged))
	assert.Contains(t, string(merged), `"title":"New"`)
	assert.Contains(t, string(merged), `"priority":"high"`)
}

func TestMergeDriverCmd_DryRun_NoFileWrite(t *testing.T) {
	resetMergeDriverFlags(t)
	ancestor := writeTempFile(t, `{"id":"1","title":"Old"}`+"\n")
	currentContent := `{"id":"1","title":"Old","priority":"high"}` + "\n"
	current := writeTempFile(t, currentContent)
	other := writeTempFile(t, `{"id":"1","title":"New"}`+"\n")

	rootCmd.SetArgs([]string{"merge-driver", "--dry-run", ancestor, current, other})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	// File must NOT have been modified in dry-run mode.
	afterContent, err := os.ReadFile(current)
	require.NoError(t, err)
	assert.Equal(t, currentContent, string(afterContent), "dry-run must not modify the current file")
}

func TestMergeDriverCmd_DryRun_OutputsMergedContent(t *testing.T) {
	resetMergeDriverFlags(t)
	ancestor := writeTempFile(t, `{"id":"1","title":"A","status":"open"}`+"\n")
	current := writeTempFile(t, `{"id":"1","title":"A","status":"closed"}`+"\n")
	other := writeTempFile(t, `{"id":"1","title":"B","status":"open"}`+"\n")

	var out strings.Builder
	rootCmd.SetOut(&out)
	t.Cleanup(func() { rootCmd.SetOut(nil) })

	rootCmd.SetArgs([]string{"merge-driver", "--dry-run", ancestor, current, other})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, `"title":"B"`, "dry-run output should contain merged title")
	assert.Contains(t, output, `"status":"closed"`, "dry-run output should contain merged status")
}

func TestMergeDriverCmd_ConflictWritesMarkers(t *testing.T) {
	resetMergeDriverFlags(t)
	ancestor := writeTempFile(t, `{"id":"1","title":"A"}`+"\n")
	current := writeTempFile(t, `{"id":"1","title":"X"}`+"\n")
	other := writeTempFile(t, `{"id":"1","title":"Y"}`+"\n")

	origExit := conflictExitFn
	t.Cleanup(func() { conflictExitFn = origExit })
	exited := false
	conflictExitFn = func(code int) { exited = true }

	rootCmd.SetArgs([]string{"merge-driver", ancestor, current, other})
	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.True(t, exited, "conflict exit should have been called")

	result, err := os.ReadFile(current)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(result), `"_conflict":true`),
		"merged file should contain conflict markers")
}

func TestMergeDriverCmd_DryRunConflict_NoExit(t *testing.T) {
	resetMergeDriverFlags(t)
	ancestor := writeTempFile(t, `{"id":"1","title":"A"}`+"\n")
	currentContent := `{"id":"1","title":"X"}` + "\n"
	current := writeTempFile(t, currentContent)
	other := writeTempFile(t, `{"id":"1","title":"Y"}`+"\n")

	origExit := conflictExitFn
	t.Cleanup(func() { conflictExitFn = origExit })
	exited := false
	conflictExitFn = func(code int) { exited = true }

	rootCmd.SetArgs([]string{"merge-driver", "--dry-run", ancestor, current, other})
	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.False(t, exited, "dry-run with conflict must not call exit")

	// File untouched in dry-run.
	afterContent, err := os.ReadFile(current)
	require.NoError(t, err)
	assert.Equal(t, currentContent, string(afterContent))
}

func TestMergeDriverCmd_CleanAddOnOneSide(t *testing.T) {
	// AC: issues that exist only in one side (clean add) are passed through without conflict.
	resetMergeDriverFlags(t)
	ancestor := writeTempFile(t, "")
	current := writeTempFile(t, `{"id":"issue-from-current","title":"Only on current"}`+"\n")
	other := writeTempFile(t, "")

	rootCmd.SetArgs([]string{"merge-driver", ancestor, current, other})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	result, err := os.ReadFile(current)
	require.NoError(t, err)
	assert.Contains(t, string(result), "issue-from-current", "clean-add issue must be in merged output")
	assert.NotContains(t, string(result), `"_conflict"`, "clean-add must not produce conflict markers")
}

func TestMergeDriverCmd_NonExistentAncestor(t *testing.T) {
	// os.ReadFile error is propagated with file path in error message.
	resetMergeDriverFlags(t)
	current := writeTempFile(t, `{"id":"1"}`+"\n")
	other := writeTempFile(t, `{"id":"1"}`+"\n")

	rootCmd.SetArgs([]string{"merge-driver", "/nonexistent/ancestor.jsonl", current, other})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "/nonexistent/ancestor.jsonl")
}
