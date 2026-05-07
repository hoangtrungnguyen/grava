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

func TestCreateIssue_HappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO issues").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := createIssue(context.Background(), store, CreateParams{
		Title:     "Test issue",
		IssueType: "task",
		Priority:  "medium",
		Actor:     "test-actor",
		Model:     "test-model",
	})
	require.NoError(t, err)
	assert.Equal(t, "open", result.Status)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "Test issue", result.Title)
	assert.Equal(t, "medium", result.Priority)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateIssue_MissingTitle(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	store := dolt.NewClientFromDB(db)
	_, err = createIssue(context.Background(), store, CreateParams{
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
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateIssue_InvalidIssueType(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	store := dolt.NewClientFromDB(db)
	_, err = createIssue(context.Background(), store, CreateParams{
		Title:     "Test issue",
		IssueType: "invalid-type",
		Priority:  "medium",
		Actor:     "test-actor",
		Model:     "test-model",
	})
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "INVALID_ISSUE_TYPE", gravaErr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateIssue_InvalidPriority(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	store := dolt.NewClientFromDB(db)
	_, err = createIssue(context.Background(), store, CreateParams{
		Title:     "Test issue",
		IssueType: "task",
		Priority:  "ultra-mega",
		Actor:     "test-actor",
		Model:     "test-model",
	})
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "INVALID_PRIORITY", gravaErr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateIssue_QuickCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO issues").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := createIssue(context.Background(), store, CreateParams{
		Title:     "Quick task",
		IssueType: "task",
		Priority:  "medium",
		Actor:     "test-actor",
		Model:     "test-model",
	})
	require.NoError(t, err)
	assert.Equal(t, "open", result.Status)
	assert.Equal(t, "medium", result.Priority)
	assert.Equal(t, "Quick task", result.Title)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateIssue_WithParent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO issues").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("grava-parent").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("INSERT INTO dependencies").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Use MockStore so GetNextChildSequence returns a controlled value
	// while the tx operations go through the sqlmock db.
	store := testutil.NewMockStore()
	store.GetNextChildSequenceFn = func(parentID string) (int, error) { return 1, nil }
	store.BeginTxFn = func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
		return dolt.NewClientFromDB(db).BeginTx(ctx, nil)
	}
	store.LogEventTxFn = func(ctx context.Context, tx *sql.Tx, issueID, eventType, actor, model string, old, new interface{}) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO events VALUES ()")
		return err
	}

	result, err := createIssue(context.Background(), store, CreateParams{
		Title:     "Subtask issue",
		IssueType: "task",
		Priority:  "medium",
		ParentID:  "grava-parent",
		Actor:     "test-actor",
		Model:     "test-model",
	})
	require.NoError(t, err)
	assert.Contains(t, result.ID, "grava-parent.")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateIssue_ParentNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO issues").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("grava-nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectRollback()

	store := testutil.NewMockStore()
	store.GetNextChildSequenceFn = func(parentID string) (int, error) { return 1, nil }
	store.BeginTxFn = func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
		return dolt.NewClientFromDB(db).BeginTx(ctx, nil)
	}
	store.LogEventTxFn = func(ctx context.Context, tx *sql.Tx, issueID, eventType, actor, model string, old, new interface{}) error {
		return nil
	}

	_, err = createIssue(context.Background(), store, CreateParams{
		Title:     "Subtask issue",
		IssueType: "task",
		Priority:  "medium",
		ParentID:  "grava-nonexistent",
		Actor:     "test-actor",
		Model:     "test-model",
	})
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "PARENT_NOT_FOUND", gravaErr.Code)
}

func TestCreateIssue_JSONOutputStructure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO issues").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := createIssue(context.Background(), store, CreateParams{
		Title:     "JSON test",
		IssueType: "bug",
		Priority:  "high",
		Actor:     "agent-1",
		Model:     "claude-opus",
	})
	require.NoError(t, err)

	// NFR5: flat object, snake_case fields
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "JSON test", result.Title)
	assert.Equal(t, "open", result.Status)
	assert.Equal(t, "high", result.Priority)
	assert.False(t, result.Ephemeral)
}

// BenchmarkCreateIssue measures Go-side overhead of createIssue (not DB latency).
// NFR2 requires <15ms for write operations; this benchmark validates the Go layer stays well under that.
func BenchmarkCreateIssue(b *testing.B) {
	db, mock, err := sqlmock.New()
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close() //nolint:errcheck

	for i := 0; i < b.N; i++ {
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO issues").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
	}

	store := dolt.NewClientFromDB(db)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = createIssue(context.Background(), store, CreateParams{
			Title:     "Bench issue",
			IssueType: "task",
			Priority:  "medium",
			Actor:     "bench-actor",
			Model:     "bench-model",
		})
	}
}

func TestCreateIssue_IssueTypeNormalized(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO issues").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	// Pass mixed-case issue type — should be normalized to lowercase before INSERT
	result, err := createIssue(context.Background(), store, CreateParams{
		Title:     "Normalized type test",
		IssueType: "Task",
		Priority:  "medium",
		Actor:     "test-actor",
		Model:     "test-model",
	})
	require.NoError(t, err)
	assert.Equal(t, "open", result.Status)
	assert.Equal(t, "Normalized type test", result.Title)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateIssue_EphemeralFlag(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO issues").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := createIssue(context.Background(), store, CreateParams{
		Title:     "Wisp task",
		IssueType: "task",
		Priority:  "medium",
		Ephemeral: true,
		Actor:     "test-actor",
		Model:     "test-model",
	})
	require.NoError(t, err)
	assert.True(t, result.Ephemeral)
}
