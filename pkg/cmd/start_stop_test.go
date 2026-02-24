package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestStartCmd(t *testing.T) {
	// 1. Setup temporary directory as the fake project root
	tmpDir, err := os.MkdirTemp("", "grava-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	origCWD, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origCWD)

	// 2. Create the .grava structure that grava start expects
	gravaDir := filepath.Join(tmpDir, ".grava")
	doltDir := filepath.Join(gravaDir, "dolt")
	binDir := filepath.Join(gravaDir, "bin")
	assert.NoError(t, os.MkdirAll(doltDir, 0755))
	assert.NoError(t, os.MkdirAll(binDir, 0755))

	// 3. Create a fake dolt binary in .grava/bin/dolt
	binaryName := "dolt"
	if runtime.GOOS == "windows" {
		binaryName = "dolt.exe"
	}
	fakeDolt := filepath.Join(binDir, binaryName)
	// A sleeping script so Start() doesn't immediately exit (simulates a server)
	assert.NoError(t, os.WriteFile(fakeDolt, []byte("#!/bin/sh\nsleep 60\n"), 0755))

	// 4. Set config with a port that is likely free
	viper.Set("db_url", "root@tcp(127.0.0.1:39901)/dolt")

	t.Run("grava start outputs startup message with correct port", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "start")
		assert.NoError(t, err)
		assert.Contains(t, output, "Starting Dolt server on port 39901")
		assert.Contains(t, output, "started in background")
	})

	t.Run("grava start fails when dolt database dir is missing", func(t *testing.T) {
		// Temporarily rename dolt dir
		bakDir := doltDir + "_bak"
		assert.NoError(t, os.Rename(doltDir, bakDir))
		defer os.Rename(bakDir, doltDir)

		_, err := executeCommand(rootCmd, "start")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("grava start fails when no dolt binary available", func(t *testing.T) {
		// Temporarily rename the local binary
		bakBin := fakeDolt + ".bak"
		assert.NoError(t, os.Rename(fakeDolt, bakBin))
		defer os.Rename(bakBin, fakeDolt)

		// If dolt is installed on the system PATH this scenario can't be forced in tests.
		// Skip rather than fail, since ResolveDoltBinary falls back to system dolt.
		if _, lookErr := exec.LookPath("dolt"); lookErr == nil {
			t.Skip("system dolt is installed — cannot test 'no dolt found' scenario")
		}

		_, err := executeCommand(rootCmd, "start")
		assert.Error(t, err)
	})
}
