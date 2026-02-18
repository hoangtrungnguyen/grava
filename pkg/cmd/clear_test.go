package cmd

import (
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
)

func TestClearCmd(t *testing.T) {
	t.Run("missing flags", func(t *testing.T) {
		db, _, err := sqlmock.New()
		assert.NoError(t, err)
		Store = dolt.NewClientFromDB(db)

		clearFrom = ""
		clearTo = ""
		_, err = executeCommand(rootCmd, "clear")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required")
	})

	t.Run("invalid date format", func(t *testing.T) {
		db, _, err := sqlmock.New()
		assert.NoError(t, err)
		Store = dolt.NewClientFromDB(db)

		clearFrom = "01-01-2026"
		clearTo = "2026-01-31"
		_, err = executeCommand(rootCmd, "clear", "--from", "01-01-2026", "--to", "2026-01-31")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid --from date format")
	})

	t.Run("from > to", func(t *testing.T) {
		db, _, err := sqlmock.New()
		assert.NoError(t, err)
		Store = dolt.NewClientFromDB(db)

		clearFrom = "2026-02-01"
		clearTo = "2026-01-31"
		_, err = executeCommand(rootCmd, "clear", "--from", "2026-02-01", "--to", "2026-01-31")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be before or equal")
	})

	t.Run("force delete range", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		Store = dolt.NewClientFromDB(db)

		clearFrom = "2026-01-01"
		clearTo = "2026-01-31"
		clearForce = true
		clearIncludeWisp = false

		// 1. SELECT matching IDs
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM issues WHERE created_at >= ? AND created_at < ? AND ephemeral = FALSE`)).
			WithArgs("2026-01-01", "2026-02-01").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("grava-1").AddRow("grava-2"))

		// 2. Tombstone and Delete grava-1
		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO deletions (id, reason, actor, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)`)).
			WithArgs("grava-1", "clear", "grava-clear", "unknown", "unknown", "").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM issues WHERE id = ?`)).
			WithArgs("grava-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		// 3. Tombstone and Delete grava-2
		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO deletions (id, reason, actor, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)`)).
			WithArgs("grava-2", "clear", "grava-clear", "unknown", "unknown", "").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM issues WHERE id = ?`)).
			WithArgs("grava-2").
			WillReturnResult(sqlmock.NewResult(0, 1))

		mock.ExpectClose()

		output, err := executeCommand(rootCmd, "clear", "--from", "2026-01-01", "--to", "2026-01-31", "--force")
		assert.NoError(t, err)
		assert.Contains(t, output, "Cleared 2 issue(s)")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("confirm yes", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		Store = dolt.NewClientFromDB(db)

		clearFrom = "2026-01-01"
		clearTo = "2026-01-31"
		clearForce = false
		clearIncludeWisp = false

		oldReader := clearStdinReader
		clearStdinReader = strings.NewReader("yes\n")
		defer func() { clearStdinReader = oldReader }()

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM issues WHERE created_at >= ? AND created_at < ? AND ephemeral = FALSE`)).
			WithArgs("2026-01-01", "2026-02-01").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("grava-1"))

		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO deletions`)).WithArgs("grava-1", "clear", "grava-clear", "unknown", "unknown", "").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM issues`)).WithArgs("grava-1").WillReturnResult(sqlmock.NewResult(0, 1))

		mock.ExpectClose()

		output, err := executeCommand(rootCmd, "clear", "--from", "2026-01-01", "--to", "2026-01-31")
		assert.NoError(t, err)
		assert.Contains(t, output, "Cleared 1 issue(s)")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("confirm no", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		Store = dolt.NewClientFromDB(db)

		clearFrom = "2026-01-01"
		clearTo = "2026-01-31"
		clearForce = false
		clearIncludeWisp = false

		oldReader := clearStdinReader
		clearStdinReader = strings.NewReader("no\n")
		defer func() { clearStdinReader = oldReader }()

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM issues`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("grava-1"))

		mock.ExpectClose()

		output, err := executeCommand(rootCmd, "clear", "--from", "2026-01-01", "--to", "2026-01-31")
		assert.NoError(t, err)
		assert.Contains(t, output, "Aborted")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("include wisps", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		Store = dolt.NewClientFromDB(db)

		clearFrom = "2026-01-01"
		clearTo = "2026-01-31"
		clearForce = true
		clearIncludeWisp = true

		// Query should NOT have "AND ephemeral = FALSE"
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM issues WHERE created_at >= ? AND created_at < ?`)).
			WithArgs("2026-01-01", "2026-02-01").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("grava-w1"))

		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO deletions`)).WithArgs("grava-w1", "clear", "grava-clear", "unknown", "unknown", "").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM issues`)).WillReturnResult(sqlmock.NewResult(0, 1))

		mock.ExpectClose()

		output, err := executeCommand(rootCmd, "clear", "--from", "2026-01-01", "--to", "2026-01-31", "--force", "--include-wisps")
		assert.NoError(t, err)
		assert.Contains(t, output, "Cleared 1 issue(s)")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("empty range", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		Store = dolt.NewClientFromDB(db)

		clearFrom = "2026-01-01"
		clearTo = "2026-01-31"
		clearForce = true
		clearIncludeWisp = false

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM issues`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))

		mock.ExpectClose()

		output, err := executeCommand(rootCmd, "clear", "--from", "2026-01-01", "--to", "2026-01-31", "--force")
		assert.NoError(t, err)
		assert.Contains(t, output, "No issues found")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
