package issues

import (
	"bytes"
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test list command with no filters
func TestListCmd_NoFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	expectedQuery := regexp.QuoteMeta(`SELECT id, title, issue_type, priority, status, created_at FROM issues WHERE ephemeral = 0 AND status != 'tombstone' AND status != 'archived' ORDER BY priority ASC, created_at DESC, id ASC`)
	mock.ExpectQuery(expectedQuery).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}).
			AddRow("task-1", "Task 1", "task", 1, "open", time.Now()))

	buf := &bytes.Buffer{}
	listCmd := newListCmd(deps)
	listCmd.SetOut(buf)

	err = listCmd.ExecuteContext(context.Background())
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "task-1")
	require.NoError(t, mock.ExpectationsWereMet())
}

// Test list command with priority filter
func TestListCmd_FilterByPriority(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	expectedQuery := regexp.QuoteMeta(`SELECT id, title, issue_type, priority, status, created_at FROM issues WHERE ephemeral = 0 AND status != 'tombstone' AND status != 'archived' AND priority = ? ORDER BY priority ASC, created_at DESC, id ASC`)
	mock.ExpectQuery(expectedQuery).
		WithArgs(0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}).
			AddRow("crit-1", "Critical Task", "task", 0, "open", time.Now()))

	buf := &bytes.Buffer{}
	listCmd := newListCmd(deps)
	listCmd.SetOut(buf)
	listCmd.SetArgs([]string{"--priority", "0"})

	err = listCmd.ExecuteContext(context.Background())
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "crit-1")
	require.NoError(t, mock.ExpectationsWereMet())
}

// Test list command with assignee filter
func TestListCmd_FilterByAssignee(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	expectedQuery := regexp.QuoteMeta(`SELECT id, title, issue_type, priority, status, created_at FROM issues WHERE ephemeral = 0 AND status != 'tombstone' AND status != 'archived' AND assignee = ? ORDER BY priority ASC, created_at DESC, id ASC`)
	mock.ExpectQuery(expectedQuery).
		WithArgs("agent-01").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}).
			AddRow("task-assigned", "Assigned Task", "task", 2, "in_progress", time.Now()))

	buf := &bytes.Buffer{}
	listCmd := newListCmd(deps)
	listCmd.SetOut(buf)
	listCmd.SetArgs([]string{"--assignee", "agent-01"})

	err = listCmd.ExecuteContext(context.Background())
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "task-assigned")
	require.NoError(t, mock.ExpectationsWereMet())
}

// Test list command with combined filters (AND logic)
func TestListCmd_MultipleFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	expectedQuery := regexp.QuoteMeta(`SELECT id, title, issue_type, priority, status, created_at FROM issues WHERE ephemeral = 0 AND status != 'tombstone' AND status != 'archived' AND status = ? AND priority = ? AND assignee = ? ORDER BY priority ASC, created_at DESC, id ASC`)
	mock.ExpectQuery(expectedQuery).
		WithArgs("in_progress", 1, "agent-01").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}).
			AddRow("task-combo", "Combined Filter Task", "task", 1, "in_progress", time.Now()))

	buf := &bytes.Buffer{}
	listCmd := newListCmd(deps)
	listCmd.SetOut(buf)
	listCmd.SetArgs([]string{"--status", "in_progress", "--priority", "1", "--assignee", "agent-01"})

	err = listCmd.ExecuteContext(context.Background())
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "task-combo")
	require.NoError(t, mock.ExpectationsWereMet())
}

// Test list with empty results
func TestListCmd_EmptyResults(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	expectedQuery := regexp.QuoteMeta(`SELECT id, title, issue_type, priority, status, created_at FROM issues WHERE ephemeral = 0 AND status != 'tombstone' AND status != 'archived' AND priority = ? ORDER BY priority ASC, created_at DESC, id ASC`)
	mock.ExpectQuery(expectedQuery).
		WithArgs(99).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}))

	buf := &bytes.Buffer{}
	listCmd := newListCmd(deps)
	listCmd.SetOut(buf)
	listCmd.SetArgs([]string{"--priority", "99"})

	err = listCmd.ExecuteContext(context.Background())
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
