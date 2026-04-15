// Package githooks deploys grava shim hooks into a Git repository's hooks
// directory, preserving any pre-existing non-grava hooks.
package githooks

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// ShimMarker is embedded in every grava-managed hook file.
	// Its presence identifies the file as safe to overwrite on re-install.
	ShimMarker = "# grava-shim"

	// AppendStartMarker and AppendEndMarker bracket the grava invocation
	// appended to pre-existing hooks by AppendStubs (ADR-H2 append mode).
	// Their presence is used for idempotency: if AppendStartMarker is found,
	// the hook already delegates to grava and AppendStubs is a no-op.
	AppendStartMarker = "# grava-hook-start"
	AppendEndMarker   = "# grava-hook-end"

	// preservedSuffix is appended to pre-existing hooks that grava displaces.
	preservedSuffix = ".pre-grava"
)

// InitHookNames lists the hooks registered by AppendStubs during grava init.
// Only the two hooks required for the Git Sync pipeline (FR25) are included.
var InitHookNames = []string{
	"pre-commit",
	"post-merge",
}

// AppendResult describes what happened to a single hook file during AppendStubs.
type AppendResult struct {
	Name   string
	Action string // "registered" | "appended" | "skipped"
}

// AppendStubs registers grava hook invocations for the named hooks using
// append mode (ADR-H2):
//
//   - Hook absent     → write a minimal new stub ("registered").
//   - Hook present, no grava marker → append grava invocation with
//     AppendStartMarker / AppendEndMarker ("appended").
//   - Hook present with AppendStartMarker already → no-op ("skipped").
//
// All written/appended files are made executable (0755).
// Creates hooksDir if it does not exist.
func AppendStubs(hooksDir string, hookNames []string) ([]AppendResult, error) {
	if err := os.MkdirAll(hooksDir, 0755); err != nil { //nolint:gosec
		return nil, fmt.Errorf("failed to create hooks directory %s: %w", hooksDir, err)
	}

	var results []AppendResult
	for _, name := range hookNames {
		res, err := appendOne(hooksDir, name)
		if err != nil {
			return results, err
		}
		results = append(results, res)
	}
	return results, nil
}

// appendOne handles a single hook file in append mode.
func appendOne(hooksDir, name string) (AppendResult, error) {
	path := filepath.Join(hooksDir, name)
	res := AppendResult{Name: name}

	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return res, fmt.Errorf("failed to read %s: %w", path, err)
	}

	if len(existing) == 0 {
		// Hook absent — write a new minimal stub.
		stub := fmt.Sprintf("#!/bin/sh\n%s\ngrava hook run %s \"$@\"\n%s\n",
			AppendStartMarker, name, AppendEndMarker)
		if err := os.WriteFile(path, []byte(stub), 0755); err != nil { //nolint:gosec
			return res, fmt.Errorf("failed to write hook %s: %w", path, err)
		}
		res.Action = "registered"
		return res, nil
	}

	// Hook exists — check for idempotency marker.
	if strings.Contains(string(existing), AppendStartMarker) {
		res.Action = "skipped"
		return res, nil
	}

	// Append grava invocation with start/end markers.
	suffix := fmt.Sprintf("\n%s\ngrava hook run %s \"$@\"\n%s\n",
		AppendStartMarker, name, AppendEndMarker)
	content := strings.TrimRight(string(existing), "\n") + suffix
	if err := os.WriteFile(path, []byte(content), 0755); err != nil { //nolint:gosec
		return res, fmt.Errorf("failed to append to hook %s: %w", path, err)
	}
	res.Action = "appended"
	return res, nil
}

// hookShim returns the shim content for a named hook.
// The shim delegates exclusively to 'grava hook run <name>'. Any pre-existing
// hook is preserved on disk as <name>.pre-grava but is not chained by the shim
// itself; hook dispatch (including calling the original) is handled by grava.
func hookShim(name string) string {
	return fmt.Sprintf("#!/bin/sh\n%s\ngrava hook run %s \"$@\"\n", ShimMarker, name)
}

