package issues

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/internal/testutil"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStoreForStop wires a MockStore so BeginTx returns a tx from the given sqlmock DB.
// SELECT FOR UPDATE and UPDATE queries are now inside the transaction, so all expectations
// are set on the sqlmock DB before calling stopIssue.
func mockStoreForStop(db *sql.DB) *testutil.MockStore {
	store := testutil.NewMockStore()
	store.BeginTxFn = func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
		return dolt.NewClientFromDB(db).BeginTx(ctx, nil)
	}
	store.LogEventTxFn = func(ctx context.Context, tx *sql.Tx, issueID, eventType, actor, model string, old, new interface{}) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO events VALUES ()")
		return err
	}
	return store
}

func TestStopIssue_HappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM issues WHERE id = ? FOR UPDATE")).
		WithArgs("grava-abc").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("in_progress"))
	mock.ExpectExec("UPDATE issues").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := mockStoreForStop(db)
	result, err := stopIssue(context.Background(), store, StopParams{
		ID:    "grava-abc",
		Actor: "test-actor",
		Model: "test-model",
	})
	require.NoError(t, err)
	assert.Equal(t, "grava-abc", result.ID)
	assert.Equal(t, "open", result.Status)
	assert.NotEmpty(t, result.StoppedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStopIssue_NotInProgress(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM issues WHERE id = ? FOR UPDATE")).
		WithArgs("grava-abc").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("open"))

	store := mockStoreForStop(db)
	_, err = stopIssue(context.Background(), store, StopParams{
		ID:    "grava-abc",
		Actor: "test-actor",
		Model: "test-model",
	})
	testutil.AssertGravaError(t, err, "NOT_IN_PROGRESS")
}

func TestStopIssue_IssueNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM issues WHERE id = ? FOR UPDATE")).
		WithArgs("grava-missing").
		WillReturnRows(sqlmock.NewRows([]string{"status"})) // empty = ErrNoRows

	store := mockStoreForStop(db)
	_, err = stopIssue(context.Background(), store, StopParams{
		ID:    "grava-missing",
		Actor: "test-actor",
		Model: "test-model",
	})
	testutil.AssertGravaError(t, err, "ISSUE_NOT_FOUND")
}

func TestStopIssue_DBUnreachable(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM issues WHERE id = ? FOR UPDATE")).
		WithArgs("grava-abc").
		WillReturnError(fmt.Errorf("connection refused"))

	store := mockStoreForStop(db)
	_, err = stopIssue(context.Background(), store, StopParams{
		ID:    "grava-abc",
		Actor: "test-actor",
		Model: "test-model",
	})
	testutil.AssertGravaError(t, err, "DB_UNREACHABLE")
}
