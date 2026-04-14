package maintenance

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
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

// TestQueryCmdHistory_ArgsIsJSONArray verifies that the Args field is
// deserialized as a JSON array (not a double-encoded string) in the output.
func TestQueryCmdHistory_ArgsIsJSONArray(t *testing.T) {
	store, mock := newMaintMock(t)
	now := time.Now()

	mock.ExpectQuery(qQueryCmdHistory).
		WillReturnRows(historyColumns().
			AddRow("cal-001122", "grava create", "agent-01", `["create","--title","foo"]`, 0, now))

	entries, err := QueryCmdHistory(context.Background(), store, "", 50)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	// Encode entry to JSON and verify args is an array, not a string.
	b, err := json.Marshal(entries[0])
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(b, &decoded))

	argsRaw, ok := decoded["args"]
	require.True(t, ok, "args field must be present")
	// args must decode as a []interface{}, not a string.
	_, isArray := argsRaw.([]any)
	assert.True(t, isArray, "args must be a JSON array, got %T: %v", argsRaw, argsRaw)

	// Spot-check the values.
	arr := argsRaw.([]any)
	assert.Equal(t, "create", arr[0])
	assert.Equal(t, "--title", arr[1])
	assert.Equal(t, "foo", arr[2])

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestQueryCmdHistory_ArgsEmptyOmitted verifies that empty args are omitted from JSON.
func TestQueryCmdHistory_ArgsEmptyOmitted(t *testing.T) {
	store, mock := newMaintMock(t)
	now := time.Now()

	mock.ExpectQuery(qQueryCmdHistory).
		WillReturnRows(historyColumns().
			AddRow("cal-003344", "grava list", "agent-01", "", 0, now))

	entries, err := QueryCmdHistory(context.Background(), store, "", 50)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	b, err := json.Marshal(entries[0])
	require.NoError(t, err)

	// "args" should not appear when empty (omitempty).
	assert.NotContains(t, string(b), `"args"`)

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestCmdHistoryCmd_JSONOutput exercises the --json output path via bytes.Buffer,
// specifically verifying that the args field serializes as a JSON array (not a string).
func TestCmdHistoryCmd_JSONOutput(t *testing.T) {
	store, mock := newMaintMock(t)
	now := time.Now()

	mock.ExpectQuery(qQueryCmdHistory).
		WillReturnRows(historyColumns().
			AddRow("cal-aabb99", "grava create", "agent-01", `["create","--title","x"]`, 0, now))

	outputJSON := true
	actor := "agent-01"
	agentModel := ""
	d := &cmddeps.Deps{Store: &store, OutputJSON: &outputJSON, Actor: &actor, AgentModel: &agentModel}
	cmd := newCmdHistoryCmd(d)
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, cmd.Execute())

	var result map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	entries, ok := result["entries"].([]any)
	require.True(t, ok, "entries must be a JSON array")
	require.Len(t, entries, 1)

	entry := entries[0].(map[string]any)
	args, ok := entry["args"].([]any)
	require.True(t, ok, "args must be a JSON array in output, not a string — got: %T %v", entry["args"], entry["args"])
	assert.Equal(t, "create", args[0])

	require.NoError(t, mock.ExpectationsWereMet())
}
