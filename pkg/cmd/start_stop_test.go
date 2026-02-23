package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestStartStopIntegration(t *testing.T) {
	// 1. Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "grava-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	origCWD, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origCWD)

	// 2. Create scripts directory and dummy scripts
	scriptsDir := filepath.Join(tmpDir, "scripts")
	err = os.Mkdir(scriptsDir, 0755)
	assert.NoError(t, err)

	startScript := filepath.Join(scriptsDir, "start_dolt_server.sh")
	stopScript := filepath.Join(scriptsDir, "stop_dolt_server.sh")

	// Start script writes arguments to a file for verification
	err = os.WriteFile(startScript, []byte("#!/bin/bash\necho \"start $1\" > start_called.txt\n"), 0755)
	assert.NoError(t, err)

	// Stop script writes arguments to a file for verification
	err = os.WriteFile(stopScript, []byte("#!/bin/bash\necho \"stop $1 $2\" > stop_called.txt\n"), 0755)
	assert.NoError(t, err)

	// 3. Create a fake .grava.yaml
	viper.Set("db_url", "root@tcp(127.0.0.1:3309)/dolt")

	// Ensure .grava directory exists for logs
	err = os.Mkdir(".grava", 0755)
	assert.NoError(t, err)

	t.Run("grava start calls script with correct port", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "start")
		assert.NoError(t, err)
		assert.Contains(t, output, "Starting Dolt server on port 3309")

		// Verify script was called (wait a bit since it's background)
		var content []byte
		for i := 0; i < 10; i++ {
			content, err = os.ReadFile("start_called.txt")
			if err == nil && string(content) != "" {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		assert.NoError(t, err)
		assert.Equal(t, "start 3309\n", string(content))
	})

	t.Run("grava stop calls script with correct port and flag", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "stop")
		assert.NoError(t, err)
		assert.Contains(t, output, "Stopping Dolt server on port 3309")

		// Verify script was called
		content, err := os.ReadFile("stop_called.txt")
		assert.Equal(t, "stop 3309 -y\n", string(content))
	})

	t.Run("grava start fails when script is missing", func(t *testing.T) {
		// Move scripts away temporarily
		bakDir := scriptsDir + "_bak"
		err := os.Rename(scriptsDir, bakDir)
		assert.NoError(t, err)
		defer os.Rename(bakDir, scriptsDir)

		_, err = executeCommand(rootCmd, "start")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}
