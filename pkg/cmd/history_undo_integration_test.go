//go:build integration
// +build integration

package cmd

import (
	"os"
	"regexp"
	"testing"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupIntegrationDB(t *testing.T) func() {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		// Default to the test DB setup by script
		dbURL = "root@tcp(127.0.0.1:3306)/test_grava?parseTime=true"
	}

	var err error
	Store, err = dolt.NewClient(dbURL)
	require.NoError(t, err, "Failed to connect to integration test DB")

	// Force single connection for Dolt session persistence
	Store.SetMaxOpenConns(1)
	Store.SetMaxIdleConns(1)

	// Disable auto-close for integration tests to allow multiple commands
	originalPostRun := rootCmd.PersistentPostRunE
	rootCmd.PersistentPostRunE = nil

	// Clear tables
	_, err = Store.Exec("DELETE FROM dependencies")
	require.NoError(t, err)
	_, err = Store.Exec("DELETE FROM issues")
	require.NoError(t, err)

	return func() {
		rootCmd.PersistentPostRunE = originalPostRun
		if Store != nil {
			Store.Close()
		}
	}
}

func TestHistoryIntegration(t *testing.T) {
	cleanup := setupIntegrationDB(t)
	defer cleanup()

	// 1. Create Issue
	output, err := executeCommand(rootCmd, "create", "--title", "Initial Title", "--desc", "Initial Desc")
	require.NoError(t, err)

	// Extract ID using regex
	re := regexp.MustCompile(`grava-[a-z0-9]+`)
	id := re.FindString(output)
	require.NotEmpty(t, id, "Failed to extract ID from output: %s", output)

	// 2. Commit 1
	_, err = executeCommand(rootCmd, "commit", "-m", "Initial commit")
	require.NoError(t, err)

	// 3. Update Issue
	_, err = executeCommand(rootCmd, "update", id, "--title", "Updated Title")
	require.NoError(t, err)

	// 4. Commit 2
	_, err = executeCommand(rootCmd, "commit", "-m", "Second commit")
	require.NoError(t, err)

	// 5. Run History
	output, err = executeCommand(rootCmd, "history", id)
	require.NoError(t, err)
	assert.Contains(t, output, "Initial commit")
	assert.Contains(t, output, "Second commit")
	assert.Contains(t, output, "Initial Title")
	assert.Contains(t, output, "Updated Title")
}

func TestUndoIntegration(t *testing.T) {
	cleanup := setupIntegrationDB(t)
	defer cleanup()

	// 1. Create & Commit (CLEAN state)
	output, err := executeCommand(rootCmd, "create", "--title", "v1", "--desc", "v1 desc")
	require.NoError(t, err)

	// Extract ID using regex
	re := regexp.MustCompile(`grava-[a-z0-9]+`)
	id := re.FindString(output)
	require.NotEmpty(t, id)

	_, err = executeCommand(rootCmd, "commit", "-m", "commit v1")
	require.NoError(t, err)

	// 2. Update but DON'T commit (DIRTY state)
	_, err = executeCommand(rootCmd, "update", id, "--title", "v2 dirty")
	require.NoError(t, err)

	// Verify it's dirty in DB
	var title string
	err = Store.QueryRow("SELECT title FROM issues WHERE id = ?", id).Scan(&title)
	assert.NoError(t, err)
	assert.Equal(t, "v2 dirty", title)

	// 3. Undo DIRTY state -> Revert to HEAD (v1)
	output, err = executeCommand(rootCmd, "undo", id)
	require.NoError(t, err)
	assert.Contains(t, output, "Discarding uncommitted changes")

	err = Store.QueryRow("SELECT title FROM issues WHERE id = ?", id).Scan(&title)
	assert.NoError(t, err)
	assert.Equal(t, "v1", title)

	// Verify comment added
	var metaStr string
	err = Store.QueryRow("SELECT metadata FROM issues WHERE id = ?", id).Scan(&metaStr)
	assert.NoError(t, err)
	assert.Contains(t, metaStr, "Undo: discarded uncommitted changes")

	// 4. Commit v1 (make it clean again)
	_, err = executeCommand(rootCmd, "commit", "-m", "commit v1 again")
	require.NoError(t, err)

	// 5. Update to v2 and COMMIT
	_, err = executeCommand(rootCmd, "update", id, "--title", "v2 clean")
	require.NoError(t, err)
	_, err = executeCommand(rootCmd, "commit", "-m", "commit v2")
	require.NoError(t, err)

	// 6. Undo CLEAN state -> Revert to PREVIOUS (v1)
	output, err = executeCommand(rootCmd, "undo", id)
	require.NoError(t, err)
	assert.Contains(t, output, "Issue is clean. Reverting to PREVIOUS commit")

	err = Store.QueryRow("SELECT title FROM issues WHERE id = ?", id).Scan(&title)
	assert.NoError(t, err)
	assert.Equal(t, "v1", title)

	// Verify comment added
	err = Store.QueryRow("SELECT metadata FROM issues WHERE id = ?", id).Scan(&metaStr)
	assert.NoError(t, err)
	assert.Contains(t, metaStr, "Undo: reverted to PREVIOUS commit")
}

