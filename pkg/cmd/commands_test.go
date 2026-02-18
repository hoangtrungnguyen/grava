package cmd

import (
	"bytes"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	_, err := root.ExecuteC()
	return buf.String(), err
}

func TestCreateCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	// Inject mock
	Store = dolt.NewClientFromDB(db)

	// Case 1: Base ID
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO issues`)).
		WithArgs(sqlmock.AnyArg(), "Test Issue", "Description", "task", 2, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Expect Close from PersistentPostRunE
	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "create", "--title", "Test Issue", "--desc", "Description")
	assert.NoError(t, err)
	assert.Contains(t, output, "Created issue:")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestShowCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	Store = dolt.NewClientFromDB(db)

	rows := sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "created_at", "updated_at"}).
		AddRow("My Issue", "Desc", "bug", 1, "open", time.Now(), time.Now())

	// Match query with whitespace flexibility
	mock.ExpectQuery(regexp.QuoteMeta("SELECT title, description, issue_type, priority, status, created_at, updated_at") + `\s+` + regexp.QuoteMeta("FROM issues WHERE id = ?")).
		WithArgs("grava-123").
		WillReturnRows(rows)

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "show", "grava-123")
	assert.NoError(t, err)
	assert.Contains(t, output, "Title:       My Issue")
	assert.Contains(t, output, "Type:        bug")
	assert.Contains(t, output, "Priority:    high (1)")
}

func TestListCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	Store = dolt.NewClientFromDB(db)

	rows := sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}).
		AddRow("grava-1", "I1", "task", 2, "open", time.Now()).
		AddRow("grava-2", "I2", "bug", 0, "closed", time.Now())

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, issue_type, priority, status, created_at FROM issues ORDER BY priority ASC, created_at DESC")).
		WillReturnRows(rows)

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "list")
	assert.NoError(t, err)
	assert.Contains(t, output, "grava-1")
	assert.Contains(t, output, "grava-2")
}

func TestUpdateCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	Store = dolt.NewClientFromDB(db)

	mock.ExpectExec(`UPDATE issues SET updated_at = \?.*`).
		WithArgs(sqlmock.AnyArg(), "New Title", "closed", "grava-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "update", "grava-1", "--title", "New Title", "--status", "closed")
	assert.NoError(t, err)
	assert.Contains(t, output, "Updated issue grava-1")
}

func TestSubtaskCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	Store = dolt.NewClientFromDB(db)

	parentID := "grava-123"
	lockName := "grava_cc_" + parentID

	// 1. Verify Parent Exists
	mock.ExpectQuery(`SELECT 1 FROM issues WHERE id = \?`).
		WithArgs(parentID).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	// 2. ID Generation (GetNextChildSequence)
	// GET_LOCK
	mock.ExpectQuery(`SELECT GET_LOCK\(\?, 10\)`).
		WithArgs(lockName).
		WillReturnRows(sqlmock.NewRows([]string{"xc"}).AddRow(1))

	// SELECT next_child
	mock.ExpectQuery(`SELECT next_child FROM child_counters WHERE parent_id = \?`).
		WithArgs(parentID).
		WillReturnRows(sqlmock.NewRows([]string{"next_child"}).AddRow(5))

	// UPDATE child_counters
	mock.ExpectExec(`UPDATE child_counters SET next_child = \? WHERE parent_id = \?`).
		WithArgs(6, parentID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// RELEASE_LOCK (deferred in GetNextChildSequence)
	mock.ExpectExec(`SELECT RELEASE_LOCK\(\?\)`).
		WithArgs(lockName).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// 3. Insert Subtask (after generator returns)
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO issues`)).
		WithArgs("grava-123.5", "Subtask Title", "Subtask Desc", "task", 2, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 4. Close (PersistentPostRunE)
	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "subtask", parentID, "--title", "Subtask Title", "--desc", "Subtask Desc")
	assert.NoError(t, err)
	assert.Contains(t, output, "Created subtask: grava-123.5")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
