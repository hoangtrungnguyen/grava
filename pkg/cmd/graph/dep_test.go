package cmdgraph

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddDependency_SelfLoop(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	actor := "Alice"
	model := "Gemini"
	jsonOutput := false
	var store dolt.Store = dolt.NewClientFromDB(db)
	d := &cmddeps.Deps{
		Store:      &store,
		Actor:      &actor,
		AgentModel: &model,
		OutputJSON: &jsonOutput,
	}

	cmd := newDepCmd(d)
	err = addDependency(cmd, d, "ISSUE-1", "ISSUE-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "different issues")
}

func TestRemoveDependency_Flag(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	actor := "Alice"
	model := "Gemini"
	jsonOutput := false
	var store dolt.Store = dolt.NewClientFromDB(db)
	d := &cmddeps.Deps{
		Store:      &store,
		Actor:      &actor,
		AgentModel: &model,
		OutputJSON: &jsonOutput,
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM issues WHERE id IN (?, ?) FOR UPDATE")).
		WithArgs("ISSUE-1", "ISSUE-2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ISSUE-1").AddRow("ISSUE-2"))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM dependencies WHERE from_id = ? AND to_id = ? AND type = ?")).
		WithArgs("ISSUE-1", "ISSUE-2", "blocks").
		WillReturnResult(sqlmock.NewResult(0, 1))
	// LogEventTx is called twice in removeDependency (lines 215 and 223)
	for i := 0; i < 2; i++ {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO events (issue_id, event_type, actor, old_value, new_value, created_by, updated_by, agent_model, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)")).
			WithArgs("ISSUE-1", "dependency_remove", "Alice", sqlmock.AnyArg(), "{}", "Alice", "Alice", "Gemini", sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
	}
	mock.ExpectCommit()

	cmd := newDepCmd(d)
	cmd.SetArgs([]string{"ISSUE-1", "ISSUE-2", "--remove"})
	err = cmd.Execute()
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
