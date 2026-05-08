package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initGitRepo initialises a real git repo in dir with an empty initial commit.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, c := range [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test User"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git setup %v: %v\n%s", c, err, out)
		}
	}
}

// TestInitCmd_WorktreeSyncsSettings verifies that running `grava init` inside a
// Git worktree copies .claude/settings.json from the main repo into the worktree
// (AC5, grava-4136.3).
//
// This test is deliberately failing until grava-4136.3 wires the
// SyncClaudeSettings call in initCmd's worktree branch.
func TestInitCmd_WorktreeSyncsSettings(t *testing.T) {
	// Bypass the claude-CLI preflight: this test exercises worktree sync, not
	// the preflight gate, and CI runners don't have claude installed.
	t.Setenv("GRAVA_SKIP_PREFLIGHT", "1")

	// 1. Create a real main git repo with an initial commit.
	mainRepo := t.TempDir()
	initGitRepo(t, mainRepo)

	// 2. Place settings.json in the main repo.
	settingsDir := filepath.Join(mainRepo, ".claude")
	require.NoError(t, os.MkdirAll(settingsDir, 0755))
	settingsContent := `{"initTest":true}`
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsContent), 0644))

	// 3. Create a real git worktree from the main repo.
	worktreeID := "wt-sync-test1"
	worktreeDir := filepath.Join(mainRepo, ".worktree", worktreeID)
	addCmd := exec.Command("git", "worktree", "add", worktreeDir, "-b", worktreeID)
	addCmd.Dir = mainRepo
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v\n%s", err, out)
	}

	// 4. Change cwd to the worktree so IsWorktree(cwd) returns true.
	t.Chdir(worktreeDir)

	// 5. Run initCmd (the worktree branch).
	buf := &bytes.Buffer{}
	outputJSON = false
	initCmd.SetOut(buf)
	err := initCmd.RunE(initCmd, []string{})
	require.NoError(t, err, "init in worktree should succeed")

	// 6. Verify settings.json was synced into the worktree's .claude dir.
	dest := filepath.Join(worktreeDir, ".claude", "settings.json")
	if assert.FileExists(t, dest, "settings.json must be synced into the worktree") {
		got, _ := os.ReadFile(dest)
		assert.Equal(t, settingsContent, string(got))
	}
}

// TestInitCmd_WorktreeSyncsSettings_AbsentSource verifies that init succeeds
// in a worktree even when the main repo has no .claude/settings.json.
// This variant is expected to pass now; the primary TestInitCmd_WorktreeSyncsSettings
// is the deliberately-failing TDD test (passes after grava-4136.3 wires the call).
func TestInitCmd_WorktreeSyncsSettings_AbsentSource(t *testing.T) {
	// Bypass the claude-CLI preflight (CI runners don't have claude installed).
	t.Setenv("GRAVA_SKIP_PREFLIGHT", "1")

	mainRepo := t.TempDir()
	initGitRepo(t, mainRepo)
	// No settings.json — source absent

	worktreeID := "wt-nosrc-test1"
	worktreeDir := filepath.Join(mainRepo, ".worktree", worktreeID)
	addCmd := exec.Command("git", "worktree", "add", worktreeDir, "-b", worktreeID)
	addCmd.Dir = mainRepo
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v\n%s", err, out)
	}

	t.Chdir(worktreeDir)

	buf := &bytes.Buffer{}
	outputJSON = false
	initCmd.SetOut(buf)
	err := initCmd.RunE(initCmd, []string{})
	require.NoError(t, err, "init should succeed even when settings.json is absent")
	// No settings.json should have been created from nothing
	assert.NoFileExists(t, filepath.Join(worktreeDir, ".claude", "settings.json"))
}
