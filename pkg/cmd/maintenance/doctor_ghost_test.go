package maintenance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	qSelectInProgress = regexp.QuoteMeta(`SELECT id FROM issues WHERE status = 'in_progress'`)
	qHealGhost        = regexp.QuoteMeta(`UPDATE issues SET status='open', assignee=NULL, agent_model=NULL, updated_at=NOW(), updated_by=? WHERE id=? AND status='in_progress'`)
	qLogEventGhost    = regexp.QuoteMeta(`INSERT INTO events`)
)

// --- queryGhostWorktrees ---

func TestQueryGhostWorktrees_NoInProgress(t *testing.T) {
	store, mock := newDoctorMock(t)
	mock.ExpectQuery(qSelectInProgress).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	ids, err := queryGhostWorktrees(context.Background(), store, t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, ids)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryGhostWorktrees_WorktreeExists_NotGhost(t *testing.T) {
	store, mock := newDoctorMock(t)
	mock.ExpectQuery(qSelectInProgress).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ISS-001"))

	cwd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(cwd, ".worktree", "ISS-001"), 0o755))

	ids, err := queryGhostWorktrees(context.Background(), store, cwd)
	require.NoError(t, err)
	assert.Empty(t, ids, "issue with extant worktree must not be reported as ghost")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryGhostWorktrees_WorktreeMissing_IsGhost(t *testing.T) {
	store, mock := newDoctorMock(t)
	mock.ExpectQuery(qSelectInProgress).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow("ISS-ghost-a").
			AddRow("ISS-ghost-b"))

	cwd := t.TempDir() // no .worktree/<id> directories

	ids, err := queryGhostWorktrees(context.Background(), store, cwd)
	require.NoError(t, err)
	require.Len(t, ids, 2)
	assert.Equal(t, "ISS-ghost-a", ids[0])
	assert.Equal(t, "ISS-ghost-b", ids[1])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryGhostWorktrees_Mixed(t *testing.T) {
	store, mock := newDoctorMock(t)
	mock.ExpectQuery(qSelectInProgress).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow("ISS-alive").
			AddRow("ISS-ghost"))

	cwd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(cwd, ".worktree", "ISS-alive"), 0o755))

	ids, err := queryGhostWorktrees(context.Background(), store, cwd)
	require.NoError(t, err)
	require.Equal(t, []string{"ISS-ghost"}, ids, "only the worktree-less issue is a ghost")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryGhostWorktrees_DBError(t *testing.T) {
	store, mock := newDoctorMock(t)
	mock.ExpectQuery(qSelectInProgress).
		WillReturnError(fmt.Errorf("connection lost"))

	ids, err := queryGhostWorktrees(context.Background(), store, t.TempDir())
	require.Error(t, err)
	assert.Nil(t, ids)
	assert.Contains(t, err.Error(), "connection lost")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- healGhostWorktrees ---

func TestHealGhostWorktrees_EmptyInput(t *testing.T) {
	store, mock := newDoctorMock(t)
	// No DB calls expected.
	n, err := healGhostWorktrees(context.Background(), store, "doctor", "m", nil)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHealGhostWorktrees_Success(t *testing.T) {
	store, mock := newDoctorMock(t)

	mock.ExpectBegin()
	mock.ExpectExec(qHealGhost).
		WithArgs("doctor", "ISS-g1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(qHealGhost).
		WithArgs("doctor", "ISS-g2").
		WillReturnResult(sqlmock.NewResult(0, 1))
	// Audit events: one INSERT per ID.
	mock.ExpectExec(qLogEventGhost).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(qLogEventGhost).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	n, err := healGhostWorktrees(context.Background(), store, "doctor", "m", []string{"ISS-g1", "ISS-g2"})
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHealGhostWorktrees_AlreadyReset_RowsAffectedZero(t *testing.T) {
	// Issue raced back to 'open' between query and heal — UPDATE matches nothing.
	// Not an error, but healed count reflects reality.
	store, mock := newDoctorMock(t)

	mock.ExpectBegin()
	mock.ExpectExec(qHealGhost).
		WithArgs("doctor", "ISS-already-open").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(qLogEventGhost).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	n, err := healGhostWorktrees(context.Background(), store, "doctor", "m", []string{"ISS-already-open"})
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHealGhostWorktrees_DBError_Rollback(t *testing.T) {
	store, mock := newDoctorMock(t)

	mock.ExpectBegin()
	mock.ExpectExec(qHealGhost).
		WithArgs("doctor", "ISS-bad").
		WillReturnError(fmt.Errorf("deadlock"))
	mock.ExpectRollback()

	_, err := healGhostWorktrees(context.Background(), store, "doctor", "m", []string{"ISS-bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadlock")
	assert.NoError(t, mock.ExpectationsWereMet())
}
