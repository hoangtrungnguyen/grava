//go:build integration
// +build integration

// Package merge_test contains end-to-end tests that invoke grava merge-slot
// through a real Git merge operation. These tests require git on PATH and
// compile the grava binary as part of setup.
package merge_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	gravaBinOnce sync.Once
	gravaBinPath string
	gravaBinErr  error
)

// sharedGravaBinary returns the path to a grava binary compiled once per test
// process. Subsequent callers reuse the same binary.
func sharedGravaBinary(t *testing.T) string {
	t.Helper()
	gravaBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "grava-test-bin-*")
		if err != nil {
			gravaBinErr = err
			return
		}
		gravaBinPath = filepath.Join(dir, "grava")
		out, err := exec.Command("go", "build", "-o", gravaBinPath,
			"github.com/hoangtrungnguyen/grava/cmd/grava").CombinedOutput()
		if err != nil {
			gravaBinErr = fmt.Errorf("go build failed:\n%s", string(out))
		}
	})
	require.NoError(t, gravaBinErr)
	return gravaBinPath
}

// initMergeDriverRepo creates a git repo configured with the grava merge driver
// pointing to the given binary path. Returns the repo dir.
// The caller is responsible for os.Chdir if needed.
func initMergeDriverRepo(t *testing.T, gravaBin string) string {
	t.Helper()
	dir := t.TempDir()

	driverCmd := gravaBin + " merge-slot --ancestor %O --current %A --other %B"

	for _, args := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		// Pin default branch to "master" so tests are portable across systems
		// where init.defaultBranch may be set to "main".
		{"git", "-C", dir, "config", "init.defaultBranch", "master"},
		{"git", "-C", dir, "config", "merge.grava.name", "Grava Schema-Aware Merge"},
		{"git", "-C", dir, "config", "merge.grava.driver", driverCmd},
	} {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		require.NoError(t, err, "git setup: %s", string(out))
	}

	// Write .gitattributes so git routes issues.jsonl through the driver.
	attrPath := filepath.Join(dir, ".gitattributes")
	require.NoError(t, os.WriteFile(attrPath, []byte("issues.jsonl merge=grava\n"), 0644))

	return dir
}

// gitRun runs a git command in dir and requires no error.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), string(out))
	return strings.TrimSpace(string(out))
}

// writeIssues serialises a slice of issue maps to JSONL at path.
func writeIssues(t *testing.T, path string, issues []map[string]string) {
	t.Helper()
	var lines []string
	for _, issue := range issues {
		b, err := json.Marshal(issue)
		require.NoError(t, err)
		lines = append(lines, string(b))
	}
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644))
}

// readIssues parses a JSONL file into a slice of map[string]interface{}.
func readIssues(t *testing.T, path string) []map[string]interface{} {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	var issues []map[string]interface{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var issue map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(line), &issue), "bad JSONL line: %s", line)
		issues = append(issues, issue)
	}
	return issues
}

// TestMergeDriver_NonConflictingFieldChanges verifies that when two branches
// modify different fields of the same issue, git merge produces a clean result
// containing both changes — no manual resolution needed.
func TestMergeDriver_NonConflictingFieldChanges(t *testing.T) {
	gravaBin := sharedGravaBinary(t)
	dir := initMergeDriverRepo(t, gravaBin)

	issuePath := filepath.Join(dir, "issues.jsonl")

	// Base commit: one issue with status=open and title=Original.
	base := []map[string]string{{"id": "abc123", "title": "Original", "status": "open"}}
	writeIssues(t, issuePath, base)
	gitRun(t, dir, "add", "issues.jsonl", ".gitattributes")
	gitRun(t, dir, "commit", "-m", "initial")

	// Branch A: change title only.
	gitRun(t, dir, "checkout", "-b", "feat/a")
	writeIssues(t, issuePath, []map[string]string{{"id": "abc123", "title": "Updated Title", "status": "open"}})
	gitRun(t, dir, "add", "issues.jsonl")
	gitRun(t, dir, "commit", "-m", "feat/a: update title")

	// Return to default branch and change status only.
	gitRun(t, dir, "checkout", "-")
	writeIssues(t, issuePath, []map[string]string{{"id": "abc123", "title": "Original", "status": "in_progress"}})
	gitRun(t, dir, "add", "issues.jsonl")
	gitRun(t, dir, "commit", "-m", "main: update status")

	// Merge feat/a — should succeed without conflicts.
	out, err := exec.Command("git", "-C", dir, "merge", "--no-edit", "feat/a").CombinedOutput()
	require.NoError(t, err, "git merge should succeed with no conflicts:\n%s", string(out))

	issues := readIssues(t, issuePath)
	require.Len(t, issues, 1)
	assert.Equal(t, "Updated Title", issues[0]["title"], "title from feat/a should be present")
	assert.Equal(t, "in_progress", issues[0]["status"], "status from main should be preserved")
}

