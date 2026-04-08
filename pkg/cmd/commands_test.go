package cmd

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/internal/testutil"
	"github.com/hoangtrungnguyen/grava/pkg/cmd/issues"
	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	// Reset flags before execution to avoid state leakage
	resetFlags(root)

	_, err := root.ExecuteC()
	return buf.String(), err
}

func resetFlags(cmd *cobra.Command) {
	// Reset specific global slice variables that use StringSliceVar
	issues.CreateAffectedFiles = nil
	issues.UpdateAffectedFiles = nil
	issues.SubtaskAffectedFiles = nil
	issues.LabelAddFlags = nil
	issues.LabelRemoveFlags = nil

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		val := f.DefValue
		// pflag StringSlice default logic fix is still good to have, but vars are primary now
		if val == "[]" && f.Value.Type() == "stringSlice" {
			val = ""
		}
		f.Value.Set(val) //nolint:errcheck
		f.Changed = false
	})
	for _, child := range cmd.Commands() {
		resetFlags(child)
	}
}

func TestCreateCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	// Inject mock
	Store = dolt.NewClientFromDB(db)

	// Case 1: Base ID — ephemeral defaults to 0, affected_files defaults to []
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO issues`)).
		WithArgs(sqlmock.AnyArg(), "Test Issue", "Description", "task", 2, "open", 0, sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "unknown", "", "[]").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Expect Event Log
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs(sqlmock.AnyArg(), "create", "unknown", "{}", sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

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

	rows := sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "created_at", "updated_at", "created_by", "updated_by", "agent_model", "affected_files", "assignee"}).
		AddRow("My Issue", "Desc", "bug", 1, "open", time.Now(), time.Now(), "alice", "bob", "gemini-pro", `["pkg/cmd/root.go","pkg/cmd/show.go"]`, "")

	// Match query with whitespace flexibility
	mock.ExpectQuery(regexp.QuoteMeta("SELECT title, description, issue_type, priority, status, created_at, updated_at, created_by, updated_by, agent_model, affected_files, COALESCE(assignee, '')") + `\s+` + regexp.QuoteMeta("FROM issues WHERE id = ?")).
		WithArgs("grava-123").
		WillReturnRows(rows)

	// Subtasks query — returns empty (no subtasks for this issue)
	mock.ExpectQuery("SELECT d.from_id FROM dependencies").
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"from_id"}))

	// Labels query — returns empty
	mock.ExpectQuery(regexp.QuoteMeta("SELECT label FROM issue_labels WHERE issue_id = ?")).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"label"}))

	// Comments query — returns empty
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, message, COALESCE(actor, ''), COALESCE(agent_model, ''), created_at FROM issue_comments WHERE issue_id = ?")).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"id", "message", "actor", "agent_model", "created_at"}))

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "show", "grava-123")
	assert.NoError(t, err)
	assert.Contains(t, output, "Title:       My Issue")
	assert.Contains(t, output, "Type:        bug")
	assert.Contains(t, output, "Priority:    high (1)")
}

func TestCreateEphemeralCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	Store = dolt.NewClientFromDB(db)

	// ephemeral=1 must be passed as the 7th arg, affected_files last
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO issues`)).
		WithArgs(sqlmock.AnyArg(), "Scratch Note", "", "task", 2, "open", 1, sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "unknown", "", "[]").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Audit Log
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs(sqlmock.AnyArg(), "create", "unknown", "{}", sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "create", "--title", "Scratch Note", "--ephemeral")
	assert.NoError(t, err)
	assert.Contains(t, output, "Wisp")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestListCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	Store = dolt.NewClientFromDB(db)

	rows := sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}).
		AddRow("grava-1", "I1", "task", 2, "open", time.Now()).
		AddRow("grava-2", "I2", "bug", 0, "closed", time.Now())

	// Default list excludes ephemeral (ephemeral = 0)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, issue_type, priority, status, created_at FROM issues WHERE ephemeral = 0 AND status != 'tombstone' AND status != 'archived' ORDER BY priority ASC, created_at DESC, id ASC")).
		WillReturnRows(rows)

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "list")
	assert.NoError(t, err)
	assert.Contains(t, output, "grava-1")
	assert.Contains(t, output, "grava-2")
}

func TestListWispCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	Store = dolt.NewClientFromDB(db)

	rows := sqlmock.NewRows([]string{"id", "title", "issue_type", "priority", "status", "created_at"}).
		AddRow("grava-w1", "Scratch", "task", 4, "open", time.Now())

	// --wisp filters for ephemeral = 1
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, issue_type, priority, status, created_at FROM issues WHERE ephemeral = 1 AND status != 'tombstone' AND status != 'archived' ORDER BY priority ASC, created_at DESC, id ASC")).
		WillReturnRows(rows)

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "list", "--wisp")
	assert.NoError(t, err)
	assert.Contains(t, output, "grava-w1")
	assert.Contains(t, output, "Scratch")
}

