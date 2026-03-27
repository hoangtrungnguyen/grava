package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const gravaExcludeEntry = ".grava/"

// WriteGitExclude adds ".grava/" to .git/info/exclude and migrates from .gitignore.
// Returns (migrated=true) if a .gitignore migration was performed.
// Idempotent: safe to call multiple times.
// Gracefully skips if .git directory is absent.
func WriteGitExclude(repoRoot string) (migrated bool, err error) {
	gitDir := filepath.Join(repoRoot, ".git")
	if _, statErr := os.Stat(gitDir); os.IsNotExist(statErr) {
		return false, nil // not a git repo, skip gracefully
	}

	infoDir := filepath.Join(gitDir, "info")
	if err := os.MkdirAll(infoDir, 0755); err != nil {
		return false, fmt.Errorf("failed to create .git/info/: %w", err)
	}

	excludeFile := filepath.Join(infoDir, "exclude")
	existing, _ := os.ReadFile(excludeFile)
	if strings.Contains(string(existing), gravaExcludeEntry) {
		// Already present — still try to migrate .gitignore
		return migrateGitignore(repoRoot)
	}

	f, err := os.OpenFile(excludeFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return false, fmt.Errorf("failed to open .git/info/exclude: %w", err)
	}
	defer f.Close() //nolint:errcheck
	content := string(existing)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return false, fmt.Errorf("failed to write newline to .git/info/exclude: %w", err)
		}
	}
	if _, err := f.WriteString(gravaExcludeEntry + "\n"); err != nil {
		return false, fmt.Errorf("failed to write .git/info/exclude: %w", err)
	}

	return migrateGitignore(repoRoot)
}

func migrateGitignore(repoRoot string) (bool, error) {
	gitignorePath := filepath.Join(repoRoot, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read .gitignore: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var filtered []string
	removed := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == ".grava/" || trimmed == ".grava" {
			removed = true
			continue
		}
		filtered = append(filtered, line)
	}
	if !removed {
		return false, nil
	}

	if err := os.WriteFile(gitignorePath, []byte(strings.Join(filtered, "\n")), 0644); err != nil {
		return false, fmt.Errorf("failed to write .gitignore: %w", err)
	}
	return true, nil
}
