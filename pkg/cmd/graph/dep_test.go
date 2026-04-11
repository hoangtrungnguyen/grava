package cmdgraph

import (
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
	mock.ExpectQuery("SELECT id FROM issues WHERE id IN .* FOR UPDATE").
		WithArgs("ISSUE-1", "ISSUE-2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ISSUE-1").AddRow("ISSUE-2"))
	mock.ExpectExec("DELETE FROM dependencies").
		WithArgs("ISSUE-1", "ISSUE-2", "blocks").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO events.*").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	cmd := newDepCmd(d)
	cmd.SetArgs([]string{"ISSUE-1", "ISSUE-2", "--remove"})
	err = cmd.Execute()
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
