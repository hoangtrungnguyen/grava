// Package gitconfig provides helpers for reading and writing Git configuration
// entries, focused on registering the Grava merge driver in .git/config.
package gitconfig

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

const (
	// DriverName is the git merge driver identifier used in .git/config and .gitattributes.
	DriverName = "grava"

	// DriverCmd is the command template stored in .git/config under merge.grava.driver.
	// %O = ancestor, %A = current (result written here), %B = other.
	// Requires 'grava' on PATH at merge time.
	DriverCmd = "grava merge-slot --ancestor %O --current %A --other %B"

	// DriverHumanName is the human-readable name stored under merge.grava.name.
	DriverHumanName = "Grava JSONL Merge Driver"

	keyName   = "merge." + DriverName + ".name"
	keyDriver = "merge." + DriverName + ".driver"
)

// DriverConfig holds a snapshot of the grava merge driver settings in .git/config.
type DriverConfig struct {
	Name   string
	Driver string
}

// DefaultDriverConfig returns the standard grava merge driver configuration.
func DefaultDriverConfig() DriverConfig {
	return DriverConfig{
		Name:   DriverHumanName,
		Driver: DriverCmd,
	}
}

// RegisterMergeDriver writes the grava merge driver configuration to .git/config.
// Idempotent: calling it multiple times is safe and produces the same result.
// Returns (true, nil) when already up-to-date, (false, nil) on fresh write.
func RegisterMergeDriver(cfg DriverConfig, stdout, stderr io.Writer) (alreadySet bool, err error) {
	current, ok := Get()
	if ok && current.Name == cfg.Name && current.Driver == cfg.Driver {
		return true, nil
	}

	for _, kv := range [][]string{
		{keyName, cfg.Name},
		{keyDriver, cfg.Driver},
	} {
		if err := Set(kv[0], kv[1], stdout, stderr); err != nil {
			return false, fmt.Errorf("failed to set git config %s: %w", kv[0], err)
		}
	}
	return false, nil
}

// IsRegistered reports whether the grava merge driver is configured in .git/config
// with values matching DefaultDriverConfig.
func IsRegistered() bool {
	current, ok := Get()
	if !ok {
		return false
	}
	def := DefaultDriverConfig()
	return current.Name == def.Name && current.Driver == def.Driver
}

// Get reads the current grava merge driver config from .git/config.
// Returns (zero, false) if neither key is set.
func Get() (DriverConfig, bool) {
	name, hasName := GetValue(keyName)
	driver, hasDriver := GetValue(keyDriver)
	if !hasName && !hasDriver {
		return DriverConfig{}, false
	}
	return DriverConfig{Name: name, Driver: driver}, true
}

// Set writes a single git config key-value pair into the local repo config.
func Set(key, value string, stdout, stderr io.Writer) error {
	c := exec.Command("git", "config", key, value) //nolint:gosec
	c.Stdout = stdout
	c.Stderr = stderr
	return c.Run()
}

// GetValue reads a single git config value from the effective config chain.
// Returns ("", false) if the key is not set or git is unavailable.
func GetValue(key string) (string, bool) {
	c := exec.Command("git", "config", "--get", key) //nolint:gosec
	out, err := c.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimRight(string(out), "\n"), true
}

// IsInGitRepo returns true if the current directory is inside a Git repository.
func IsInGitRepo() bool {
	return exec.Command("git", "rev-parse", "--git-dir").Run() == nil
}
