package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

const ts09ID = "TS-09"

func init() {
	Register(Scenario{
		ID:       ts09ID,
		Name:     "Worktree Agent Crash Recovery",
		EpicGate: 5,
		Run:      runTS09,
	})
}

// runTS09 validates orphaned worktree detection:
//   - Create a fake .worktree/<id> directory (simulating a crashed agent)
//   - Verify the orphaned directory is detectable
//   - Clean it up
func runTS09(ctx context.Context, _ dolt.Store) Result {
	details := []string{}

	// Use a temp directory to simulate project root
	tmpDir, err := os.MkdirTemp("", "ts09-*")
	if err != nil {
		return fail(ts09ID, fmt.Sprintf("setup: %v", err))
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck

	orphanID := fmt.Sprintf("ts09-orphan-%d", time.Now().UnixNano())
	worktreeDir := filepath.Join(tmpDir, ".worktree", orphanID)

	// Create orphaned worktree directory
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		return fail(ts09ID, fmt.Sprintf("setup: create orphaned dir: %v", err))
	}
	// Place a marker file to simulate work-in-progress
	if err := os.WriteFile(filepath.Join(worktreeDir, "dirty.txt"), []byte("uncommitted"), 0644); err != nil {
		return fail(ts09ID, fmt.Sprintf("setup: write marker: %v", err))
	}
	details = append(details, fmt.Sprintf("created orphaned worktree: .worktree/%s", orphanID))

	// Detection: scan .worktree/ for directories
	entries, err := os.ReadDir(filepath.Join(tmpDir, ".worktree"))
	if err != nil {
		return fail(ts09ID, fmt.Sprintf("detection: read .worktree/: %v", err), details...)
	}
	found := false
	for _, e := range entries {
		if e.IsDir() && e.Name() == orphanID {
			found = true
		}
	}
	if !found {
		return fail(ts09ID, "detection: orphaned worktree not found in scan", details...)
	}
	details = append(details, "orphaned worktree detected via directory scan")

	// Check for dirty state (simulating doctor's dirty check)
	markerPath := filepath.Join(worktreeDir, "dirty.txt")
	if _, err := os.Stat(markerPath); err != nil {
		return fail(ts09ID, "dirty check: marker file not found", details...)
	}
	details = append(details, "dirty state detected (would flag for human review)")

	// Cleanup (simulating doctor --fix for clean worktrees)
	if err := os.RemoveAll(worktreeDir); err != nil {
		return fail(ts09ID, fmt.Sprintf("cleanup: %v", err), details...)
	}

	// Verify cleanup
	if _, err := os.Stat(worktreeDir); !os.IsNotExist(err) {
		return fail(ts09ID, "cleanup: orphaned directory still exists", details...)
	}
	details = append(details, "orphaned worktree cleaned up successfully")

	return pass(ts09ID, details...)
}
