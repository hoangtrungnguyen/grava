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

	driverCmd := gravaBin + " merge-driver %O %A %B"

	for _, args := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		// Pin default branch to "master" so tests are portable across systems
		// where init.defaultBranch may be set to "main".
		{"git", "-C", dir, "config", "init.defaultBranch", "master"},
		{"git", "-C", dir, "config", "merge.grava-merge.name", "Grava Schema-Aware Merge Driver"},
		{"git", "-C", dir, "config", "merge.grava-merge.driver", driverCmd},
	} {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		require.NoError(t, err, "git setup: %s", string(out))
	}

	// Write .gitattributes so git routes issues.jsonl through the driver.
	attrPath := filepath.Join(dir, ".gitattributes")
	require.NoError(t, os.WriteFile(attrPath, []byte("issues.jsonl merge=grava-merge\n"), 0644))

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
	out, err := exec.Command("git", "-C", dir, "config", "--local", "merge.grava-merge.driver").Output()
	require.NoError(t, err)
	assert.Contains(t, strings.TrimSpace(string(out)), "merge-driver")

	// Verify .gitattributes contains the entry.
	attrs, err := os.ReadFile(filepath.Join(dir, ".gitattributes"))
	require.NoError(t, err)
	assert.Contains(t, string(attrs), "issues.jsonl merge=grava-merge")
}

// writeRawJSONL writes a preformatted JSONL string (one object per line) to path.
// Use this when the test needs nested fields or heterogeneous value types that
// don't fit into map[string]string.
func writeRawJSONL(t *testing.T, path, content string) {
	t.Helper()
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

// gitMergeWithEnv runs `git -C dir merge --no-edit feat/a` with extra env vars.
// Returns the combined output and any error from git merge itself.
func gitMergeWithEnv(t *testing.T, dir string, env []string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "merge", "--no-edit", "feat/a")
	cmd.Env = append(os.Environ(), env...)
	return cmd.CombinedOutput()
}

// TestMergeDriver_DeleteVsModify_DeleteWins verifies that when one branch deletes
// an issue and the other modifies it, the deletion wins (per ProcessMergeWithLWW
// semantics) and git merge succeeds without conflict markers.
//
// This fills the gap flagged in FIX-REPORT-20260423.md §06 — the sandbox scenario
// confirms the driver is installable but does not execute a real three-way merge
// of delete-vs-modify.
func TestMergeDriver_DeleteVsModify_DeleteWins(t *testing.T) {
	gravaBin := sharedGravaBinary(t)
	dir := initMergeDriverRepo(t, gravaBin)

	issuePath := filepath.Join(dir, "issues.jsonl")

	// Base commit: two issues. Only issue-keep will be touched by the branches;
	// issue-vanishing is the one that will be deleted / modified.
	writeRawJSONL(t, issuePath,
		`{"id":"issue-keep","title":"Keep me","status":"open"}`+"\n"+
			`{"id":"issue-vanishing","title":"Contested","status":"open","updated_at":"2026-04-20T10:00:00Z"}`)
	gitRun(t, dir, "add", "issues.jsonl", ".gitattributes")
	gitRun(t, dir, "commit", "-m", "initial")

	// Branch A: DELETE issue-vanishing (only issue-keep remains).
	gitRun(t, dir, "checkout", "-b", "feat/a")
	writeRawJSONL(t, issuePath,
		`{"id":"issue-keep","title":"Keep me","status":"open"}`)
	gitRun(t, dir, "add", "issues.jsonl")
	gitRun(t, dir, "commit", "-m", "feat/a: delete issue-vanishing")

	// Return to default branch and MODIFY issue-vanishing (newer timestamp).
	gitRun(t, dir, "checkout", "-")
	writeRawJSONL(t, issuePath,
		`{"id":"issue-keep","title":"Keep me","status":"open"}`+"\n"+
			`{"id":"issue-vanishing","title":"Contested — updated","status":"in_progress","updated_at":"2026-04-22T10:00:00Z"}`)
	gitRun(t, dir, "add", "issues.jsonl")
	gitRun(t, dir, "commit", "-m", "main: modify issue-vanishing")

	// Merge feat/a. Per merge semantics: delete wins deterministically, no git conflict.
	out, err := exec.Command("git", "-C", dir, "merge", "--no-edit", "feat/a").CombinedOutput()
	require.NoError(t, err,
		"git merge should succeed: delete-vs-modify is deterministic (delete wins):\n%s", string(out))

	// Assert: only issue-keep remains; the modified side's newer timestamp is
	// NOT enough to override a delete.
	issues := readIssues(t, issuePath)
	require.Len(t, issues, 1, "delete should win — only issue-keep should remain")
	assert.Equal(t, "issue-keep", issues[0]["id"])

	merged, _ := os.ReadFile(issuePath)
	assert.NotContains(t, string(merged), `"_conflict"`,
		"delete-wins must not produce inline conflict markers")
}

