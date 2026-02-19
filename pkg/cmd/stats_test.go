package cmd

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
)

func TestStatsCmd(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	Store = dolt.NewClientFromDB(db)

	// Reset global flag to default
	statsDays = 7

	// Mock 1: By Status
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, COUNT(*) FROM issues WHERE ephemeral = 0 GROUP BY status")).
		WillReturnRows(sqlmock.NewRows([]string{"status", "count"}).
			AddRow("open", 5).
			AddRow("closed", 3).
			AddRow("in_progress", 2))

	// Mock 2: By Priority
	mock.ExpectQuery(regexp.QuoteMeta("SELECT priority, COUNT(*) FROM issues WHERE ephemeral = 0 GROUP BY priority")).
		WillReturnRows(sqlmock.NewRows([]string{"priority", "count"}).
			AddRow(1, 2).
			AddRow(2, 3).
			AddRow(3, 5))

	// Mock 3: By Author
	mock.ExpectQuery(regexp.QuoteMeta("SELECT created_by, COUNT(*) FROM issues WHERE ephemeral = 0 GROUP BY created_by ORDER BY COUNT(*) DESC LIMIT 10")).
		WillReturnRows(sqlmock.NewRows([]string{"created_by", "count"}).
			AddRow("alice", 6).
			AddRow("bob", 4))

	// Mock 4: By Assignee
	mock.ExpectQuery(regexp.QuoteMeta("SELECT assignee, COUNT(*) FROM issues WHERE ephemeral = 0 AND assignee IS NOT NULL AND assignee != '' GROUP BY assignee ORDER BY COUNT(*) DESC LIMIT 10")).
		WillReturnRows(sqlmock.NewRows([]string{"assignee", "count"}).
			AddRow("alice", 5).
			AddRow("bob", 2))

	// Mock 5: Created By Date (using 7 days default)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT DATE_FORMAT(created_at, '%Y-%m-%d') as day, COUNT(*) FROM issues WHERE ephemeral = 0 AND created_at >= DATE_SUB(NOW(), INTERVAL 7 DAY) GROUP BY day ORDER BY day DESC")).
		WillReturnRows(sqlmock.NewRows([]string{"day", "count"}).
			AddRow(time.Now().Format("2006-01-02"), 2))

	// Mock 6: Closed By Date
	mock.ExpectQuery(regexp.QuoteMeta("SELECT DATE_FORMAT(updated_at, '%Y-%m-%d') as day, COUNT(*) FROM issues WHERE ephemeral = 0 AND status = 'closed' AND updated_at >= DATE_SUB(NOW(), INTERVAL 7 DAY) GROUP BY day ORDER BY day DESC")).
		WillReturnRows(sqlmock.NewRows([]string{"day", "count"}).
			AddRow(time.Now().Format("2006-01-02"), 1))

	mock.ExpectClose()

	// Run command
	output, err := executeCommand(rootCmd, "stats")
	assert.NoError(t, err)

	// Assert Output
	assert.Contains(t, output, "Total Issues:")
	assert.Contains(t, output, "10")
	assert.Contains(t, output, "Open Issues:")
	assert.Contains(t, output, "7")
	assert.Contains(t, output, "Closed Issues:")
	assert.Contains(t, output, "3")

	assert.Regexp(t, regexp.MustCompile(`alice:\s+6`), output)
	assert.Regexp(t, regexp.MustCompile(`bob:\s+4`), output)

	assert.NoError(t, mock.ExpectationsWereMet())
}
