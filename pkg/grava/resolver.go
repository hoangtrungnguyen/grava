package grava

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
)

// ResolveGravaDir resolves the .grava/ directory using the ADR-004 priority chain:
// 1. GRAVA_DIR env var (if set)
// 2. .grava/redirect file (relative path redirect)
// 3. CWD walk up to filesystem root
func ResolveGravaDir() (string, error) {
	// Priority 1: GRAVA_DIR env var
	if dir := os.Getenv("GRAVA_DIR"); dir != "" {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
		return "", gravaerrors.New("NOT_INITIALIZED",
			fmt.Sprintf("GRAVA_DIR=%q does not exist or is not a directory", dir), nil)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// Priority 2: Walk upward checking for .grava/redirect file
	dir := cwd
	for {
		redirectPath := filepath.Join(dir, ".grava", "redirect")
		if data, err := os.ReadFile(redirectPath); err == nil {
			target := strings.TrimSpace(string(data))
			if !filepath.IsAbs(target) {
				// Relative to the .grava/ directory
				target = filepath.Join(dir, ".grava", target)
			}
			target = filepath.Clean(target)
			if info, err := os.Stat(target); err == nil && info.IsDir() {
				return target, nil
			}
			return "", gravaerrors.New("REDIRECT_STALE",
				fmt.Sprintf(".grava/redirect points to %q which does not exist", target), nil)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Priority 3: Walk upward looking for .grava/ directory
	dir = cwd
	for {
		candidate := filepath.Join(dir, ".grava")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", gravaerrors.New("NOT_INITIALIZED",
		"no .grava/ directory found — run 'grava init' to initialise", nil)
}
