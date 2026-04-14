package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IsWorktree checks if the current directory is a Git worktree by examining
// whether .git is a file (worktree) or directory (main repo).
// Zero subprocess cost — pure filesystem check.
func IsWorktree(cwd string) bool {
	gitPath := filepath.Join(cwd, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		// .git doesn't exist or can't be stat'd — not in a repo
		return false
	}
	// If .git is a file, it's a worktree pointer
	return !info.IsDir()
}

// FindMainRepo walks up the directory tree from cwd until it finds a directory
// whose .git entry is a directory (not a file), which indicates a main repository
// (as opposed to a worktree). Returns the absolute path of the main repo root.
func FindMainRepo(cwd string) (string, error) {
	current := cwd
	for depth := 0; depth <= 20; depth++ {
		gitPath := filepath.Join(current, ".git")
		info, err := os.Stat(gitPath)
		if err == nil && info.IsDir() {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("main repository not found: walked up to filesystem root")
		}
		current = parent
	}
	return "", fmt.Errorf("main repository not found within 20 parent directories")
}

// ComputeRedirectPath computes the relative path from a worktree to the main
// repo's .grava directory (e.g. "../../.grava"). Returns an error if the main
// repo cannot be found by walking up from cwd.
func ComputeRedirectPath(cwd string) (string, error) {
	mainRepo, err := FindMainRepo(cwd)
	if err != nil {
		return "", err
	}
	relPath, err := filepath.Rel(cwd, filepath.Join(mainRepo, ".grava"))
	if err != nil {
		return "", fmt.Errorf("failed to compute relative path: %w", err)
	}
	return relPath, nil
}

// WriteRedirectFile creates or updates .grava/redirect in a worktree.
// The redirect file contains a relative path to the main repo's .grava directory.
// Idempotent: if file exists, verifies content is correct (or updates silently).
// Returns an error if cwd is not a worktree.
func WriteRedirectFile(cwd string) (bool, error) {
	// Verify this is a worktree before creating redirect
	if !IsWorktree(cwd) {
		return false, fmt.Errorf("not a git worktree: .git must be a file (worktree pointer), not a directory")
	}

	gravaDir := filepath.Join(cwd, ".grava")
	redirectPath := filepath.Join(gravaDir, "redirect")

	// Ensure .grava directory exists
	if err := os.MkdirAll(gravaDir, 0755); err != nil {
		return false, fmt.Errorf("failed to create .grava directory: %w", err)
	}

	// Compute the redirect path
	relPath, err := ComputeRedirectPath(cwd)
	if err != nil {
		return false, fmt.Errorf("failed to compute redirect path: %w", err)
	}

	// Check if redirect file already exists
	if _, err := os.Stat(redirectPath); err == nil {
		// File exists — verify content (idempotent)
		existingContent, err := os.ReadFile(redirectPath)
		if err == nil && strings.TrimSpace(string(existingContent)) == relPath {
			// Content is correct, no change needed
			return false, nil
		}
		// Content differs or unreadable — update it
	}

	// Write redirect file
	if err := os.WriteFile(redirectPath, []byte(relPath+"\n"), 0644); err != nil {
		return false, fmt.Errorf("failed to write redirect file: %w", err)
	}

	// Check if this is the first time writing
	return true, nil
}

// ResolveGravaDirWithRedirect resolves the .grava directory using the priority chain:
// 1. GRAVA_DIR environment variable (if set)
// 2. Per-worktree .grava/redirect file (if present)
// 3. Main repository's .grava/ directory
// 4. Walk up filesystem from cwd
//
// Returns an absolute path to the .grava directory.
// This is the ADR-004 worktree-aware version.
func ResolveGravaDirWithRedirect(cwd string) (string, error) {
	// Priority 1: GRAVA_DIR environment variable
	if envDir := os.Getenv("GRAVA_DIR"); envDir != "" {
		// Convert to absolute path if relative
		if !filepath.IsAbs(envDir) {
			absDir, err := filepath.Abs(envDir)
			if err != nil {
				return "", fmt.Errorf("failed to resolve GRAVA_DIR: %w", err)
			}
			envDir = absDir
		}
		if _, err := os.Stat(envDir); err == nil {
			return envDir, nil
		}
		// GRAVA_DIR set but doesn't exist — fall through
	}

	// Priority 2: Per-worktree .grava/redirect file (if in a worktree)
	if IsWorktree(cwd) {
		redirectPath := filepath.Join(cwd, ".grava", "redirect")
		if content, err := os.ReadFile(redirectPath); err == nil {
			relPath := strings.TrimSpace(string(content))
			absPath := filepath.Join(cwd, relPath)
			absPath, _ = filepath.Abs(absPath)
			if _, err := os.Stat(absPath); err == nil {
				return absPath, nil
			}
			// Redirect exists but path doesn't — fall through
		}
	}

	// Priority 3: .grava/ in current directory
	gravaDir := filepath.Join(cwd, ".grava")
	if _, err := os.Stat(gravaDir); err == nil {
		return gravaDir, nil
	}

	// Priority 4: Walk up filesystem from cwd
	current := cwd
	for {
		gravaDir := filepath.Join(current, ".grava")
		if _, err := os.Stat(gravaDir); err == nil {
			return gravaDir, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root without finding .grava
			return "", fmt.Errorf(".grava directory not found: searched from %s to filesystem root", cwd)
		}
		current = parent
	}
}

