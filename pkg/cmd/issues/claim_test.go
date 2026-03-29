package issues

import (
	"context"
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
	mock.ExpectQuery("SELECT status FROM issues WHERE id").
		WithArgs("grava-abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("open"))
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
	mock.ExpectQuery("SELECT status FROM issues WHERE id").
		WithArgs("grava-notfound").
		WillReturnRows(sqlmock.NewRows([]string{"status"})) // empty → ErrNoRows
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
	mock.ExpectQuery("SELECT status FROM issues WHERE id").
		WithArgs("grava-abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("in_progress"))
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
	mock.ExpectQuery("SELECT status FROM issues WHERE id").
		WithArgs("grava-abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("closed"))
	mock.ExpectRollback()

	store := dolt.NewClientFromDB(db)
	_, err = claimIssue(context.Background(), store, "grava-abc123def456", "actor1", "model1")
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "INVALID_STATUS_TRANSITION", gravaErr.Code)
}