func TestCompactCmd(t *testing.T) {
	t.Run("purges old wisps", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		Store = dolt.NewClientFromDB(db)

		// 1. SELECT ephemeral issues older than cutoff
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM issues WHERE ephemeral = 1 AND created_at < ?`)).
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).
				AddRow("grava-w1").
				AddRow("grava-w2"))

		// Expect Transaction
		mock.ExpectBegin()

		// 2a. INSERT into deletions for grava-w1
		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO deletions`)).
			WithArgs("grava-w1", sqlmock.AnyArg(), "compact", "grava-compact", "unknown", "unknown", "").
			WillReturnResult(sqlmock.NewResult(1, 1))
		// 2b. Soft delete grava-w1
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE issues SET status = 'tombstone', updated_at = NOW(), updated_by = ?, agent_model = ? WHERE id = ?`)).
			WithArgs("unknown", "", "grava-w1").
			WillReturnResult(sqlmock.NewResult(1, 1))

		// 3a. INSERT into deletions for grava-w2
		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO deletions`)).
			WithArgs("grava-w2", sqlmock.AnyArg(), "compact", "grava-compact", "unknown", "unknown", "").
			WillReturnResult(sqlmock.NewResult(1, 1))
		// 3b. Soft delete grava-w2
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE issues SET status = 'tombstone', updated_at = NOW(), updated_by = ?, agent_model = ? WHERE id = ?`)).
			WithArgs("unknown", "", "grava-w2").
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectCommit()

		mock.ExpectClose()

		output, err := executeCommand(rootCmd, "compact", "--days", "7")
		assert.NoError(t, err)
		assert.Contains(t, output, "Compacted 2 Wisp(s)")

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("nothing to compact", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		Store = dolt.NewClientFromDB(db)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM issues WHERE ephemeral = 1 AND created_at < ?`)).
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))

		mock.ExpectClose()

		output, err := executeCommand(rootCmd, "compact")
		assert.NoError(t, err)
		assert.Contains(t, output, "Nothing to compact")

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUpdateCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	Store = dolt.NewClientFromDB(db)

	// Pre-read current row before WithAuditedTx
	mock.ExpectQuery(`SELECT title, description, issue_type, priority, status`).
		WithArgs("grava-1").
		WillReturnRows(sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "assignee"}).
			AddRow("Old Title", "desc", "task", 2, "open", ""))

	// Phase 1: graph load for status propagation
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, issue_type, status, priority, created_at, await_type, await_id, ephemeral, metadata FROM issues WHERE status != 'tombstone'")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "status", "priority", "created_at", "await_type", "await_id", "ephemeral", "metadata"}).
			AddRow("grava-1", "Old Title", "task", "open", 2, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT from_id, to_id, type, metadata FROM dependencies")).
		WillReturnRows(sqlmock.NewRows([]string{"from_id", "to_id", "type", "metadata"}))

	// Phase 1b: SetNodeStatus → UPDATE + audit event
	mock.ExpectExec(regexp.QuoteMeta("UPDATE issues SET status = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?")).
		WithArgs("closed", sqlmock.AnyArg(), "unknown", "", "grava-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Phase 2: WithAuditedTx for title field
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE issues SET updated_at = \?, updated_by = \?, agent_model = \?.*`).
		WithArgs(sqlmock.AnyArg(), "unknown", "", "New Title", "grava-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "update", "grava-1", "--title", "New Title", "--status", "closed")
	assert.NoError(t, err)
	assert.Contains(t, output, "Updated issue grava-1")
}

func TestUpdateTitleCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	Store = dolt.NewClientFromDB(db)

	// Pre-read current row before WithAuditedTx
	mock.ExpectQuery(`SELECT title, description, issue_type, priority, status`).
		WithArgs("grava-1").
		WillReturnRows(sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "assignee"}).
			AddRow("Old Title", "desc", "task", 2, "open", ""))

	// WithAuditedTx for title field
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE issues SET updated_at = \?, updated_by = \?, agent_model = \?.*`).
		WithArgs(sqlmock.AnyArg(), "unknown", "", "New Title", "grava-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "update", "grava-1", "--title", "New Title")
	assert.NoError(t, err)
	assert.Contains(t, output, "Updated issue grava-1")
}

func TestUpdateWithFilesCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	Store = dolt.NewClientFromDB(db)

	// Pre-read current row before WithAuditedTx
	mock.ExpectQuery(`SELECT title, description, issue_type, priority, status`).
		WithArgs("grava-1").
		WillReturnRows(sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "assignee"}).
			AddRow("Old Title", "desc", "task", 2, "open", ""))

	// WithAuditedTx for files field
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE issues SET updated_at = \?, updated_by = \?, agent_model = \?.*`).
		WithArgs(sqlmock.AnyArg(), "unknown", "", `["f1.go","f2.go"]`, "grava-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "update", "grava-1", "--files", "f1.go,f2.go")
	assert.NoError(t, err)
	assert.Contains(t, output, "Updated issue grava-1")
}

func TestCreateWithFilesCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	Store = dolt.NewClientFromDB(db)

	// Case: Create with files
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO issues`)).
		WithArgs(sqlmock.AnyArg(), "File Issue", "", "task", 2, "open", 0, sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "unknown", "", `["f1.go"]`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Audit Log
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs(sqlmock.AnyArg(), "create", "unknown", "{}", sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "create", "--title", "File Issue", "--files", "f1.go")
	assert.NoError(t, err)
	assert.Contains(t, output, "Created issue:")
}

func TestSubtaskCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	// Use MockStore: GetNextChildSequence returns 5 directly (no nested tx against sqlmock),
	// while BeginTx routes actual SQL through the sqlmock-backed db.
	ms := testutil.NewMockStore()
	ms.GetNextChildSequenceFn = func(parentID string) (int, error) { return 5, nil }
	ms.BeginTxFn = func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
		return dolt.NewClientFromDB(db).BeginTx(ctx, nil)
	}
	ms.LogEventTxFn = func(ctx context.Context, tx *sql.Tx, issueID, eventType, actor, model string, old, new interface{}) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO events VALUES ()")
		return err
	}
	// Parent existence pre-check now uses store.QueryRow (not the tx). Wire it on the mock.
	ms.QueryRowFn = func(query string, args ...any) *sql.Row {
		mockDB, qmock, _ := sqlmock.New()
		qmock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		return mockDB.QueryRow("SELECT", args...)
	}
	Store = ms

	// 1. Transaction Start
	mock.ExpectBegin()
	// 2. Insert subtask (parent check now happens before tx via store.QueryRow)
	mock.ExpectExec(`INSERT INTO issues`).
		WithArgs("grava-123.5", "Subtask Title", "Subtask Desc", "task", 1, "open", 0,
			sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "unknown", "", "[]").
		WillReturnResult(sqlmock.NewResult(1, 1))
	// 4. Insert dependency
	mock.ExpectExec(`INSERT INTO dependencies`).
		WithArgs("grava-123.5", "grava-123", "subtask-of", "unknown", "unknown", "").
		WillReturnResult(sqlmock.NewResult(1, 1))
	// 5. Audit log (subtask event)
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	// 6. Audit log (dependency_add event)
	mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
	// 7. Commit
	mock.ExpectCommit()
	// PersistentPostRunE calls Store.Close() — MockStore.Close() is a no-op, no ExpectClose needed

	output, err := executeCommand(rootCmd, "subtask", "grava-123", "--title", "Subtask Title", "--desc", "Subtask Desc", "--priority", "high")
	assert.NoError(t, err)
	assert.Contains(t, output, "Created subtask: grava-123.5")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSubtaskCmd_ParentNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	ms := testutil.NewMockStore()
	// Parent check uses store.QueryRow before GenerateChildID; return count=0 to trigger ISSUE_NOT_FOUND.
	ms.QueryRowFn = func(query string, args ...any) *sql.Row {
		mockDB, qmock, _ := sqlmock.New()
		qmock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		return mockDB.QueryRow("SELECT", args...)
	}
	Store = ms

	_, err = executeCommand(rootCmd, "subtask", "grava-missing", "--title", "Some subtask")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "grava-missing not found")
	// db (sqlmock) is no longer used — no expectations to verify
	_ = mock
}

