package issues

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/internal/testutil"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStoreForSubtask returns a MockStore wired to route actual SQL through
// the given sqlmock db, while GetNextChildSequence returns seq directly
// (avoiding the nested-transaction conflict described in Dev Notes).
func mockStoreForSubtask(db *sql.DB, seq int) *testutil.MockStore {
	store := testutil.NewMockStore()
	store.GetNextChildSequenceFn = func(parentID string) (int, error) { return seq, nil }
	store.BeginTxFn = func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
		return dolt.NewClientFromDB(db).BeginTx(ctx, nil)
	}
	store.LogEventTxFn = func(ctx context.Context, tx *sql.Tx, issueID, eventType, actor, model string, old, new interface{}) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO events VALUES ()")
		return err
	}
	return store
}

func TestSubtaskIssue_HappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("grava-parent").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("INSERT INTO issues").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO dependencies").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := mockStoreForSubtask(db, 1)
	result, err := subtaskIssue(context.Background(), store, SubtaskParams{
		ParentID:  "grava-parent",
		Title:     "Write unit tests",
		IssueType: "task",
		Priority:  "medium",
		Actor:     "test-actor",
		Model:     "test-model",
	})
	require.NoError(t, err)
	assert.Equal(t, "grava-parent.1", result.ID)
	assert.Equal(t, "open", result.Status)
	assert.Equal(t, "medium", result.Priority)
	assert.Equal(t, "Write unit tests", result.Title)
	assert.False(t, result.Ephemeral)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSubtaskIssue_MissingTitle(t *testing.T) {
	// No DB calls expected — validation fails before any DB work
	store := testutil.NewMockStore()
	_, err := subtaskIssue(context.Background(), store, SubtaskParams{
		ParentID:  "grava-parent",
		IssueType: "task",
		Priority:  "medium",
		Actor:     "test-actor",
		Model:     "test-model",
	})
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "MISSING_REQUIRED_FIELD", gravaErr.Code)
	assert.Equal(t, "title is required", gravaErr.Message)
}

func TestSubtaskIssue_ParentNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("grava-missing").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectRollback()

	store := mockStoreForSubtask(db, 1)
	_, err = subtaskIssue(context.Background(), store, SubtaskParams{
		ParentID:  "grava-missing",
		Title:     "Some subtask",
		IssueType: "task",
		Priority:  "medium",
		Actor:     "test-actor",
		Model:     "test-model",
	})
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "ISSUE_NOT_FOUND", gravaErr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSubtaskIssue_JSONOutputStructure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("grava-abc").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("INSERT INTO issues").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO dependencies").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := mockStoreForSubtask(db, 3)
	result, err := subtaskIssue(context.Background(), store, SubtaskParams{
		ParentID:  "grava-abc",
		Title:     "JSON test",
		IssueType: "bug",
		Priority:  "high",
		Actor:     "agent-1",
		Model:     "claude-opus",
	})
	require.NoError(t, err)

	// NFR5: flat object, snake_case fields, string priority label
	assert.Equal(t, "grava-abc.3", result.ID)
	assert.Equal(t, "JSON test", result.Title)
	assert.Equal(t, "open", result.Status)
	assert.Equal(t, "high", result.Priority) // string label, not integer
	assert.False(t, result.Ephemeral)
}

func TestSubtaskIssue_InvalidPriority(t *testing.T) {
	store := testutil.NewMockStore()
	_, err := subtaskIssue(context.Background(), store, SubtaskParams{
		ParentID:  "grava-parent",
		Title:     "Some subtask",
		IssueType: "task",
		Priority:  "ultra-mega",
		Actor:     "test-actor",
		Model:     "test-model",
	})
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "INVALID_PRIORITY", gravaErr.Code)
}

func TestSubtaskIssue_EphemeralFlag(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("grava-parent").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("INSERT INTO issues").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO dependencies").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := mockStoreForSubtask(db, 1)
	result, err := subtaskIssue(context.Background(), store, SubtaskParams{
		ParentID:  "grava-parent",
		Title:     "Wisp subtask",
		IssueType: "task",
		Priority:  "medium",
		Ephemeral: true,
		Actor:     "test-actor",
		Model:     "test-model",
	})
	require.NoError(t, err)
	assert.True(t, result.Ephemeral)
}
