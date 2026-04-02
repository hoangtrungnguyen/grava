package issues

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/internal/testutil"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStoreForStart wires a MockStore so BeginTx returns a tx from the given sqlmock DB.
// SELECT FOR UPDATE and UPDATE queries are now inside the transaction, so all expectations
// are set on the sqlmock DB before calling startIssue.
func mockStoreForStart(db *sql.DB) *testutil.MockStore {
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

func TestStartIssue_HappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, COALESCE(assignee, '') FROM issues WHERE id = ? FOR UPDATE")).
		WithArgs("grava-abc").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee"}).AddRow("open", ""))
	mock.ExpectExec("UPDATE issues").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := mockStoreForStart(db)
	result, err := startIssue(context.Background(), store, StartParams{
		ID:    "grava-abc",
		Actor: "test-actor",
		Model: "test-model",
	})
	require.NoError(t, err)
	assert.Equal(t, "grava-abc", result.ID)
	assert.Equal(t, "in_progress", result.Status)
	assert.NotEmpty(t, result.StartedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStartIssue_AlreadyInProgress(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, COALESCE(assignee, '') FROM issues WHERE id = ? FOR UPDATE")).
		WithArgs("grava-abc").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee"}).AddRow("in_progress", "agent-01"))

	store := mockStoreForStart(db)
	_, err = startIssue(context.Background(), store, StartParams{
		ID:    "grava-abc",
		Actor: "test-actor",
		Model: "test-model",
	})
	testutil.AssertGravaError(t, err, "ALREADY_IN_PROGRESS")
	assert.Contains(t, err.Error(), "agent-01")
}

func TestStartIssue_IssueNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, COALESCE(assignee, '') FROM issues WHERE id = ? FOR UPDATE")).
		WithArgs("grava-missing").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee"})) // empty = ErrNoRows

	store := mockStoreForStart(db)
	_, err = startIssue(context.Background(), store, StartParams{
		ID:    "grava-missing",
		Actor: "test-actor",
		Model: "test-model",
	})
	testutil.AssertGravaError(t, err, "ISSUE_NOT_FOUND")
}
