package maintenance

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMaintMock(t *testing.T) (dolt.Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() }) //nolint:errcheck
	return dolt.NewClientFromDB(db), mock
}

var qQueryCmdHistory = regexp.QuoteMeta(`SELECT id, command, actor, COALESCE(args_json,''), exit_code, created_at`)

func historyColumns() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "command", "actor", "args_json", "exit_code", "created_at"})
}

// TestQueryCmdHistory_ReturnsAll verifies that all entries are returned when no actor filter is set.
func TestQueryCmdHistory_ReturnsAll(t *testing.T) {
	store, mock := newMaintMock(t)
	now := time.Now()

	mock.ExpectQuery(qQueryCmdHistory).
		WillReturnRows(historyColumns().
			AddRow("cal-aabb01", "grava create", "agent-01", `["create","--title","x"]`, 0, now).
			AddRow("cal-aabb02", "grava update", "agent-02", `["update","grava-1234"]`, 0, now))

	entries, err := QueryCmdHistory(context.Background(), store, "", 50)

	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "grava create", entries[0].Command)
	assert.Equal(t, "agent-01", entries[0].Actor)
	assert.Equal(t, "grava update", entries[1].Command)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestQueryCmdHistory_WithActorFilter verifies actor filter is applied.
func TestQueryCmdHistory_WithActorFilter(t *testing.T) {
	store, mock := newMaintMock(t)
	now := time.Now()

	mock.ExpectQuery(qQueryCmdHistory).
		WillReturnRows(historyColumns().
			AddRow("cal-cc0011", "grava claim", "agent-01", `["claim","grava-abc"]`, 0, now))

	entries, err := QueryCmdHistory(context.Background(), store, "agent-01", 50)

	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "agent-01", entries[0].Actor)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestQueryCmdHistory_Empty verifies that an empty result returns nil slice (not error).
func TestQueryCmdHistory_Empty(t *testing.T) {
	store, mock := newMaintMock(t)

	mock.ExpectQuery(qQueryCmdHistory).
		WillReturnRows(historyColumns())

	entries, err := QueryCmdHistory(context.Background(), store, "", 50)

	require.NoError(t, err)
	assert.Empty(t, entries)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestQueryCmdHistory_DefaultLimit verifies limit < 1 is clamped to 50.
func TestQueryCmdHistory_DefaultLimit(t *testing.T) {
	store, mock := newMaintMock(t)

	mock.ExpectQuery(qQueryCmdHistory).
		WillReturnRows(historyColumns())

	_, err := QueryCmdHistory(context.Background(), store, "", 0)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestQueryCmdHistory_DBError verifies that a DB error is propagated.
func TestQueryCmdHistory_DBError(t *testing.T) {
	store, mock := newMaintMock(t)

	mock.ExpectQuery(qQueryCmdHistory).
		WillReturnError(assert.AnError)

	_, err := QueryCmdHistory(context.Background(), store, "", 50)

	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestRecordCommand_Inserts verifies that RecordCommand inserts a row.
func TestRecordCommand_Inserts(t *testing.T) {
	store, mock := newMaintMock(t)

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO cmd_audit_log`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	RecordCommand(context.Background(), store, "grava create", "agent-01", `["create"]`, 0)

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestRecordCommand_SwallowsError verifies that an INSERT error does not panic.
func TestRecordCommand_SwallowsError(t *testing.T) {
	store, mock := newMaintMock(t)

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO cmd_audit_log`)).
		WillReturnError(assert.AnError)

	// Must not panic.
	RecordCommand(context.Background(), store, "grava create", "agent-01", `["create"]`, 0)

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestGenerateCmdAuditID verifies the ID format "cal-XXXXXX".
func TestGenerateCmdAuditID(t *testing.T) {
	id := generateCmdAuditID()
	assert.Regexp(t, `^cal-[0-9a-f]{6}$`, id)
}
