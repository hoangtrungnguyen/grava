package cmd

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
)

func TestHistoryCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	Store = dolt.NewClientFromDB(db)
	defer func() { Store = nil }()

	id := "issue-123"

	// Expectation
	rows := sqlmock.NewRows([]string{"commit_hash", "committer", "commit_date", "title", "status"}).
		AddRow("hash123456", "Alice", time.Now(), "Fix bug", "open").
		AddRow("hash654321", "Bob", time.Now().Add(-1*time.Hour), "Init task", "backlog")

	mock.ExpectQuery("SELECT commit_hash, committer, commit_date, title, status FROM dolt_history_issues").
		WithArgs(id).
		WillReturnRows(rows)

	// Execute
	cmd := historyCmd
	err = cmd.RunE(cmd, []string{id})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUndoCmd_Dirty(t *testing.T) {
	// Scenario: Current state differs from HEAD (Uncommitted changes)
	// Expectation: Revert to HEAD

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	Store = dolt.NewClientFromDB(db)
	defer func() { Store = nil }()

	id := "issue-dirty"
	now := time.Now()

	// 1. Get Current State
	// Current: Title="Changed Title", UpdatedAt=Now
	mock.ExpectQuery("SELECT title, description, issue_type, priority, status, affected_files, updated_at FROM issues").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "affected_files", "updated_at"}).
			AddRow("Changed Title", "Desc", "task", 1, "open", nil, now))

	// 2. Get History
	// Head (Hist[0]): Title="Original Title", UpdatedAt=Now-1h
	// Prev (Hist[1]): Title="Old Title", UpdatedAt=Now-2h
	mock.ExpectQuery("SELECT title, description, issue_type, priority, status, affected_files, updated_at FROM dolt_history_issues").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "affected_files", "updated_at"}).
			AddRow("Original Title", "Desc", "task", 1, "open", nil, now.Add(-1*time.Hour)).
			AddRow("Old Title", "Desc", "task", 1, "open", nil, now.Add(-2*time.Hour)))

	// 3. Update (Revert to HEAD)
	// Expect update with "Original Title"
	mock.ExpectExec("UPDATE issues SET title = \\?, .* WHERE id = \\?").
		WithArgs(
			"Original Title", "Desc", "task", 1, "open", nil,
			sqlmock.AnyArg(), // updated_by (actor)
			sqlmock.AnyArg(), // agent_model
			id,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Execute
	cmd := undoCmd
	err = cmd.RunE(cmd, []string{id})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUndoCmd_Clean(t *testing.T) {
	// Scenario: Current state matching HEAD (Clean)
	// Expectation: Revert to HEAD~1

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	Store = dolt.NewClientFromDB(db)
	defer func() { Store = nil }()

	id := "issue-clean"
	now := time.Now()

	// 1. Get Current State
	// Current: Title="Original Title", UpdatedAt=Now-1h (Same as HEAD)
	mock.ExpectQuery("SELECT title, description, issue_type, priority, status, affected_files, updated_at FROM issues").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "affected_files", "updated_at"}).
			AddRow("Original Title", "Desc", "task", 1, "open", nil, now.Add(-1*time.Hour)))

	// 2. Get History
	// Head (Hist[0]): Title="Original Title", UpdatedAt=Now-1h
	// Prev (Hist[1]): Title="Old Title", UpdatedAt=Now-2h
	mock.ExpectQuery("SELECT title, description, issue_type, priority, status, affected_files, updated_at FROM dolt_history_issues").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "affected_files", "updated_at"}).
			AddRow("Original Title", "Desc", "task", 1, "open", nil, now.Add(-1*time.Hour)).
			AddRow("Old Title", "Desc", "task", 1, "open", nil, now.Add(-2*time.Hour)))

	// 3. Update (Revert to HEAD~1)
	// Expect update with "Old Title"
	mock.ExpectExec("UPDATE issues SET title = \\?, .* WHERE id = \\?").
		WithArgs(
			"Old Title", "Desc", "task", 1, "open", nil,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			id,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Execute
	cmd := undoCmd
	err = cmd.RunE(cmd, []string{id})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUndoCmd_NoHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	Store = dolt.NewClientFromDB(db)
	defer func() { Store = nil }()

	id := "issue-new"

	// Current found
	mock.ExpectQuery("SELECT .* FROM issues").
		WillReturnRows(sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "affected_files", "updated_at"}).
			AddRow("Title", "Desc", "task", 1, "open", nil, time.Now()))

	// History empty
	mock.ExpectQuery("SELECT .* FROM dolt_history_issues").
		WillReturnRows(sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "affected_files", "updated_at"}))

	// Execute
	cmd := undoCmd
	err = cmd.RunE(cmd, []string{id})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no history found")
}

func TestUndoCmd_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	Store = dolt.NewClientFromDB(db)
	defer func() { Store = nil }()

	id := "issue-missing"

	mock.ExpectQuery("SELECT .* FROM issues").
		WithArgs(id).
		WillReturnError(sql.ErrNoRows)

	// Execute
	cmd := undoCmd
	err = cmd.RunE(cmd, []string{id})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
