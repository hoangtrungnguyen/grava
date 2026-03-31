package issues

import (
	"context"
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/internal/testutil"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStoreForUpdate wires a MockStore so QueryRow returns a current-row scan
// and tx operations run through the given sqlmock db.
func mockStoreForUpdate(db *sql.DB, exists bool, title, desc, iType string, priority int, status string) *testutil.MockStore {
	store := testutil.NewMockStore()
	store.QueryRowFn = func(query string, args ...any) *sql.Row {
		mockDB, mock, _ := sqlmock.New()
		if exists {
			mock.ExpectQuery("SELECT").WillReturnRows(
				sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "assignee"}).
					AddRow(title, desc, iType, priority, status, ""),
			)
		} else {
			mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{})) // ErrNoRows
		}
		return mockDB.QueryRow("SELECT", args...)
	}
	store.BeginTxFn = func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
		return dolt.NewClientFromDB(db).BeginTx(ctx, nil)
	}
	store.LogEventTxFn = func(ctx context.Context, tx *sql.Tx, issueID, eventType, actor, model string, old, new interface{}) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO events VALUES ()")
		return err
	}
	return store
}

func TestUpdateIssue_HappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE issues").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := mockStoreForUpdate(db, true, "Old title", "Old desc", "task", 3, "open")
	result, err := updateIssue(context.Background(), store, UpdateParams{
		ID:            "grava-abc",
		Title:         "New title",
		Actor:         "test-actor",
		Model:         "test-model",
		ChangedFields: []string{"title"},
	})
	require.NoError(t, err)
	assert.Equal(t, "grava-abc", result.ID)
	assert.Equal(t, "updated", result.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateIssue_IssueNotFound(t *testing.T) {
	store := mockStoreForUpdate(nil, false, "", "", "", 0, "")
	_, err := updateIssue(context.Background(), store, UpdateParams{
		ID:            "grava-missing",
		Title:         "New title",
		Actor:         "test-actor",
		Model:         "test-model",
		ChangedFields: []string{"title"},
	})
	testutil.AssertGravaError(t, err, "ISSUE_NOT_FOUND")
}

func TestUpdateIssue_InvalidStatus(t *testing.T) {
	store := testutil.NewMockStore()
	_, err := updateIssue(context.Background(), store, UpdateParams{
		ID:            "grava-abc",
		Status:        "flying",
		Actor:         "test-actor",
		Model:         "test-model",
		ChangedFields: []string{"status"},
	})
	testutil.AssertGravaError(t, err, "INVALID_STATUS")
}

func TestUpdateIssue_InvalidPriority(t *testing.T) {
	store := testutil.NewMockStore()
	_, err := updateIssue(context.Background(), store, UpdateParams{
		ID:            "grava-abc",
		Priority:      "ultra-mega",
		Actor:         "test-actor",
		Model:         "test-model",
		ChangedFields: []string{"priority"},
	})
	testutil.AssertGravaError(t, err, "INVALID_PRIORITY")
}

func TestUpdateIssue_NoFieldsChanged(t *testing.T) {
	store := testutil.NewMockStore()
	_, err := updateIssue(context.Background(), store, UpdateParams{
		ID:            "grava-abc",
		Actor:         "test-actor",
		Model:         "test-model",
		ChangedFields: []string{},
	})
	testutil.AssertGravaError(t, err, "MISSING_REQUIRED_FIELD")
}
