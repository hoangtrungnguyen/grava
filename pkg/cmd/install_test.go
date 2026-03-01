package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstallCmd(t *testing.T) {
	// Create a temporary directory for the git repo
	tempDir, err := os.MkdirTemp("", "grava-install-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Init git repo
	initCmd := exec.Command("git", "init")
	initCmd.Dir = tempDir
	err = initCmd.Run()
	assert.NoError(t, err)

	// Change working directory to temp dir
	originalWd, err := os.Getwd()
	assert.NoError(t, err)
	err = os.Chdir(tempDir)
	assert.NoError(t, err)
	defer os.Chdir(originalWd)

	// Execute install command
	output, err := executeCommand(rootCmd, "install")
	assert.NoError(t, err)
	assert.Contains(t, output, "Grava Git integration installed successfully")

	// 1. Check .git/config
	configContent, err := os.ReadFile(filepath.Join(".git", "config"))
	assert.NoError(t, err)
	assert.Contains(t, string(configContent), `[merge "grava"]`)
	assert.Contains(t, string(configContent), `name = Grava JSONL Merge Driver`)
	assert.Contains(t, string(configContent), `driver = grava merge-slot --ancestor %O --current %A --other %B --output %A`)

	// 2. Check .gitattributes
	attributesContent, err := os.ReadFile(".gitattributes")
	assert.NoError(t, err)
	assert.Contains(t, string(attributesContent), "issues.jsonl merge=grava")

	// 3. Check hooks
	hooks := []string{"pre-commit", "post-merge", "post-checkout"}
	for _, hook := range hooks {
		hookPath := filepath.Join(".git", "hooks", hook)
		_, err := os.Stat(hookPath)
		assert.NoError(t, err, "hook %s should exist", hookPath)

		info, _ := os.Stat(hookPath)
		assert.True(t, info.Mode()&0111 != 0, "hook %s should be executable", hookPath)

		content, _ := os.ReadFile(hookPath)
		assert.Contains(t, string(content), "# grava-shim")
	}

	// Test Hook Chaining
	// Create a dummy non-grava hook
	dummyHookPath := filepath.Join(".git", "hooks", "pre-commit")
	os.WriteFile(dummyHookPath, []byte("#!/bin/sh\necho 'custom'"), 0755)

	// Re-run install
	_, err = executeCommand(rootCmd, "install")
	assert.NoError(t, err)

	// Verify .old exists and contains the custom script
	oldHookPath := dummyHookPath + ".old"
	_, err = os.Stat(oldHookPath)
	assert.NoError(t, err, "pre-commit.old should exist")
	oldContent, _ := os.ReadFile(oldHookPath)
	assert.Contains(t, string(oldContent), "echo 'custom'")

	// Verify the new shim exists at the primary path
	newContent, _ := os.ReadFile(dummyHookPath)
	assert.Contains(t, string(newContent), "# grava-shim")

	// Test Shared Install
	// Run install with --shared
	_, err = executeCommand(rootCmd, "install", "--shared")
	assert.NoError(t, err)

	// Verify .grava/hooks exists and is populated
	for _, hook := range hooks {
		hookPath := filepath.Join(".grava", "hooks", hook)
		_, err := os.Stat(hookPath)
		assert.NoError(t, err, "shared hook %s should exist", hookPath)
	}

	// Verify core.hooksPath is set
	configCmd := exec.Command("git", "config", "core.hooksPath")
	configOut, err := configCmd.Output()
	assert.NoError(t, err)
	assert.Contains(t, string(configOut), ".grava/hooks")
}