func TestShowCmd_WithSubtasks(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	Store = dolt.NewClientFromDB(db)

	issueRows := sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "created_at", "updated_at", "created_by", "updated_by", "agent_model", "affected_files", "assignee"}).
		AddRow("Parent Issue", "Desc", "task", 2, "open", time.Now(), time.Now(), "alice", "alice", nil, nil, "")

	mock.ExpectQuery(`SELECT title`).
		WithArgs("grava-abc").
		WillReturnRows(issueRows)

	// Subtask query via dependencies table
	subtaskRows := sqlmock.NewRows([]string{"from_id"}).
		AddRow("grava-abc.1").
		AddRow("grava-abc.2")
	mock.ExpectQuery(`SELECT d.from_id FROM dependencies`).
		WithArgs("grava-abc").
		WillReturnRows(subtaskRows)

	// Labels query — returns empty
	mock.ExpectQuery(regexp.QuoteMeta("SELECT label FROM issue_labels WHERE issue_id = ?")).
		WithArgs("grava-abc").
		WillReturnRows(sqlmock.NewRows([]string{"label"}))

	// Comments query — returns empty
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, message, COALESCE(actor, ''), COALESCE(agent_model, ''), created_at FROM issue_comments WHERE issue_id = ?")).
		WithArgs("grava-abc").
		WillReturnRows(sqlmock.NewRows([]string{"id", "message", "actor", "agent_model", "created_at"}))

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "show", "grava-abc")
	assert.NoError(t, err)
	assert.Contains(t, output, "Parent Issue")
	assert.Contains(t, output, "grava-abc.1")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShowCmd_LabelsAndComments(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	issueRow := sqlmock.NewRows([]string{"title", "description", "issue_type", "priority", "status", "created_at", "updated_at", "created_by", "updated_by", "agent_model", "affected_files", "assignee"}).
		AddRow("Labeled Issue", "Has labels and comments", "bug", 1, "open", time.Now(), time.Now(), "alice", "bob", nil, "[]", "")

	mock.ExpectQuery(regexp.QuoteMeta("SELECT title, description, issue_type, priority, status, created_at, updated_at, created_by, updated_by, agent_model, affected_files, COALESCE(assignee, '')") + `\s+` + regexp.QuoteMeta("FROM issues WHERE id = ?")).
		WithArgs("grava-lbl").
		WillReturnRows(issueRow)

	// Subtasks — empty
	mock.ExpectQuery("SELECT d.from_id FROM dependencies").
		WithArgs("grava-lbl").
		WillReturnRows(sqlmock.NewRows([]string{"from_id"}))

	// Labels — return two labels
	mock.ExpectQuery(regexp.QuoteMeta("SELECT label FROM issue_labels WHERE issue_id = ?")).
		WithArgs("grava-lbl").
		WillReturnRows(sqlmock.NewRows([]string{"label"}).AddRow("bug").AddRow("critical"))

	// Comments — return one comment
	commentTime := time.Date(2026, 4, 3, 10, 30, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, message, COALESCE(actor, ''), COALESCE(agent_model, ''), created_at FROM issue_comments WHERE issue_id = ?")).
		WithArgs("grava-lbl").
		WillReturnRows(sqlmock.NewRows([]string{"id", "message", "actor", "agent_model", "created_at"}).
			AddRow(1, "Reproduced on macOS ARM", "agent-01", "claude-opus", commentTime))

	mock.ExpectClose()

	// Test JSON output
	output, err := executeCommand(rootCmd, "show", "grava-lbl", "--json")
	assert.NoError(t, err)
	assert.Contains(t, output, `"labels"`)
	assert.Contains(t, output, `"bug"`)
	assert.Contains(t, output, `"critical"`)
	assert.Contains(t, output, `"comments"`)
	assert.Contains(t, output, `"Reproduced on macOS ARM"`)
	assert.Contains(t, output, `"agent-01"`)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCommentCmd_Message(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	mock.ExpectBegin()
	// Pre-read: issue exists
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM issues WHERE id = ?`)).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("grava-123"))
	// INSERT comment into issue_comments
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO issue_comments`)).
		WithArgs("grava-123", "This is a test comment", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	// Audit event
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs(sqlmock.AnyArg(), "comment", "unknown", sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "comment", "grava-123", "--message", "This is a test comment")
	assert.NoError(t, err)
	assert.Contains(t, output, "💬 Comment added to grava-123")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCommentCmd_Positional(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM issues WHERE id = ?`)).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("grava-123"))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO issue_comments`)).
		WithArgs("grava-123", "Positional text", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs(sqlmock.AnyArg(), "comment", "unknown", sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "comment", "grava-123", "Positional text")
	assert.NoError(t, err)
	assert.Contains(t, output, "💬 Comment added to grava-123")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCommentCmd_IssueNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM issues WHERE id = ?`)).
		WithArgs("grava-999").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err = executeCommand(rootCmd, "comment", "grava-999", "--message", "text")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCommentCmd_JSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM issues WHERE id = ?`)).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("grava-123"))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO issue_comments`)).
		WithArgs("grava-123", "JSON test", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs(sqlmock.AnyArg(), "comment", "unknown", sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "comment", "grava-123", "--message", "JSON test", "--json")
	assert.NoError(t, err)
	assert.Contains(t, output, `"id": "grava-123"`)
	assert.Contains(t, output, `"message": "JSON test"`)
	assert.Contains(t, output, `"comment_id"`)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDepCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// Graph load for validation
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, issue_type, status, priority, created_at, await_type, await_id, ephemeral, metadata FROM issues WHERE status != 'tombstone'")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "status", "priority", "created_at", "await_type", "await_id", "ephemeral", "metadata"}).
			AddRow("grava-abc", "T1", "task", "open", 2, time.Now(), nil, nil, 0, nil).
			AddRow("grava-def", "T2", "task", "open", 2, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT from_id, to_id, type, metadata FROM dependencies")).
		WillReturnRows(sqlmock.NewRows([]string{"from_id", "to_id", "type", "metadata"}))

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)`)).
		WithArgs("grava-abc", "grava-def", "blocks", "unknown", "unknown", "").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Audit Log
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs("grava-abc", "dependency_add", "unknown", "{}", sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "dep", "grava-abc", "grava-def")
	assert.NoError(t, err)
	assert.Contains(t, output, "🔗 Dependency created: grava-abc -[blocks]-> grava-def")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDepCmdCustomType(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// Graph load for validation
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, issue_type, status, priority, created_at, await_type, await_id, ephemeral, metadata FROM issues WHERE status != 'tombstone'")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "status", "priority", "created_at", "await_type", "await_id", "ephemeral", "metadata"}).
			AddRow("grava-abc", "T1", "task", "open", 2, time.Now(), nil, nil, 0, nil).
			AddRow("grava-def", "T2", "task", "open", 2, time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT from_id, to_id, type, metadata FROM dependencies")).
		WillReturnRows(sqlmock.NewRows([]string{"from_id", "to_id", "type", "metadata"}))

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)`)).
		WithArgs("grava-abc", "grava-def", "relates-to", "unknown", "unknown", "").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Audit Log
	_ = mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs("grava-abc", "dependency_add", "unknown", "{}", sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "dep", "grava-abc", "grava-def", "--type", "relates-to")
	assert.NoError(t, err)
	assert.Contains(t, output, "-[relates-to]->")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDepCmdSameID(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	_, err = executeCommand(rootCmd, "dep", "grava-abc", "grava-abc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "from_id and to_id must be different")
}

func TestLabelCmd_Add(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	mock.ExpectBegin()
	// Pre-read: issue exists
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM issues WHERE id = ?`)).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("grava-123"))
	// INSERT IGNORE for "bug"
	mock.ExpectExec(regexp.QuoteMeta(`INSERT IGNORE INTO issue_labels`)).
		WithArgs("grava-123", "bug", "unknown").
		WillReturnResult(sqlmock.NewResult(1, 1))
	// INSERT IGNORE for "critical"
	mock.ExpectExec(regexp.QuoteMeta(`INSERT IGNORE INTO issue_labels`)).
		WithArgs("grava-123", "critical", "unknown").
		WillReturnResult(sqlmock.NewResult(2, 1))
	// Query final labels
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT label FROM issue_labels WHERE issue_id = ?`)).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"label"}).AddRow("bug").AddRow("critical"))
	// Audit event
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs(sqlmock.AnyArg(), "label", "unknown", sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "label", "grava-123", "--add", "bug", "--add", "critical")
	assert.NoError(t, err)
	assert.Contains(t, output, "Labels added")
	assert.Contains(t, output, "Current labels")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLabelCmd_Remove(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM issues WHERE id = ?`)).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("grava-123"))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM issue_labels WHERE issue_id = ? AND label = ?`)).
		WithArgs("grava-123", "bug").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT label FROM issue_labels WHERE issue_id = ?`)).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"label"}).AddRow("critical"))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs(sqlmock.AnyArg(), "label", "unknown", sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "label", "grava-123", "--remove", "bug")
	assert.NoError(t, err)
	assert.Contains(t, output, "Labels removed")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLabelCmd_IssueNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM issues WHERE id = ?`)).
		WithArgs("grava-missing").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err = executeCommand(rootCmd, "label", "grava-missing", "--add", "bug")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLabelCmd_JSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM issues WHERE id = ?`)).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("grava-123"))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT IGNORE INTO issue_labels`)).
		WithArgs("grava-123", "bug", "unknown").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT label FROM issue_labels WHERE issue_id = ?`)).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"label"}).AddRow("bug"))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs(sqlmock.AnyArg(), "label", "unknown", sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "label", "grava-123", "--add", "bug", "--json")
	assert.NoError(t, err)
	assert.Contains(t, output, `"id": "grava-123"`)
	assert.Contains(t, output, `"labels_added"`)
	assert.Contains(t, output, `"current_labels"`)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateCmdIssueNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// Pre-read returns ErrNoRows → ISSUE_NOT_FOUND
	mock.ExpectQuery(`SELECT title, description, issue_type, priority, status`).
		WithArgs("grava-missing").
		WillReturnRows(sqlmock.NewRows([]string{}))

	// No ExpectClose: RunE returns error so PersistentPostRunE is skipped
	_, err = executeCommand(rootCmd, "update", "grava-missing", "--title", "New Title")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "grava-missing not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAssignCmdUnassign(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// Pre-read current assignee
	mock.ExpectQuery(`SELECT COALESCE\(assignee`).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"assignee"}).AddRow("alice"))

	// WithAuditedTx for unassign
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE issues SET assignee`).
		WithArgs(nil, sqlmock.AnyArg(), "unknown", "", "grava-123").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "assign", "grava-123", "--unassign")
	assert.NoError(t, err)
	assert.Contains(t, output, "Assignee cleared on grava-123")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAssignCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// Pre-read current assignee before WithAuditedTx
	mock.ExpectQuery(`SELECT COALESCE\(assignee`).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"assignee"}).AddRow(""))

	// WithAuditedTx for assign (assigneeVal, now, actor, model, id)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE issues SET assignee`).
		WithArgs("alice", sqlmock.AnyArg(), "unknown", "", "grava-123").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "assign", "grava-123", "--actor", "alice")
	assert.NoError(t, err)
	assert.Contains(t, output, "Assigned grava-123 to alice")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAssignCmdNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// Pre-read returns ErrNoRows → ISSUE_NOT_FOUND
	mock.ExpectQuery(`SELECT COALESCE\(assignee`).
		WithArgs("grava-999").
		WillReturnRows(sqlmock.NewRows([]string{}))

	// No ExpectClose: RunE returns error so PersistentPostRunE is skipped
	_, err = executeCommand(rootCmd, "assign", "grava-999", "--actor", "alice")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "grava-999 not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAssignCmdMissingActorAndUnassign(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// Neither --actor nor --unassign → MISSING_REQUIRED_FIELD
	_, err = executeCommand(rootCmd, "assign", "grava-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "either --actor")
}


func TestQuickCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// quick now creates an issue: expect begin, insert, events insert, commit, close
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO issues").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "quick", "Fix login bug")
	assert.NoError(t, err)
	assert.Contains(t, output, "Created issue")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestQuickCmdAllCaughtUp(t *testing.T) {
	// quick no longer has a "caught up" state — it's a create command.
	// Verify it requires exactly one argument.
	_, err := executeCommand(rootCmd, "quick")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}


func TestQuickCmdCustomPriority(t *testing.T) {
	// quick no longer accepts --priority flag — verify unknown flag error.
	_, err := executeCommand(rootCmd, "quick", "Some task", "--priority", "2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag: --priority")
}

func TestDoctorCmd(t *testing.T) {
	// Helper: expect the 4 table-existence checks
	expectTableChecks := func(mock sqlmock.Sqlmock, tables []string, present bool) {
		for _, tbl := range tables {
			count := 1
			if !present {
				count = 0
			}
			mock.ExpectQuery(regexp.QuoteMeta(
				"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
			)).WithArgs(tbl).
				WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(count))
		}
	}

	tables := []string{"issues", "dependencies", "deletions", "child_counters"}

	t.Run("all checks pass", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		Store = dolt.NewClientFromDB(db)

		// 1. DB connectivity
		mock.ExpectQuery(regexp.QuoteMeta("SELECT VERSION()")).
			WillReturnRows(sqlmock.NewRows([]string{"VERSION()"}).AddRow("8.0.31"))

		// 2. Table checks — all present
		expectTableChecks(mock, tables, true)

		// 3. Orphaned dependencies — none
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM dependencies").
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))

		// 4. Untitled issues — none
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM issues WHERE title IS NULL OR title = ''")).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))

		// 5. Wisp count — low
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM issues WHERE ephemeral = 1")).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(3))

		mock.ExpectClose()

		output, err := executeCommand(rootCmd, "doctor")
		assert.NoError(t, err)
		assert.Contains(t, output, "All critical checks passed")
		assert.Contains(t, output, "connected")
		assert.NotContains(t, output, "FAIL")

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("missing table causes failure", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		Store = dolt.NewClientFromDB(db)

		// 1. DB connectivity — OK
		mock.ExpectQuery(regexp.QuoteMeta("SELECT VERSION()")).
			WillReturnRows(sqlmock.NewRows([]string{"VERSION()"}).AddRow("8.0.31"))

		// 2. Table checks — "deletions" is missing
		for _, tbl := range tables {
			count := 1
			if tbl == "deletions" {
				count = 0
			}
			mock.ExpectQuery(regexp.QuoteMeta(
				"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
			)).WithArgs(tbl).
				WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(count))
		}

		// 3. Orphaned deps
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM dependencies").
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))

		// 4. Untitled issues
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM issues WHERE title IS NULL OR title = ''")).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))

		// 5. Wisp count
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM issues WHERE ephemeral = 1")).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))

		// No ExpectClose — RunE returns error, PersistentPostRunE is skipped

		_, err = executeCommand(rootCmd, "doctor")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "doctor found critical issues")

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("warnings for orphaned deps and high wisp count", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		Store = dolt.NewClientFromDB(db)

		// 1. DB connectivity — OK
		mock.ExpectQuery(regexp.QuoteMeta("SELECT VERSION()")).
			WillReturnRows(sqlmock.NewRows([]string{"VERSION()"}).AddRow("8.0.31"))

		// 2. All tables present
		expectTableChecks(mock, tables, true)

		// 3. Orphaned deps — 2 found (WARN)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM dependencies").
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(2))

		// 4. Untitled issues — none
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM issues WHERE title IS NULL OR title = ''")).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))

		// 5. Wisp count — 150 (WARN: > 100)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM issues WHERE ephemeral = 1")).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(150))

		mock.ExpectClose()

		output, err := executeCommand(rootCmd, "doctor")
		// Warnings don't cause a non-zero exit
		assert.NoError(t, err)
		assert.Contains(t, output, "All critical checks passed")
		assert.Contains(t, output, "2 edge(s) reference non-existent issues")
		assert.Contains(t, output, "grava compact")

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDropCmdForce(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// Expect Transaction
	mock.ExpectBegin()

	// Expect DELETE in FK-safe order (Story 2.6 — now 8 tables)
	for _, table := range []string{"dependencies", "events", "work_sessions", "issue_labels", "issue_comments", "deletions", "child_counters", "issues"} {
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(`DELETE FROM %s`, table))).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}

	mock.ExpectCommit()

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "drop", "--all", "--force")
	assert.NoError(t, err)
	assert.Contains(t, output, "All Grava data has been dropped")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDropCmdConfirmYes(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// Inject "yes" into stdin reader
	oldReader := issues.StdinReader
	issues.StdinReader = strings.NewReader("yes\n")
	defer func() { issues.StdinReader = oldReader }()

	// Expect Transaction
	mock.ExpectBegin()

	// Expect all 8 DELETE statements (Story 2.6)
	for _, table := range []string{"dependencies", "events", "work_sessions", "issue_labels", "issue_comments", "deletions", "child_counters", "issues"} {
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(`DELETE FROM %s`, table))).
			WillReturnResult(sqlmock.NewResult(0, 0))
	}

	mock.ExpectCommit()

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "drop", "--all")
	assert.NoError(t, err)
	assert.Contains(t, output, "All Grava data has been dropped")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDropCmdConfirmNo(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// Inject "no" into stdin reader
	oldReader := issues.StdinReader
	issues.StdinReader = strings.NewReader("no\n")
	defer func() { issues.StdinReader = oldReader }()

	// No ExpectExec — no deletes should happen
	// No ExpectClose — RunE returns error so PersistentPostRunE is skipped

	_, err = executeCommand(rootCmd, "drop", "--all")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user cancelled drop operation")
}

