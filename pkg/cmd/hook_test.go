package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Safety check helpers ---

func TestHashFile_ComputesSHA256(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "hash-*.txt")
	require.NoError(t, err)
	_, err = f.WriteString("hello grava")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	h, err := hashFile(f.Name())
	require.NoError(t, err)
	assert.Len(t, h, 64, "SHA-256 hex digest must be 64 characters")
}

func TestHashFile_SameContentSameHash(t *testing.T) {
	dir := t.TempDir()
	writeFile := func(name, content string) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte(content), 0644))
		return p
	}
	a := writeFile("a.txt", "same content")
	b := writeFile("b.txt", "same content")

	ha, err := hashFile(a)
	require.NoError(t, err)
	hb, err := hashFile(b)
	require.NoError(t, err)
	assert.Equal(t, ha, hb)
}

func TestHashFile_DifferentContentDifferentHash(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(a, []byte("content A"), 0644))
	require.NoError(t, os.WriteFile(b, []byte("content B"), 0644))

	ha, _ := hashFile(a)
	hb, _ := hashFile(b)
	assert.NotEqual(t, ha, hb)
}

func TestReadWriteLastImportHash(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// No .grava dir yet — returns empty string (not found)
	assert.Equal(t, "", readLastImportHash())

	// Create .grava and write a hash
	require.NoError(t, os.MkdirAll(".grava", 0755))
	writeLastImportHash("abc123def456")

	// Should round-trip
	assert.Equal(t, "abc123def456", readLastImportHash())
}

// --- hasDoltUncommittedChanges (Check C) ---

func TestHasDoltUncommittedChanges_ReturnsTrueWhenRowsExist(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM dolt_status")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(3))

	store := dolt.NewClientFromDB(db)
	assert.True(t, hasDoltUncommittedChanges(store))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHasDoltUncommittedChanges_ReturnsFalseWhenNoRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM dolt_status")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))

	store := dolt.NewClientFromDB(db)
	assert.False(t, hasDoltUncommittedChanges(store))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHasDoltUncommittedChanges_ReturnsFalseOnQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM dolt_status")).
		WillReturnError(assert.AnError)

	store := dolt.NewClientFromDB(db)
	// Non-Dolt backend — treat as no changes (fail-open).
	assert.False(t, hasDoltUncommittedChanges(store))
}

