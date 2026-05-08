package cmdgraph

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	qDepLoadIssues = regexp.QuoteMeta("SELECT id, title, issue_type, status, priority, created_at, updated_at, await_type, await_id, ephemeral, metadata FROM issues WHERE status != 'tombstone'")
	qDepLoadDeps   = regexp.QuoteMeta("SELECT from_id, to_id, type, metadata FROM dependencies")
	qDepLockIssues = regexp.QuoteMeta("SELECT id FROM issues WHERE id IN (?, ?) FOR UPDATE")
	qDepInsert     = regexp.QuoteMeta("INSERT INTO dependencies")
	qDepDelete     = regexp.QuoteMeta("DELETE FROM dependencies WHERE from_id = ? AND to_id = ? AND type = ?")
	qDepEvent      = regexp.QuoteMeta("INSERT INTO events (issue_id, event_type, actor, old_value, new_value, created_by, updated_by, agent_model, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)")
)

func newDepTestCmd(d *cmddeps.Deps) (*cobra.Command, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	cmd := newDepCmd(d)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	return cmd, buf
}

func newDepDeps(client *dolt.Client, outputJSON bool) *cmddeps.Deps {
	actor := "test-actor"
	model := "test-model"
	var store dolt.Store = client
	return &cmddeps.Deps{
		Store:      &store,
		Actor:      &actor,
		AgentModel: &model,
		OutputJSON: &outputJSON,
	}
}

// resetFlags resets package-level dep flags before each test.
func resetFlags() {
	depType = "blocks"
	removeDep = false
}

