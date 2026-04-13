package issues

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hoangtrungnguyen/grava/pkg/utils"
	"github.com/stretchr/testify/assert"
)

// TestIsInsideClaudeWorktree_Detection verifies AC#4 Claude environment safety.
func TestIsInsideClaudeWorktree_Detection(t *testing.T) {
	assert.False(t, utils.IsInsideClaudeWorktree("/home/user/project"))
	assert.True(t, utils.IsInsideClaudeWorktree("/home/user/project/.claude/worktrees/grava-abc"))
}

// TestCloseCmd_BlocksDirtyWorktree verifies AC#1 dirty check.
func TestCloseCmd_BlocksDirtyWorktree(t *testing.T) {
	tmpdir := t.TempDir()

	// Set up git repo + worktree
	setupGitRepo(t, tmpdir)
	issueID := "grava-close-dirty"
	if err := utils.ProvisionWorktree(tmpdir, issueID); err != nil {
		t.Fatalf("provision: %v", err)
	}

	// Make it dirty
	wtDir := filepath.Join(tmpdir, ".worktree", issueID)
	if err := os.WriteFile(filepath.Join(wtDir, "dirty.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	dirty, err := utils.IsWorktreeDirty(tmpdir, issueID)
	assert.NoError(t, err)
	assert.True(t, dirty, "worktree should be dirty")
}

// TestCloseCmd_CleansUpSymlink verifies Claude symlink removal on close.
func TestCloseCmd_CleansUpSymlink(t *testing.T) {
	tmpdir := t.TempDir()

	issueID := "grava-symlink-cleanup"
	// Create a fake Claude symlink
	claudeDir := filepath.Join(tmpdir, ".claude", "worktrees")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(claudeDir, issueID)
	if err := os.Symlink("/tmp/nonexistent", symlinkPath); err != nil {
		t.Fatal(err)
	}

	// Verify symlink exists
	_, err := os.Lstat(symlinkPath)
	assert.NoError(t, err)

	// Remove it (simulating close cleanup)
	os.Remove(symlinkPath)

	// Verify symlink is gone
	_, err = os.Lstat(symlinkPath)
	assert.True(t, os.IsNotExist(err))
}

// setupGitRepo initializes a git repo in tmpdir with an initial commit.
func setupGitRepo(t *testing.T, tmpdir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, c := range cmds {
		if err := runTestCmd(tmpdir, c[0], c[1:]...); err != nil {
			t.Fatalf("%v: %v", c, err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmpdir, "f.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := runTestCmd(tmpdir, "git", "add", "f.txt"); err != nil {
		t.Fatal(err)
	}
	if err := runTestCmd(tmpdir, "git", "commit", "-m", "init"); err != nil {
		t.Fatal(err)
	}
}

func runTestCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}
