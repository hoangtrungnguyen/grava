package utils

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIsWorktree verifies detection of Git worktree vs main repo.
func TestIsWorktree(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) string // Returns test directory
		expected bool
	}{
		{
			name: "main repo (.git is directory)",
			setup: func(t *testing.T) string {
				tmpdir := t.TempDir()
				gitDir := filepath.Join(tmpdir, ".git")
				if err := os.MkdirAll(gitDir, 0755); err != nil {
					t.Fatalf("failed to create .git directory: %v", err)
				}
				return tmpdir
			},
			expected: false,
		},
		{
			name: "worktree (.git is file)",
			setup: func(t *testing.T) string {
				tmpdir := t.TempDir()
				gitFile := filepath.Join(tmpdir, ".git")
				if err := os.WriteFile(gitFile, []byte("gitdir: /path/to/main/.git/worktrees/wt\n"), 0644); err != nil {
					t.Fatalf("failed to create .git file: %v", err)
				}
				return tmpdir
			},
			expected: true,
		},
		{
			name: "no .git",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cwd := tt.setup(t)
			result := IsWorktree(cwd)
			if result != tt.expected {
				t.Errorf("IsWorktree(%s) = %v, want %v", cwd, result, tt.expected)
			}
		})
	}
}

// TestComputeRedirectPath verifies redirect path computation.
func TestComputeRedirectPath(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T) (string, string) // Returns (worktree cwd, main repo dir)
		expectErr bool
		validate  func(t *testing.T, path string, mainRepoDir string, worktreeCwd string)
	}{
		{
			name: "worktree one level deep",
			setup: func(t *testing.T) (string, string) {
				tmpdir := t.TempDir()
				// Create main repo with .git directory
				mainGitDir := filepath.Join(tmpdir, ".git")
				if err := os.MkdirAll(mainGitDir, 0755); err != nil {
					t.Fatalf("failed to create main .git: %v", err)
				}
				// Create main repo .grava
				mainGravaDir := filepath.Join(tmpdir, ".grava")
				if err := os.MkdirAll(mainGravaDir, 0755); err != nil {
					t.Fatalf("failed to create main .grava: %v", err)
				}

				// Create worktree directory
				wtDir := filepath.Join(tmpdir, "worktree1")
				if err := os.MkdirAll(wtDir, 0755); err != nil {
					t.Fatalf("failed to create worktree: %v", err)
				}
				// Create worktree .git file pointing to main
				wtGitFile := filepath.Join(wtDir, ".git")
				if err := os.WriteFile(wtGitFile, []byte("gitdir: "+filepath.Join(tmpdir, ".git/worktrees/worktree1")+"\n"), 0644); err != nil {
					t.Fatalf("failed to create worktree .git: %v", err)
				}

				return wtDir, tmpdir
			},
			expectErr: false,
			validate: func(t *testing.T, path string, mainRepoDir string, worktreeCwd string) {
				// Path should be relative from worktree to main repo .grava
				// e.g., "../.grava"
				absPath := filepath.Join(worktreeCwd, path)
				absPath, _ = filepath.Abs(absPath)
				expectedPath := filepath.Join(mainRepoDir, ".grava")
				expectedPath, _ = filepath.Abs(expectedPath)
				if absPath != expectedPath {
					t.Errorf("computed path resolves to %s, want %s", absPath, expectedPath)
				}
			},
		},
		{
			name: "worktree multiple levels deep",
			setup: func(t *testing.T) (string, string) {
				tmpdir := t.TempDir()
				mainGitDir := filepath.Join(tmpdir, ".git")
				if err := os.MkdirAll(mainGitDir, 0755); err != nil {
					t.Fatalf("failed to create main .git: %v", err)
				}
				mainGravaDir := filepath.Join(tmpdir, ".grava")
				if err := os.MkdirAll(mainGravaDir, 0755); err != nil {
					t.Fatalf("failed to create main .grava: %v", err)
				}

				// Create nested worktree directory
				wtDir := filepath.Join(tmpdir, "level1", "level2", "worktree")
				if err := os.MkdirAll(wtDir, 0755); err != nil {
					t.Fatalf("failed to create nested worktree: %v", err)
				}
				wtGitFile := filepath.Join(wtDir, ".git")
				if err := os.WriteFile(wtGitFile, []byte("gitdir: "+filepath.Join(tmpdir, ".git/worktrees/wt")+"\n"), 0644); err != nil {
					t.Fatalf("failed to create worktree .git: %v", err)
				}

				return wtDir, tmpdir
			},
			expectErr: false,
			validate: func(t *testing.T, path string, mainRepoDir string, worktreeCwd string) {
				absPath := filepath.Join(worktreeCwd, path)
				absPath, _ = filepath.Abs(absPath)
				expectedPath := filepath.Join(mainRepoDir, ".grava")
				expectedPath, _ = filepath.Abs(expectedPath)
				if absPath != expectedPath {
					t.Errorf("computed path resolves to %s, want %s", absPath, expectedPath)
				}
			},
		},
		{
			name: "no main repo found",
			setup: func(t *testing.T) (string, string) {
				// Isolated directory with no parent repos
				return t.TempDir(), ""
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worktreeCwd, mainRepoDir := tt.setup(t)
			path, err := ComputeRedirectPath(worktreeCwd)
			if (err != nil) != tt.expectErr {
				t.Errorf("ComputeRedirectPath() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if !tt.expectErr && tt.validate != nil {
				tt.validate(t, path, mainRepoDir, worktreeCwd)
			}
		})
	}
}

