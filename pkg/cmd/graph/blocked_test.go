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
	qBlockedLoadIssues = regexp.QuoteMeta("SELECT id, title, issue_type, status, priority, created_at, await_type, await_id, ephemeral, metadata FROM issues")
	qBlockedLoadDeps   = regexp.QuoteMeta("SELECT from_id, to_id, type, metadata FROM dependencies")
)

// BlockedInfo matches the JSON schema for --json output.
type BlockedInfo struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Blockers    []string `json:"blockers"`
	GateBlocked bool     `json:"gate_blocked"`
	AwaitType   string   `json:"await_type,omitempty"`
	Ephemeral   bool     `json:"ephemeral"`
}

// issueCols returns the column names for issues table mock.
func issueCols() []string {
	return []string{"id", "title", "issue_type", "status", "priority", "created_at", "await_type", "await_id", "ephemeral", "metadata"}
}

// depCols returns the column names for dependencies table mock.
func depCols() []string {
	return []string{"from_id", "to_id", "type", "metadata"}
}

// --- AC#1: Show blocked tasks ---

func TestBlockedCmd_ShowsBlockedTasks(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	// Load issues
	mock.ExpectQuery(qBlockedLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-A", "Open Task", "task", "open", 1, time.Now(), nil, nil, 0, nil).
			AddRow("task-B", "Blocked by A", "task", "open", 2, time.Now(), nil, nil, 0, nil))

	// Load dependencies - task-A blocks task-B
	mock.ExpectQuery(qBlockedLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()).
			AddRow("task-A", "task-B", "blocks", []byte("{}")))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	rcmd := newBlockedCmd(deps)
	err = rcmd.RunE(cmd, []string{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "task-B", "should show blocked task")
	assert.Contains(t, buf.String(), "task-A", "should show blocker")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#2: JSON output includes all fields ---

func TestBlockedCmd_JSONOutput(t *testing.T) {
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

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	rcmd := newBlockedCmd(deps)
	err = rcmd.RunE(cmd, []string{})
	require.NoError(t, err)

	var result []BlockedInfo
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	require.Len(t, result, 1)
	assert.Equal(t, "task-B", result[0].ID)
	assert.Equal(t, "Blocked", result[0].Title)
	assert.Contains(t, result[0].Blockers, "task-A")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#3: Multiple blocked tasks ---

func TestBlockedCmd_MultipleBlockedTasks(t *testing.T) {
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
			AddRow("task-B", "Blocked 1", "task", "open", 2, time.Now(), nil, nil, 0, nil).
			AddRow("task-C", "Blocked 2", "task", "open", 3, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qBlockedLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()).
			AddRow("task-A", "task-B", "blocks", []byte("{}")).
			AddRow("task-A", "task-C", "blocks", []byte("{}")))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	rcmd := newBlockedCmd(deps)
	err = rcmd.RunE(cmd, []string{})
	require.NoError(t, err)

	var result []BlockedInfo
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	require.Len(t, result, 2)
	ids := map[string]bool{}
	for _, r := range result {
		ids[r.ID] = true
	}
	assert.True(t, ids["task-B"])
	assert.True(t, ids["task-C"])
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#4: No blockers case ---

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
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	rcmd := newBlockedCmd(deps)
	err = rcmd.RunE(cmd, []string{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No blocked tasks found")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- AC#5: Includes ephemeral tasks ---

func TestBlockedCmd_ShowsEphemeralTasks(t *testing.T) {
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
			AddRow("task-B", "Ephemeral Blocked", "task", "open", 2, time.Now(), nil, nil, 1, nil))
	mock.ExpectQuery(qBlockedLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()).
			AddRow("task-A", "task-B", "blocks", []byte("{}")))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	rcmd := newBlockedCmd(deps)
	err = rcmd.RunE(cmd, []string{})
	require.NoError(t, err)

	var result []BlockedInfo
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	require.Len(t, result, 1)
	assert.True(t, result[0].Ephemeral)
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- Per-issue blocker queries (Story 4.3) ---

var qBlockersForIssue = regexp.QuoteMeta(
	`SELECT DISTINCT i.id, i.title, i.status, COALESCE(i.assignee, '') as assignee
		FROM issues i
		INNER JOIN dependencies dep ON
			(dep.from_id = i.id AND dep.to_id = ? AND dep.type = 'blocks')
			OR (dep.to_id = i.id AND dep.from_id = ? AND dep.type = 'blocked-by')
		ORDER BY i.priority ASC`,
)

var qIssueExists = regexp.QuoteMeta("SELECT COUNT(*) FROM issues WHERE id = ?")

func TestBlockedCmd_PerIssue_ShowsBlockers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	mock.ExpectQuery(qIssueExists).
		WithArgs("task-B").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(qBlockersForIssue).
		WithArgs("task-B", "task-B").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "status", "assignee"}).
			AddRow("task-A", "Blocker Task", "open", "alice").
			AddRow("task-C", "Another Blocker", "in_progress", "bob"))

	buf := &bytes.Buffer{}
	cmd := newBlockedCmd(deps)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"task-B"})
	err = cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "task-A")
	assert.Contains(t, out, "Blocker Task")
	assert.Contains(t, out, "alice")
	assert.Contains(t, out, "task-C")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBlockedCmd_PerIssue_JSONOutput(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := true
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	mock.ExpectQuery(qIssueExists).
		WithArgs("task-B").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(qBlockersForIssue).
		WithArgs("task-B", "task-B").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "status", "assignee"}).
			AddRow("task-A", "Blocker", "open", "alice"))

	buf := &bytes.Buffer{}
	cmd := newBlockedCmd(deps)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"task-B"})
	err = cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	var result []BlockerItem
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	require.Len(t, result, 1)
	assert.Equal(t, "task-A", result[0].ID)
	assert.Equal(t, "Blocker", result[0].Title)
	assert.Equal(t, "open", result[0].Status)
	assert.Equal(t, "alice", result[0].Assignee)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBlockedCmd_PerIssue_NoBlockers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := true
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	mock.ExpectQuery(qIssueExists).
		WithArgs("task-A").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(qBlockersForIssue).
		WithArgs("task-A", "task-A").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "status", "assignee"}))

	buf := &bytes.Buffer{}
	cmd := newBlockedCmd(deps)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"task-A"})
	err = cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	var result []BlockerItem
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Len(t, result, 0)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBlockedCmd_PerIssue_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	mock.ExpectQuery(qIssueExists).
		WithArgs("nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	buf := &bytes.Buffer{}
	cmd := newBlockedCmd(deps)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"nonexistent"})
	err = cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ISSUE_NOT_FOUND")
	require.NoError(t, mock.ExpectationsWereMet())
}
