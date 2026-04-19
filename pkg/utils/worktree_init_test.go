package utils

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- AC#1: EnsureWorktreeDir ---

func TestEnsureWorktreeDir_Creates(t *testing.T) {
	dir := t.TempDir()
	created, err := EnsureWorktreeDir(dir)
	require.NoError(t, err)
	assert.True(t, created)
	assert.DirExists(t, filepath.Join(dir, ".worktree"))
}

func TestEnsureWorktreeDir_Idempotent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".worktree"), 0755))
	created, err := EnsureWorktreeDir(dir)
	require.NoError(t, err)
	assert.False(t, created)
}

// --- AC#1: EnsureWorktreeGitignore ---

func TestEnsureWorktreeGitignore_AddsEntry(t *testing.T) {
	dir := t.TempDir()
	added, err := EnsureWorktreeGitignore(dir)
	require.NoError(t, err)
	assert.True(t, added)

	content, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	assert.Contains(t, string(content), ".worktree/")
}

func TestEnsureWorktreeGitignore_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules/\n"), 0644))

	added, err := EnsureWorktreeGitignore(dir)
	require.NoError(t, err)
	assert.True(t, added)

	content, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	assert.Contains(t, string(content), "node_modules/")
	assert.Contains(t, string(content), ".worktree/")
}

func TestEnsureWorktreeGitignore_Idempotent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".worktree/\n"), 0644))

	added, err := EnsureWorktreeGitignore(dir)
	require.NoError(t, err)
	assert.False(t, added)
}

func TestEnsureWorktreeGitignore_NoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules/"), 0644))

	added, err := EnsureWorktreeGitignore(dir)
	require.NoError(t, err)
	assert.True(t, added)

	content, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	assert.Contains(t, string(content), "node_modules/\n.worktree/\n")
}

// --- AC#2: SetWorktreeGitConfig ---

func TestSetWorktreeGitConfig(t *testing.T) {
	dir := t.TempDir()
	// Need a real git repo to set config
	for _, c := range [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	} {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = dir
		require.NoError(t, cmd.Run())
	}

	err := SetWorktreeGitConfig(dir)
	require.NoError(t, err)

	// Verify
	val, err := gitConfigGet(dir, "grava.worktreeDir")
	require.NoError(t, err)
	assert.Equal(t, ".worktree", val)
}

// --- AC#3: EnsureClaudeWorktreeSettings ---

func TestEnsureClaudeWorktreeSettings_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	added, err := EnsureClaudeWorktreeSettings(dir)
	require.NoError(t, err)
	assert.True(t, added)

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	require.NoError(t, err)

	var settings map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &settings))

	wt, ok := settings["worktree"]
	require.True(t, ok, "worktree key must exist")
	wtMap := wt.(map[string]interface{})
	assert.NotNil(t, wtMap["symlinkDirectories"])
	assert.NotNil(t, wtMap["sparsePaths"])

	ep, ok := settings["enabledPlugins"]
	require.True(t, ok, "enabledPlugins key must exist")
	epMap := ep.(map[string]interface{})
	assert.Equal(t, true, epMap["skill-creator@claude-plugins-official"])
}

func TestEnsureClaudeWorktreeSettings_MergesExisting(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	existing := `{"allowedTools":["bash"]}`
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0644))

	added, err := EnsureClaudeWorktreeSettings(dir)
	require.NoError(t, err)
	assert.True(t, added)

	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	require.NoError(t, err)

	var settings map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &settings))

	// Original key preserved
	assert.NotNil(t, settings["allowedTools"])
	// Worktree block added
	assert.NotNil(t, settings["worktree"])
	// enabledPlugins added
	assert.NotNil(t, settings["enabledPlugins"])
}

func TestEnsureClaudeWorktreeSettings_NullJSON(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("null"), 0644))

	added, err := EnsureClaudeWorktreeSettings(dir)
	require.NoError(t, err, "should not panic on null JSON")
	assert.True(t, added)

	data, _ := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	var settings map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &settings))
	assert.NotNil(t, settings["worktree"])
}

func TestEnsureClaudeWorktreeSettings_Idempotent(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	existing := `{"enabledPlugins":{"skill-creator@claude-plugins-official":true},"worktree":{"symlinkDirectories":["custom"],"sparsePaths":[]}}`
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0644))

	added, err := EnsureClaudeWorktreeSettings(dir)
	require.NoError(t, err)
	assert.False(t, added, "should not modify when both blocks exist")

	// Content should remain unchanged
	data, _ := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	assert.Equal(t, existing, string(data))
}
