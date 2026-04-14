package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
)

// SchemaVersion is the expected database schema version.
// Must match the number of migration files in pkg/migrate/migrations/.
const SchemaVersion = 9

// schemaVersionFile is the name of the version file inside the .grava/ directory.
const schemaVersionFile = "schema_version"

// CheckSchemaVersion reads .grava/schema_version and verifies it matches expectedVersion.
// Returns SCHEMA_MISMATCH if the versions differ, NOT_INITIALIZED if the file is absent.
func CheckSchemaVersion(gravaDir string, expectedVersion int) error {
	versionPath := filepath.Join(gravaDir, schemaVersionFile)

	data, err := os.ReadFile(versionPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return gravaerrors.New("NOT_INITIALIZED",
				"grava is not initialised: run 'grava init' first", err)
		}
		return gravaerrors.New("DB_UNREACHABLE",
			fmt.Sprintf("failed to read schema_version: %v", err), err)
	}

	actual, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return gravaerrors.New("SCHEMA_MISMATCH",
			fmt.Sprintf("schema_version file is corrupt (not an integer): %q", strings.TrimSpace(string(data))), err)
	}

	if actual != expectedVersion {
		return gravaerrors.New("SCHEMA_MISMATCH",
			fmt.Sprintf("schema version mismatch: expected %d, got %d — run 'grava init' to migrate", expectedVersion, actual), nil)
	}

	return nil
}

// WriteSchemaVersion writes the current SchemaVersion to .grava/schema_version.
// Called by 'grava init' after migrations succeed.
func WriteSchemaVersion(gravaDir string, version int) error {
	versionPath := filepath.Join(gravaDir, schemaVersionFile)
	return os.WriteFile(versionPath, []byte(strconv.Itoa(version)), 0644)
}

// ResolveGravaDir returns the path to the .grava/ directory for the current workspace.
// Resolution order: GRAVA_DIR env var → CWD walk.
// Returns an error if no .grava/ directory is found.
//
// Deprecated: Use pkg/grava.ResolveGravaDir() for the full ADR-004 redirect chain.
// Kept for backward compatibility with existing tests.
func ResolveGravaDir() (string, error) {
	// 1. Explicit override via env var
	if dir := os.Getenv("GRAVA_DIR"); dir != "" {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
		return "", gravaerrors.New("NOT_INITIALIZED",
			fmt.Sprintf("GRAVA_DIR=%q does not exist or is not a directory", dir), nil)
	}

	// 2. Walk up from CWD
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	dir := cwd
	for {
		candidate := filepath.Join(dir, ".grava")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}

	return "", gravaerrors.New("NOT_INITIALIZED",
		"no .grava/ directory found — run 'grava init' to initialise", nil)
}
