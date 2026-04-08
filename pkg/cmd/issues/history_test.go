package issues

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── issueHistory tests (Task 3) ────────────────────────────────────────────

func TestHistory_HappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	ts1 := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC)
	ts3 := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("abc123def456"))
	mock.ExpectQuery("SELECT event_type, actor, old_value, new_value, timestamp").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"event_type", "actor", "old_value", "new_value", "timestamp"}).
			AddRow("create", "agent-01", "{}", `{"status":"open"}`, ts1).
			AddRow("claim", "agent-02", `{"status":"open"}`, `{"status":"in_progress","actor":"agent-02"}`, ts2).
			AddRow("wisp_write", "agent-02", "{}", `{"key":"checkpoint","value":"step-1"}`, ts3))

	store := dolt.NewClientFromDB(db)
	entries, err := issueHistory(context.Background(), store, "abc123def456", "")
	require.NoError(t, err)
	assert.Len(t, entries, 3)
	assert.Equal(t, "create", entries[0].EventType)
	assert.Equal(t, "agent-01", entries[0].Actor)
	assert.Equal(t, "claim", entries[1].EventType)
	assert.Equal(t, "agent-02", entries[1].Actor)
	assert.Equal(t, "wisp_write", entries[2].EventType)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHistory_SinceFilter(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	ts := time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)
	sinceTime := time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("abc123def456"))
	mock.ExpectQuery("SELECT event_type, actor, old_value, new_value, timestamp").
		WithArgs("abc123def456", sinceTime).
		WillReturnRows(sqlmock.NewRows([]string{"event_type", "actor", "old_value", "new_value", "timestamp"}).
			AddRow("update", "agent-03", `{"status":"in_progress"}`, `{"status":"closed"}`, ts))

	store := dolt.NewClientFromDB(db)
	entries, err := issueHistory(context.Background(), store, "abc123def456", "2026-03-21")
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "update", entries[0].EventType)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHistory_EmptyHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("abc123def456"))
	mock.ExpectQuery("SELECT event_type, actor, old_value, new_value, timestamp").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"event_type", "actor", "old_value", "new_value", "timestamp"}))

	store := dolt.NewClientFromDB(db)
	entries, err := issueHistory(context.Background(), store, "abc123def456", "")
	require.NoError(t, err)
	assert.Empty(t, entries)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHistory_IssueNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{"id"})) // empty → ErrNoRows

	store := dolt.NewClientFromDB(db)
	_, err = issueHistory(context.Background(), store, "nonexistent", "")
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "ISSUE_NOT_FOUND", gravaErr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHistory_JSONStructure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	ts := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("abc123def456"))
	mock.ExpectQuery("SELECT event_type, actor, old_value, new_value, timestamp").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"event_type", "actor", "old_value", "new_value", "timestamp"}).
			AddRow("create", "agent-01", "{}", `{"status":"open","title":"Test issue"}`, ts))

	store := dolt.NewClientFromDB(db)
	entries, err := issueHistory(context.Background(), store, "abc123def456", "")
	require.NoError(t, err)
	require.Len(t, entries, 1)

	// Verify HistoryEntry struct fields.
	entry := entries[0]
	assert.Equal(t, "create", entry.EventType)
	assert.Equal(t, "agent-01", entry.Actor)
	assert.Equal(t, ts, entry.Timestamp)
	assert.NotNil(t, entry.Details)
	assert.Equal(t, "open", entry.Details["status"])
	assert.Equal(t, "Test issue", entry.Details["title"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHistory_EventTypesCoverage(t *testing.T) {
	// Verify that create, claim, update, wisp_write, comment, label events all appear.
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	ts := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	eventTypes := []string{"create", "claim", "update", "wisp_write", "comment", "label"}

	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("abc123def456"))

	rowData := sqlmock.NewRows([]string{"event_type", "actor", "old_value", "new_value", "timestamp"})
	for i, et := range eventTypes {
		rowData.AddRow(et, "agent-01", "{}", `{"action":"test"}`, ts.Add(time.Duration(i)*time.Minute))
	}
	mock.ExpectQuery("SELECT event_type, actor, old_value, new_value, timestamp").
		WithArgs("abc123def456").
		WillReturnRows(rowData)

	store := dolt.NewClientFromDB(db)
	entries, err := issueHistory(context.Background(), store, "abc123def456", "")
	require.NoError(t, err)
	assert.Len(t, entries, 6)

	for i, et := range eventTypes {
		assert.Equal(t, et, entries[i].EventType, "event at index %d should be %s", i, et)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

// ─── Task 4: Integration with Wisp and Claim events ─────────────────────────

func TestHistory_WispWriteEventAppears(t *testing.T) {
	// Verify EventWispWrite ("wisp_write") events appear in history output.
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	ts := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("abc123def456"))
	mock.ExpectQuery("SELECT event_type, actor, old_value, new_value, timestamp").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"event_type", "actor", "old_value", "new_value", "timestamp"}).
			AddRow(dolt.EventWispWrite, "agent-02", "{}", `{"key":"checkpoint","value":"step-3"}`, ts))

	store := dolt.NewClientFromDB(db)
	entries, err := issueHistory(context.Background(), store, "abc123def456", "")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "wisp_write", entries[0].EventType)
	assert.Equal(t, "checkpoint", entries[0].Details["key"])
	assert.Equal(t, "step-3", entries[0].Details["value"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHistory_ClaimEventAppears(t *testing.T) {
	// Verify EventClaim ("claim") events appear in history output.
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	ts := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT id FROM issues WHERE id").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("abc123def456"))
	mock.ExpectQuery("SELECT event_type, actor, old_value, new_value, timestamp").
		WithArgs("abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"event_type", "actor", "old_value", "new_value", "timestamp"}).
			AddRow(dolt.EventClaim, "agent-01", `{"status":"open"}`, `{"status":"in_progress","actor":"agent-01"}`, ts))

	store := dolt.NewClientFromDB(db)
	entries, err := issueHistory(context.Background(), store, "abc123def456", "")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "claim", entries[0].EventType)
	assert.Equal(t, "in_progress", entries[0].Details["status"])
	assert.Equal(t, "open", entries[0].Details["old_status"])
	require.NoError(t, mock.ExpectationsWereMet())
}

// ─── Helper tests ────────────────────────────────────────────────────────────

func TestParseSinceDate_RFC3339(t *testing.T) {
	parsed, err := parseSinceDate("2026-03-21T10:30:00Z")
	require.NoError(t, err)
	assert.Equal(t, 2026, parsed.Year())
	assert.Equal(t, time.March, parsed.Month())
	assert.Equal(t, 21, parsed.Day())
}

func TestParseSinceDate_DateOnly(t *testing.T) {
	parsed, err := parseSinceDate("2026-03-21")
	require.NoError(t, err)
	assert.Equal(t, 2026, parsed.Year())
	assert.Equal(t, time.March, parsed.Month())
	assert.Equal(t, 21, parsed.Day())
	assert.Equal(t, 0, parsed.Hour())
}

func TestParseSinceDate_Invalid(t *testing.T) {
	_, err := parseSinceDate("not-a-date")
	require.Error(t, err)
}

func TestMergeEventDetails_BothValues(t *testing.T) {
	details := mergeEventDetails(
		toNullString(`{"status":"open"}`),
		toNullString(`{"status":"in_progress","actor":"agent-01"}`),
	)
	assert.Equal(t, "open", details["old_status"])
	assert.Equal(t, "in_progress", details["status"])
	assert.Equal(t, "agent-01", details["actor"])
}

func TestMergeEventDetails_NewOnly(t *testing.T) {
	details := mergeEventDetails(
		toNullString("{}"),
		toNullString(`{"key":"checkpoint"}`),
	)
	assert.Equal(t, "checkpoint", details["key"])
	_, hasOld := details["old_key"]
	assert.False(t, hasOld)
}

func TestMergeEventDetails_Empty(t *testing.T) {
	details := mergeEventDetails(toNullString("{}"), toNullString("{}"))
	assert.Empty(t, details)
}

func toNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}
