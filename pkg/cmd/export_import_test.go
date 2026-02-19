package cmd

import (
	"io/ioutil"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
)

func TestExportCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	createdAt := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt

	rows := sqlmock.NewRows([]string{
		"id", "title", "description", "issue_type", "priority", "status", "metadata",
		"created_at", "updated_at", "created_by", "updated_by", "agent_model", "affected_files", "ephemeral",
	}).AddRow(
		"grava-1", "Export Test", "Desc", "task", 1, "open", "{}",
		createdAt, updatedAt, "user", "user", "", "[]", false,
	)

	// Mock Issue Query
	// Matches strict sequence of columns
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, title, description, issue_type, priority, status, metadata, created_at, updated_at, created_by, updated_by, agent_model, affected_files, ephemeral FROM issues WHERE ephemeral = 0`)).
		WillReturnRows(rows)

	// Mock Dependency Query
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT d.from_id, d.to_id, d.type, d.created_by, d.updated_by, d.agent_model FROM dependencies d`)).
		WillReturnRows(sqlmock.NewRows([]string{"from_id", "to_id", "type", "created_by", "updated_by", "agent_model"}))

	// Run Export to stdout (captured via pipe?)
	// Actually executeCommand captures stdout.

	// Reset flags
	exportFile = ""
	exportIncludeWisps = false
	exportSkipTombstones = false

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "export")
	assert.NoError(t, err)

	// Verify JSONL
	assert.Contains(t, output, `"id":"grava-1"`)
	assert.Contains(t, output, `"title":"Export Test"`)
	assert.Contains(t, output, `"type":"issue"`)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestImportCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	// Create temp file
	content := `{"type":"issue","data":{"id":"grava-1","title":"Imported","description":"Desc","issue_type":"task","priority":1,"status":"open","created_at":"2024-01-01T10:00:00Z","updated_at":"2024-01-01T10:00:00Z","created_by":"user","updated_by":"user","ephemeral":false,"metadata":{},"affected_files":[]}}
{"type":"dependency","data":{"from_id":"grava-1","to_id":"grava-2","type":"blocks","created_by":"user","updated_by":"user"}}`

	tmpFile, err := ioutil.TempFile("", "grava_import_test.jsonl")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	assert.NoError(t, err)
	tmpFile.Close()

	// Reset flags
	importFile = ""
	importOverwrite = false
	importSkipExisting = false

	// Expectations
	mock.ExpectBegin()

	// 1. Issue Insert
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO issues`)).
		WithArgs("grava-1", "Imported", "Desc", "task", 1, "open", "{}", sqlmock.AnyArg(), sqlmock.AnyArg(), "user", "user", nil, "[]", false).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 2. Dependency Insert
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO dependencies`)).
		WithArgs("grava-1", "grava-2", "blocks", "user", "user", nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "import", "--file", tmpFile.Name())
	assert.NoError(t, err)
	assert.Contains(t, output, "Imported 1 items")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestImportCmdSkipExisting(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	content := `{"type":"issue","data":{"id":"grava-1","title":"Exist","description":"","issue_type":"task","priority":1,"status":"open","created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z","created_by":"","updated_by":"","ephemeral":false,"metadata":{},"affected_files":[]}}`
	tmpFile, err := ioutil.TempFile("", "grava_import_skip.jsonl")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(content)
	tmpFile.Close()

	// Reset flags
	importFile = ""
	importOverwrite = false
	importSkipExisting = true // FLAG SET

	mock.ExpectBegin()

	// Expect INSERT IGNORE
	mock.ExpectExec(regexp.QuoteMeta(`INSERT IGNORE INTO issues`)).
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected = skipped

	mock.ExpectCommit()

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "import", "--file", tmpFile.Name(), "--skip-existing")
	assert.NoError(t, err)
	assert.Contains(t, output, "Skipped: 1")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestImportCmdOverwrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	Store = dolt.NewClientFromDB(db)

	content := `{"type":"issue","data":{"id":"grava-1","title":"Upd","description":"","issue_type":"task","priority":1,"status":"open","created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z","created_by":"","updated_by":"","ephemeral":false,"metadata":{},"affected_files":[]}}`
	tmpFile, err := ioutil.TempFile("", "grava_import_over.jsonl")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(content)
	tmpFile.Close()

	// Reset flags
	importFile = ""
	importOverwrite = true // FLAG SET
	importSkipExisting = false

	mock.ExpectBegin()

	// Expect INSERT ... ON DUPLICATE KEY UPDATE
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO issues`) + `.*` + regexp.QuoteMeta(`ON DUPLICATE KEY UPDATE`)).
		WillReturnResult(sqlmock.NewResult(1, 2)) // MySQL returns 2 for update

	mock.ExpectCommit()

	mock.ExpectClose()

	output, err := executeCommand(rootCmd, "import", "--file", tmpFile.Name(), "--overwrite")
	assert.NoError(t, err)
	assert.Contains(t, output, "Updated: 1")

	assert.NoError(t, mock.ExpectationsWereMet())
}
