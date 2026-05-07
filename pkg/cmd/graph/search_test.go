package cmdgraph

import (
	"bytes"
	"context"
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

// Test search across title field
func TestSearchCmd_SearchTitle(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	expectedQuery := regexp.QuoteMeta(`SELECT DISTINCT i.id, i.title, i.issue_type, i.priority, i.status, i.created_at
		        FROM issues i
		        LEFT JOIN issue_comments c ON i.id = c.issue_id
		        WHERE i.ephemeral = ?
		          AND i.status != 'tombstone'
		          AND i.status != 'archived'
		          AND (i.title LIKE ? OR i.description LIKE ? OR COALESCE(i.metadata,'') LIKE ? OR COALESCE(c.message,'') LIKE ?)
		        ORDER BY i.priority ASC, i.created_at DESC`)
	mock.ExpectQuery(expectedQuery).
		WithArgs(0, "%login%", "%login%", "%login%", "%login%").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}).
			AddRow("bug-1", "Fix login bug", "bug", 0, "open", time.Now()))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	searchCmd := newSearchCmd(deps)
	searchCmd.SetOut(buf)
	err = searchCmd.RunE(cmd, []string{"login"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Fix login bug")
	require.NoError(t, mock.ExpectationsWereMet())
}

// Test search across comments
func TestSearchCmd_SearchComments(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	expectedQuery := regexp.QuoteMeta(`SELECT DISTINCT i.id, i.title, i.issue_type, i.priority, i.status, i.created_at
		        FROM issues i
		        LEFT JOIN issue_comments c ON i.id = c.issue_id
		        WHERE i.ephemeral = ?
		          AND i.status != 'tombstone'
		          AND i.status != 'archived'
		          AND (i.title LIKE ? OR i.description LIKE ? OR COALESCE(i.metadata,'') LIKE ? OR COALESCE(c.message,'') LIKE ?)
		        ORDER BY i.priority ASC, i.created_at DESC`)
	mock.ExpectQuery(expectedQuery).
		WithArgs(0, "%discussed%", "%discussed%", "%discussed%", "%discussed%").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}).
			AddRow("task-1", "Task with discussion", "task", 2, "open", time.Now()))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	searchCmd := newSearchCmd(deps)
	searchCmd.SetOut(buf)
	err = searchCmd.RunE(cmd, []string{"discussed"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Task with discussion")
	require.NoError(t, mock.ExpectationsWereMet())
}

// Test search with DISTINCT (no duplicates when multiple comments match)
func TestSearchCmd_DistinctResults(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	expectedQuery := regexp.QuoteMeta(`SELECT DISTINCT i.id, i.title, i.issue_type, i.priority, i.status, i.created_at
		        FROM issues i
		        LEFT JOIN issue_comments c ON i.id = c.issue_id
		        WHERE i.ephemeral = ?
		          AND i.status != 'tombstone'
		          AND i.status != 'archived'
		          AND (i.title LIKE ? OR i.description LIKE ? OR COALESCE(i.metadata,'') LIKE ? OR COALESCE(c.message,'') LIKE ?)
		        ORDER BY i.priority ASC, i.created_at DESC`)
	mock.ExpectQuery(expectedQuery).
		WithArgs(0, "%bug%", "%bug%", "%bug%", "%bug%").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}).
			AddRow("bug-1", "Critical bug", "bug", 0, "open", time.Now()))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	searchCmd := newSearchCmd(deps)
	searchCmd.SetOut(buf)
	err = searchCmd.RunE(cmd, []string{"bug"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Critical bug")
	require.NoError(t, mock.ExpectationsWereMet())
}

// Test search JSON output
func TestSearchCmd_JSONOutput(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := true
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	expectedQuery := regexp.QuoteMeta(`SELECT DISTINCT i.id, i.title, i.issue_type, i.priority, i.status, i.created_at
		        FROM issues i
		        LEFT JOIN issue_comments c ON i.id = c.issue_id
		        WHERE i.ephemeral = ?
		          AND i.status != 'tombstone'
		          AND i.status != 'archived'
		          AND (i.title LIKE ? OR i.description LIKE ? OR COALESCE(i.metadata,'') LIKE ? OR COALESCE(c.message,'') LIKE ?)
		        ORDER BY i.priority ASC, i.created_at DESC`)
	now := time.Now()
	mock.ExpectQuery(expectedQuery).
		WithArgs(0, "%search%", "%search%", "%search%", "%search%").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}).
			AddRow("task-1", "Search functionality", "feature", 1, "open", now))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	searchCmd := newSearchCmd(deps)
	searchCmd.SetOut(buf)
	err = searchCmd.RunE(cmd, []string{"search"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "\"id\"")
	assert.Contains(t, buf.String(), "Search functionality")
	require.NoError(t, mock.ExpectationsWereMet())
}

