package cmdgraph

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
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Shared query patterns for ready tests.
var (
	qReadyLoadIssues = regexp.QuoteMeta("SELECT id, title, issue_type, status, priority, created_at, await_type, await_id, ephemeral, metadata FROM issues WHERE status")
	qReadyLoadDeps   = regexp.QuoteMeta("SELECT from_id, to_id, type, metadata FROM dependencies")
	qReadyAssignee   = "SELECT id, COALESCE"
)

func TestReadyQueue_EmptyDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectQuery(qReadyLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()))
	mock.ExpectQuery(qReadyLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()))

	store := dolt.NewClientFromDB(db)
	tasks, err := readyQueue(context.Background(), store, 20)
	require.NoError(t, err)
	assert.NotNil(t, tasks)
	assert.Empty(t, tasks)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReadyQueue_LimitZero(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectQuery(qReadyLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()))
	mock.ExpectQuery(qReadyLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()))

	store := dolt.NewClientFromDB(db)
	tasks, err := readyQueue(context.Background(), store, 0)
	require.NoError(t, err)
	assert.Empty(t, tasks)
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- Task 3.2: Independent tasks are ready ---

func TestReadyCmd_IndependentTasks(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	// LoadGraphFromDB: 2 open tasks, 0 deps
	mock.ExpectQuery(qReadyLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-1", "Task 1", "task", "open", 1, time.Now(), nil, nil, 0, nil).
			AddRow("task-2", "Task 2", "task", "open", 2, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qReadyLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()))

	// Assignee query
	mock.ExpectQuery(qReadyAssignee).
		WithArgs("task-1", "task-2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "assignee"}).
			AddRow("task-1", "agent-a").
			AddRow("task-2", ""))

	readyLimit = 20
	readyPriority = -1
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetContext(context.Background())

	rcmd := newReadyCmd(deps)
	rcmd.SetOut(buf)
	rcmd.SetErr(errBuf)

	err = rcmd.RunE(cmd, []string{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "task-1")
	assert.Contains(t, buf.String(), "task-2")
	assert.Empty(t, errBuf.String()) // not empty state
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- Task 3.3: Tasks blocked by 'done' tasks are ready ---

func TestReadyCmd_BlockedByDoneTask(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	// task-1 is done, task-2 is open. task-1 blocks task-2. Since task-1 is done, task-2 is ready.
	mock.ExpectQuery(qReadyLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-1", "Done Task", "task", "closed", 1, time.Now(), nil, nil, 0, nil).
			AddRow("task-2", "Ready Task", "task", "open", 2, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qReadyLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()).AddRow("task-1", "task-2", "blocks", []byte("{}")))

	// Assignee query
	mock.ExpectQuery(qReadyAssignee).
		WithArgs("task-2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "assignee"}).AddRow("task-2", "agent-b"))

	readyLimit = 20
	readyPriority = -1

	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetContext(context.Background())

	rcmd := newReadyCmd(deps)
	err = rcmd.RunE(cmd, []string{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "task-2")
	assert.NotContains(t, buf.String(), "task-1")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- Task 3.4: Tasks blocked by 'open' tasks are NOT ready ---

func TestReadyCmd_BlockedByOpenTask(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := true
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	// task-1 is open, task-2 is open. task-1 blocks task-2. task-2 is NOT ready.
	mock.ExpectQuery(qReadyLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-1", "Blocker", "task", "open", 1, time.Now(), nil, nil, 0, nil).
			AddRow("task-2", "Blocked", "task", "open", 2, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qReadyLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()).AddRow("task-1", "task-2", "blocks", []byte("{}")))

	// Only task-1 should be ready (it has no blockers)
	mock.ExpectQuery(qReadyAssignee).
		WithArgs("task-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "assignee"}).AddRow("task-1", ""))

	readyLimit = 20
	readyPriority = -1

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	rcmd := newReadyCmd(deps)
	err = rcmd.RunE(cmd, []string{})
	require.NoError(t, err)

	// JSON should contain only task-1
	var result []ReadyTaskOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	require.Len(t, result, 1)
	assert.Equal(t, "task-1", result[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#3: Empty state with --json returns [] ---

func TestReadyCmd_EmptyStateJSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := true
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	// All tasks are blocked
	mock.ExpectQuery(qReadyLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-1", "Open", "task", "open", 1, time.Now(), nil, nil, 0, nil).
			AddRow("task-2", "Open", "task", "open", 1, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qReadyLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()).
			AddRow("task-1", "task-2", "blocks", []byte("{}")).
			AddRow("task-2", "task-1", "blocks", []byte("{}")))

	readyLimit = 20
	readyPriority = -1

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	rcmd := newReadyCmd(deps)
	err = rcmd.RunE(cmd, []string{})
	require.NoError(t, err)

	// JSON should be empty array []
	var result []ReadyTaskOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Empty(t, result)
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#3: Empty state human-readable shows message on stderr ---

func TestReadyCmd_EmptyStateHuman(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	// Empty DB
	mock.ExpectQuery(qReadyLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()))
	mock.ExpectQuery(qReadyLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()))

	readyLimit = 20
	readyPriority = -1

	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetContext(context.Background())

	rcmd := newReadyCmd(deps)
	err = rcmd.RunE(cmd, []string{})
	require.NoError(t, err)
	assert.Contains(t, errBuf.String(), "No tasks are currently ready")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#2: JSON output includes assignee field ---

func TestReadyCmd_JSONIncludesAssignee(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := true
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	mock.ExpectQuery(qReadyLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-1", "My Task", "task", "open", 1, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qReadyLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()))

	mock.ExpectQuery(qReadyAssignee).
		WithArgs("task-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "assignee"}).AddRow("task-1", "agent-x"))

	readyLimit = 20
	readyPriority = -1

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	rcmd := newReadyCmd(deps)
	err = rcmd.RunE(cmd, []string{})
	require.NoError(t, err)

	var result []ReadyTaskOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	require.Len(t, result, 1)
	assert.Equal(t, "task-1", result[0].ID)
	assert.Equal(t, "My Task", result[0].Title)
	assert.Equal(t, "open", result[0].Status)
	assert.Equal(t, 1, result[0].Priority)
	assert.Equal(t, "agent-x", result[0].Assignee)
	require.NoError(t, mock.ExpectationsWereMet())
}
