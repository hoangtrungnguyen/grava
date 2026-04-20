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

func mockStoreForLabel(db *sql.DB) *testutil.MockStore {
	store := testutil.NewMockStore()
	// guardNotArchived calls store.QueryRow before WithAuditedTx opens the tx.
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

func TestLabelIssue_AddLabels(t *testing.T) {
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
	// INSERT IGNORE for each label
	mock.ExpectExec(regexp.QuoteMeta("INSERT IGNORE INTO issue_labels")).
		WithArgs("grava-abc", "bug", "test-actor").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT IGNORE INTO issue_labels")).
		WithArgs("grava-abc", "critical", "test-actor").
		WillReturnResult(sqlmock.NewResult(2, 1))
	// Query final labels
	mock.ExpectQuery(regexp.QuoteMeta("SELECT label FROM issue_labels WHERE issue_id = ?")).
		WithArgs("grava-abc").
		WillReturnRows(sqlmock.NewRows([]string{"label"}).AddRow("bug").AddRow("critical"))
	// Audit event
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := mockStoreForLabel(db)
	result, err := labelIssue(context.Background(), store, LabelParams{
		ID:        "grava-abc",
		AddLabels: []string{"bug", "critical"},
		Actor:     "test-actor",
		Model:     "test-model",
	})
	require.NoError(t, err)
	assert.Equal(t, "grava-abc", result.ID)
	assert.Equal(t, []string{"bug", "critical"}, result.LabelsAdded)
	assert.Equal(t, []string{}, result.LabelsRemoved)
	assert.Equal(t, []string{"bug", "critical"}, result.CurrentLabels)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLabelIssue_RemoveLabel(t *testing.T) {
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
	// DELETE for removed label
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM issue_labels WHERE issue_id = ? AND label = ?")).
		WithArgs("grava-abc", "bug").
		WillReturnResult(sqlmock.NewResult(0, 1))
	// Query final labels (only critical remains)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT label FROM issue_labels WHERE issue_id = ?")).
		WithArgs("grava-abc").
		WillReturnRows(sqlmock.NewRows([]string{"label"}).AddRow("critical"))
	// Audit event
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := mockStoreForLabel(db)
	result, err := labelIssue(context.Background(), store, LabelParams{
		ID:           "grava-abc",
		RemoveLabels: []string{"bug"},
		Actor:        "test-actor",
		Model:        "test-model",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{}, result.LabelsAdded)
	assert.Equal(t, []string{"bug"}, result.LabelsRemoved)
	assert.Equal(t, []string{"critical"}, result.CurrentLabels)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLabelIssue_AddAndRemove(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	// guardNotArchived runs before WithAuditedTx
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM issues WHERE id = ?")).
		WithArgs("grava-abc").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("open"))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM issues WHERE id = ?")).
		WithArgs("grava-abc").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("grava-abc"))
	// Add "urgent"
	mock.ExpectExec(regexp.QuoteMeta("INSERT IGNORE INTO issue_labels")).
		WithArgs("grava-abc", "urgent", "test-actor").
		WillReturnResult(sqlmock.NewResult(1, 1))
	// Remove "low"
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM issue_labels WHERE issue_id = ? AND label = ?")).
		WithArgs("grava-abc", "low").
		WillReturnResult(sqlmock.NewResult(0, 1))
	// Final labels
	mock.ExpectQuery(regexp.QuoteMeta("SELECT label FROM issue_labels WHERE issue_id = ?")).
		WithArgs("grava-abc").
		WillReturnRows(sqlmock.NewRows([]string{"label"}).AddRow("critical").AddRow("urgent"))
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := mockStoreForLabel(db)
	result, err := labelIssue(context.Background(), store, LabelParams{
		ID:           "grava-abc",
		AddLabels:    []string{"urgent"},
		RemoveLabels: []string{"low"},
		Actor:        "test-actor",
		Model:        "test-model",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"urgent"}, result.LabelsAdded)
	assert.Equal(t, []string{"low"}, result.LabelsRemoved)
	assert.Equal(t, []string{"critical", "urgent"}, result.CurrentLabels)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLabelIssue_IssueNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	// guardNotArchived returns ISSUE_NOT_FOUND before WithAuditedTx is entered
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM issues WHERE id = ?")).
		WithArgs("grava-missing").
		WillReturnRows(sqlmock.NewRows([]string{"status"})) // empty → sql.ErrNoRows

	store := mockStoreForLabel(db)
	_, err = labelIssue(context.Background(), store, LabelParams{
		ID:        "grava-missing",
		AddLabels: []string{"bug"},
		Actor:     "test-actor",
		Model:     "test-model",
	})
	testutil.AssertGravaError(t, err, "ISSUE_NOT_FOUND")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLabelIssue_NoFlags(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	store := mockStoreForLabel(db)
	_, err = labelIssue(context.Background(), store, LabelParams{
		ID:    "grava-abc",
		Actor: "test-actor",
		Model: "test-model",
	})
	testutil.AssertGravaError(t, err, "MISSING_REQUIRED_FIELD")
}
