// Package gitattributes manages the .gitattributes file entries required for
// the Grava merge driver to be invoked by Git on JSONL issue files.
package gitattributes

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// MergeAttrLine is the entry written to .gitattributes.
	// It instructs Git to use the 'grava' merge driver for issues.jsonl.
	MergeAttrLine = "issues.jsonl merge=grava"

	// AttrFileName is the name of the gitattributes file.
	AttrFileName = ".gitattributes"
)

// EnsureMergeAttr ensures MergeAttrLine is present in the .gitattributes file
// located at repoRoot. Creates the file if it does not exist.
//
// Idempotent: safe to call multiple times; the line is never duplicated.
// Returns (true, nil) if the line was written, (false, nil) if already present.
func EnsureMergeAttr(repoRoot string) (added bool, err error) {
	path := filepath.Join(repoRoot, AttrFileName)

	present, err := HasMergeAttr(repoRoot)
	if err != nil {
		return false, err
	}
	if present {
		return false, nil
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644) //nolint:gosec
	if err != nil {
		return false, fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck

	// Ensure the entry starts on its own line.
	fi, err := f.Stat()
	if err != nil {
		return false, fmt.Errorf("failed to stat %s: %w", path, err)
	}
	prefix := ""
	if fi.Size() > 0 {
		// Read the last byte to check whether the file already ends with a newline.
		rf, err := os.Open(path) //nolint:gosec
		if err != nil {
			return false, fmt.Errorf("failed to read %s: %w", path, err)
		}
		defer rf.Close() //nolint:errcheck
		if _, err := rf.Seek(-1, 2); err == nil {
			b := make([]byte, 1)
			if _, err := rf.Read(b); err == nil && b[0] != '\n' {
				prefix = "\n"
			}
		}
	}

	if _, err := fmt.Fprintf(f, "%s%s\n", prefix, MergeAttrLine); err != nil {
		return false, fmt.Errorf("failed to write to %s: %w", path, err)
	}
	return true, nil
}

// HasMergeAttr reports whether MergeAttrLine is present in the .gitattributes
// file at repoRoot. Returns (false, nil) if the file does not exist.
func HasMergeAttr(repoRoot string) (bool, error) {
	path := filepath.Join(repoRoot, AttrFileName)
	f, err := os.Open(path) //nolint:gosec
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == MergeAttrLine {
			return true, nil
		}
	}
	return false, scanner.Err()
}

// RepoRoot returns the absolute path of the top-level git repository directory.
func RepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}
