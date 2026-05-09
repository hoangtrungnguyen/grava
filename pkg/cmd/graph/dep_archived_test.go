package cmdgraph

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/internal/testutil"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// qDepLockIssuesWithStatus is the post-fix lock query: it pulls status alongside
// id so the addDependency guard can reject archived/tombstoned endpoints
// without a second round-trip.
var qDepLockIssuesWithStatus = regexp.QuoteMeta("SELECT id, status FROM issues WHERE id IN (?, ?) FOR UPDATE")

// TestAddDependency_RejectsArchivedFrom verifies that adding a dependency where
// the FROM endpoint is archived returns ISSUE_READ_ONLY (grava-08ea: previously
// the dep command silently accepted writes against archived issues).
func TestAddDependency_RejectsArchivedFrom(t *testing.T) {
	resetFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	d := newDepDeps(dolt.NewClientFromDB(db), false)
	cmd, _ := newDepTestCmd(d)
	cmd.SetContext(context.Background())

	// LoadGraphFromDB (blocking type)
	mock.ExpectQuery(qDepLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("ISSUE-1", "Task 1", "task", "archived", 1, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("ISSUE-2", "Task 2", "task", "open", 2, time.Now(), time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qDepLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()))

	mock.ExpectBegin()
	mock.ExpectQuery(qDepLockIssuesWithStatus).WithArgs("ISSUE-1", "ISSUE-2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).
			AddRow("ISSUE-1", "archived").
			AddRow("ISSUE-2", "open"))
	mock.ExpectRollback()

	err = addDependency(cmd, d, "ISSUE-1", "ISSUE-2")
	require.Error(t, err)
	testutil.AssertGravaError(t, err, "ISSUE_READ_ONLY")
	assert.Contains(t, err.Error(), "ISSUE-1")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestAddDependency_RejectsArchivedTo is the symmetric guard: the TO endpoint
// must also be writable for a dep edge to be created.
func TestAddDependency_RejectsArchivedTo(t *testing.T) {
	resetFlags()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	d := newDepDeps(dolt.NewClientFromDB(db), false)
	cmd, _ := newDepTestCmd(d)
	cmd.SetContext(context.Background())

	mock.ExpectQuery(qDepLoadIssues).
		WillReturnRows(sqlmock.NewRows(issueCols()).
			AddRow("ISSUE-1", "Task 1", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("ISSUE-2", "Task 2", "task", "tombstone", 2, time.Now(), time.Now(), nil, nil, 0, nil))
	mock.ExpectQuery(qDepLoadDeps).
		WillReturnRows(sqlmock.NewRows(depCols()))

	mock.ExpectBegin()
	mock.ExpectQuery(qDepLockIssuesWithStatus).WithArgs("ISSUE-1", "ISSUE-2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).
			AddRow("ISSUE-1", "open").
			AddRow("ISSUE-2", "tombstone"))
	mock.ExpectRollback()

	err = addDependency(cmd, d, "ISSUE-1", "ISSUE-2")
	require.Error(t, err)
	testutil.AssertGravaError(t, err, "ISSUE_READ_ONLY")
	assert.Contains(t, err.Error(), "ISSUE-2")
	require.NoError(t, mock.ExpectationsWereMet())
}
