package issues

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaimIssue_HappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, assignee, wisp_heartbeat_at FROM issues WHERE id").
		WithArgs("grava-abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee", "wisp_heartbeat_at"}).AddRow("open", nil, nil))
	mock.ExpectExec("UPDATE issues SET").
		WithArgs("actor1", "model1", "actor1", "grava-abc123def456").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := claimIssue(context.Background(), store, "grava-abc123def456", "actor1", "model1")
	require.NoError(t, err)
	assert.Equal(t, "grava-abc123def456", result.IssueID)
	assert.Equal(t, "in_progress", result.Status)
	assert.Equal(t, "actor1", result.Actor)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimIssue_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, assignee, wisp_heartbeat_at FROM issues WHERE id").
		WithArgs("grava-notfound").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee", "wisp_heartbeat_at"})) // empty → ErrNoRows
	mock.ExpectRollback()

	store := dolt.NewClientFromDB(db)
	_, err = claimIssue(context.Background(), store, "grava-notfound", "actor1", "model1")
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "ISSUE_NOT_FOUND", gravaErr.Code)
}

func TestClaimIssue_AlreadyClaimed(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, assignee, wisp_heartbeat_at FROM issues WHERE id").
		WithArgs("grava-abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee", "wisp_heartbeat_at"}).AddRow("in_progress", "actor1", nil))
	mock.ExpectRollback()

	store := dolt.NewClientFromDB(db)
	_, err = claimIssue(context.Background(), store, "grava-abc123def456", "actor1", "model1")
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "ALREADY_CLAIMED", gravaErr.Code)
}

func TestClaimIssue_InvalidTransition(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, assignee, wisp_heartbeat_at FROM issues WHERE id").
		WithArgs("grava-abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee", "wisp_heartbeat_at"}).AddRow("closed", nil, nil))
	mock.ExpectRollback()

	store := dolt.NewClientFromDB(db)
	_, err = claimIssue(context.Background(), store, "grava-abc123def456", "actor1", "model1")
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "INVALID_STATUS_TRANSITION", gravaErr.Code)
}

// TestClaimIssue_AlreadyClaimed_OpenWithAssignee covers the data-inconsistency edge case:
// status is "open" but assignee is already set — should still reject as ALREADY_CLAIMED.
func TestClaimIssue_AlreadyClaimed_OpenWithAssignee(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, assignee, wisp_heartbeat_at FROM issues WHERE id").
		WithArgs("grava-abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee", "wisp_heartbeat_at"}).AddRow("open", "sneaky-actor", nil))
	mock.ExpectRollback()

	store := dolt.NewClientFromDB(db)
	_, err = claimIssue(context.Background(), store, "grava-abc123def456", "actor1", "model1")
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "ALREADY_CLAIMED", gravaErr.Code)
}


// TestRollbackClaimDB verifies that rollback restores DB state on worktree failure.
func TestRollbackClaimDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	issueID := "grava-rollback-test"

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE issues SET status='open'").
		WithArgs(issueID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	err = rollbackClaimDB(context.Background(), store, issueID)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestRollbackClaimDB_DBError verifies error handling when rollback fails.
func TestRollbackClaimDB_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	issueID := "grava-rollback-error"

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE issues SET status='open'").
		WithArgs(issueID).
		WillReturnError(sql.ErrConnDone)

	store := dolt.NewClientFromDB(db)
	err = rollbackClaimDB(context.Background(), store, issueID)
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "DB_UNREACHABLE", gravaErr.Code)
}
