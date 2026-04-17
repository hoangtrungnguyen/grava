package utils

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// MinGitMajor and MinGitMinor define the minimum Git version required for
// worktree operations (git worktree remove requires 2.17+).
const (
	MinGitMajor = 2
	MinGitMinor = 17
)

// CheckGitVersion verifies that the installed Git version meets the minimum
// requirement (2.17+). Returns nil if satisfied, or an error describing the
// mismatch or inability to determine the version.
func CheckGitVersion() error {
	cmd := exec.Command("git", "--version")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run git --version: %w", err)
	}
	return ParseAndCheckGitVersion(strings.TrimSpace(string(out)))
}

// ParseAndCheckGitVersion parses a "git version X.Y.Z" string and checks
// whether the version meets the minimum requirement.
func ParseAndCheckGitVersion(versionStr string) error {
	// Expected format: "git version 2.39.3 (Apple Git-146)" or "git version 2.39.3"
	parts := strings.Fields(versionStr)
	if len(parts) < 3 {
		return fmt.Errorf("unexpected git version format: %s", versionStr)
	}

	versionParts := strings.SplitN(parts[2], ".", 3)
	if len(versionParts) < 2 {
		return fmt.Errorf("unexpected git version number: %s", parts[2])
	}

	major, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return fmt.Errorf("cannot parse git major version: %s", versionParts[0])
	}
	minor, err := strconv.Atoi(versionParts[1])
	if err != nil {
		return fmt.Errorf("cannot parse git minor version: %s", versionParts[1])
	}

	if major < MinGitMajor || (major == MinGitMajor && minor < MinGitMinor) {
		return fmt.Errorf("git version %d.%d is below minimum required %d.%d", major, minor, MinGitMajor, MinGitMinor)
	}
	return nil
}