func TestDropCmdDeleteError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// Expect Transaction
	mock.ExpectBegin()

	// First DELETE succeeds, second fails
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM dependencies`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM events`)).
		WillReturnError(fmt.Errorf("connection lost"))

	mock.ExpectRollback()

	// No ExpectClose — RunE returns error so PersistentPostRunE is skipped

	_, err = executeCommand(rootCmd, "drop", "--all", "--force")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete from events")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- JSON error envelope tests ---

func TestWriteJSONError_GenericError(t *testing.T) {
	errBuf := new(bytes.Buffer)

	err := cmddeps.WriteJSONError(errBuf, fmt.Errorf("something went wrong"))
	require.NoError(t, err)

	var envelope struct {
		Error cmddeps.GravaError `json:"error"`
	}
	require.NoError(t, json.Unmarshal(errBuf.Bytes(), &envelope))
	assert.Equal(t, "INTERNAL_ERROR", envelope.Error.Code)
	assert.Equal(t, "something went wrong", envelope.Error.Message)
}

func TestWriteJSONError_GravaError(t *testing.T) {
	errBuf := new(bytes.Buffer)

	gravaErr := gravaerrors.New("SCHEMA_MISMATCH", "schema version mismatch", nil)
	err := cmddeps.WriteJSONError(errBuf, gravaErr)
	require.NoError(t, err)

	var envelope struct {
		Error cmddeps.GravaError `json:"error"`
	}
	require.NoError(t, json.Unmarshal(errBuf.Bytes(), &envelope))
	assert.Equal(t, "SCHEMA_MISMATCH", envelope.Error.Code)
	assert.Equal(t, "schema version mismatch", envelope.Error.Message)
}

