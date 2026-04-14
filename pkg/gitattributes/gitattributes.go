// Package gitattributes manages the .gitattributes file entries required for
// the Grava merge driver to be invoked by Git on JSONL issue files.
package gitattributes

import (
	"bufio"
	"bytes"
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
// Safe for sequential callers: the line is never duplicated across multiple calls
// from the same process. Concurrent callers (two simultaneous 'grava install'
// runs) may produce a duplicate on the very first installation; subsequent calls
// are still safe.
//
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

	// Read the existing content (empty slice if file does not exist).
	existing, err := os.ReadFile(path) //nolint:gosec
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to read %s: %w", path, err)
	}

	// Ensure the new entry starts on its own line.
	var buf bytes.Buffer
	buf.Write(existing)
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		buf.WriteByte('\n')
	}
	buf.WriteString(MergeAttrLine + "\n")

	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil { //nolint:gosec
		return false, fmt.Errorf("failed to write %s: %w", path, err)
	}
	return true, nil
}

// HasMergeAttr reports whether MergeAttrLine is present in the .gitattributes
// file at repoRoot. Returns (false, nil) if the file does not exist.
//
// Matching is exact on the line content after stripping trailing whitespace
// and carriage returns only; leading whitespace is preserved because Git does
// not strip it when parsing attributes.
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
		line := strings.TrimRight(scanner.Text(), " \t\r")
		if line == MergeAttrLine {
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