// TestMergeDriver_AddedOnBothSides verifies that when each branch adds a
// different issue (no shared ID), both issues appear in the merged result.
func TestMergeDriver_AddedOnBothSides(t *testing.T) {
	gravaBin := sharedGravaBinary(t)
	dir := initMergeDriverRepo(t, gravaBin)

	issuePath := filepath.Join(dir, "issues.jsonl")

	// Base commit: empty issues file.
	require.NoError(t, os.WriteFile(issuePath, []byte(""), 0644))
	gitRun(t, dir, "add", "issues.jsonl", ".gitattributes")
	gitRun(t, dir, "commit", "-m", "initial")

	// Branch A: add issue-1.
	gitRun(t, dir, "checkout", "-b", "feat/a")
	writeIssues(t, issuePath, []map[string]string{{"id": "issue-1", "title": "First"}})
	gitRun(t, dir, "add", "issues.jsonl")
	gitRun(t, dir, "commit", "-m", "add issue-1")

	// Return to default branch and add issue-2.
	gitRun(t, dir, "checkout", "-")
	writeIssues(t, issuePath, []map[string]string{{"id": "issue-2", "title": "Second"}})
	gitRun(t, dir, "add", "issues.jsonl")
	gitRun(t, dir, "commit", "-m", "add issue-2")

	// Merge feat/a into default branch.
	out, err := exec.Command("git", "-C", dir, "merge", "--no-edit", "feat/a").CombinedOutput()
	require.NoError(t, err, "git merge should succeed:\n%s", string(out))

	issues := readIssues(t, issuePath)
	require.Len(t, issues, 2, "merged file should contain both issues")

	ids := make(map[string]bool)
	for _, issue := range issues {
		ids[issue["id"].(string)] = true
	}
	assert.True(t, ids["issue-1"], "issue-1 should be present")
	assert.True(t, ids["issue-2"], "issue-2 should be present")
}

// TestMergeDriver_SameFieldConflict verifies that when both branches modify
// the same field of the same issue, git merge exits non-zero (conflict).
func TestMergeDriver_SameFieldConflict(t *testing.T) {
	gravaBin := sharedGravaBinary(t)
	dir := initMergeDriverRepo(t, gravaBin)

	issuePath := filepath.Join(dir, "issues.jsonl")

	// Base commit.
	writeIssues(t, issuePath, []map[string]string{{"id": "abc123", "status": "open"}})
	gitRun(t, dir, "add", "issues.jsonl", ".gitattributes")
	gitRun(t, dir, "commit", "-m", "initial")

	// Branch A: set status=paused.
	gitRun(t, dir, "checkout", "-b", "feat/a")
	writeIssues(t, issuePath, []map[string]string{{"id": "abc123", "status": "paused"}})
	gitRun(t, dir, "add", "issues.jsonl")
	gitRun(t, dir, "commit", "-m", "feat/a: set paused")

	// Return to default branch and set status=in_progress (conflicting).
	gitRun(t, dir, "checkout", "-")
	writeIssues(t, issuePath, []map[string]string{{"id": "abc123", "status": "in_progress"}})
	gitRun(t, dir, "add", "issues.jsonl")
	gitRun(t, dir, "commit", "-m", "main: set in_progress")

	// Merge feat/a — should exit non-zero because of conflicting status field.
	cmd := exec.Command("git", "-C", dir, "merge", "--no-edit", "feat/a")
	out, err := cmd.CombinedOutput()
	assert.Error(t, err, "git merge should exit non-zero on field conflict:\n%s", string(out))
}

// TestMergeDriver_IdempotentInstall verifies that running grava install twice
// in a git repo produces the same configuration and exit 0 both times.
func TestMergeDriver_IdempotentInstall(t *testing.T) {
	gravaBin := sharedGravaBinary(t)
	dir := t.TempDir()
	for _, args := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	} {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		require.NoError(t, err, "git setup: %s", string(out))
	}

	// grava install uses git rev-parse --show-toplevel, so run it from within the repo.
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(orig) }()

	for i := 0; i < 2; i++ {
		out, err := exec.Command(gravaBin, "install").CombinedOutput()
		require.NoError(t, err, "grava install run %d failed:\n%s", i+1, string(out))
	}

	// Verify git config has merge driver registered.
	out, err := exec.Command("git", "-C", dir, "config", "--local", "merge.grava.driver").Output()
	require.NoError(t, err)
	assert.Contains(t, strings.TrimSpace(string(out)), "merge-slot")

	// Verify .gitattributes contains the entry.
	attrs, err := os.ReadFile(filepath.Join(dir, ".gitattributes"))
	require.NoError(t, err)
	assert.Contains(t, string(attrs), "issues.jsonl merge=grava")
}
