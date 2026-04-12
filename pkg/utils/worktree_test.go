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