// TestAddDependency_SelfLoop verifies that adding a self-loop returns an error.
func TestAddDependency_SelfLoop(t *testing.T) {
	resetFlags()
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	d := newDepDeps(dolt.NewClientFromDB(db), false)
	cmd := newDepCmd(d)
	err = addDependency(cmd, d, "ISSUE-1", "ISSUE-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "different issues")
}

// TestAddDependency_HappyPath verifies AC1: inserting a dependency row using WithDeadlockRetry.
func TestAddDependency_HappyPath(t *testing.T) {
	resetFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	d := newDepDeps(dolt.NewClientFromDB(db), false)
	cmd, buf := newDepTestCmd(d)
	cmd.SetContext(context.Background())

	// LoadGraphFromDB (blocking type)
	mock.ExpectQuery(qDepLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("ISSUE-1", "Task 1", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("ISSUE-2", "Task 2", "task", "open", 2, time.Now(), time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qDepLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()))

	// WithAuditedTx
	mock.ExpectBegin()
	mock.ExpectQuery(qDepLockIssues).WithArgs("ISSUE-1", "ISSUE-2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ISSUE-1").AddRow("ISSUE-2"))
	mock.ExpectExec(qDepInsert).
		WithArgs("ISSUE-1", "ISSUE-2", "blocks", "test-actor", "test-actor", "test-model").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(qDepEvent).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = addDependency(cmd, d, "ISSUE-1", "ISSUE-2")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "🔗 Dependency created: ISSUE-1 -[blocks]-> ISSUE-2")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestAddDependency_HappyPath_JSON verifies AC1 JSON output.
func TestAddDependency_HappyPath_JSON(t *testing.T) {
	resetFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	d := newDepDeps(dolt.NewClientFromDB(db), true)
	cmd, buf := newDepTestCmd(d)
	cmd.SetContext(context.Background())

	mock.ExpectQuery(qDepLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("ISSUE-1", "Task 1", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("ISSUE-2", "Task 2", "task", "open", 2, time.Now(), time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qDepLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()))
	mock.ExpectBegin()
	mock.ExpectQuery(qDepLockIssues).WithArgs("ISSUE-1", "ISSUE-2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ISSUE-1").AddRow("ISSUE-2"))
	mock.ExpectExec(qDepInsert).
		WithArgs("ISSUE-1", "ISSUE-2", "blocks", "test-actor", "test-actor", "test-model").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(qDepEvent).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = addDependency(cmd, d, "ISSUE-1", "ISSUE-2")
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, "ISSUE-1", result["from_id"])
	assert.Equal(t, "ISSUE-2", result["to_id"])
	assert.Equal(t, "blocks", result["type"])
	assert.Equal(t, "created", result["status"])
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestAddDependency_NodeNotFound verifies AC5: adding dep with non-existent issue returns NODE_NOT_FOUND.
func TestAddDependency_NodeNotFound(t *testing.T) {
	resetFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	d := newDepDeps(dolt.NewClientFromDB(db), false)
	cmd, _ := newDepTestCmd(d)
	cmd.SetContext(context.Background())

	// LoadGraphFromDB — only ISSUE-1 exists
	mock.ExpectQuery(qDepLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("ISSUE-1", "Task 1", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qDepLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()))

	// Transaction starts, lock query returns only ISSUE-1 (ISSUE-MISSING absent)
	mock.ExpectBegin()
	mock.ExpectQuery(qDepLockIssues).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ISSUE-1"))
	mock.ExpectRollback()

	err = addDependency(cmd, d, "ISSUE-1", "ISSUE-MISSING")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ISSUE-MISSING")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestAddDependency_CircularDependency verifies AC4: adding a cycle returns an error.
func TestAddDependency_CircularDependency(t *testing.T) {
	resetFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	d := newDepDeps(dolt.NewClientFromDB(db), false)
	cmd, _ := newDepTestCmd(d)
	cmd.SetContext(context.Background())

	// LoadGraphFromDB — both issues exist, ISSUE-2 already blocks ISSUE-1
	mock.ExpectQuery(qDepLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("ISSUE-1", "Task 1", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("ISSUE-2", "Task 2", "task", "open", 2, time.Now(), time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qDepLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()).
			AddRow("ISSUE-2", "ISSUE-1", "blocks", nil)) // existing: 2→1

	// Transaction: lock both, then cycle check fails
	mock.ExpectBegin()
	mock.ExpectQuery(qDepLockIssues).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ISSUE-1").AddRow("ISSUE-2"))
	mock.ExpectRollback()

	// Try to add ISSUE-1 → ISSUE-2 (would create cycle: 1→2→1)
	err = addDependency(cmd, d, "ISSUE-1", "ISSUE-2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid dependency")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestRemoveDependency_Flag verifies AC2: --remove flag deletes the dependency row.
func TestRemoveDependency_Flag(t *testing.T) {
	resetFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	d := newDepDeps(dolt.NewClientFromDB(db), false)
	cmd, buf := newDepTestCmd(d)

	mock.ExpectBegin()
	mock.ExpectQuery(qDepLockIssues).WithArgs("ISSUE-1", "ISSUE-2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ISSUE-1").AddRow("ISSUE-2"))
	mock.ExpectExec(qDepDelete).WithArgs("ISSUE-1", "ISSUE-2", "blocks").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(qDepEvent).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	cmd.SetArgs([]string{"ISSUE-1", "ISSUE-2", "--remove"})
	err = cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "✂️ Dependency removed: ISSUE-1 -[blocks]-> ISSUE-2")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestRemoveDependency_NotFound verifies that removing a non-existent dep prints info and returns nil.
func TestRemoveDependency_NotFound(t *testing.T) {
	resetFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	d := newDepDeps(dolt.NewClientFromDB(db), false)
	cmd, buf := newDepTestCmd(d)

	mock.ExpectBegin()
	mock.ExpectQuery(qDepLockIssues).WithArgs("ISSUE-1", "ISSUE-2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ISSUE-1").AddRow("ISSUE-2"))
	mock.ExpectExec(qDepDelete).WithArgs("ISSUE-1", "ISSUE-2", "blocks").
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected
	mock.ExpectCommit()

	cmd.SetArgs([]string{"ISSUE-1", "ISSUE-2", "--remove"})
	err = cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No dependency found")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- grava-cd50: filter CLOSED/tombstone blockers by default ---

// qBlockersForIssueActive matches the default (active-only) query: includes a
// status filter so closed and tombstoned upstream issues are excluded.
var qBlockersForIssueActive = regexp.QuoteMeta(
	`SELECT DISTINCT i.id, i.title, i.status, COALESCE(i.assignee, '') as assignee
			FROM issues i
			INNER JOIN dependencies dep ON
				(dep.from_id = i.id AND dep.to_id = ? AND dep.type = 'blocks')
				OR (dep.to_id = i.id AND dep.from_id = ? AND dep.type = 'blocked-by')
			WHERE i.status NOT IN ('closed', 'tombstone')
			ORDER BY i.priority ASC`,
)

// qBlockersForIssueAll matches the --all variant: no status filter (includes
// closed and tombstoned blockers, for archaeology).
var qBlockersForIssueAll = regexp.QuoteMeta(
	`SELECT DISTINCT i.id, i.title, i.status, COALESCE(i.assignee, '') as assignee
			FROM issues i
			INNER JOIN dependencies dep ON
				(dep.from_id = i.id AND dep.to_id = ? AND dep.type = 'blocks')
				OR (dep.to_id = i.id AND dep.from_id = ? AND dep.type = 'blocked-by')
			ORDER BY i.priority ASC`,
)

// TestShowBlockers_ExcludesClosedByDefault verifies grava-cd50 AC#1: a closed
// upstream blocker is filtered out by default, so /ship Phase 0.2 will not
// halt on already-resolved dependencies.
func TestShowBlockers_ExcludesClosedByDefault(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	d := newDepDeps(dolt.NewClientFromDB(db), true)

	mock.ExpectQuery(qIssueExists).
		WithArgs("task-B").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	// Active-only query (status filter applied) — A is closed so DB returns
	// no rows; the array is empty.
	mock.ExpectQuery(qBlockersForIssueActive).
		WithArgs("task-B", "task-B").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "status", "assignee"}))

	buf := &bytes.Buffer{}
	cmd := newBlockedCmd(d)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"task-B"})
	err = cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	var result []BlockerItem
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Len(t, result, 0, "closed blockers must be filtered out by default")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestShowBlockers_AllFlagIncludesClosed verifies grava-cd50 layer 2: passing
// --all omits the status filter so archaeologists can see closed/tombstoned
// upstream issues.
func TestShowBlockers_AllFlagIncludesClosed(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	d := newDepDeps(dolt.NewClientFromDB(db), true)

	mock.ExpectQuery(qIssueExists).
		WithArgs("task-B").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	// --all variant: no status filter, closed A is returned.
	mock.ExpectQuery(qBlockersForIssueAll).
		WithArgs("task-B", "task-B").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "status", "assignee"}).
			AddRow("task-A", "Closed Blocker", "closed", "alice"))

	buf := &bytes.Buffer{}
	cmd := newBlockedCmd(d)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"task-B", "--all"})
	err = cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	var result []BlockerItem
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	require.Len(t, result, 1, "--all must include closed blockers")
	assert.Equal(t, "task-A", result[0].ID)
	assert.Equal(t, "closed", result[0].Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestShowBlockers_OpenBlockersStillReported is the regression guard for
// grava-cd50: an open upstream blocker must still show up in the default
// (active-only) query. /ship Phase 0.2 must continue to halt on real blockers.
func TestShowBlockers_OpenBlockersStillReported(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	d := newDepDeps(dolt.NewClientFromDB(db), true)

	mock.ExpectQuery(qIssueExists).
		WithArgs("task-B").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(qBlockersForIssueActive).
		WithArgs("task-B", "task-B").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "status", "assignee"}).
			AddRow("task-A", "Open Blocker", "open", "alice"))

	buf := &bytes.Buffer{}
	cmd := newBlockedCmd(d)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"task-B"})
	err = cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	var result []BlockerItem
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	require.Len(t, result, 1, "open blockers must still be reported")
	assert.Equal(t, "task-A", result[0].ID)
	assert.Equal(t, "open", result[0].Status)
	require.NoError(t, mock.ExpectationsWereMet())
}
