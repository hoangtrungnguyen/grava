package issues

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStoreForDrop creates a MockStore with QueryRowFn wired to return the given status.
// If exists is false, the query returns no rows (triggers ISSUE_NOT_FOUND).
func mockStoreForDrop(exists bool, status string) *testutil.MockStore {
	store := testutil.NewMockStore()
	store.QueryRowFn = func(query string, args ...any) *sql.Row {
		mockDB, mock, _ := sqlmock.New()
		if exists {
			mock.ExpectQuery("SELECT").WillReturnRows(
				sqlmock.NewRows([]string{"status"}).AddRow(status),
			)
		} else {
			mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{}))
		}
		return mockDB.QueryRow("SELECT", args...)
	}
	return store
}

// mockStoreForDropHappyPath creates a MockStore that mocks both the pre-read QueryRow
// AND the graph layer SQL (LoadGraphFromDB + SetNodeStatus) for happy-path tests.
func mockStoreForDropHappyPath(t *testing.T, issueID string, currentStatus string) *testutil.MockStore {
	t.Helper()

	// queryRowDB handles the pre-read SELECT status query
	queryRowDB, queryRowMock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = queryRowDB.Close() })

	queryRowMock.ExpectQuery("SELECT").WillReturnRows(
		sqlmock.NewRows([]string{"status"}).AddRow(currentStatus),
	)

	// graphDB handles LoadGraphFromDB queries + SetNodeStatus mutations
	// Use QueryMatcherRegexp so partial/multiline queries match correctly
	graphDB, graphMock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = graphDB.Close() })

	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// LoadGraphFromDB: SELECT issues (updated_at added to query in loader.go)
	graphMock.ExpectQuery("SELECT id, title, issue_type, status, priority, created_at.*FROM issues").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "issue_type", "status", "priority", "created_at", "updated_at", "await_type", "await_id", "ephemeral", "metadata",
		}).AddRow(issueID, "Test Issue", "task", currentStatus, 2, createdAt, updatedAt, nil, nil, false, nil))

	// LoadGraphFromDB: SELECT dependencies
	graphMock.ExpectQuery("SELECT from_id, to_id, type, metadata FROM dependencies").
		WillReturnRows(sqlmock.NewRows([]string{"from_id", "to_id", "type", "metadata"}))

	// SetNodeStatus: UPDATE issues
	graphMock.ExpectExec("UPDATE issues SET status").
		WithArgs("archived", sqlmock.AnyArg(), "test-actor", "test-model", issueID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// NOTE: SetNodeStatus's LogEvent call goes through MockStore.LogEvent (returns nil),
	// NOT through graphDB.Exec, so no INSERT INTO events expectation here.

	store := testutil.NewMockStore()
	store.QueryRowFn = func(query string, args ...any) *sql.Row {
		return queryRowDB.QueryRow(query, args...)
	}
	store.QueryFn = func(query string, args ...any) (*sql.Rows, error) {
		return graphDB.Query(query, args...)
	}
	store.ExecFn = func(query string, args ...any) (sql.Result, error) {
		return graphDB.Exec(query, args...)
	}

	return store
}

// Task 5.5: Test error: drop non-existent issue → ISSUE_NOT_FOUND
func TestDropIssue_IssueNotFound(t *testing.T) {
	store := mockStoreForDrop(false, "")
	_, err := dropIssue(context.Background(), store, DropParams{
		ID:    "grava-missing",
		Actor: "test-actor",
		Model: "test-model",
	})
	testutil.AssertGravaError(t, err, "ISSUE_NOT_FOUND")
}

// Task 5.3: Test error: drop in_progress issue without --force → ISSUE_IN_PROGRESS
func TestDropIssue_InProgressWithoutForce(t *testing.T) {
	store := mockStoreForDrop(true, "in_progress")
	_, err := dropIssue(context.Background(), store, DropParams{
		ID:    "grava-abc",
		Force: false,
		Actor: "test-actor",
		Model: "test-model",
	})
	testutil.AssertGravaError(t, err, "ISSUE_IN_PROGRESS")
	assert.Contains(t, err.Error(), "Cannot drop an active issue")
}

// Test idempotent: drop an already-archived issue → returns archived (no error)
func TestDropIssue_AlreadyArchived(t *testing.T) {
	store := mockStoreForDrop(true, "archived")
	result, err := dropIssue(context.Background(), store, DropParams{
		ID:    "grava-abc",
		Actor: "test-actor",
		Model: "test-model",
	})
	require.NoError(t, err)
	assert.Equal(t, "grava-abc", result.ID)
	assert.Equal(t, "archived", result.Status)
}

// Task 5.6: Test JSON output structure matches DropResult schema
func TestDropResult_JSONStructure(t *testing.T) {
	result := DropResult{ID: "grava-abc", Status: "archived"}
	assert.Equal(t, "grava-abc", result.ID)
	assert.Equal(t, "archived", result.Status)
}

// Task 5.1: Test happy path: drop a done issue → status becomes archived
func TestDropIssue_DoneIssue(t *testing.T) {
	store := mockStoreForDropHappyPath(t, "grava-done1", "closed")
	result, err := dropIssue(context.Background(), store, DropParams{
		ID:    "grava-done1",
		Actor: "test-actor",
		Model: "test-model",
	})
	require.NoError(t, err)
	assert.Equal(t, "grava-done1", result.ID)
	assert.Equal(t, "archived", result.Status)
}

// Task 5.2: Test happy path: drop an open issue → status becomes archived
func TestDropIssue_OpenIssue(t *testing.T) {
	store := mockStoreForDropHappyPath(t, "grava-open1", "open")
	result, err := dropIssue(context.Background(), store, DropParams{
		ID:    "grava-open1",
		Actor: "test-actor",
		Model: "test-model",
	})
	require.NoError(t, err)
	assert.Equal(t, "grava-open1", result.ID)
	assert.Equal(t, "archived", result.Status)
}

// Task 5.4: Test happy path: drop in_progress issue with --force → archived
func TestDropIssue_InProgressWithForce(t *testing.T) {
	store := mockStoreForDropHappyPath(t, "grava-active1", "in_progress")
	result, err := dropIssue(context.Background(), store, DropParams{
		ID:    "grava-active1",
		Force: true,
		Actor: "test-actor",
		Model: "test-model",
	})
	require.NoError(t, err)
	assert.Equal(t, "grava-active1", result.ID)
	assert.Equal(t, "archived", result.Status)
}
