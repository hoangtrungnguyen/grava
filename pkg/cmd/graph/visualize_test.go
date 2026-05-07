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
	"github.com/hoangtrungnguyen/grava/pkg/graph"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	qVisualizeLoadIssues = regexp.QuoteMeta("SELECT id, title, issue_type, status, priority, created_at, updated_at, await_type, await_id, ephemeral, metadata FROM issues WHERE status")
	qVisualizeLoadDeps   = regexp.QuoteMeta("SELECT from_id, to_id, type, metadata FROM dependencies")
)

func TestGraphVisualizeCmd_DefaultFormatASCII(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	// Load two tasks with a dependency
	mock.ExpectQuery(qVisualizeLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-1", "First Task", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("task-2", "Second Task", "task", "open", 2, time.Now(), time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qVisualizeLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()).
			AddRow("task-1", "task-2", "blocks", []byte("{}")))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	vcmd := newGraphVisualizeCmd(deps)
	vcmd.SetOut(buf)
	err = vcmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "First Task")
	assert.Contains(t, output, "Second Task")
	assert.Contains(t, output, "blocks")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGraphVisualizeCmd_FormatDOT(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	mock.ExpectQuery(qVisualizeLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-1", "Task A", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qVisualizeLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	vcmd := newGraphVisualizeCmd(deps)
	vcmd.SetOut(buf)
	vcmd.ParseFlags([]string{"--format", "dot"})
	err = vcmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "digraph G")
	assert.Contains(t, output, "Task A")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGraphVisualizeCmd_FormatJSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	mock.ExpectQuery(qVisualizeLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-1", "Task One", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("task-2", "Task Two", "task", "open", 2, time.Now(), time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qVisualizeLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()).
			AddRow("task-1", "task-2", "blocks", []byte("{}")))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	vcmd := newGraphVisualizeCmd(deps)
	vcmd.SetOut(buf)
	vcmd.ParseFlags([]string{"--format", "json"})
	err = vcmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	var result graph.GraphJSON
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Equal(t, 2, len(result.Nodes))
	assert.Equal(t, 1, len(result.Edges))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGraphVisualizeCmd_RootFlag(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	// Create chain: task-1 -> task-2 -> task-3, plus unrelated task-4
	mock.ExpectQuery(qVisualizeLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-1", "Root", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("task-2", "Child", "task", "open", 2, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("task-3", "Grandchild", "task", "open", 3, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("task-4", "Unrelated", "task", "open", 4, time.Now(), time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qVisualizeLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()).
			AddRow("task-1", "task-2", "blocks", []byte("{}")).
			AddRow("task-2", "task-3", "blocks", []byte("{}")))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	vcmd := newGraphVisualizeCmd(deps)
	vcmd.SetOut(buf)
	vcmd.ParseFlags([]string{"--root", "task-1"})
	err = vcmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Root")
	assert.Contains(t, output, "Child")
	assert.Contains(t, output, "Grandchild")
	assert.NotContains(t, output, "Unrelated")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGraphVisualizeCmd_InvalidFormat(t *testing.T) {
	// Format validation fires before LoadGraphFromDB — no DB connection needed.
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	vcmd := newGraphVisualizeCmd(deps)
	vcmd.SetOut(buf)
	vcmd.ParseFlags([]string{"--format", "invalid"})
	err = vcmd.RunE(cmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestGraphVisualizeCmd_IncludesIsolatedNodes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	// Create one task with no dependencies
	mock.ExpectQuery(qVisualizeLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-1", "Isolated Task", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qVisualizeLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	vcmd := newGraphVisualizeCmd(deps)
	vcmd.SetOut(buf)
	err = vcmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Isolated Task")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGraphVisualizeCmd_ExcludesArchivedNodes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	// Create one active and one archived task
	mock.ExpectQuery(qVisualizeLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("task-1", "Active", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("task-2", "Archived", "task", "archived", 2, time.Now(), time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qVisualizeLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	vcmd := newGraphVisualizeCmd(deps)
	vcmd.SetOut(buf)
	err = vcmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Active")
	assert.NotContains(t, output, "Archived")
	require.NoError(t, mock.ExpectationsWereMet())
}
