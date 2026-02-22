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
	defer db.Close()

	// Mock issues query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, status, priority, created_at, await_type, await_id FROM issues")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "status", "priority", "created_at", "await_type", "await_id"}).
			AddRow("grava-1", "Ready Task", "open", 1, time.Now(), nil, nil).
			AddRow("grava-2", "Blocked Task", "open", 1, time.Now(), nil, nil))

	// Mock dependencies query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT from_id, to_id, type FROM dependencies")).
		WillReturnRows(sqlmock.NewRows([]string{"from_id", "to_id", "type"}).
			AddRow("grava-1", "grava-2", "blocks"))

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
	defer db.Close()

	// Mock issues query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, title, status, priority, created_at, await_type, await_id FROM issues")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "status", "priority", "created_at", "await_type", "await_id"}).
			AddRow("grava-1", "Ready Task", "open", 1, time.Now(), nil, nil).
			AddRow("grava-2", "Blocked Task", "open", 1, time.Now(), nil, nil))

	// Mock dependencies query
	mock.ExpectQuery(regexp.QuoteMeta("SELECT from_id, to_id, type FROM dependencies")).
		WillReturnRows(sqlmock.NewRows([]string{"from_id", "to_id", "type"}).
			AddRow("grava-1", "grava-2", "blocks"))

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "blocked")
	assert.NoError(t, err)
	assert.Contains(t, output, "grava-2")
	assert.Contains(t, output, "Blocked Task")
	assert.Contains(t, output, "grava-1")                // The blocker
	assert.NotContains(t, output, "grava-1\tReady Task") // grava-1 itself is not blocked
	assert.NoError(t, mock.ExpectationsWereMet())
}
