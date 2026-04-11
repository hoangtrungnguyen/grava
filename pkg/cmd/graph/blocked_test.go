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
