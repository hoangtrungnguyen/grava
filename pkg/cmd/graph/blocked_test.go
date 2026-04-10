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

// Query patterns for blocked command tests.
var (
	qBlockedLoadIssues = regexp.QuoteMeta("SELECT id, title, issue_type, status, priority, created_at, await_type, await_id, ephemeral, metadata FROM issues WHERE status")
	qBlockedLoadDeps   = regexp.QuoteMeta("SELECT from_id, to_id, type, metadata FROM dependencies")
	qBlockedAssignee   = "SELECT id, COALESCE"
)

// BlockedTaskOutput matches the JSON schema for --json output (AC#3).
type BlockedTaskOutput struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Assignee string `json:"assignee"`
}

// newBlockedTestCmd creates a blocked command and sets flags AFTER creation to avoid default resets.
func newBlockedTestCmd(deps *cmddeps.Deps, all, recursive bool, depth int) *cobra.Command {
	rcmd := newBlockedCmd(deps)
	blockedAll = all
	blockedRecursive = recursive
	blockedDepth = depth
	return rcmd
}

// --- AC#1: Query Blockers Happy Path ---

func TestBlockedCmd_DirectBlocker(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	mock.ExpectQuery(qBlockedLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-A", "Blocker", "task", "open", 1, time.Now(), nil, nil, 0, nil).
			AddRow("task-B", "Blocked Task", "task", "open", 2, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qBlockedLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()).
			AddRow("task-A", "task-B", "blocks", []byte("{}")))

	mock.ExpectQuery(qBlockedAssignee).
		WithArgs("task-A").
		WillReturnRows(sqlmock.NewRows([]string{"id", "assignee"}).
			AddRow("task-A", "agent-x"))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	rcmd := newBlockedTestCmd(deps, false, false, 1)
	err = rcmd.RunE(cmd, []string{"task-B"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "task-A")
	assert.Contains(t, buf.String(), "Blocker")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#1: JSON output includes assignee ---

func TestBlockedCmd_JSONWithAssignee(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := true
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	mock.ExpectQuery(qBlockedLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-A", "Blocker", "task", "open", 1, time.Now(), nil, nil, 0, nil).
			AddRow("task-B", "Blocked", "task", "open", 2, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qBlockedLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()).
			AddRow("task-A", "task-B", "blocks", []byte("{}")))

	mock.ExpectQuery(qBlockedAssignee).
		WithArgs("task-A").
		WillReturnRows(sqlmock.NewRows([]string{"id", "assignee"}).
			AddRow("task-A", "agent-y"))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	rcmd := newBlockedTestCmd(deps, false, false, 1)
	err = rcmd.RunE(cmd, []string{"task-B"})
	require.NoError(t, err)

	var result []BlockedTaskOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	require.Len(t, result, 1)
	assert.Equal(t, "task-A", result[0].ID)
	assert.Equal(t, "Blocker", result[0].Title)
	assert.Equal(t, "open", result[0].Status)
	assert.Equal(t, "agent-y", result[0].Assignee)
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#2: Include Done Blockers with --all ---

func TestBlockedCmd_AllIncludesDoneBlockers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := true
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	mock.ExpectQuery(qBlockedLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-A", "Done Blocker", "task", "closed", 1, time.Now(), nil, nil, 0, nil).
			AddRow("task-B", "Blocked", "task", "open", 2, time.Now(), nil, nil, 0, nil).
			AddRow("task-C", "Open Blocker", "task", "open", 3, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qBlockedLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()).
			AddRow("task-A", "task-B", "blocks", []byte("{}")).
			AddRow("task-C", "task-B", "blocks", []byte("{}")))

	// Assignee for both blockers (order may vary due to map iteration)
	mock.ExpectQuery(qBlockedAssignee).
		WillReturnRows(sqlmock.NewRows([]string{"id", "assignee"}).
			AddRow("task-A", "agent-done").
			AddRow("task-C", "agent-open"))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	rcmd := newBlockedTestCmd(deps, true, false, 1) // --all=true
	err = rcmd.RunE(cmd, []string{"task-B"})
	require.NoError(t, err)

	var result []BlockedTaskOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	require.Len(t, result, 2)
	ids := map[string]bool{}
	for _, r := range result {
		ids[r.ID] = true
	}
	assert.True(t, ids["task-A"], "should include done blocker with --all")
	assert.True(t, ids["task-C"], "should include open blocker")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#2: Without --all, done blockers are excluded ---

func TestBlockedCmd_DefaultExcludesDoneBlockers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := true
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	mock.ExpectQuery(qBlockedLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-A", "Done Blocker", "task", "closed", 1, time.Now(), nil, nil, 0, nil).
			AddRow("task-B", "Blocked", "task", "open", 2, time.Now(), nil, nil, 0, nil).
			AddRow("task-C", "Open Blocker", "task", "open", 3, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qBlockedLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()).
			AddRow("task-A", "task-B", "blocks", []byte("{}")).
			AddRow("task-C", "task-B", "blocks", []byte("{}")))

	mock.ExpectQuery(qBlockedAssignee).
		WithArgs("task-C").
		WillReturnRows(sqlmock.NewRows([]string{"id", "assignee"}).
			AddRow("task-C", ""))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	rcmd := newBlockedTestCmd(deps, false, false, 1) // --all=false
	err = rcmd.RunE(cmd, []string{"task-B"})
	require.NoError(t, err)

	var result []BlockedTaskOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	require.Len(t, result, 1)
	assert.Equal(t, "task-C", result[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#4: Non-existent Issue ---

func TestBlockedCmd_IssueNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	mock.ExpectQuery(qBlockedLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-A", "Some Task", "task", "open", 1, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qBlockedLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	rcmd := newBlockedTestCmd(deps, false, false, 1)
	err = rcmd.RunE(cmd, []string{"nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "not found")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#5: Recursive Blockers ---

func TestBlockedCmd_RecursiveBlockers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := true
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	// A blocks B, B blocks C. Recursive on C should return both A and B.
	mock.ExpectQuery(qBlockedLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-A", "Root Blocker", "task", "open", 1, time.Now(), nil, nil, 0, nil).
			AddRow("task-B", "Mid Blocker", "task", "open", 2, time.Now(), nil, nil, 0, nil).
			AddRow("task-C", "Target", "task", "open", 3, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qBlockedLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()).
			AddRow("task-A", "task-B", "blocks", []byte("{}")).
			AddRow("task-B", "task-C", "blocks", []byte("{}")))

	// Assignee for both blockers (order may vary)
	mock.ExpectQuery(qBlockedAssignee).
		WillReturnRows(sqlmock.NewRows([]string{"id", "assignee"}).
			AddRow("task-B", "").
			AddRow("task-A", ""))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	rcmd := newBlockedTestCmd(deps, false, true, 10) // --recursive=true
	err = rcmd.RunE(cmd, []string{"task-C"})
	require.NoError(t, err)

	var result []BlockedTaskOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	require.Len(t, result, 2)
	ids := map[string]bool{}
	for _, r := range result {
		ids[r.ID] = true
	}
	assert.True(t, ids["task-B"], "should include direct blocker")
	assert.True(t, ids["task-A"], "should include transitive blocker with --recursive")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- No blockers case ---

func TestBlockedCmd_NoBlockers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	mock.ExpectQuery(qBlockedLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-A", "Unblocked", "task", "open", 1, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qBlockedLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()))

	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetContext(context.Background())

	rcmd := newBlockedTestCmd(deps, false, false, 1)
	err = rcmd.RunE(cmd, []string{"task-A"})
	require.NoError(t, err)
	assert.Contains(t, errBuf.String(), "No blockers found")
	require.NoError(t, mock.ExpectationsWereMet())
}