// TestWriteRedirectFile verifies redirect file creation and idempotency.
func TestWriteRedirectFile(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T) string
		expectCreated  bool
		expectErr      bool
		runTwice       bool // Run WriteRedirectFile twice to test idempotency
		expectSecondOK bool
	}{
		{
			name: "creates redirect file in valid worktree",
			setup: func(t *testing.T) string {
				tmpdir := t.TempDir()
				mainDir := filepath.Join(tmpdir, "main")
				wtDir := filepath.Join(tmpdir, "main", "wt1")

				if err := os.MkdirAll(mainDir, 0755); err != nil {
					t.Fatalf("failed to create main: %v", err)
				}
				mainGitDir := filepath.Join(mainDir, ".git")
				if err := os.MkdirAll(mainGitDir, 0755); err != nil {
					t.Fatalf("failed to create main .git: %v", err)
				}
				mainGravaDir := filepath.Join(mainDir, ".grava")
				if err := os.MkdirAll(mainGravaDir, 0755); err != nil {
					t.Fatalf("failed to create main .grava: %v", err)
				}

				if err := os.MkdirAll(wtDir, 0755); err != nil {
					t.Fatalf("failed to create worktree: %v", err)
				}
				wtGitFile := filepath.Join(wtDir, ".git")
				if err := os.WriteFile(wtGitFile, []byte("gitdir: "+filepath.Join(mainDir, ".git/worktrees/wt1")+"\n"), 0644); err != nil {
					t.Fatalf("failed to create worktree .git: %v", err)
				}

				return wtDir
			},
			expectCreated: true,
			expectErr:     false,
			runTwice:      true,
			expectSecondOK: false, // Second call should return false (already created)
		},
		{
			name: "fails if not in a worktree",
			setup: func(t *testing.T) string {
				tmpdir := t.TempDir()
				gitDir := filepath.Join(tmpdir, ".git")
				if err := os.MkdirAll(gitDir, 0755); err != nil {
					t.Fatalf("failed to create .git: %v", err)
				}
				return tmpdir
			},
			expectCreated: false,
			expectErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cwd := tt.setup(t)
			created, err := WriteRedirectFile(cwd)
			if (err != nil) != tt.expectErr {
				t.Errorf("WriteRedirectFile() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if !tt.expectErr && created != tt.expectCreated {
				t.Errorf("WriteRedirectFile() created = %v, want %v", created, tt.expectCreated)
			}

			if tt.runTwice && !tt.expectErr {
				created2, err2 := WriteRedirectFile(cwd)
				if err2 != nil {
					t.Errorf("WriteRedirectFile() second call error = %v", err2)
				}
				if created2 != tt.expectSecondOK {
					t.Errorf("WriteRedirectFile() second call created = %v, want %v", created2, tt.expectSecondOK)
				}
			}

			// Verify redirect file exists and has correct content
			if !tt.expectErr {
				redirectPath := filepath.Join(cwd, ".grava", "redirect")
				content, err := os.ReadFile(redirectPath)
				if err != nil {
					t.Errorf("failed to read redirect file: %v", err)
					return
				}
				path := strings.TrimSpace(string(content))
				if path == "" {
					t.Error("redirect file is empty")
				}
			}
		})
	}
}

