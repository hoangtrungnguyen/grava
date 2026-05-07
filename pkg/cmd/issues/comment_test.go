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

func mockStoreForComment(db *sql.DB) *testutil.MockStore {
	store := testutil.NewMockStore()
	// guardNotArchived calls store.QueryRow before WithAuditedTx opens the tx.
	// Route it through the same sqlmock db so the expectation can be declared.
	store.QueryRowFn = func(query string, args ...any) *sql.Row {
		return dolt.NewClientFromDB(db).QueryRow(query, args...)
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

func TestCommentIssue_HappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	// guardNotArchived runs before WithAuditedTx
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM issues WHERE id = ?")).
		WithArgs("grava-abc").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("open"))
	mock.ExpectBegin()
	// Pre-read: issue exists
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM issues WHERE id = ?")).
		WithArgs("grava-abc").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("grava-abc"))
	// INSERT comment
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO issue_comments")).
		WithArgs("grava-abc", "Reproduced on macOS ARM", "test-actor", "test-model", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	// Audit event
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := mockStoreForComment(db)
	result, err := commentIssue(context.Background(), store, CommentParams{
		ID:      "grava-abc",
		Message: "Reproduced on macOS ARM",
		Actor:   "test-actor",
		Model:   "test-model",
	})
	require.NoError(t, err)
	assert.Equal(t, "grava-abc", result.ID)
	assert.Equal(t, int64(1), result.CommentID)
	assert.Equal(t, "Reproduced on macOS ARM", result.Message)
	assert.Equal(t, "test-actor", result.Actor)
	assert.NotEmpty(t, result.CreatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommentIssue_IssueNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	// guardNotArchived returns ISSUE_NOT_FOUND before WithAuditedTx is entered
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM issues WHERE id = ?")).
		WithArgs("grava-missing").
		WillReturnRows(sqlmock.NewRows([]string{"status"})) // empty → sql.ErrNoRows

	store := mockStoreForComment(db)
	_, err = commentIssue(context.Background(), store, CommentParams{
		ID:      "grava-missing",
		Message: "test comment",
		Actor:   "test-actor",
		Model:   "test-model",
	})
	testutil.AssertGravaError(t, err, "ISSUE_NOT_FOUND")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommentIssue_EmptyMessage(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	store := mockStoreForComment(db)
	_, err = commentIssue(context.Background(), store, CommentParams{
		ID:      "grava-abc",
		Message: "",
		Actor:   "test-actor",
		Model:   "test-model",
	})
	testutil.AssertGravaError(t, err, "MISSING_REQUIRED_FIELD")
}