// TestSyncFromFile_SkipsWhenDoltHasUncommittedChanges verifies that syncFromFile
// skips the import and writes a warning to stderr when Check C fires.
func TestSyncFromFile_SkipsWhenDoltHasUncommittedChanges(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	content := `{"type":"issue","data":{"id":"abc","title":"T"}}` + "\n"
	require.NoError(t, os.WriteFile("issues.jsonl", []byte(content), 0644))
	require.NoError(t, os.MkdirAll(".grava", 0755))
	// No stored hash so Check A does not fire.

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	// dolt_status returns 2 rows — uncommitted changes present.
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM dolt_status")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(2))

	// Override connectDBFn to return the mock store without a real Dolt instance.
	origConnect := connectDBFn
	connectDBFn = func() (dolt.Store, error) { return dolt.NewClientFromDB(db), nil }
	t.Cleanup(func() { connectDBFn = origConnect })

	var errBuf strings.Builder
	hookRunCmd.SetErr(&errBuf)
	defer hookRunCmd.SetErr(nil)

	err = syncFromFile(hookRunCmd, "merge", false)
	assert.NoError(t, err)
	assert.Contains(t, errBuf.String(), "uncommitted changes")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSyncFromFile_SkipsWhenHashUnchanged verifies that syncFromFile returns
// early (without attempting a DB connection) when issues.jsonl hash matches the
// stored hash from the last import.
func TestSyncFromFile_SkipsWhenHashUnchanged(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	content := `{"type":"issue","data":{"id":"abc","title":"T"}}` + "\n"
	require.NoError(t, os.WriteFile("issues.jsonl", []byte(content), 0644))
	require.NoError(t, os.MkdirAll(".grava", 0755))

	// Pre-store the hash of the current file so Check A fires.
	h, err := hashFile("issues.jsonl")
	require.NoError(t, err)
	writeLastImportHash(h)

	// Point DB to an unreachable port — if the skip works, we never connect.
	t.Setenv("DB_URL", "root@tcp(127.0.0.1:19999)/grava?parseTime=true")

	var outBuf strings.Builder
	hookRunCmd.SetOut(&outBuf)
	defer hookRunCmd.SetOut(nil)

	err = syncFromFile(hookRunCmd, "merge", false)
	assert.NoError(t, err)
	assert.Contains(t, outBuf.String(), "unchanged since last sync")
}

// --- hookRunCmd tests ---

func TestHookRunCmd_UnknownHookExitsZero(t *testing.T) {
	rootCmd.SetArgs([]string{"hook", "run", "unknown-hook"})
	assert.NoError(t, rootCmd.Execute())
}

func TestHookRunCmd_PrepareCommitMsgNoOp(t *testing.T) {
	rootCmd.SetArgs([]string{"hook", "run", "prepare-commit-msg"})
	assert.NoError(t, rootCmd.Execute())
}

func TestHookRunCmd_PreCommitNoIssuesFile(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// issues.jsonl does not exist — pre-commit should exit 0
	rootCmd.SetArgs([]string{"hook", "run", "pre-commit"})
	assert.NoError(t, rootCmd.Execute())
}

func TestHookRunCmd_PreCommitValidFile(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	valid := `{"id":"abc123","title":"Test","type":"task","status":"open","priority":1,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","created_by":"x","updated_by":"x"}` + "\n"
	require.NoError(t, os.WriteFile("issues.jsonl", []byte(valid), 0644))

	rootCmd.SetArgs([]string{"hook", "run", "pre-commit"})
	assert.NoError(t, rootCmd.Execute())
}

func TestHookRunCmd_PreCommitInvalidFile(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// Invalid JSON
	require.NoError(t, os.WriteFile("issues.jsonl", []byte("{broken json\n"), 0644))

	rootCmd.SetArgs([]string{"hook", "run", "pre-commit"})
	assert.Error(t, rootCmd.Execute(), "pre-commit should fail on malformed JSONL")
}

func TestHookRunCmd_PostMergeNoIssuesFile(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// No issues.jsonl — post-merge should exit 0 (nothing to sync)
	rootCmd.SetArgs([]string{"hook", "run", "post-merge"})
	assert.NoError(t, rootCmd.Execute())
}

func TestHookRunCmd_PostCheckoutNoArgs(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// Called without prev/new HEAD args — should exit 0 gracefully
	rootCmd.SetArgs([]string{"hook", "run", "post-checkout"})
	assert.NoError(t, rootCmd.Execute())
}

func TestHookRunCmd_PostCheckoutSameRefs(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// When prev==new, issues.jsonl hasn't changed — should exit 0 without syncing
	rootCmd.SetArgs([]string{"hook", "run", "post-checkout", "abc123", "abc123", "1"})
	assert.NoError(t, rootCmd.Execute())
}

// --- issuesChangedInCheckout ---

func TestIssuesChangedInCheckout_ReturnsFalseOnBadRefs(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// Invalid refs — git diff errors — should return false (safe default)
	changed := issuesChangedInCheckout("not-a-ref", "also-not-a-ref")
	assert.False(t, changed)
}

// --- issuesChangedInMerge ---

func TestIssuesChangedInMerge_ReturnsFalseWhenNoFile(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// No issues.jsonl in a fresh repo with no commits — issuesChangedInMerge
	// falls back to os.Stat; file absent → false.
	changed := issuesChangedInMerge()
	assert.False(t, changed)
}

func TestIssuesChangedInMerge_ReturnsTrueWhenFileExists(t *testing.T) {
	dir, cleanup := initTempGitRepo(t)
	defer cleanup()

	// Create issues.jsonl — issuesChangedInMerge fallback returns true when
	// HEAD@{1} doesn't exist (fresh repo) but the file is present.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "issues.jsonl"), []byte(""), 0644))

	changed := issuesChangedInMerge()
	assert.True(t, changed)
}

// --- hook command wiring ---

func TestHookCmd_IsRegistered(t *testing.T) {
	var found bool
	for _, c := range rootCmd.Commands() {
		if c.Name() == "hook" {
			found = true
			break
		}
	}
	assert.True(t, found, "hook command should be registered on rootCmd")
}

func TestHookRunCmd_IsRegistered(t *testing.T) {
	var found bool
	for _, c := range hookCmd.Commands() {
		if c.Name() == "run" {
			found = true
			break
		}
	}
	assert.True(t, found, "run subcommand should be registered on hookCmd")
}

// --- dry-run mode ---

func TestSyncFromFile_DryRun_PrintsCountWithoutDB(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	content := `{"id":"a","title":"T1"}` + "\n" +
		`{"id":"b","title":"T2"}` + "\n" +
		"\n" // blank line should not be counted
	require.NoError(t, os.WriteFile("issues.jsonl", []byte(content), 0644))
	require.NoError(t, os.MkdirAll(".grava", 0755))

	var outBuf strings.Builder
	hookRunCmd.SetOut(&outBuf)
	defer hookRunCmd.SetOut(nil)

	// dryRun=true — no DB connection attempted even with unreachable URL.
	t.Setenv("DB_URL", "root@tcp(127.0.0.1:19999)/grava?parseTime=true")
	err := syncFromFile(hookRunCmd, "merge", true)
	assert.NoError(t, err)
	assert.Contains(t, outBuf.String(), "dry-run")
	assert.Contains(t, outBuf.String(), "2") // 2 non-blank lines
}

func TestSyncFromFile_DryRun_NoFileIsNoOp(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// No issues.jsonl — dry-run should exit 0 silently
	err := syncFromFile(hookRunCmd, "merge", true)
	assert.NoError(t, err)
}

func TestHookRunCmd_PostMergeDryRun(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// Reset hookDryRun after this test so subsequent tests are not affected.
	// cobra/pflag does not reset unspecified flags between Execute() calls.
	t.Cleanup(func() { hookDryRun = false })

	require.NoError(t, os.WriteFile("issues.jsonl",
		[]byte(`{"id":"x","title":"T"}`+"\n"), 0644))

	var outBuf strings.Builder
	hookRunCmd.SetOut(&outBuf)
	defer hookRunCmd.SetOut(nil)

	rootCmd.SetArgs([]string{"hook", "run", "post-merge", "--dry-run"})
	// post-merge checks issuesChangedInMerge() — may be false in a fresh repo
	// (no HEAD@{1}), which means the dry-run path is never reached.
	// We verify the command exits 0 and that hookDryRun was wired correctly
	// (syncFromFile_DryRun_PrintsCountWithoutDB tests the actual output path).
	assert.NoError(t, rootCmd.Execute())
}

// --- syncFromFile gracefully handles missing DB ---

func TestSyncFromFile_DBUnavailableExitsZero(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	valid := `{"type":"issue","data":{"id":"xyz","title":"T"}}` + "\n"
	require.NoError(t, os.WriteFile("issues.jsonl", []byte(valid), 0644))

	// Point to a port where nothing is listening.
	var errBuf strings.Builder
	hookRunCmd.SetErr(&errBuf)
	defer hookRunCmd.SetErr(nil)

	// Override db_url to an unreachable address.
	// viper.AutomaticEnv maps key "db_url" → env var "DB_URL" (no prefix configured).
	t.Setenv("DB_URL", "root@tcp(127.0.0.1:19999)/grava?parseTime=true")

	// syncFromFile should warn but return nil.
	err := syncFromFile(hookRunCmd, "test", false)
	assert.NoError(t, err, "syncFromFile should exit 0 when DB is unreachable")
	assert.Contains(t, errBuf.String(), "DB unavailable")
}