// TestResolveGravaDirWithRedirect verifies the priority chain for .grava resolution.
func TestResolveGravaDirWithRedirect(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T) (string, string) // Returns (cwd, expected .grava path)
		env       map[string]string                     // Environment variables to set
		expectErr bool
	}{
		{
			name: "priority 1: GRAVA_DIR env var",
			setup: func(t *testing.T) (string, string) {
				tmpdir := t.TempDir()
				envDir := filepath.Join(tmpdir, "custom_grava")
				if err := os.MkdirAll(envDir, 0755); err != nil {
					t.Fatalf("failed to create custom grava: %v", err)
				}
				cwd := filepath.Join(tmpdir, "work")
				if err := os.MkdirAll(cwd, 0755); err != nil {
					t.Fatalf("failed to create cwd: %v", err)
				}
				return cwd, envDir
			},
			env: map[string]string{"GRAVA_DIR": ""}, // Will be set to envDir in test
			expectErr: false,
		},
		{
			name: "priority 2: worktree redirect file",
			setup: func(t *testing.T) (string, string) {
				tmpdir := t.TempDir()
				mainDir := filepath.Join(tmpdir, "main")
				wtDir := filepath.Join(tmpdir, "main", "wt")

				if err := os.MkdirAll(mainDir, 0755); err != nil {
					t.Fatalf("failed to create main: %v", err)
				}
				mainGitDir := filepath.Join(mainDir, ".git")
				if err := os.MkdirAll(mainGitDir, 0755); err != nil {
					t.Fatalf("failed to create main .git: %v", err)
				}
				mainGravaDir := filepath.Join(mainDir, ".grava")
				if err := os.MkdirAll(mainGravaDir, 0755); err != nil {
					t.Fatalf("failed to create main .grava: %v", err)
				}

				if err := os.MkdirAll(wtDir, 0755); err != nil {
					t.Fatalf("failed to create worktree: %v", err)
				}
				wtGitFile := filepath.Join(wtDir, ".git")
				if err := os.WriteFile(wtGitFile, []byte("gitdir: "+filepath.Join(mainDir, ".git/worktrees/wt")+"\n"), 0644); err != nil {
					t.Fatalf("failed to create worktree .git: %v", err)
				}
				_, _ = WriteRedirectFile(wtDir)

				return wtDir, mainGravaDir
			},
			expectErr: false,
		},
		{
			name: "priority 3: local .grava directory",
			setup: func(t *testing.T) (string, string) {
				tmpdir := t.TempDir()
				gravaDir := filepath.Join(tmpdir, ".grava")
				if err := os.MkdirAll(gravaDir, 0755); err != nil {
					t.Fatalf("failed to create .grava: %v", err)
				}
				return tmpdir, gravaDir
			},
			expectErr: false,
		},
		{
			name: "priority 4: walk up filesystem",
			setup: func(t *testing.T) (string, string) {
				tmpdir := t.TempDir()
				gravaDir := filepath.Join(tmpdir, ".grava")
				if err := os.MkdirAll(gravaDir, 0755); err != nil {
					t.Fatalf("failed to create .grava: %v", err)
				}
				// Create nested directory without local .grava
				nestedDir := filepath.Join(tmpdir, "a", "b", "c")
				if err := os.MkdirAll(nestedDir, 0755); err != nil {
					t.Fatalf("failed to create nested dir: %v", err)
				}
				return nestedDir, gravaDir
			},
			expectErr: false,
		},
		{
			name: "no .grava found",
			setup: func(t *testing.T) (string, string) {
				return t.TempDir(), ""
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cwd, expectedPath := tt.setup(t)

			// Set up environment variables
			for key, val := range tt.env {
				if key == "GRAVA_DIR" && val == "" && expectedPath != "" {
					os.Setenv(key, expectedPath)
					defer os.Unsetenv(key)
				}
			}

			resolved, err := ResolveGravaDirWithRedirect(cwd)
			if (err != nil) != tt.expectErr {
				t.Errorf("ResolveGravaDir() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if !tt.expectErr {
				expectedAbs, _ := filepath.Abs(expectedPath)
				if resolved != expectedAbs {
					t.Errorf("ResolveGravaDir() = %s, want %s", resolved, expectedAbs)
				}
			}
		})
	}
}