// CheckWorktreeConflict checks if a worktree directory or branch already exists.
// Returns an error if either .worktree/<issueID> or grava/<issueID> branch exists.
func CheckWorktreeConflict(cwd, issueID string) error {
	// Check if worktree directory exists
	worktreeDir := filepath.Join(cwd, ".worktree", issueID)
	if _, err := os.Stat(worktreeDir); err == nil {
		return fmt.Errorf("worktree directory %s already exists", worktreeDir)
	}

	// Check if branch exists (grava/<issueID>)
	cmd := exec.Command("git", "rev-parse", "--verify", fmt.Sprintf("grava/%s", issueID))
	cmd.Dir = cwd
	if err := cmd.Run(); err == nil {
		// Branch exists
		return fmt.Errorf("branch grava/%s already exists", issueID)
	}

	return nil
}

// WorktreePath returns the canonical worktree directory for an issue:
// <cwd>/.worktree/<issueID>. Used by ProvisionWorktree and any callers that
// need to reference the worktree directory after provisioning.
func WorktreePath(cwd, issueID string) string {
	return filepath.Join(cwd, ".worktree", issueID)
}

// ProvisionWorktree creates a git worktree at .worktree/<issueID> with branch grava/<issueID>.
// Executes: git worktree add .worktree/<issueID> -b grava/<issueID>
// Assumes cwd is the main repository root.
func ProvisionWorktree(cwd, issueID string) error {
	worktreeDir := WorktreePath(cwd, issueID)
	branchName := fmt.Sprintf("grava/%s", issueID)

	cmd := exec.Command("git", "worktree", "add", worktreeDir, "-b", branchName)
	cmd.Dir = cwd

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to provision worktree: %w\ngit output: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// DeleteWorktree removes the worktree directory and prunes the branch (for rollback).
// Executes: git worktree remove .worktree/<issueID> --force
// Then: git branch -D grava/<issueID> (if branch exists)
// Returns an error if worktree removal fails; branch deletion failures are collected
// but do not block cleanup.
func DeleteWorktree(cwd, issueID string) error {
	worktreeDir := filepath.Join(cwd, ".worktree", issueID)
	branchName := fmt.Sprintf("grava/%s", issueID)
	var errs []string

	// Remove worktree
	cmd := exec.Command("git", "worktree", "remove", worktreeDir, "--force")
	cmd.Dir = cwd
	if output, err := cmd.CombinedOutput(); err != nil {
		errs = append(errs, fmt.Sprintf("worktree remove: %v (%s)", err, strings.TrimSpace(string(output))))
	}

	// Delete branch
	cmd = exec.Command("git", "branch", "-D", branchName)
	cmd.Dir = cwd
	if output, err := cmd.CombinedOutput(); err != nil {
		errs = append(errs, fmt.Sprintf("branch delete: %v (%s)", err, strings.TrimSpace(string(output))))
	}

	// Clean up empty .worktree directory
	worktreeParent := filepath.Join(cwd, ".worktree")
	if entries, err := os.ReadDir(worktreeParent); err == nil && len(entries) == 0 {
		_ = os.RemoveAll(worktreeParent)
	}

	if len(errs) > 0 {
		return fmt.Errorf("delete worktree cleanup errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// LinkClaudeWorktree creates a symlink from .claude/worktrees/<issueID> to .worktree/<issueID>.
// This redirects Claude Code's default worktree location to grava's unified .worktree/ directory.
// Idempotent: if the symlink already exists and points to the correct target, returns nil.
// Returns an error if the target .worktree/<issueID> does not exist.
func LinkClaudeWorktree(cwd, issueID string) error {
	worktreeDir := filepath.Join(cwd, ".worktree", issueID)
	if _, err := os.Stat(worktreeDir); err != nil {
		return fmt.Errorf("worktree target does not exist: %s", worktreeDir)
	}

	claudeWorktreesDir := filepath.Join(cwd, ".claude", "worktrees")
	symlinkPath := filepath.Join(claudeWorktreesDir, issueID)

	// Check if symlink already exists
	if target, err := os.Readlink(symlinkPath); err == nil {
		resolved := filepath.Join(filepath.Dir(symlinkPath), target)
		absResolved, _ := filepath.Abs(resolved)
		absWorktree, _ := filepath.Abs(worktreeDir)
		if absResolved == absWorktree {
			return nil // Already correct
		}
		// Wrong target — remove and recreate
		if err := os.Remove(symlinkPath); err != nil {
			return fmt.Errorf("failed to remove stale symlink: %w", err)
		}
	}

	// Ensure .claude/worktrees/ directory exists
	if err := os.MkdirAll(claudeWorktreesDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claude/worktrees directory: %w", err)
	}

	// Compute relative path from symlink location to target
	relPath, err := filepath.Rel(claudeWorktreesDir, worktreeDir)
	if err != nil {
		return fmt.Errorf("failed to compute relative path: %w", err)
	}

	if err := os.Symlink(relPath, symlinkPath); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	return nil
}

// IsWorktreeDirty checks if the worktree at .worktree/<issueID> has uncommitted changes.
// Returns true if there are staged, unstaged, or untracked files.
func IsWorktreeDirty(cwd, issueID string) (bool, error) {
	worktreeDir := filepath.Join(cwd, ".worktree", issueID)
	if _, err := os.Stat(worktreeDir); err != nil {
		return false, fmt.Errorf("worktree does not exist: %s", worktreeDir)
	}

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = worktreeDir
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check worktree status: %w", err)
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

// RemoveWorktreeOnly removes the worktree directory but keeps the branch intact.
// Used by `grava stop` for pausing work — the branch is preserved for future resumption.
func RemoveWorktreeOnly(cwd, issueID string) error {
	worktreeDir := filepath.Join(cwd, ".worktree", issueID)

	cmd := exec.Command("git", "worktree", "remove", worktreeDir, "--force")
	cmd.Dir = cwd
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove worktree: %w\ngit output: %s", err, strings.TrimSpace(string(output)))
	}

	// Clean up empty .worktree directory
	worktreeParent := filepath.Join(cwd, ".worktree")
	if entries, err := os.ReadDir(worktreeParent); err == nil && len(entries) == 0 {
		_ = os.RemoveAll(worktreeParent)
	}

	return nil
}

// IsInsideClaudeWorktree checks if the given path is inside a .claude/worktrees/ directory.
func IsInsideClaudeWorktree(cwd string) bool {
	return strings.Contains(filepath.ToSlash(cwd), ".claude/worktrees/")
}

// SyncClaudeSettings copies .claude/settings.json from the main repo into the worktree.
// Returns nil (not an error) when the source file does not exist.
// Idempotent: skips the write if destination already has identical content.
func SyncClaudeSettings(mainRepoDir, worktreeDir string) error {
	src := filepath.Join(mainRepoDir, ".claude", "settings.json")
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read source settings: %w", err)
	}

	destDir := filepath.Join(worktreeDir, ".claude")
	dest := filepath.Join(destDir, "settings.json")

	// Idempotent: skip if destination already has identical content
	if existing, err := os.ReadFile(dest); err == nil && string(existing) == string(data) {
		return nil
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claude dir in worktree: %w", err)
	}
	if err := os.WriteFile(dest, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings.json to worktree: %w", err)
	}
	return nil
}

// ConfigureGitUser reads user.name and user.email from the main repo's git config
// and writes them as worktree-local git config in worktreeDir.
// Returns nil (not an error) when either value is absent in the main repo config.
func ConfigureGitUser(mainRepoDir, worktreeDir string) error {
	name, nameErr := gitConfigGet(mainRepoDir, "user.name")
	email, emailErr := gitConfigGet(mainRepoDir, "user.email")

	if nameErr != nil && emailErr != nil {
		// Neither value present — nothing to configure
		return nil
	}

	if name != "" {
		if err := gitConfigSet(worktreeDir, "user.name", name); err != nil {
			return fmt.Errorf("failed to set git user.name in worktree: %w", err)
		}
	}
	if email != "" {
		if err := gitConfigSet(worktreeDir, "user.email", email); err != nil {
			return fmt.Errorf("failed to set git user.email in worktree: %w", err)
		}
	}
	return nil
}

// gitConfigGet reads a single git config value from the repo at dir.
// Returns ("", nil) when the key is not set.
func gitConfigGet(dir, key string) (string, error) {
	cmd := exec.Command("git", "config", "--local", key)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// Exit code 1 means key not set — treat as absent, not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", fmt.Errorf("git config get %s: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// gitConfigSet writes a single git config value in the repo at dir (--local scope).
func gitConfigSet(dir, key, value string) error {
	cmd := exec.Command("git", "config", "--local", key, value)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git config set %s: %w\n%s", key, err, strings.TrimSpace(string(out)))
	}
	return nil
}
