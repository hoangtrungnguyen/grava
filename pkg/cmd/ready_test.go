package cmd

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
)

func TestReadyCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)
	defer db.Close() //nolint:errcheck

	// Mock issues query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, issue_type, status, priority, created_at, updated_at, await_type, await_id, ephemeral, metadata FROM issues WHERE status != 'tombstone'")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "status", "priority", "created_at", "updated_at", "await_type", "await_id", "ephemeral", "metadata"}).
			AddRow("grava-1", "Ready Task", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("grava-2", "Blocked Task", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil))

	// Mock dependencies query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT from_id, to_id, type, metadata FROM dependencies")).
		WillReturnRows(sqlmock.NewRows([]string{"from_id", "to_id", "type", "metadata"}).
			AddRow("grava-1", "grava-2", "blocks", nil))

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "ready")
	assert.NoError(t, err)
	assert.Contains(t, output, "grava-1")
	assert.Contains(t, output, "Ready Task")
	assert.NotContains(t, output, "grava-2") // grava-2 is blocked by grava-1
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBlockedCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)
	defer db.Close() //nolint:errcheck

	// Mock issues query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, issue_type, status, priority, created_at, updated_at, await_type, await_id, ephemeral, metadata FROM issues WHERE status != 'tombstone'")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "issue_type", "status", "priority", "created_at", "updated_at", "await_type", "await_id", "ephemeral", "metadata"}).
			AddRow("grava-1", "Ready Task", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("grava-2", "Blocked Task", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil))

	// Mock dependencies query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT from_id, to_id, type, metadata FROM dependencies")).
		WillReturnRows(sqlmock.NewRows([]string{"from_id", "to_id", "type", "metadata"}).
			AddRow("grava-1", "grava-2", "blocks", nil))

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "blocked")
	assert.NoError(t, err)
	assert.Contains(t, output, "grava-2")
	assert.Contains(t, output, "Blocked Task")
	assert.Contains(t, output, "grava-1")                // The blocker
	assert.NotContains(t, output, "grava-1\tReady Task") // grava-1 itself is not blocked
	assert.NoError(t, mock.ExpectationsWereMet())
}