// TestCheckWorktreeConflict verifies conflict detection for existing worktrees/branches.
func TestCheckWorktreeConflict(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T) (string, string) // Returns (cwd, issueID)
		expectErr bool
		errMsg    string
	}{
		{
			name: "no conflict",
			setup: func(t *testing.T) (string, string) {
				return t.TempDir(), "grava-test"
			},
			expectErr: false,
		},
		{
			name: "existing worktree directory",
			setup: func(t *testing.T) (string, string) {
				tmpdir := t.TempDir()
				issueID := "grava-test"
				worktreeDir := filepath.Join(tmpdir, ".worktree", issueID)
				if err := os.MkdirAll(worktreeDir, 0755); err != nil {
					t.Fatalf("failed to create worktree dir: %v", err)
				}
				return tmpdir, issueID
			},
			expectErr: true,
			errMsg:    "already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cwd, issueID := tt.setup(t)
			err := CheckWorktreeConflict(cwd, issueID)
			if (err != nil) != tt.expectErr {
				t.Errorf("CheckWorktreeConflict() error = %v, expectErr %v", err, tt.expectErr)
			}
			if tt.expectErr && tt.errMsg != "" && (err == nil || !strings.Contains(err.Error(), tt.errMsg)) {
				t.Errorf("CheckWorktreeConflict() error message doesn't contain %q: %v", tt.errMsg, err)
			}
		})
	}
}

// TestDeleteWorktree verifies worktree deletion for directory cleanup.
func TestDeleteWorktree(t *testing.T) {
	tmpdir := t.TempDir()
	issueID := "grava-test"

	// Initialize a test git repo
	if err := runCmd(tmpdir, "git", "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	if err := runCmd(tmpdir, "git", "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("failed to configure git: %v", err)
	}
	if err := runCmd(tmpdir, "git", "config", "user.name", "Test User"); err != nil {
		t.Fatalf("failed to configure git: %v", err)
	}

	// Create initial commit
	dummyFile := filepath.Join(tmpdir, "dummy.txt")
	if err := os.WriteFile(dummyFile, []byte("dummy"), 0644); err != nil {
		t.Fatalf("failed to create dummy file: %v", err)
	}
	if err := runCmd(tmpdir, "git", "add", "dummy.txt"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}
	if err := runCmd(tmpdir, "git", "commit", "-m", "initial commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Create worktree using ProvisionWorktree
	if err := ProvisionWorktree(tmpdir, issueID); err != nil {
		t.Fatalf("ProvisionWorktree() error = %v", err)
	}

	// Test deletion
	if err := DeleteWorktree(tmpdir, issueID); err != nil {
		t.Errorf("DeleteWorktree() error = %v", err)
	}

	// Verify directory was deleted
	worktreeDir := filepath.Join(tmpdir, ".worktree", issueID)
	if _, err := os.Stat(worktreeDir); !os.IsNotExist(err) {
		t.Errorf("worktree directory still exists")
	}
}

// runCmd executes a command with given arguments in the specified directory.
// Used by tests for git operations.
func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}

// TestLinkClaudeWorktree verifies symlink creation from .claude/worktrees/<id> to .worktree/<id>.
func TestLinkClaudeWorktree(t *testing.T) {
	tmpdir := t.TempDir()
	issueID := "grava-symlink-test"

	// Create the target worktree directory (as if ProvisionWorktree already ran)
	worktreeDir := filepath.Join(tmpdir, ".worktree", issueID)
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}
	// Write a marker file so we can verify the symlink works
	if err := os.WriteFile(filepath.Join(worktreeDir, "marker.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write marker: %v", err)
	}

	// Run the function
	if err := LinkClaudeWorktree(tmpdir, issueID); err != nil {
		t.Fatalf("LinkClaudeWorktree() error = %v", err)
	}

	// Verify symlink exists at .claude/worktrees/<id>
	symlinkPath := filepath.Join(tmpdir, ".claude", "worktrees", issueID)
	info, err := os.Lstat(symlinkPath)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected symlink, got %v", info.Mode())
	}

	// Verify symlink resolves to the worktree directory
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("failed to read symlink: %v", err)
	}
	// Resolve and compare absolute paths
	resolvedTarget := filepath.Join(filepath.Dir(symlinkPath), target)
	absTarget, _ := filepath.Abs(resolvedTarget)
	absWorktree, _ := filepath.Abs(worktreeDir)
	if absTarget != absWorktree {
		t.Errorf("symlink target = %s, want %s", absTarget, absWorktree)
	}

	// Verify we can read through the symlink
	content, err := os.ReadFile(filepath.Join(symlinkPath, "marker.txt"))
	if err != nil {
		t.Errorf("failed to read through symlink: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("content = %q, want %q", string(content), "hello")
	}
}