// Test search with empty query validation
func TestSearchCmd_EmptyQuery(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	searchCmd := newSearchCmd(deps)
	searchCmd.SetOut(buf)
	err = searchCmd.RunE(cmd, []string{"   "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provide a search query or --label flag")
}

// Test search with no results
func TestSearchCmd_NoResults(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	expectedQuery := regexp.QuoteMeta(`SELECT DISTINCT i.id, i.title, i.issue_type, i.priority, i.status, i.created_at
		        FROM issues i
		        LEFT JOIN issue_comments c ON i.id = c.issue_id
		        WHERE i.ephemeral = ?
		          AND i.status != 'tombstone'
		          AND i.status != 'archived'
		          AND (i.title LIKE ? OR i.description LIKE ? OR COALESCE(i.metadata,'') LIKE ? OR COALESCE(c.message,'') LIKE ?)
		        ORDER BY i.priority ASC, i.created_at DESC`)
	mock.ExpectQuery(expectedQuery).
		WithArgs(0, "%nonexistent%", "%nonexistent%", "%nonexistent%", "%nonexistent%").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	searchCmd := newSearchCmd(deps)
	searchCmd.SetOut(buf)
	err = searchCmd.RunE(cmd, []string{"nonexistent"})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// Test search with --label flag (single label, no text query)
func TestSearchCmd_LabelOnly(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := true
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	expectedQuery := regexp.QuoteMeta(`SELECT DISTINCT i.id, i.title, i.issue_type, i.priority, i.status, i.created_at
		        FROM issues i
		        LEFT JOIN issue_comments c ON i.id = c.issue_id
		        WHERE i.ephemeral = ?
		          AND i.status != 'tombstone'
		          AND i.status != 'archived'`) +
		regexp.QuoteMeta(`
		          AND i.id IN (SELECT issue_id FROM issue_labels WHERE label IN (?) GROUP BY issue_id HAVING COUNT(DISTINCT label) = ?)`) +
		regexp.QuoteMeta(`
		        ORDER BY i.priority ASC, i.created_at DESC`)
	mock.ExpectQuery(expectedQuery).
		WithArgs(0, "code_review", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}).
			AddRow("task-1", "Needs review", "task", 1, "open", time.Now()))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	searchCmd := newSearchCmd(deps)
	// Set labels AFTER newSearchCmd (StringSliceVar resets to default)
	*SearchLabels = []string{"code_review"}
	defer func() { *SearchLabels = nil }()
	searchCmd.SetOut(buf)
	err = searchCmd.RunE(cmd, []string{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Needs review")
	require.NoError(t, mock.ExpectationsWereMet())
}

// Test search with --label and text query combined
func TestSearchCmd_LabelWithQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := true
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	expectedQuery := regexp.QuoteMeta(`SELECT DISTINCT i.id, i.title, i.issue_type, i.priority, i.status, i.created_at
		        FROM issues i
		        LEFT JOIN issue_comments c ON i.id = c.issue_id
		        WHERE i.ephemeral = ?
		          AND i.status != 'tombstone'
		          AND i.status != 'archived'`) +
		regexp.QuoteMeta(`
		          AND (i.title LIKE ? OR i.description LIKE ? OR COALESCE(i.metadata,'') LIKE ? OR COALESCE(c.message,'') LIKE ?)`) +
		regexp.QuoteMeta(`
		          AND i.id IN (SELECT issue_id FROM issue_labels WHERE label IN (?) GROUP BY issue_id HAVING COUNT(DISTINCT label) = ?)`) +
		regexp.QuoteMeta(`
		        ORDER BY i.priority ASC, i.created_at DESC`)
	mock.ExpectQuery(expectedQuery).
		WithArgs(0, "%auth%", "%auth%", "%auth%", "%auth%", "reviewed", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}).
			AddRow("task-2", "Auth reviewed", "task", 2, "open", time.Now()))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	searchCmd := newSearchCmd(deps)
	*SearchLabels = []string{"reviewed"}
	defer func() { *SearchLabels = nil }()
	searchCmd.SetOut(buf)
	err = searchCmd.RunE(cmd, []string{"auth"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Auth reviewed")
	require.NoError(t, mock.ExpectationsWereMet())
}

// Test search with multiple --label flags (AND semantics)
func TestSearchCmd_MultipleLabels(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := true
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	expectedQuery := regexp.QuoteMeta(`SELECT DISTINCT i.id, i.title, i.issue_type, i.priority, i.status, i.created_at
		        FROM issues i
		        LEFT JOIN issue_comments c ON i.id = c.issue_id
		        WHERE i.ephemeral = ?
		          AND i.status != 'tombstone'
		          AND i.status != 'archived'`) +
		regexp.QuoteMeta(`
		          AND i.id IN (SELECT issue_id FROM issue_labels WHERE label IN (?,?) GROUP BY issue_id HAVING COUNT(DISTINCT label) = ?)`) +
		regexp.QuoteMeta(`
		        ORDER BY i.priority ASC, i.created_at DESC`)
	mock.ExpectQuery(expectedQuery).
		WithArgs(0, "reviewed", "qa_passed", 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}).
			AddRow("task-3", "Fully reviewed", "task", 1, "open", time.Now()))

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	searchCmd := newSearchCmd(deps)
	*SearchLabels = []string{"reviewed", "qa_passed"}
	defer func() { *SearchLabels = nil }()
	searchCmd.SetOut(buf)
	err = searchCmd.RunE(cmd, []string{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Fully reviewed")
	require.NoError(t, mock.ExpectationsWereMet())
}

// Test search with no query and no label
func TestSearchCmd_NoQueryNoLabel(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	*SearchLabels = nil

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(context.Background())

	searchCmd := newSearchCmd(deps)
	searchCmd.SetOut(buf)
	err = searchCmd.RunE(cmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provide a search query or --label flag")
}