func TestUndoAffectedFilesIntegration(t *testing.T) {
	cleanup := setupIntegrationDB(t)
	defer cleanup()

	// 1. Create with no files and commit
	output, err := executeCommand(rootCmd, "create", "--title", "v1")
	require.NoError(t, err)
	re := regexp.MustCompile(`grava-[a-z0-9]+`)
	id := re.FindString(output)
	require.NotEmpty(t, id)

	executeCommand(rootCmd, "commit", "-m", "initial")

	// 2. Update with files and commit
	executeCommand(rootCmd, "update", id, "--title", "v2", "--files", "a.go,b.go")
	executeCommand(rootCmd, "commit", "-m", "added files")

	// 3. Dirty update (change files but don't commit)
	executeCommand(rootCmd, "update", id, "--files", "c.go")

	// 4. Undo Dirty -> should revert to v2 state (a.go, b.go)
	executeCommand(rootCmd, "undo", id)
	var files string
	err = Store.QueryRow("SELECT affected_files FROM issues WHERE id = ?", id).Scan(&files)
	require.NoError(t, err)
	assert.Contains(t, files, "a.go")
	assert.Contains(t, files, "b.go")
	assert.NotContains(t, files, "c.go")

	// 5. Undo Clean -> should revert to v1 state (empty files)
	executeCommand(rootCmd, "undo", id)
	err = Store.QueryRow("SELECT affected_files FROM issues WHERE id = ?", id).Scan(&files)
	require.NoError(t, err)
	assert.Equal(t, "[]", files)
}

func TestSessionUndoIntegration(t *testing.T) {
	cleanup := setupIntegrationDB(t)
	defer cleanup()

	// 1. Create Issue
	output, err := executeCommand(rootCmd, "create", "--title", "session v1", "--desc", "desc")
	require.NoError(t, err)
	re := regexp.MustCompile(`grava-[a-z0-9]+`)
	id := re.FindString(output)

	// 2. Commit and capture hash
	output, err = executeCommand(rootCmd, "commit", "-m", "first session commit")
	require.NoError(t, err)
	reHash := regexp.MustCompile(`Hash: ([a-z0-9]+)`)
	matches := reHash.FindStringSubmatch(output)
	require.Len(t, matches, 2, "Failed to find Hash in output: %s", output)
	hash := matches[1]
	require.NotEmpty(t, hash)

	// 3. Link hash to issue metadata
	_, err = executeCommand(rootCmd, "update", id, "--last-commit", hash)
	require.NoError(t, err)

	// 4. Update the issue state
	_, err = executeCommand(rootCmd, "update", id, "--title", "session v2")
	require.NoError(t, err)
	output, err = executeCommand(rootCmd, "commit", "-m", "second session commit")
	require.NoError(t, err)
	matches = reHash.FindStringSubmatch(output)
	require.Len(t, matches, 2)
	hash2 := matches[1]

	// 5. Link LATEST hash to issue metadata
	_, err = executeCommand(rootCmd, "update", id, "--last-commit", hash2)
	require.NoError(t, err)

	// 6. Undo with Session Support
	output, err = executeCommand(rootCmd, "undo", id)
	require.NoError(t, err)
	assert.Contains(t, output, "Found last session commit")
	assert.Contains(t, output, "Reverting session commit")

	// 7. Verify title reverted to 'session v1' (which it was after C1 but before C2)
	// Wait, actually 'v1' was updated by 'v2' in the DB BEFORE the revert.
	// After reverting C2, it should be back to whatever it was after C1.
	var title string
	err = Store.QueryRow("SELECT title FROM issues WHERE id = ?", id).Scan(&title)
	require.NoError(t, err)
	assert.Equal(t, "session v1", title)
}
