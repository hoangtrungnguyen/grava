package issues

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── wispWrite tests (Task 6) ───────────────────────────────────────────────

func TestWispWrite_HappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("abc123def456"))
	mock.ExpectExec("INSERT INTO wisp_entries").
		WithArgs("abc123def456", "checkpoint", "step-3-complete", "agent-01").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE issues SET wisp_heartbeat_at").
		WithArgs("abc123def456").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := wispWrite(context.Background(), store, WispWriteParams{
		IssueID: "abc123def456",
		Key:     "checkpoint",
		Value:   "step-3-complete",
		Actor:   "agent-01",
	})
	require.NoError(t, err)
	assert.Equal(t, "abc123def456", result.IssueID)
	assert.Equal(t, "checkpoint", result.Key)
	assert.Equal(t, "step-3-complete", result.Value)
	assert.Equal(t, "agent-01", result.WrittenBy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWispWrite_Upsert(t *testing.T) {
	// Write same key twice — second call should still succeed (ON DUPLICATE KEY UPDATE).
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	for i := 0; i < 2; i++ {
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT id FROM issues WHERE id").
			WithArgs("abc123def456").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("abc123def456"))
		mock.ExpectExec("INSERT INTO wisp_entries").
			WithArgs("abc123def456", "checkpoint", "step-4-complete", "agent-01").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("UPDATE issues SET wisp_heartbeat_at").
			WithArgs("abc123def456").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("INSERT INTO events").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
	}

	store := dolt.NewClientFromDB(db)
	for i := 0; i < 2; i++ {
		_, err := wispWrite(context.Background(), store, WispWriteParams{
			IssueID: "abc123def456",
			Key:     "checkpoint",
			Value:   "step-4-complete",
			Actor:   "agent-01",
		})
		require.NoError(t, err)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWispWrite_IssueNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{"id"})) // empty → ErrNoRows
	mock.ExpectRollback()

	store := dolt.NewClientFromDB(db)
	_, err = wispWrite(context.Background(), store, WispWriteParams{
		IssueID: "nonexistent",
		Key:     "key",
		Value:   "value",
		Actor:   "agent-01",
	})
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "ISSUE_NOT_FOUND", gravaErr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWispWrite_HeartbeatUpdated(t *testing.T) {
	// Verify the UPDATE issues SET wisp_heartbeat_at expectation is included (same as HappyPath).
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("abc123def456"))
	mock.ExpectExec("INSERT INTO wisp_entries").
		WithArgs("abc123def456", "status", "running", "agent-01").
		WillReturnResult(sqlmock.NewResult(1, 1))
	// This expectation verifies wisp_heartbeat_at is updated.
	mock.ExpectExec("UPDATE issues SET wisp_heartbeat_at").
		WithArgs("abc123def456").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	_, err = wispWrite(context.Background(), store, WispWriteParams{
		IssueID: "abc123def456",
		Key:     "status",
		Value:   "running",
		Actor:   "agent-01",
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWispWrite_JSONResultStructure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("abc123def456"))
	mock.ExpectExec("INSERT INTO wisp_entries").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE issues SET wisp_heartbeat_at").
		WithArgs("abc123def456").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := wispWrite(context.Background(), store, WispWriteParams{
		IssueID: "abc123def456",
		Key:     "checkpoint",
		Value:   "step-3-complete",
		Actor:   "agent-01",
	})
	require.NoError(t, err)
	// Verify all JSON fields are populated.
	assert.NotEmpty(t, result.IssueID)
	assert.NotEmpty(t, result.Key)
	assert.NotEmpty(t, result.Value)
	assert.NotEmpty(t, result.WrittenBy)
}

// ─── wispRead tests (Task 7) ────────────────────────────────────────────────

func TestWispRead_SingleKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	writtenAt := time.Now().Truncate(time.Second)

	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("abc123def456"))
	mock.ExpectQuery("SELECT key_name, value, written_by, written_at").
		WithArgs("abc123def456", "checkpoint").
		WillReturnRows(sqlmock.NewRows([]string{"key_name", "value", "written_by", "written_at"}).
			AddRow("checkpoint", "step-3-complete", "agent-01", writtenAt))

	store := dolt.NewClientFromDB(db)
	result, err := wispRead(context.Background(), store, "abc123def456", "checkpoint")
	require.NoError(t, err)
	entry, ok := result.(*WispEntry)
	require.True(t, ok)
	assert.Equal(t, "checkpoint", entry.Key)
	assert.Equal(t, "step-3-complete", entry.Value)
	assert.Equal(t, "agent-01", entry.WrittenBy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWispRead_AllEntries(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	writtenAt := time.Now().Truncate(time.Second)

	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("abc123def456"))
	mock.ExpectQuery("SELECT key_name, value, written_by, written_at").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"key_name", "value", "written_by", "written_at"}).
			AddRow("checkpoint", "step-3-complete", "agent-01", writtenAt).
			AddRow("status", "running", "agent-01", writtenAt))

	store := dolt.NewClientFromDB(db)
	result, err := wispRead(context.Background(), store, "abc123def456", "")
	require.NoError(t, err)
	entries, ok := result.([]WispEntry)
	require.True(t, ok)
	assert.Len(t, entries, 2)
	assert.Equal(t, "checkpoint", entries[0].Key)
	assert.Equal(t, "status", entries[1].Key)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWispRead_MissingKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("abc123def456"))
	mock.ExpectQuery("SELECT key_name, value, written_by, written_at").
		WithArgs("abc123def456", "missing-key").
		WillReturnRows(sqlmock.NewRows([]string{"key_name", "value", "written_by", "written_at"})) // empty

	store := dolt.NewClientFromDB(db)
	_, err = wispRead(context.Background(), store, "abc123def456", "missing-key")
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "WISP_NOT_FOUND", gravaErr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWispRead_EmptyIssue(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("abc123def456"))
	mock.ExpectQuery("SELECT key_name, value, written_by, written_at").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"key_name", "value", "written_by", "written_at"})) // empty

	store := dolt.NewClientFromDB(db)
	result, err := wispRead(context.Background(), store, "abc123def456", "")
	require.NoError(t, err)
	entries, ok := result.([]WispEntry)
	require.True(t, ok)
	assert.Empty(t, entries) // must be [], not nil/error
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWispRead_IssueNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{"id"})) // empty → ErrNoRows

	store := dolt.NewClientFromDB(db)
	_, err = wispRead(context.Background(), store, "nonexistent", "some-key")
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "ISSUE_NOT_FOUND", gravaErr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}
