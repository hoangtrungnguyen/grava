package utils_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/hoangtrungnguyen/grava/pkg/utils"
)

func writeSchemaFile(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "schema_version"), []byte(content), 0644))
}

func TestCheckSchemaVersion_Match(t *testing.T) {
	dir := t.TempDir()
	writeSchemaFile(t, dir, fmt.Sprint(utils.SchemaVersion))
	err := utils.CheckSchemaVersion(dir, utils.SchemaVersion)
	assert.NoError(t, err)
}

func TestCheckSchemaVersion_MatchWithNewline(t *testing.T) {
	dir := t.TempDir()
	writeSchemaFile(t, dir, fmt.Sprint(utils.SchemaVersion)+"\n")
	err := utils.CheckSchemaVersion(dir, utils.SchemaVersion)
	assert.NoError(t, err)
}

func TestCheckSchemaVersion_Mismatch(t *testing.T) {
	dir := t.TempDir()
	writeSchemaFile(t, dir, fmt.Sprint(utils.SchemaVersion-1))
	err := utils.CheckSchemaVersion(dir, utils.SchemaVersion)
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "SCHEMA_MISMATCH", gravaErr.Code)
}

func TestCheckSchemaVersion_FileMissing(t *testing.T) {
	dir := t.TempDir()
	// No schema_version file written
	err := utils.CheckSchemaVersion(dir, utils.SchemaVersion)
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "NOT_INITIALIZED", gravaErr.Code)
}

func TestCheckSchemaVersion_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	writeSchemaFile(t, dir, "not-a-number")
	err := utils.CheckSchemaVersion(dir, utils.SchemaVersion)
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "SCHEMA_MISMATCH", gravaErr.Code)
}

func TestWriteSchemaVersion_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, utils.WriteSchemaVersion(dir, utils.SchemaVersion))
	require.NoError(t, utils.CheckSchemaVersion(dir, utils.SchemaVersion))
}

func TestSchemaVersion_MatchesMigrationFileCount(t *testing.T) {
	// Sentinel: SchemaVersion must equal the number of files in pkg/migrate/migrations/.
	// If this test fails, update SchemaVersion in schema.go to match the migration count.
	entries, err := os.ReadDir("../../pkg/migrate/migrations")
	require.NoError(t, err, "could not read pkg/migrate/migrations/ directory")
	migrationCount := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			migrationCount++
		}
	}
	assert.Equal(t, migrationCount, utils.SchemaVersion,
		"SchemaVersion (%d) must equal migration file count (%d) in pkg/migrate/migrations/",
		utils.SchemaVersion, migrationCount)
}

func TestResolveGravaDir_EnvVar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GRAVA_DIR", dir)
	result, err := utils.ResolveGravaDir()
	require.NoError(t, err)
	assert.Equal(t, dir, result)
}

func TestResolveGravaDir_EnvVarNotExist(t *testing.T) {
	t.Setenv("GRAVA_DIR", "/nonexistent-grava-path-xyz")
	_, err := utils.ResolveGravaDir()
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "NOT_INITIALIZED", gravaErr.Code)
}

func TestResolveGravaDir_CWDWalk(t *testing.T) {
	// Create a temp directory with a .grava/ subdirectory
	base := t.TempDir()
	gravaPath := filepath.Join(base, ".grava")
	require.NoError(t, os.Mkdir(gravaPath, 0755))

	// Change to a subdirectory under base — resolver should walk up and find .grava/
	subDir := filepath.Join(base, "sub", "nested")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	orig, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	require.NoError(t, os.Chdir(subDir))

	result, err := utils.ResolveGravaDir()
	require.NoError(t, err)

	// Resolve symlinks on both sides (macOS /var → /private/var)
	wantResolved, err := filepath.EvalSymlinks(gravaPath)
	require.NoError(t, err)
	gotResolved, err := filepath.EvalSymlinks(result)
	require.NoError(t, err)
	assert.Equal(t, wantResolved, gotResolved)
}

func TestResolveGravaDir_NotFound(t *testing.T) {
	// Unset GRAVA_DIR and use a temp dir with no .grava/
	t.Setenv("GRAVA_DIR", "")
	dir := t.TempDir()

	orig, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	require.NoError(t, os.Chdir(dir))

	_, err = utils.ResolveGravaDir()
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "NOT_INITIALIZED", gravaErr.Code)
}