// TestMergeDriver_ConcurrentEditsMultipleIssues_LWW verifies that when both
// branches edit DIFFERENT issues (plus one shared issue resolved by LWW
// timestamp), all edits appear in the merged result without conflicts.
//
// Fills the gap flagged in FIX-REPORT-20260423.md §07 — sandbox only confirms
// the file_reservations table is queryable, not that concurrent-edit merges
// actually compose correctly.
func TestMergeDriver_ConcurrentEditsMultipleIssues_LWW(t *testing.T) {
	gravaBin := sharedGravaBinary(t)
	dir := initMergeDriverRepo(t, gravaBin)

	issuePath := filepath.Join(dir, "issues.jsonl")

	// Base: 3 issues, all open, same starting state.
	writeRawJSONL(t, issuePath,
		`{"id":"iss-1","status":"open","title":"One","updated_at":"2026-04-20T00:00:00Z"}`+"\n"+
			`{"id":"iss-2","status":"open","title":"Two","updated_at":"2026-04-20T00:00:00Z"}`+"\n"+
			`{"id":"iss-3","status":"open","title":"Three","updated_at":"2026-04-20T00:00:00Z"}`)
	gitRun(t, dir, "add", "issues.jsonl", ".gitattributes")
	gitRun(t, dir, "commit", "-m", "initial")

	// Branch A: close iss-1, update iss-3 title. iss-2 untouched.
	// iss-3 has the EARLIER timestamp — LWW should pick main's version.
	gitRun(t, dir, "checkout", "-b", "feat/a")
	writeRawJSONL(t, issuePath,
		`{"id":"iss-1","status":"closed","title":"One","updated_at":"2026-04-22T10:00:00Z"}`+"\n"+
			`{"id":"iss-2","status":"open","title":"Two","updated_at":"2026-04-20T00:00:00Z"}`+"\n"+
			`{"id":"iss-3","status":"open","title":"Three (A)","updated_at":"2026-04-22T09:00:00Z"}`)
	gitRun(t, dir, "add", "issues.jsonl")
	gitRun(t, dir, "commit", "-m", "feat/a: close iss-1, rename iss-3")

	// Main: update iss-2 status, update iss-3 title (NEWER timestamp → wins).
	gitRun(t, dir, "checkout", "-")
	writeRawJSONL(t, issuePath,
		`{"id":"iss-1","status":"open","title":"One","updated_at":"2026-04-20T00:00:00Z"}`+"\n"+
			`{"id":"iss-2","status":"in_progress","title":"Two","updated_at":"2026-04-22T11:00:00Z"}`+"\n"+
			`{"id":"iss-3","status":"open","title":"Three (main)","updated_at":"2026-04-22T12:00:00Z"}`)
	gitRun(t, dir, "add", "issues.jsonl")
	gitRun(t, dir, "commit", "-m", "main: progress iss-2, rename iss-3")

	// Merge: all three issues should compose cleanly via LWW.
	out, err := exec.Command("git", "-C", dir, "merge", "--no-edit", "feat/a").CombinedOutput()
	require.NoError(t, err, "concurrent LWW-resolvable edits must merge cleanly:\n%s", string(out))

	issues := readIssues(t, issuePath)
	require.Len(t, issues, 3, "all 3 issues should be present in merged output")

	byID := map[string]map[string]interface{}{}
	for _, iss := range issues {
		byID[iss["id"].(string)] = iss
	}

	// iss-1: only feat/a changed it — feat/a's closed wins (cleanly).
	assert.Equal(t, "closed", byID["iss-1"]["status"], "iss-1: feat/a's close should win")
	// iss-2: only main changed it — main wins.
	assert.Equal(t, "in_progress", byID["iss-2"]["status"], "iss-2: main's in_progress should win")
	// iss-3: both sides changed title; main's timestamp is newer → main wins.
	assert.Equal(t, "Three (main)", byID["iss-3"]["title"],
		"iss-3: main's newer timestamp should win LWW tiebreak")
}

