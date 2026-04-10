package cmdgraph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	qValidateIssues = regexp.QuoteMeta("SELECT id FROM issues WHERE id IN")
	qLoadIssues     = regexp.QuoteMeta("SELECT id, title, issue_type, status, priority, created_at, await_type, await_id, ephemeral, metadata FROM issues WHERE status")
	qLoadDeps       = regexp.QuoteMeta("SELECT from_id, to_id, type, metadata FROM dependencies")
	qLockIssue      = regexp.QuoteMeta("SELECT id FROM issues WHERE id = ? FOR UPDATE")
	qInsertDep      = regexp.QuoteMeta("INSERT INTO dependencies")
	qInsertEvent    = regexp.QuoteMeta("INSERT INTO events")
	qCountDep       = regexp.QuoteMeta("SELECT COUNT(*) FROM dependencies WHERE from_id = ? AND to_id = ?")
	qDeleteDep      = regexp.QuoteMeta("DELETE FROM dependencies WHERE from_id = ? AND to_id = ?")
)

func newTestCmd() (*cobra.Command, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(context.Background())
	return cmd, buf
}

func wrapStore(client *dolt.Client) *dolt.Store {
	var s dolt.Store = client
	return &s
}

func newTestDeps(store *dolt.Store, outputJSON bool) *cmddeps.Deps {
	actor := "test-actor"
	model := "test-model"
	return &cmddeps.Deps{
		Store:      store,
		Actor:      &actor,
		AgentModel: &model,
		OutputJSON: &outputJSON,
	}
}

func issueCols() []string {
	return []string{"id", "title", "issue_type", "status", "priority", "created_at", "await_type", "await_id", "ephemeral", "metadata"}
}

func depCols() []string {
	return []string{"from_id", "to_id", "type", "metadata"}
}

// resetDepFlags resets package-level flag variables used by add/removeDependency.
func resetDepFlags() {
	depType = "blocks"
	depRemove = false
}

// mockAddHappyPath sets up all sqlmock expectations for a successful add dependency.
func mockAddHappyPath(mock sqlmock.Sqlmock, fromID, toID string) {
	// validateIssuesExist
	mock.ExpectQuery(qValidateIssues).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(fromID).AddRow(toID))

	// LoadGraphFromDB
	mock.ExpectQuery(qLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow(fromID, "Task From", "task", "open", 1, time.Now(), nil, nil, 0, nil).
			AddRow(toID, "Task To", "task", "open", 2, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()))

	// WithAuditedTx
	mock.ExpectBegin()
	// Locks in sorted order
	ids := []string{fromID, toID}
	if ids[0] > ids[1] {
		ids[0], ids[1] = ids[1], ids[0]
	}
	mock.ExpectQuery(qLockIssue).WithArgs(ids[0]).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(ids[0]))
	mock.ExpectQuery(qLockIssue).WithArgs(ids[1]).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(ids[1]))
	mock.ExpectExec(qInsertDep).
		WithArgs(fromID, toID, "blocks", "test-actor", "test-actor", "test-model").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(qInsertEvent).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
}

// --- AC#1: Add Dependency Happy Path ---

func TestAddDependency_HappyPath(t *testing.T) {
	resetDepFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := wrapStore(dolt.NewClientFromDB(db))
	deps := newTestDeps(store, false)
	cmd, buf := newTestCmd()

	mockAddHappyPath(mock, "issue-1", "issue-2")

	err = addDependency(cmd, deps, "issue-1", "issue-2")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "🔗 Dependency created: issue-1 -[blocks]-> issue-2")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#1: Add Dependency JSON Output ---

func TestAddDependency_JSONOutput(t *testing.T) {
	resetDepFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := wrapStore(dolt.NewClientFromDB(db))
	deps := newTestDeps(store, true)
	cmd, buf := newTestCmd()

	mockAddHappyPath(mock, "issue-1", "issue-2")

	err = addDependency(cmd, deps, "issue-1", "issue-2")
	require.NoError(t, err)

	var result map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, "issue-1", result["from_id"])
	assert.Equal(t, "issue-2", result["to_id"])
	assert.Equal(t, "blocks", result["type"])
	assert.Equal(t, "created", result["status"])
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#2: Remove Dependency Happy Path ---

