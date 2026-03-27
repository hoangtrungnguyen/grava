package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteGitExclude_NoGitDir(t *testing.T) {
	// Directory without .git — should skip gracefully
	dir := t.TempDir()
	migrated, err := WriteGitExclude(dir)
	require.NoError(t, err)
	assert.False(t, migrated)
}

func TestWriteGitExclude_AddsEntryToNewFile(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	migrated, err := WriteGitExclude(dir)
	require.NoError(t, err)
	assert.False(t, migrated) // no .gitignore to migrate

	excludeFile := filepath.Join(gitDir, "info", "exclude")
	data, err := os.ReadFile(excludeFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), ".grava/")
}

func TestWriteGitExclude_Idempotent(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	// First call
	_, err := WriteGitExclude(dir)
	require.NoError(t, err)

	// Second call should be idempotent
	_, err = WriteGitExclude(dir)
	require.NoError(t, err)

	excludeFile := filepath.Join(gitDir, "info", "exclude")
	data, err := os.ReadFile(excludeFile)
	require.NoError(t, err)

	// ".grava/" should appear exactly once
	count := strings.Count(string(data), ".grava/")
	assert.Equal(t, 1, count, "expected .grava/ to appear exactly once")
}

func TestWriteGitExclude_AppendsToExistingFile(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	infoDir := filepath.Join(gitDir, "info")
	require.NoError(t, os.MkdirAll(infoDir, 0755))

	excludeFile := filepath.Join(infoDir, "exclude")
	require.NoError(t, os.WriteFile(excludeFile, []byte("# existing content\n*.log\n"), 0644))

	migrated, err := WriteGitExclude(dir)
	require.NoError(t, err)
	assert.False(t, migrated)

	data, err := os.ReadFile(excludeFile)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "# existing content")
	assert.Contains(t, content, "*.log")
	assert.Contains(t, content, ".grava/")
}

func TestWriteGitExclude_MigratesGitignore(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	// Write .gitignore with .grava/ entry
	gitignorePath := filepath.Join(dir, ".gitignore")
	require.NoError(t, os.WriteFile(gitignorePath, []byte("node_modules/\n.grava/\n*.tmp\n"), 0644))

	migrated, err := WriteGitExclude(dir)
	require.NoError(t, err)
	assert.True(t, migrated)

	// .grava/ should be removed from .gitignore
	data, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)
	assert.NotContains(t, string(data), ".grava/")
	assert.Contains(t, string(data), "node_modules/")
	assert.Contains(t, string(data), "*.tmp")
}

func TestWriteGitExclude_MigratesGitignoreWithoutSlash(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	// Write .gitignore with .grava (no trailing slash)
	gitignorePath := filepath.Join(dir, ".gitignore")
	require.NoError(t, os.WriteFile(gitignorePath, []byte(".grava\n"), 0644))

	migrated, err := WriteGitExclude(dir)
	require.NoError(t, err)
	assert.True(t, migrated)

	data, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)
	assert.NotContains(t, string(data), ".grava")
}

func TestWriteGitExclude_NoGitignoreNoMigration(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	migrated, err := WriteGitExclude(dir)
	require.NoError(t, err)
	assert.False(t, migrated)
}

func TestWriteGitExclude_GitignoreWithoutGravaEntry(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	gitignorePath := filepath.Join(dir, ".gitignore")
	require.NoError(t, os.WriteFile(gitignorePath, []byte("*.log\nnode_modules/\n"), 0644))

	migrated, err := WriteGitExclude(dir)
	require.NoError(t, err)
	assert.False(t, migrated) // no .grava entry to remove

	// .gitignore should be unchanged
	data, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "*.log")
}