// HookNames lists the hooks deployed by DeployAll.
var HookNames = []string{
	"pre-commit",
	"post-merge",
	"post-checkout",
	"prepare-commit-msg",
}

// DeployResult describes what happened to a single hook file.
type DeployResult struct {
	Name     string
	Action   string // "installed" | "updated" | "skipped" | "preserved-existing"
	Existing string // path to preserved pre-existing hook, if any
}

// DeployAll deploys grava shim hooks for all HookNames into hooksDir.
// Creates hooksDir if it does not exist.
//
// For each hook:
//   - If absent: writes a new shim (Action="installed").
//   - If already a grava shim with identical content: no-op (Action="skipped").
//   - If already a grava shim with stale content: overwrites (Action="updated").
//   - If a non-grava file exists: renames it to <name>.pre-grava, then writes
//     the shim (Action="installed", Existing=<renamed path>).
//     If <name>.pre-grava already exists the original is left untouched and
//     grava writes the shim on top of the primary path (Action="installed").
//
// All shim files are written with mode 0755.
//
// On error, results contains the successfully processed hooks up to the failure
// point; the caller should treat results as partial and not assume completeness.
func DeployAll(hooksDir string, w io.Writer) ([]DeployResult, error) {
	if err := os.MkdirAll(hooksDir, 0755); err != nil { //nolint:gosec
		return nil, fmt.Errorf("failed to create hooks directory %s: %w", hooksDir, err)
	}

	var results []DeployResult
	for _, name := range HookNames {
		res, err := deployOne(hooksDir, name, w)
		if err != nil {
			return results, err
		}
		results = append(results, res)
	}
	return results, nil
}

// deployOne handles a single hook file.
func deployOne(hooksDir, name string, w io.Writer) (DeployResult, error) {
	path := filepath.Join(hooksDir, name)
	shim := hookShim(name)

	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return DeployResult{}, fmt.Errorf("failed to read %s: %w", path, err)
	}

	res := DeployResult{Name: name}

	if len(existing) > 0 {
		if strings.Contains(string(existing), ShimMarker) {
			// Already a grava shim.
			if string(existing) == shim {
				res.Action = "skipped"
				return res, nil
			}
			// Stale shim — update silently.
			res.Action = "updated"
		} else {
			// Foreign hook — preserve it.
			preserved := path + preservedSuffix
			_, statErr := os.Stat(preserved)
			if statErr != nil && !os.IsNotExist(statErr) {
				return DeployResult{}, fmt.Errorf("failed to stat %s: %w", preserved, statErr)
			}
			if os.IsNotExist(statErr) {
				_, _ = fmt.Fprintf(w, "⚠️  Existing hook %s renamed to %s\n", path, preserved)
				if err := os.Rename(path, preserved); err != nil {
					return DeployResult{}, fmt.Errorf("failed to rename %s: %w", path, err)
				}
				res.Existing = preserved
			} else {
				_, _ = fmt.Fprintf(w, "⚠️  %s already exists; overwriting primary hook only\n", preserved)
			}
			res.Action = "installed"
		}
	} else {
		res.Action = "installed"
	}

	if err := os.WriteFile(path, []byte(shim), 0755); err != nil { //nolint:gosec
		return DeployResult{}, fmt.Errorf("failed to write hook %s: %w", path, err)
	}
	return res, nil
}

// DefaultHooksDir returns the path to .git/hooks for the given repo root.
func DefaultHooksDir(repoRoot string) string {
	return filepath.Join(repoRoot, ".git", "hooks")
}

// SharedHooksDir returns the path to .grava/hooks for the given repo root,
// used when --shared is passed to 'grava install'.
func SharedHooksDir(repoRoot string) string {
	return filepath.Join(repoRoot, ".grava", "hooks")
}
