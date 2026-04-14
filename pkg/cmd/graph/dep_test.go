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
	qDepLoadIssues = regexp.QuoteMeta("SELECT id, title, issue_type, status, priority, created_at, await_type, await_id, ephemeral, metadata FROM issues WHERE status != 'tombstone'")
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
			AddRow("ISSUE-1", "Task 1", "task", "open", 1, time.Now(), nil, nil, 0, nil).
			AddRow("ISSUE-2", "Task 2", "task", "open", 2, time.Now(), nil, nil, 0, nil))
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
			AddRow("ISSUE-1", "Task 1", "task", "open", 1, time.Now(), nil, nil, 0, nil).
			AddRow("ISSUE-2", "Task 2", "task", "open", 2, time.Now(), nil, nil, 0, nil))
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
			AddRow("ISSUE-1", "Task 1", "task", "open", 1, time.Now(), nil, nil, 0, nil))
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
			AddRow("ISSUE-1", "Task 1", "task", "open", 1, time.Now(), nil, nil, 0, nil).
			AddRow("ISSUE-2", "Task 2", "task", "open", 2, time.Now(), nil, nil, 0, nil))
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
