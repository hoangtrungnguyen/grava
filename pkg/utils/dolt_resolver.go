package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ResolveDoltBinary returns the absolute path to the dolt binary to use.
// It checks <projectRoot>/.grava/bin/dolt first (local install), then
// falls back to the system PATH. Returns an error if dolt cannot be found.
func ResolveDoltBinary(projectRoot string) (string, error) {
	binaryName := "dolt"
	if runtime.GOOS == "windows" {
		binaryName = "dolt.exe"
	}

	localPath := filepath.Join(projectRoot, ".grava", "bin", binaryName)
	if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
		return localPath, nil
	}

	systemPath, err := exec.LookPath(binaryName)
	if err != nil {
		return "", fmt.Errorf("dolt not found locally at %s and not on system PATH: %w", localPath, err)
	}
	return systemPath, nil
}

// LocalDoltBinDir returns the path to the .grava/bin directory for a given project root.
func LocalDoltBinDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".grava", "bin")
}

// LocalDoltBinaryPath returns the expected path of the locally installed dolt binary.
func LocalDoltBinaryPath(projectRoot string) string {
	binaryName := "dolt"
	if runtime.GOOS == "windows" {
		binaryName = "dolt.exe"
	}
	return filepath.Join(projectRoot, ".grava", "bin", binaryName)
}
