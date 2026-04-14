package githooks_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hoangtrungnguyen/grava/pkg/githooks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tempHooksDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func hookPath(dir, name string) string {
	return filepath.Join(dir, name)
}

func readHook(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(hookPath(dir, name))
	require.NoError(t, err)
	return string(b)
}

// --- DeployAll ---

func TestDeployAll_InstallsAllHooks(t *testing.T) {
	dir := tempHooksDir(t)
	var buf strings.Builder
	results, err := githooks.DeployAll(dir, &buf)
	require.NoError(t, err)
	assert.Len(t, results, len(githooks.HookNames))

	for _, name := range githooks.HookNames {
		content := readHook(t, dir, name)
		assert.Contains(t, content, githooks.ShimMarker, "%s should contain shim marker", name)
		assert.Contains(t, content, name, "%s should reference its hook name", name)

		info, err := os.Stat(hookPath(dir, name))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0755), info.Mode().Perm(), "%s should be executable", name)
	}
}

func TestDeployAll_CreatesHooksDirIfAbsent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "hooks") // does not exist yet
	var buf strings.Builder
	_, err := githooks.DeployAll(dir, &buf)
	require.NoError(t, err)

	_, statErr := os.Stat(dir)
	assert.NoError(t, statErr, "hooks directory should have been created")
}

func TestDeployAll_IdempotentOnRerun(t *testing.T) {
	dir := tempHooksDir(t)
	var buf strings.Builder

	_, err := githooks.DeployAll(dir, &buf)
	require.NoError(t, err)

	results, err := githooks.DeployAll(dir, &buf)
	require.NoError(t, err)

	for _, r := range results {
		assert.Equal(t, "skipped", r.Action, "hook %s should be skipped on re-install", r.Name)
	}
}

func TestDeployAll_UpdatesStaleShim(t *testing.T) {
	dir := tempHooksDir(t)

	// Write a stale grava shim (contains marker but different content)
	stale := "#!/bin/sh\n# grava-shim\ngrava hook run pre-commit --old-flag\n"
	require.NoError(t, os.WriteFile(hookPath(dir, "pre-commit"), []byte(stale), 0755))

	var buf strings.Builder
	results, err := githooks.DeployAll(dir, &buf)
	require.NoError(t, err)

	var preCommit githooks.DeployResult
	for _, r := range results {
		if r.Name == "pre-commit" {
			preCommit = r
		}
	}
	assert.Equal(t, "updated", preCommit.Action)
	// Content should now be the current shim
	assert.NotContains(t, readHook(t, dir, "pre-commit"), "--old-flag")
}

func TestDeployAll_PreservesExistingForeignHook(t *testing.T) {
	dir := tempHooksDir(t)
	original := "#!/bin/sh\necho 'custom hook'\n"
	require.NoError(t, os.WriteFile(hookPath(dir, "pre-commit"), []byte(original), 0755))

	var buf strings.Builder
	results, err := githooks.DeployAll(dir, &buf)
	require.NoError(t, err)

	var preCommit githooks.DeployResult
	for _, r := range results {
		if r.Name == "pre-commit" {
			preCommit = r
		}
	}
	assert.Equal(t, "installed", preCommit.Action)
	assert.NotEmpty(t, preCommit.Existing, "Existing field should point to preserved hook")

	// Original should be renamed
	preserved, err := os.ReadFile(preCommit.Existing)
	require.NoError(t, err)
	assert.Equal(t, original, string(preserved))

	// New shim should be installed
	assert.Contains(t, readHook(t, dir, "pre-commit"), githooks.ShimMarker)
	assert.Contains(t, buf.String(), "renamed", "output should mention rename")
}

func TestDeployAll_DoesNotOverwriteExistingPreGravaFile(t *testing.T) {
	dir := tempHooksDir(t)

	// Simulate: both pre-commit and pre-commit.pre-grava exist
	require.NoError(t, os.WriteFile(hookPath(dir, "pre-commit"), []byte("#!/bin/sh\nforeign\n"), 0755))
	require.NoError(t, os.WriteFile(hookPath(dir, "pre-commit.pre-grava"), []byte("#!/bin/sh\noriginal\n"), 0755))

	var buf strings.Builder
	_, err := githooks.DeployAll(dir, &buf)
	require.NoError(t, err)

	// .pre-grava must be untouched
	preserved, err := os.ReadFile(hookPath(dir, "pre-commit.pre-grava"))
	require.NoError(t, err)
	assert.Contains(t, string(preserved), "original")

	// Primary hook should now be the shim
	assert.Contains(t, readHook(t, dir, "pre-commit"), githooks.ShimMarker)
	assert.Contains(t, buf.String(), "already exists")
}

func TestDeployAll_ShimContainsHookName(t *testing.T) {
	dir := tempHooksDir(t)
	var buf strings.Builder
	_, err := githooks.DeployAll(dir, &buf)
	require.NoError(t, err)

	for _, name := range githooks.HookNames {
		content := readHook(t, dir, name)
		assert.Contains(t, content, "grava hook run "+name,
			"shim for %s should invoke correct hook name", name)
	}
}

// --- Dir helpers ---

func TestDefaultHooksDir(t *testing.T) {
	dir := githooks.DefaultHooksDir("/repo")
	assert.Equal(t, "/repo/.git/hooks", filepath.ToSlash(dir))
}

func TestSharedHooksDir(t *testing.T) {
	dir := githooks.SharedHooksDir("/repo")
	assert.Equal(t, "/repo/.grava/hooks", filepath.ToSlash(dir))
}