// TestLinkClaudeWorktree_Idempotent verifies that calling it twice doesn't fail.
func TestLinkClaudeWorktree_Idempotent(t *testing.T) {
	tmpdir := t.TempDir()
	issueID := "grava-idem-test"

	worktreeDir := filepath.Join(tmpdir, ".worktree", issueID)
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	// Call twice
	if err := LinkClaudeWorktree(tmpdir, issueID); err != nil {
		t.Fatalf("first call error = %v", err)
	}
	if err := LinkClaudeWorktree(tmpdir, issueID); err != nil {
		t.Fatalf("second call error = %v", err)
	}
}

// TestLinkClaudeWorktree_MissingTarget verifies error when .worktree/<id> doesn't exist.
func TestLinkClaudeWorktree_MissingTarget(t *testing.T) {
	tmpdir := t.TempDir()
	err := LinkClaudeWorktree(tmpdir, "grava-nonexistent")
	if err == nil {
		t.Error("expected error for missing worktree target, got nil")
	}
}

// TestIsWorktreeDirty verifies dirty-state detection in a worktree.
func TestIsWorktreeDirty(t *testing.T) {
	tmpdir := t.TempDir()

	// Set up a real git repo with a worktree
	if err := runCmd(tmpdir, "git", "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := runCmd(tmpdir, "git", "config", "user.email", "test@test.com"); err != nil {
		t.Fatalf("git config: %v", err)
	}
	if err := runCmd(tmpdir, "git", "config", "user.name", "Test"); err != nil {
		t.Fatalf("git config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpdir, "f.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(tmpdir, "git", "add", "f.txt"); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(tmpdir, "git", "commit", "-m", "init"); err != nil {
		t.Fatal(err)
	}

	issueID := "grava-dirty-test"
	if err := ProvisionWorktree(tmpdir, issueID); err != nil {
		t.Fatalf("provision: %v", err)
	}
	wtDir := filepath.Join(tmpdir, ".worktree", issueID)

	// Clean worktree — should not be dirty
	dirty, err := IsWorktreeDirty(tmpdir, issueID)
	if err != nil {
		t.Fatalf("IsWorktreeDirty (clean): %v", err)
	}
	if dirty {
		t.Error("expected clean worktree, got dirty")
	}

	// Create untracked file — should be dirty
	if err := os.WriteFile(filepath.Join(wtDir, "new.txt"), []byte("y"), 0644); err != nil {
		t.Fatal(err)
	}
	dirty, err = IsWorktreeDirty(tmpdir, issueID)
	if err != nil {
		t.Fatalf("IsWorktreeDirty (untracked): %v", err)
	}
	if !dirty {
		t.Error("expected dirty worktree (untracked file)")
	}
}

// TestRemoveWorktreeOnly verifies worktree directory removal while keeping the branch.
func TestRemoveWorktreeOnly(t *testing.T) {
	tmpdir := t.TempDir()

	if err := runCmd(tmpdir, "git", "init"); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(tmpdir, "git", "config", "user.email", "t@t.com"); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(tmpdir, "git", "config", "user.name", "T"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpdir, "f.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(tmpdir, "git", "add", "f.txt"); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(tmpdir, "git", "commit", "-m", "init"); err != nil {
		t.Fatal(err)
	}

	issueID := "grava-remove-only"
	if err := ProvisionWorktree(tmpdir, issueID); err != nil {
		t.Fatalf("provision: %v", err)
	}

	// Remove worktree only
	if err := RemoveWorktreeOnly(tmpdir, issueID); err != nil {
		t.Fatalf("RemoveWorktreeOnly: %v", err)
	}

	// Directory should be gone
	wtDir := filepath.Join(tmpdir, ".worktree", issueID)
	if _, err := os.Stat(wtDir); !os.IsNotExist(err) {
		t.Error("worktree directory still exists")
	}

	// Branch should still exist
	cmd := exec.Command("git", "rev-parse", "--verify", "grava/"+issueID)
	cmd.Dir = tmpdir
	if err := cmd.Run(); err != nil {
		t.Error("branch grava/" + issueID + " was deleted, should have been kept")
	}
}

// TestSyncClaudeSettings verifies settings.json is copied from main repo to worktree.
func TestSyncClaudeSettings(t *testing.T) {
	mainRepo := t.TempDir()
	worktree := t.TempDir()

	t.Run("copies settings when source exists", func(t *testing.T) {
		srcDir := filepath.Join(mainRepo, ".claude")
		if err := os.MkdirAll(srcDir, 0755); err != nil {
			t.Fatal(err)
		}
		content := `{"enabledPlugins":{}}`
		if err := os.WriteFile(filepath.Join(srcDir, "settings.json"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		if err := SyncClaudeSettings(mainRepo, worktree); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := os.ReadFile(filepath.Join(worktree, ".claude", "settings.json"))
		if err != nil {
			t.Fatalf("destination not written: %v", err)
		}
		if string(got) != content {
			t.Errorf("content mismatch: got %q want %q", got, content)
		}
	})

	t.Run("idempotent on re-run", func(t *testing.T) {
		if err := SyncClaudeSettings(mainRepo, worktree); err != nil {
			t.Fatalf("second call failed: %v", err)
		}
	})

	t.Run("graceful when source absent", func(t *testing.T) {
		emptyRepo := t.TempDir()
		if err := SyncClaudeSettings(emptyRepo, worktree); err != nil {
			t.Fatalf("should return nil when source missing, got: %v", err)
		}
	})
}

// TestConfigureGitUser verifies git user config is propagated into the worktree.
func TestConfigureGitUser(t *testing.T) {
	// Create a real git repo for mainRepo so git config --local works
	mainRepo := t.TempDir()
	for _, cmd := range [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test User"},
	} {
		if err := runCmd(mainRepo, cmd[0], cmd[1:]...); err != nil {
			t.Fatalf("setup %v: %v", cmd, err)
		}
	}

	// Create a real git worktree dir
	worktreeDir := t.TempDir()
	if err := runCmd(worktreeDir, "git", "init"); err != nil {
		t.Fatal(err)
	}

	t.Run("sets git identity in worktree", func(t *testing.T) {
		if err := ConfigureGitUser(mainRepo, worktreeDir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		name, err := gitConfigGet(worktreeDir, "user.name")
		if err != nil || name != "Test User" {
			t.Errorf("user.name: got %q err %v", name, err)
		}
		email, err := gitConfigGet(worktreeDir, "user.email")
		if err != nil || email != "test@example.com" {
			t.Errorf("user.email: got %q err %v", email, err)
		}
	})

	t.Run("graceful when main repo has no identity", func(t *testing.T) {
		emptyRepo := t.TempDir()
		if err := runCmd(emptyRepo, "git", "init"); err != nil {
			t.Fatal(err)
		}
		emptyWorktree := t.TempDir()
		if err := runCmd(emptyWorktree, "git", "init"); err != nil {
			t.Fatal(err)
		}
		if err := ConfigureGitUser(emptyRepo, emptyWorktree); err != nil {
			t.Fatalf("should return nil when identity absent, got: %v", err)
		}
	})
}

// TestIsInsideClaudeWorktree verifies Claude worktree environment detection.
func TestIsInsideClaudeWorktree(t *testing.T) {
	tmpdir := t.TempDir()

	// Not inside Claude worktree
	if IsInsideClaudeWorktree(tmpdir) {
		t.Error("false positive: tmpdir is not a Claude worktree")
	}

	// Create .claude/worktrees/<id> structure
	claudeWT := filepath.Join(tmpdir, ".claude", "worktrees", "grava-test")
	if err := os.MkdirAll(claudeWT, 0755); err != nil {
		t.Fatal(err)
	}
	if IsInsideClaudeWorktree(claudeWT) {
		t.Log("correctly detected Claude worktree")
	} else {
		t.Error("failed to detect Claude worktree directory")
	}
}
