package synccmd

import (
	"context"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportIssues_EmptyInput(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := importIssues(context.Background(), store, strings.NewReader(""), false, false)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Imported)
	assert.Equal(t, 0, result.Updated)
	assert.Equal(t, 0, result.Skipped)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImportIssues_SingleIssue(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC().Truncate(time.Second)
	agentModel := "model1"
	line := `{"type":"issue","data":{"id":"grava-test001","title":"Test Issue","description":"desc","issue_type":"task","priority":2,"status":"open","metadata":{},"created_at":"` +
		now.Format(time.RFC3339) + `","updated_at":"` + now.Format(time.RFC3339) + `","created_by":"actor1","updated_by":"actor1","agent_model":"` + agentModel + `","affected_files":[],"ephemeral":false}}`

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO issues").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := importIssues(context.Background(), store, strings.NewReader(line), false, false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)
	assert.Equal(t, 0, result.Updated)
	assert.Equal(t, 0, result.Skipped)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImportIssues_SkipExisting(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC().Truncate(time.Second)
	agentModel := "model1"
	line := `{"type":"issue","data":{"id":"grava-test001","title":"Test Issue","description":"desc","issue_type":"task","priority":2,"status":"open","metadata":{},"created_at":"` +
		now.Format(time.RFC3339) + `","updated_at":"` + now.Format(time.RFC3339) + `","created_by":"actor1","updated_by":"actor1","agent_model":"` + agentModel + `","affected_files":[],"ephemeral":false}}`

	mock.ExpectBegin()
	// INSERT IGNORE returns 0 affected rows when row exists
	mock.ExpectExec("INSERT IGNORE INTO issues").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := importIssues(context.Background(), store, strings.NewReader(line), false, true)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Imported)
	assert.Equal(t, 0, result.Updated)
	assert.Equal(t, 1, result.Skipped)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImportIssues_TwoIssuesImported(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC().Truncate(time.Second)
	agentModel := "model1"
	makeLine := func(id string) string {
		return `{"type":"issue","data":{"id":"` + id + `","title":"Issue ` + id + `","description":"desc","issue_type":"task","priority":2,"status":"open","metadata":{},"created_at":"` +
			now.Format(time.RFC3339) + `","updated_at":"` + now.Format(time.RFC3339) + `","created_by":"actor1","updated_by":"actor1","agent_model":"` + agentModel + `","affected_files":[],"ephemeral":false}}`
	}
	input := makeLine("grava-aaa001") + "\n" + makeLine("grava-bbb002")

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO issues").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO issues").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := importIssues(context.Background(), store, strings.NewReader(input), false, false)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Imported)
	assert.Equal(t, 0, result.Updated)
	assert.Equal(t, 0, result.Skipped)
	require.NoError(t, mock.ExpectationsWereMet())
}
