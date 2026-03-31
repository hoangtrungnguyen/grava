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

// mockStoreForAssign wires a MockStore so QueryRow returns the current assignee
// and tx operations run through the given sqlmock db.
func mockStoreForAssign(db *sql.DB, exists bool, currentAssignee string) *testutil.MockStore {
	store := testutil.NewMockStore()
	store.QueryRowFn = func(query string, args ...any) *sql.Row {
		mockDB, mock, _ := sqlmock.New()
		if exists {
			mock.ExpectQuery("SELECT").WillReturnRows(
				sqlmock.NewRows([]string{"assignee"}).AddRow(currentAssignee),
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

func TestAssignIssue_HappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE issues").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := mockStoreForAssign(db, true, "")
	result, err := assignIssue(context.Background(), store, AssignParams{
		ID:       "grava-abc",
		Assignee: "alice",
		Actor:    "test-actor",
		Model:    "test-model",
	})
	require.NoError(t, err)
	assert.Equal(t, "grava-abc", result.ID)
	assert.Equal(t, "updated", result.Status)
	assert.Equal(t, "alice", result.Assignee)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAssignIssue_Unassign(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE issues").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := mockStoreForAssign(db, true, "alice")
	result, err := assignIssue(context.Background(), store, AssignParams{
		ID:       "grava-abc",
		Unassign: true,
		Actor:    "test-actor",
		Model:    "test-model",
	})
	require.NoError(t, err)
	assert.Equal(t, "grava-abc", result.ID)
	assert.Equal(t, "updated", result.Status)
	assert.Equal(t, "", result.Assignee)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAssignIssue_IssueNotFound(t *testing.T) {
	store := mockStoreForAssign(nil, false, "")
	_, err := assignIssue(context.Background(), store, AssignParams{
		ID:       "grava-missing",
		Assignee: "alice",
		Actor:    "test-actor",
		Model:    "test-model",
	})
	testutil.AssertGravaError(t, err, "ISSUE_NOT_FOUND")
}
