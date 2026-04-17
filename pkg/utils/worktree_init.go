package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureWorktreeDir creates the .worktree/ directory at repoRoot if it doesn't exist.
// Returns true if the directory was created, false if it already existed.
func EnsureWorktreeDir(repoRoot string) (bool, error) {
	dir := filepath.Join(repoRoot, ".worktree")
	if _, err := os.Stat(dir); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false, fmt.Errorf("failed to create .worktree directory: %w", err)
	}
	return true, nil
}

// EnsureWorktreeGitignore adds ".worktree/" to the project's .gitignore if not already present.
// Creates the .gitignore file if it doesn't exist.
// Idempotent: returns false if entry already present.
func EnsureWorktreeGitignore(repoRoot string) (bool, error) {
	gitignorePath := filepath.Join(repoRoot, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to read .gitignore: %w", err)
	}

	const entry = ".worktree/"
	if hasExactLine(string(content), entry) {
		return false, nil
	}

	f, err := os.OpenFile(gitignorePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return false, fmt.Errorf("failed to open .gitignore: %w", err)
	}
	defer f.Close() //nolint:errcheck

	// Ensure we start on a new line
	if len(content) > 0 && !strings.HasSuffix(string(content), "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return false, fmt.Errorf("failed to write newline to .gitignore: %w", err)
		}
	}
	if _, err := f.WriteString(entry + "\n"); err != nil {
		return false, fmt.Errorf("failed to write .gitignore entry: %w", err)
	}
	return true, nil
}

// SetWorktreeGitConfig sets the local git config entry grava.worktreeDir = .worktree.
// This serves as the source of truth for the binary.
func SetWorktreeGitConfig(repoRoot string) error {
	return gitConfigSet(repoRoot, "grava.worktreeDir", ".worktree")
}

// EnsureClaudeWorktreeSettings updates .claude/settings.json to include the
// worktree configuration block with symlinkDirectories and sparsePaths.
// Creates the file and directory if they don't exist.
// If the file already has a worktree block, it is left unchanged (idempotent).
func EnsureClaudeWorktreeSettings(repoRoot string) (bool, error) {
	claudeDir := filepath.Join(repoRoot, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.json")

	// Read existing settings or start fresh
	var settings map[string]interface{}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, fmt.Errorf("failed to read .claude/settings.json: %w", err)
		}
		settings = make(map[string]interface{})
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			return false, fmt.Errorf("failed to parse .claude/settings.json: %w", err)
		}
		if settings == nil {
			settings = make(map[string]interface{})
		}
	}

	// Check if worktree block already exists
	if _, exists := settings["worktree"]; exists {
		return false, nil
	}

	// Add worktree block
	settings["worktree"] = map[string]interface{}{
		"symlinkDirectories": []string{"node_modules", ".cache"},
		"sparsePaths":        []string{},
	}

	// Marshal with indentation
	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, fmt.Errorf("failed to serialize settings: %w", err)
	}

	// Ensure .claude directory exists
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return false, fmt.Errorf("failed to create .claude directory: %w", err)
	}

	if err := os.WriteFile(settingsPath, append(output, '\n'), 0644); err != nil {
		return false, fmt.Errorf("failed to write .claude/settings.json: %w", err)
	}

	return true, nil
}
