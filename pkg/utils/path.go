package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindScript searches for a script with the given name in standard locations.
// 1. Current working directory's scripts/ folder.
// 2. The directory of the current executable's scripts/ folder.
func FindScript(name string) (string, error) {
	// 1. Check CWD
	cwd, err := os.Getwd()
	if err == nil {
		scriptPath := filepath.Join(cwd, "scripts", name)
		if _, err := os.Stat(scriptPath); err == nil {
			return scriptPath, nil
		}
	}

	// 2. Check executable directory
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		scriptPath := filepath.Join(exeDir, "scripts", name)
		if _, err := os.Stat(scriptPath); err == nil {
			return scriptPath, nil
		}
	}

	return "", fmt.Errorf("script %s not found in ./scripts/ or <exe_dir>/scripts/", name)
}
