package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupSyncStatusRepo creates a temp git repo with .grava dir and optionally
// writes issues.jsonl. Returns cleanup func.
func setupSyncStatusRepo(t *testing.T, issuesContent string) func() {
	t.Helper()
	_, cleanup := initTempGitRepo(t)
	require.NoError(t, os.MkdirAll(".grava", 0755))
	if issuesContent != "" {
		require.NoError(t, os.WriteFile("issues.jsonl", []byte(issuesContent), 0644))
	}
	return cleanup
}

// --- computeSyncStatus unit tests ---

func TestComputeSyncStatus_NoFile(t *testing.T) {
	cleanup := setupSyncStatusRepo(t, "")
	defer cleanup()

	r := computeSyncStatus("issues.jsonl")
	assert.Equal(t, "no_file", r.Status)
	assert.False(t, r.FileExists)
}

func TestComputeSyncStatus_NeverImported(t *testing.T) {
	cleanup := setupSyncStatusRepo(t, `{"type":"issue","data":{"id":"1","title":"T"}}`+"\n")
	defer cleanup()
	// No stored hash

	// DB unavailable so DoltAvailable=false
	origConnect := connectDBFn
	connectDBFn = func() (dolt.Store, error) { return nil, assert.AnError }
	defer func() { connectDBFn = origConnect }()

	r := computeSyncStatus("issues.jsonl")
	assert.Equal(t, "never_imported", r.Status)
	assert.True(t, r.FileExists)
	assert.Equal(t, "", r.StoredHash)
	assert.NotEmpty(t, r.FileHash)
}

func TestComputeSyncStatus_InSync(t *testing.T) {
	cleanup := setupSyncStatusRepo(t, `{"type":"issue","data":{"id":"1","title":"T"}}`+"\n")
	defer cleanup()

	// Store matching hash.
	h, err := hashFile("issues.jsonl")
	require.NoError(t, err)
	writeLastImportHash(h)

	// Dolt clean (count=0).
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM dolt_status")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))

	origConnect := connectDBFn
	connectDBFn = func() (dolt.Store, error) { return dolt.NewClientFromDB(db), nil }
	defer func() { connectDBFn = origConnect }()

	r := computeSyncStatus("issues.jsonl")
	assert.Equal(t, "in_sync", r.Status)
	assert.False(t, r.FileChanged)
	assert.Equal(t, 0, r.DoltUncommitted)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComputeSyncStatus_FileChanged(t *testing.T) {
	cleanup := setupSyncStatusRepo(t, `{"type":"issue","data":{"id":"1","title":"T"}}`+"\n")
	defer cleanup()

	// Store a stale hash.
	writeLastImportHash("oldoldhash")

	origConnect := connectDBFn
	connectDBFn = func() (dolt.Store, error) { return nil, assert.AnError }
	defer func() { connectDBFn = origConnect }()

	r := computeSyncStatus("issues.jsonl")
	assert.Equal(t, "file_changed", r.Status)
	assert.True(t, r.FileChanged)
}

func TestComputeSyncStatus_DoltDirty(t *testing.T) {
	cleanup := setupSyncStatusRepo(t, `{"type":"issue","data":{"id":"1","title":"T"}}`+"\n")
	defer cleanup()

	// Matching hash — file hasn't changed.
	h, err := hashFile("issues.jsonl")
	require.NoError(t, err)
	writeLastImportHash(h)

	// Dolt has 2 uncommitted rows.
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM dolt_status")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(2))

	origConnect := connectDBFn
	connectDBFn = func() (dolt.Store, error) { return dolt.NewClientFromDB(db), nil }
	defer func() { connectDBFn = origConnect }()

	r := computeSyncStatus("issues.jsonl")
	assert.Equal(t, "dolt_dirty", r.Status)
	assert.Equal(t, 2, r.DoltUncommitted)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComputeSyncStatus_FileChangedAndDoltDirty(t *testing.T) {
	cleanup := setupSyncStatusRepo(t, `{"type":"issue","data":{"id":"1","title":"T"}}`+"\n")
	defer cleanup()

	writeLastImportHash("stale")

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM dolt_status")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(5))

	origConnect := connectDBFn
	connectDBFn = func() (dolt.Store, error) { return dolt.NewClientFromDB(db), nil }
	defer func() { connectDBFn = origConnect }()

	r := computeSyncStatus("issues.jsonl")
	assert.Equal(t, "file_changed_and_dolt_dirty", r.Status)
	assert.True(t, r.FileChanged)
	assert.Equal(t, 5, r.DoltUncommitted)
}

func TestComputeSyncStatus_DoltUnavailableDoesNotBlockFileCheck(t *testing.T) {
	cleanup := setupSyncStatusRepo(t, `{"type":"issue","data":{"id":"1","title":"T"}}`+"\n")
	defer cleanup()

	// Matching hash.
	h, _ := hashFile("issues.jsonl")
	writeLastImportHash(h)

	// DB unreachable.
	origConnect := connectDBFn
	connectDBFn = func() (dolt.Store, error) { return nil, assert.AnError }
	defer func() { connectDBFn = origConnect }()

	r := computeSyncStatus("issues.jsonl")
	// When Dolt is unavailable, we can't confirm dirty, so status = in_sync
	// based on file hash alone.
	assert.Equal(t, "in_sync", r.Status)
	assert.False(t, r.DoltAvailable)
	assert.Equal(t, -1, r.DoltUncommitted)
}

// --- CLI integration tests ---

func TestSyncStatusCmd_OutputsHumanReadable(t *testing.T) {
	cleanup := setupSyncStatusRepo(t, `{"type":"issue","data":{"id":"1","title":"T"}}`+"\n")
	defer cleanup()

	h, _ := hashFile("issues.jsonl")
	writeLastImportHash(h)

	db, mock, _ := sqlmock.New()
	defer db.Close() //nolint:errcheck
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM dolt_status")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))
	origConnect := connectDBFn
	connectDBFn = func() (dolt.Store, error) { return dolt.NewClientFromDB(db), nil }
	defer func() { connectDBFn = origConnect }()

	var buf bytes.Buffer
	syncStatusCmd.SetOut(&buf)
	defer syncStatusCmd.SetOut(nil)

	rootCmd.SetArgs([]string{"sync-status"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "in_sync")
	assert.Contains(t, out, "matches last import")
	assert.Contains(t, out, "clean")
}

func TestSyncStatusCmd_JSONOutput(t *testing.T) {
	cleanup := setupSyncStatusRepo(t, `{"type":"issue","data":{"id":"1","title":"T"}}`+"\n")
	defer cleanup()

	writeLastImportHash("oldhash")

	origConnect := connectDBFn
	connectDBFn = func() (dolt.Store, error) { return nil, assert.AnError }
	defer func() { connectDBFn = origConnect }()

	var buf bytes.Buffer
	syncStatusCmd.SetOut(&buf)
	defer syncStatusCmd.SetOut(nil)

	rootCmd.SetArgs([]string{"sync-status", "--json"})
	err := rootCmd.Execute()
	// file_changed exits non-zero
	assert.Error(t, err)

	var result SyncStatusResult
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &result))
	assert.Equal(t, "file_changed", result.Status)
	assert.True(t, result.FileChanged)
}
