package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- hookRunCmd tests ---

func TestHookRunCmd_UnknownHookExitsZero(t *testing.T) {
	rootCmd.SetArgs([]string{"hook", "run", "unknown-hook"})
	assert.NoError(t, rootCmd.Execute())
}

func TestHookRunCmd_PrepareCommitMsgNoOp(t *testing.T) {
	rootCmd.SetArgs([]string{"hook", "run", "prepare-commit-msg"})
	assert.NoError(t, rootCmd.Execute())
}

func TestHookRunCmd_PreCommitNoIssuesFile(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// issues.jsonl does not exist — pre-commit should exit 0
	rootCmd.SetArgs([]string{"hook", "run", "pre-commit"})
	assert.NoError(t, rootCmd.Execute())
}

func TestHookRunCmd_PreCommitValidFile(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	valid := `{"type":"issue","data":{"id":"abc123","title":"Test"}}` + "\n"
	require.NoError(t, os.WriteFile("issues.jsonl", []byte(valid), 0644))

	rootCmd.SetArgs([]string{"hook", "run", "pre-commit"})
	assert.NoError(t, rootCmd.Execute())
}

func TestHookRunCmd_PreCommitInvalidFile(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// Invalid JSON
	require.NoError(t, os.WriteFile("issues.jsonl", []byte("{broken json\n"), 0644))

	rootCmd.SetArgs([]string{"hook", "run", "pre-commit"})
	assert.Error(t, rootCmd.Execute(), "pre-commit should fail on malformed JSONL")
}

func TestHookRunCmd_PostMergeNoIssuesFile(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// No issues.jsonl — post-merge should exit 0 (nothing to sync)
	rootCmd.SetArgs([]string{"hook", "run", "post-merge"})
	assert.NoError(t, rootCmd.Execute())
}

func TestHookRunCmd_PostCheckoutNoArgs(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// Called without prev/new HEAD args — should exit 0 gracefully
	rootCmd.SetArgs([]string{"hook", "run", "post-checkout"})
	assert.NoError(t, rootCmd.Execute())
}

func TestHookRunCmd_PostCheckoutSameRefs(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// When prev==new, issues.jsonl hasn't changed — should exit 0 without syncing
	rootCmd.SetArgs([]string{"hook", "run", "post-checkout", "abc123", "abc123", "1"})
	assert.NoError(t, rootCmd.Execute())
}

// --- issuesChangedInCheckout ---

func TestIssuesChangedInCheckout_ReturnsFalseOnBadRefs(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// Invalid refs — git diff errors — should return false (safe default)
	changed := issuesChangedInCheckout("not-a-ref", "also-not-a-ref")
	assert.False(t, changed)
}

// --- issuesChangedInMerge ---

func TestIssuesChangedInMerge_ReturnsFalseWhenNoFile(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// No issues.jsonl in a fresh repo with no commits — issuesChangedInMerge
	// falls back to os.Stat; file absent → false.
	changed := issuesChangedInMerge()
	assert.False(t, changed)
}

func TestIssuesChangedInMerge_ReturnsTrueWhenFileExists(t *testing.T) {
	dir, cleanup := initTempGitRepo(t)
	defer cleanup()

	// Create issues.jsonl — issuesChangedInMerge fallback returns true when
	// HEAD@{1} doesn't exist (fresh repo) but the file is present.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "issues.jsonl"), []byte(""), 0644))

	changed := issuesChangedInMerge()
	assert.True(t, changed)
}

// --- hook command wiring ---

func TestHookCmd_IsRegistered(t *testing.T) {
	var found bool
	for _, c := range rootCmd.Commands() {
		if c.Name() == "hook" {
			found = true
			break
		}
	}
	assert.True(t, found, "hook command should be registered on rootCmd")
}

func TestHookRunCmd_IsRegistered(t *testing.T) {
	var found bool
	for _, c := range hookCmd.Commands() {
		if c.Name() == "run" {
			found = true
			break
		}
	}
	assert.True(t, found, "run subcommand should be registered on hookCmd")
}

// --- syncFromFile gracefully handles missing DB ---

func TestSyncFromFile_DBUnavailableExitsZero(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	valid := `{"type":"issue","data":{"id":"xyz","title":"T"}}` + "\n"
	require.NoError(t, os.WriteFile("issues.jsonl", []byte(valid), 0644))

	// Point to a port where nothing is listening.
	var errBuf strings.Builder
	hookRunCmd.SetErr(&errBuf)
	defer hookRunCmd.SetErr(nil)

	// Override db_url to an unreachable address.
	// viper.AutomaticEnv maps key "db_url" → env var "DB_URL" (no prefix configured).
	t.Setenv("DB_URL", "root@tcp(127.0.0.1:19999)/grava?parseTime=true")

	// syncFromFile should warn but return nil.
	err := syncFromFile(hookRunCmd, "test")
	assert.NoError(t, err, "syncFromFile should exit 0 when DB is unreachable")
	assert.Contains(t, errBuf.String(), "DB unavailable")
}
