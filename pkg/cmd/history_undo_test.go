package cmd

import (
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
)

func TestUndoCmd_Dirty(t *testing.T) {
	// Scenario: Current state differs from HEAD (Uncommitted changes)
	// Expectation: Revert to HEAD

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close() //nolint:errcheck

	Store = dolt.NewClientFromDB(db)
	defer func() { Store = nil }()

	id := "issue-dirty"

	// 1. Get Current State
	mock.ExpectQuery(regexp.QuoteMeta("SELECT title, description, issue_type, priority, status, affected_files FROM issues")).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "affected_files"}).
			AddRow("Changed Title", "Desc", "task", 1, "open", nil))

	// 2. Get History
	mock.ExpectQuery(regexp.QuoteMeta("SELECT title, description, issue_type, priority, status, affected_files FROM dolt_history_issues")).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "affected_files"}).
			AddRow("Original Title", "Desc", "task", 1, "open", nil).
			AddRow("Old Title", "Desc", "task", 1, "open", nil))

	// 3. Update (Revert to HEAD)
	mock.ExpectExec("UPDATE issues SET title = \\?, .* WHERE id = \\?").
		WithArgs(
			"Original Title", "Desc", "task", 1, "open", nil,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			id,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 4. Add Comment (Audited Tx)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM issues WHERE id = ?")).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO issue_comments (issue_id, message, actor, agent_model, created_at) VALUES (?, ?, ?, ?, ?)")).
		WithArgs(id, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events (issue_id, event_type, actor, old_value, new_value, created_by, updated_by, agent_model, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)")).
		WithArgs(id, "comment", sqlmock.AnyArg(), "{}", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	// Execute
	_, err = executeCommand(rootCmd, "undo", id)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUndoCmd_Clean(t *testing.T) {
	// Scenario: Current state matching HEAD (Clean)
	// Expectation: Revert to HEAD~1

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close() //nolint:errcheck

	Store = dolt.NewClientFromDB(db)
	defer func() { Store = nil }()

	id := "issue-clean"

	// 1. Get Current State
	mock.ExpectQuery(regexp.QuoteMeta("SELECT title, description, issue_type, priority, status, affected_files FROM issues")).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "affected_files"}).
			AddRow("Original Title", "Desc", "task", 1, "open", nil))

	// 2. Get History
	mock.ExpectQuery(regexp.QuoteMeta("SELECT title, description, issue_type, priority, status, affected_files FROM dolt_history_issues")).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "affected_files"}).
			AddRow("Original Title", "Desc", "task", 1, "open", nil).
			AddRow("Old Title", "Desc", "task", 1, "open", nil))

	// 3. Update (Revert to HEAD~1)
	mock.ExpectExec("UPDATE issues SET title = \\?, .* WHERE id = \\?").
		WithArgs(
			"Old Title", "Desc", "task", 1, "open", nil,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			id,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 4. Add Comment (Audited Tx)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM issues WHERE id = ?")).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO issue_comments (issue_id, message, actor, agent_model, created_at) VALUES (?, ?, ?, ?, ?)")).
		WithArgs(id, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events (issue_id, event_type, actor, old_value, new_value, created_by, updated_by, agent_model, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)")).
		WithArgs(id, "comment", sqlmock.AnyArg(), "{}", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	// Execute
	_, err = executeCommand(rootCmd, "undo", id)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUndoCmd_NoHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close() //nolint:errcheck

	Store = dolt.NewClientFromDB(db)
	defer func() { Store = nil }()

	id := "issue-new"

	// 1. Get Current State
	mock.ExpectQuery(regexp.QuoteMeta("SELECT title, description, issue_type, priority, status, affected_files FROM issues")).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "affected_files"}).
			AddRow("Title", "Desc", "task", 1, "open", nil))

	// 2. Get History (Only HEAD)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT title, description, issue_type, priority, status, affected_files FROM dolt_history_issues")).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "affected_files"}).
			AddRow("Title", "Desc", "task", 1, "open", nil))

	mock.ExpectClose()

	// Execute
	_, err = executeCommand(rootCmd, "undo", id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "initial state")
}

func TestUndoCmd_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close() //nolint:errcheck

	Store = dolt.NewClientFromDB(db)
	defer func() { Store = nil }()

	id := "issue-missing"

	mock.ExpectQuery("SELECT .* FROM issues").
		WithArgs(id).
		WillReturnError(sql.ErrNoRows)

	mock.ExpectClose()

	// Execute
	_, err = executeCommand(rootCmd, "undo", id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