func TestWriteJSONError_WrappedGravaError(t *testing.T) {
	errBuf := new(bytes.Buffer)

	gravaErr := gravaerrors.New("NOT_INITIALIZED", "not initialized", nil)
	wrapped := fmt.Errorf("outer: %w", gravaErr)

	err := cmddeps.WriteJSONError(errBuf, wrapped)
	require.NoError(t, err)

	var envelope struct {
		Error cmddeps.GravaError `json:"error"`
	}
	require.NoError(t, json.Unmarshal(errBuf.Bytes(), &envelope))
	assert.Equal(t, "NOT_INITIALIZED", envelope.Error.Code)
	assert.Equal(t, "not initialized", envelope.Error.Message)
}

func TestWriteJSONError_OutputIsValidJSON(t *testing.T) {
	errBuf := new(bytes.Buffer)

	_ = cmddeps.WriteJSONError(errBuf, fmt.Errorf("any error"))

	var raw map[string]any
	require.NoError(t, json.Unmarshal(errBuf.Bytes(), &raw), "output must be valid JSON")
	_, hasError := raw["error"]
	assert.True(t, hasError, "JSON must contain top-level 'error' key")
}

func TestStartCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// SELECT FOR UPDATE is now inside the transaction (atomic read+write)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT status, COALESCE(assignee, '') FROM issues WHERE id = ? FOR UPDATE`)).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee"}).AddRow("open", ""))

	// Start work: update status to in_progress, set assignee, clear stopped_at
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE issues SET status = ?, started_at = ?, stopped_at = NULL, assignee = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?`)).
		WithArgs("in_progress", sqlmock.AnyArg(), "unknown", sqlmock.AnyArg(), "unknown", "", "grava-123").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Insert audit event
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs(sqlmock.AnyArg(), "start", "unknown", sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()
	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "start", "grava-123")
	assert.NoError(t, err)
	assert.Contains(t, output, "Started work on")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStartCmd_AlreadyInProgress(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// SELECT FOR UPDATE inside transaction returns already in_progress
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT status, COALESCE(assignee, '') FROM issues WHERE id = ? FOR UPDATE`)).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee"}).AddRow("in_progress", "agent-01"))

	// No ExpectClose: RunE returns error so PersistentPostRunE is skipped

	_, err = executeCommand(rootCmd, "start", "grava-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already being worked on by agent-01")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStartCmd_IssueNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// SELECT FOR UPDATE inside transaction returns empty result (ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT status, COALESCE(assignee, '') FROM issues WHERE id = ? FOR UPDATE`)).
		WithArgs("grava-missing").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee"}))

	// No ExpectClose: RunE returns error so PersistentPostRunE is skipped

	_, err = executeCommand(rootCmd, "start", "grava-missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStopCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// SELECT FOR UPDATE is now inside the transaction (atomic read+write)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT status FROM issues WHERE id = ? FOR UPDATE`)).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("in_progress"))

	// Stop work: update status to open
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE issues SET status = ?, stopped_at = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?`)).
		WithArgs("open", sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "", "grava-123").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Insert audit event
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs(sqlmock.AnyArg(), "stop", "unknown", sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()
	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "stop", "grava-123")
	assert.NoError(t, err)
	assert.Contains(t, output, "Stopped work on")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStopCmd_NotInProgress(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// SELECT FOR UPDATE inside transaction returns open (not in_progress)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT status FROM issues WHERE id = ? FOR UPDATE`)).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("open"))

	// No ExpectClose: RunE returns error so PersistentPostRunE is skipped

	_, err = executeCommand(rootCmd, "stop", "grava-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in progress")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStopCmd_IssueNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// SELECT FOR UPDATE inside transaction returns empty result (ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT status FROM issues WHERE id = ? FOR UPDATE`)).
		WithArgs("grava-missing").
		WillReturnRows(sqlmock.NewRows([]string{"status"}))

	// No ExpectClose: RunE returns error so PersistentPostRunE is skipped

	_, err = executeCommand(rootCmd, "stop", "grava-missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStartCmd_JSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT status, COALESCE(assignee, '') FROM issues WHERE id = ? FOR UPDATE`)).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee"}).AddRow("open", ""))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE issues SET status = ?, started_at = ?, stopped_at = NULL, assignee = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?`)).
		WithArgs("in_progress", sqlmock.AnyArg(), "unknown", sqlmock.AnyArg(), "unknown", "", "grava-123").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs(sqlmock.AnyArg(), "start", "unknown", sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "start", "grava-123", "--json")
	require.NoError(t, err)

	// Verify NFR5 JSON shape: {id, status, started_at}
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result))
	assert.Equal(t, "grava-123", result["id"])
	assert.Equal(t, "in_progress", result["status"])
	assert.NotEmpty(t, result["started_at"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStopCmd_JSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT status FROM issues WHERE id = ? FOR UPDATE`)).
		WithArgs("grava-123").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("in_progress"))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE issues SET status = ?, stopped_at = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?`)).
		WithArgs("open", sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "", "grava-123").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs(sqlmock.AnyArg(), "stop", "unknown", sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "stop", "grava-123", "--json")
	require.NoError(t, err)

	// Verify NFR5 JSON shape: {id, status, stopped_at}
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result))
	assert.Equal(t, "grava-123", result["id"])
	assert.Equal(t, "open", result["status"])
	assert.NotEmpty(t, result["stopped_at"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStart_Stop_Cycle(t *testing.T) {
	// Phase 1: start — SELECT FOR UPDATE inside tx, transitions to in_progress
	db1, mock1, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db1)

	mock1.ExpectBegin()
	mock1.ExpectQuery(regexp.QuoteMeta(`SELECT status, COALESCE(assignee, '') FROM issues WHERE id = ? FOR UPDATE`)).
		WithArgs("grava-cycle").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee"}).AddRow("open", ""))
	mock1.ExpectExec(regexp.QuoteMeta(`UPDATE issues SET status = ?, started_at = ?, stopped_at = NULL, assignee = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?`)).
		WithArgs("in_progress", sqlmock.AnyArg(), "unknown", sqlmock.AnyArg(), "unknown", "", "grava-cycle").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock1.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs(sqlmock.AnyArg(), "start", "unknown", sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock1.ExpectCommit()
	mock1.ExpectClose()

	output, err := executeCommand(rootCmd, "start", "grava-cycle")
	assert.NoError(t, err)
	assert.Contains(t, output, "Started work on")
	assert.NoError(t, mock1.ExpectationsWereMet())

	// Phase 2: stop — SELECT FOR UPDATE inside tx, transitions to open
	db2, mock2, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db2)

	mock2.ExpectBegin()
	mock2.ExpectQuery(regexp.QuoteMeta(`SELECT status FROM issues WHERE id = ? FOR UPDATE`)).
		WithArgs("grava-cycle").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("in_progress"))
	mock2.ExpectExec(regexp.QuoteMeta(`UPDATE issues SET status = ?, stopped_at = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?`)).
		WithArgs("open", sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "", "grava-cycle").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock2.ExpectExec(regexp.QuoteMeta(`INSERT INTO events`)).
		WithArgs(sqlmock.AnyArg(), "stop", "unknown", sqlmock.AnyArg(), sqlmock.AnyArg(), "unknown", "unknown", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock2.ExpectCommit()
	mock2.ExpectClose()

	output, err = executeCommand(rootCmd, "stop", "grava-cycle")
	assert.NoError(t, err)
	assert.Contains(t, output, "Stopped work on")
	assert.NoError(t, mock2.ExpectationsWereMet())
}

// --- History command Cobra-boundary tests ---

func TestHistoryCmd_HumanReadable(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	ts := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM issues WHERE id")).
		WithArgs("grava-hist1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("grava-hist1"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT event_type, actor, old_value, new_value, timestamp")).
		WithArgs("grava-hist1").
		WillReturnRows(sqlmock.NewRows([]string{"event_type", "actor", "old_value", "new_value", "timestamp"}).
			AddRow("create", "agent-01", "{}", `{"status":"open"}`, ts))

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "history", "grava-hist1")
	assert.NoError(t, err)
	assert.Contains(t, output, "History for grava-hist1")
	assert.Contains(t, output, "create")
	assert.Contains(t, output, "agent-01")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHistoryCmd_JSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	ts := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM issues WHERE id")).
		WithArgs("grava-hist2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("grava-hist2"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT event_type, actor, old_value, new_value, timestamp")).
		WithArgs("grava-hist2").
		WillReturnRows(sqlmock.NewRows([]string{"event_type", "actor", "old_value", "new_value", "timestamp"}).
			AddRow("create", "agent-01", "{}", `{"status":"open"}`, ts))

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "history", "grava-hist2", "--json")
	assert.NoError(t, err)

	// Verify output is valid JSON array
	var entries []map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &entries))
	require.Len(t, entries, 1)
	assert.Equal(t, "create", entries[0]["event_type"])
	assert.Equal(t, "agent-01", entries[0]["actor"])
	assert.Equal(t, "open", entries[0]["details"].(map[string]any)["status"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHistoryCmd_JSON_IssueNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM issues WHERE id")).
		WithArgs("grava-missing").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	// writeJSONError returns nil, so PersistentPostRunE still runs → Close is called
	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "history", "grava-missing", "--json")
	assert.NoError(t, err) // writeJSONError returns nil

	// Verify JSON error envelope is present in output (stderr merged into same buffer)
	var envelope map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &envelope))
	errObj, ok := envelope["error"].(map[string]any)
	require.True(t, ok, "expected 'error' key in JSON")
	assert.Equal(t, "ISSUE_NOT_FOUND", errObj["code"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHistoryCmd_JSON_EmptyHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM issues WHERE id")).
		WithArgs("grava-empty").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("grava-empty"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT event_type, actor, old_value, new_value, timestamp")).
		WithArgs("grava-empty").
		WillReturnRows(sqlmock.NewRows([]string{"event_type", "actor", "old_value", "new_value", "timestamp"}))

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "history", "grava-empty", "--json")
	assert.NoError(t, err)

	// Empty history should produce an empty JSON array
	var entries []map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &entries))
	assert.Empty(t, entries)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHistoryCmd_JSON_WithSinceFilter(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	sinceTime := time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC)
	ts := time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM issues WHERE id")).
		WithArgs("grava-since1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("grava-since1"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT event_type, actor, old_value, new_value, timestamp")).
		WithArgs("grava-since1", sinceTime).
		WillReturnRows(sqlmock.NewRows([]string{"event_type", "actor", "old_value", "new_value", "timestamp"}).
			AddRow("update", "agent-01", `{"status":"open"}`, `{"status":"closed"}`, ts))

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "history", "grava-since1", "--json", "--since", "2026-03-21")
	assert.NoError(t, err)

	var entries []map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &entries))
	require.Len(t, entries, 1)
	assert.Equal(t, "update", entries[0]["event_type"])
	assert.Equal(t, "closed", entries[0]["details"].(map[string]any)["status"])

	assert.NoError(t, mock.ExpectationsWereMet())
}