func TestRemoveDependency_HappyPath(t *testing.T) {
	resetDepFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := wrapStore(dolt.NewClientFromDB(db))
	deps := newTestDeps(store, false)
	cmd, buf := newTestCmd()

	mock.ExpectQuery(qValidateIssues).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("issue-1").AddRow("issue-2"))
	mock.ExpectBegin()
	mock.ExpectQuery(qLockIssue).WithArgs("issue-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("issue-1"))
	mock.ExpectQuery(qLockIssue).WithArgs("issue-2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("issue-2"))
	mock.ExpectQuery(qCountDep).WithArgs("issue-1", "issue-2").
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	mock.ExpectExec(qDeleteDep).WithArgs("issue-1", "issue-2").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(qInsertEvent).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = removeDependency(cmd, deps, "issue-1", "issue-2")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "🔗 Dependency removed: issue-1 -/-> issue-2")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#2: Remove Dependency JSON Output ---

func TestRemoveDependency_JSONOutput(t *testing.T) {
	resetDepFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := wrapStore(dolt.NewClientFromDB(db))
	deps := newTestDeps(store, true)
	cmd, buf := newTestCmd()

	mock.ExpectQuery(qValidateIssues).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("issue-1").AddRow("issue-2"))
	mock.ExpectBegin()
	mock.ExpectQuery(qLockIssue).WithArgs("issue-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("issue-1"))
	mock.ExpectQuery(qLockIssue).WithArgs("issue-2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("issue-2"))
	mock.ExpectQuery(qCountDep).WithArgs("issue-1", "issue-2").
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	mock.ExpectExec(qDeleteDep).WithArgs("issue-1", "issue-2").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(qInsertEvent).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = removeDependency(cmd, deps, "issue-1", "issue-2")
	require.NoError(t, err)

	var result map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, "issue-1", result["from_id"])
	assert.Equal(t, "issue-2", result["to_id"])
	assert.Equal(t, "removed", result["status"])
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#3: Circular Dependency Rejection ---

func TestAddDependency_CircularRejection(t *testing.T) {
	resetDepFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := wrapStore(dolt.NewClientFromDB(db))
	deps := newTestDeps(store, false)
	cmd, _ := newTestCmd()

	// B -> A already exists. Now trying A -> B should fail.
	mock.ExpectQuery(qValidateIssues).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("A").AddRow("B"))

	// LoadGraphFromDB: 2 issues, 1 dep (B -> A)
	mock.ExpectQuery(qLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("A", "Task A", "task", "open", 1, time.Now(), nil, nil, 0, nil).
			AddRow("B", "Task B", "task", "open", 1, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()).AddRow("B", "A", "blocks", []byte("{}")))

	err = addDependency(cmd, deps, "A", "B")
	require.Error(t, err)

	var gerr *gravaerrors.GravaError
	require.ErrorAs(t, err, &gerr)
	assert.Equal(t, "CIRCULAR_DEPENDENCY", gerr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#4: Non-existent Issue ---

func TestAddDependency_NonExistentIssue(t *testing.T) {
	resetDepFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := wrapStore(dolt.NewClientFromDB(db))
	deps := newTestDeps(store, false)
	cmd, _ := newTestCmd()

	// Only "someID" exists, "nonexistent" does not
	mock.ExpectQuery(qValidateIssues).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("someID"))

	err = addDependency(cmd, deps, "nonexistent", "someID")
	require.Error(t, err)

	var gerr *gravaerrors.GravaError
	require.ErrorAs(t, err, &gerr)
	assert.Equal(t, "ISSUE_NOT_FOUND", gerr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#5: Self-loop Rejection ---

func TestAddDependency_SelfLoop(t *testing.T) {
	resetDepFlags()
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := wrapStore(dolt.NewClientFromDB(db))
	deps := newTestDeps(store, false)
	cmd, _ := newTestCmd()

	err = addDependency(cmd, deps, "abc123", "abc123")
	require.Error(t, err)

	var gerr *gravaerrors.GravaError
	require.ErrorAs(t, err, &gerr)
	assert.Equal(t, "SELF_LOOP", gerr.Code)
}

// --- AC#6: Remove Non-existent Dependency ---

func TestRemoveDependency_NonExistent(t *testing.T) {
	resetDepFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := wrapStore(dolt.NewClientFromDB(db))
	deps := newTestDeps(store, false)
	cmd, _ := newTestCmd()

	mock.ExpectQuery(qValidateIssues).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("A").AddRow("B"))
	mock.ExpectBegin()
	mock.ExpectQuery(qLockIssue).WithArgs("A").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("A"))
	mock.ExpectQuery(qLockIssue).WithArgs("B").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("B"))
	mock.ExpectQuery(qCountDep).WithArgs("A", "B").
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))
	mock.ExpectRollback()

	err = removeDependency(cmd, deps, "A", "B")
	require.Error(t, err)

	var gerr *gravaerrors.GravaError
	require.ErrorAs(t, err, &gerr)
	assert.Equal(t, "DEPENDENCY_NOT_FOUND", gerr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- Integration: dep --remove via cobra command ---

func TestDepCmd_RemoveFlag(t *testing.T) {
	resetDepFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := wrapStore(dolt.NewClientFromDB(db))
	deps := newTestDeps(store, false)

	mock.ExpectQuery(qValidateIssues).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("A").AddRow("B"))
	mock.ExpectBegin()
	mock.ExpectQuery(qLockIssue).WithArgs("A").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("A"))
	mock.ExpectQuery(qLockIssue).WithArgs("B").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("B"))
	mock.ExpectQuery(qCountDep).WithArgs("A", "B").
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	mock.ExpectExec(qDeleteDep).WithArgs("A", "B").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(qInsertEvent).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	buf := &bytes.Buffer{}
	depCmd := newDepCmd(deps)
	depCmd.SetOut(buf)
	depCmd.SetArgs([]string{"--remove", "A", "B"})

	_, err = depCmd.ExecuteC()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "🔗 Dependency removed: A -/-> B")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- Integration: dep add via cobra command (default path) ---

func TestDepCmd_AddDefault(t *testing.T) {
	resetDepFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := wrapStore(dolt.NewClientFromDB(db))
	deps := newTestDeps(store, false)

	mockAddHappyPath(mock, "X-1", "Y-2")

	buf := &bytes.Buffer{}
	depCmd := newDepCmd(deps)
	depCmd.SetOut(buf)
	depCmd.SetArgs([]string{"X-1", "Y-2"})

	_, err = depCmd.ExecuteC()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), fmt.Sprintf("🔗 Dependency created: %s -[blocks]-> %s", "X-1", "Y-2"))
	require.NoError(t, mock.ExpectationsWereMet())
}