// TestMergeDriver_EqualTimestamp_ProducesGitConflict verifies that when two
// branches modify the same field with IDENTICAL updated_at timestamps, LWW
// cannot resolve and git merge exits non-zero with conflict markers in the file.
//
// This exercises the HasGitConflict=true code path through a live git merge.
func TestMergeDriver_EqualTimestamp_ProducesGitConflict(t *testing.T) {
	gravaBin := sharedGravaBinary(t)
	dir := initMergeDriverRepo(t, gravaBin)

	issuePath := filepath.Join(dir, "issues.jsonl")

	// Base: one issue with some updated_at.
	writeRawJSONL(t, issuePath,
		`{"id":"contested","title":"Start","status":"open","updated_at":"2026-04-20T00:00:00Z"}`)
	gitRun(t, dir, "add", "issues.jsonl", ".gitattributes")
	gitRun(t, dir, "commit", "-m", "initial")

	// Branch A: modify title, set updated_at to T.
	gitRun(t, dir, "checkout", "-b", "feat/a")
	writeRawJSONL(t, issuePath,
		`{"id":"contested","title":"Alpha","status":"open","updated_at":"2026-04-22T12:00:00Z"}`)
	gitRun(t, dir, "add", "issues.jsonl")
	gitRun(t, dir, "commit", "-m", "feat/a: rename to Alpha")

	// Main: modify title, SAME updated_at → equal-timestamp conflict.
	gitRun(t, dir, "checkout", "-")
	writeRawJSONL(t, issuePath,
		`{"id":"contested","title":"Beta","status":"open","updated_at":"2026-04-22T12:00:00Z"}`)
	gitRun(t, dir, "add", "issues.jsonl")
	gitRun(t, dir, "commit", "-m", "main: rename to Beta")

	// Merge — driver writes conflict markers and exits 1, so git merge fails.
	out, err := exec.Command("git", "-C", dir, "merge", "--no-edit", "feat/a").CombinedOutput()
	assert.Error(t, err, "equal-timestamp field conflict must surface as git merge failure:\n%s", string(out))

	merged, readErr := os.ReadFile(issuePath)
	require.NoError(t, readErr)
	assert.Contains(t, string(merged), `"_conflict":true`,
		"merged file must retain conflict markers when LWW can't resolve")
}

// TestMergeDriver_ConflictsJsonPersisted verifies that when conflicts are
// detected AND a .grava directory is available (via GRAVA_DIR env), the driver
// writes conflict records to <GRAVA_DIR>/conflicts.json so `grava resolve list`
// can surface them post-merge.
func TestMergeDriver_ConflictsJsonPersisted(t *testing.T) {
	gravaBin := sharedGravaBinary(t)
	dir := initMergeDriverRepo(t, gravaBin)

	// Create a .grava directory in the test repo so ResolveGravaDir can find it.
	gravaDir := filepath.Join(dir, ".grava")
	require.NoError(t, os.MkdirAll(gravaDir, 0o755))

	issuePath := filepath.Join(dir, "issues.jsonl")

	// Base with one contested issue.
	writeRawJSONL(t, issuePath,
		`{"id":"contested","title":"Start","updated_at":"2026-04-20T00:00:00Z"}`)
	gitRun(t, dir, "add", "issues.jsonl", ".gitattributes")
	gitRun(t, dir, "commit", "-m", "initial")

	// Equal-timestamp conflict to force a conflict record.
	gitRun(t, dir, "checkout", "-b", "feat/a")
	writeRawJSONL(t, issuePath,
		`{"id":"contested","title":"Alpha","updated_at":"2026-04-22T12:00:00Z"}`)
	gitRun(t, dir, "add", "issues.jsonl")
	gitRun(t, dir, "commit", "-m", "feat/a")

	gitRun(t, dir, "checkout", "-")
	writeRawJSONL(t, issuePath,
		`{"id":"contested","title":"Beta","updated_at":"2026-04-22T12:00:00Z"}`)
	gitRun(t, dir, "add", "issues.jsonl")
	gitRun(t, dir, "commit", "-m", "main")

	// Invoke merge with GRAVA_DIR pointing at our test .grava so the driver
	// writes conflicts.json there.
	out, err := gitMergeWithEnv(t, dir, []string{"GRAVA_DIR=" + gravaDir})
	assert.Error(t, err, "merge should fail on equal-timestamp conflict:\n%s", string(out))

	// Verify conflicts.json was persisted.
	conflictsPath := filepath.Join(gravaDir, "conflicts.json")
	data, readErr := os.ReadFile(conflictsPath)
	require.NoError(t, readErr, "driver should have written %s", conflictsPath)

	var records []map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &records), "conflicts.json must be valid JSON array")
	require.NotEmpty(t, records, "at least one conflict record expected")
	assert.Equal(t, "contested", records[0]["issue_id"],
		"conflict record must reference the contested issue")
}
