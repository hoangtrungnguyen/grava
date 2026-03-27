package cmdgraph

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadyQueue_EmptyDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// graph.LoadGraphFromDB runs two queries:
	// 1. SELECT issues WHERE status != 'tombstone'
	// 2. SELECT dependencies
	mock.ExpectQuery("SELECT id, title, issue_type, status, priority, created_at, await_type, await_id, ephemeral, metadata FROM issues WHERE status").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "issue_type", "status", "priority",
			"created_at", "await_type", "await_id", "ephemeral", "metadata",
		}))
	mock.ExpectQuery("SELECT from_id, to_id, type, metadata FROM dependencies").
		WillReturnRows(sqlmock.NewRows([]string{"from_id", "to_id", "type", "metadata"}))

	store := dolt.NewClientFromDB(db)
	tasks, err := readyQueue(context.Background(), store, 20)
	require.NoError(t, err)
	assert.NotNil(t, tasks)
	assert.Empty(t, tasks)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReadyQueue_LimitZero(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT id, title, issue_type, status, priority, created_at, await_type, await_id, ephemeral, metadata FROM issues WHERE status").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "issue_type", "status", "priority",
			"created_at", "await_type", "await_id", "ephemeral", "metadata",
		}))
	mock.ExpectQuery("SELECT from_id, to_id, type, metadata FROM dependencies").
		WillReturnRows(sqlmock.NewRows([]string{"from_id", "to_id", "type", "metadata"}))

	store := dolt.NewClientFromDB(db)
	tasks, err := readyQueue(context.Background(), store, 0)
	require.NoError(t, err)
	assert.Empty(t, tasks)
	require.NoError(t, mock.ExpectationsWereMet())
}
