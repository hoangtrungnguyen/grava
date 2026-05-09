package issues

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaimIssue_HappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, assignee, wisp_heartbeat_at FROM issues WHERE id").
		WithArgs("grava-abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee", "wisp_heartbeat_at"}).AddRow("open", nil, nil))
	mock.ExpectExec("UPDATE issues SET").
		WithArgs("actor1", "model1", "actor1", "grava-abc123def456").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := claimIssue(context.Background(), store, "grava-abc123def456", "actor1", "model1")
	require.NoError(t, err)
	assert.Equal(t, "grava-abc123def456", result.IssueID)
	assert.Equal(t, "in_progress", result.Status)
	assert.Equal(t, "actor1", result.Actor)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimIssue_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, assignee, wisp_heartbeat_at FROM issues WHERE id").
		WithArgs("grava-notfound").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee", "wisp_heartbeat_at"})) // empty → ErrNoRows
	mock.ExpectRollback()

	store := dolt.NewClientFromDB(db)
	_, err = claimIssue(context.Background(), store, "grava-notfound", "actor1", "model1")
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "ISSUE_NOT_FOUND", gravaErr.Code)
}

// TestClaimIssue_AlreadyClaimed: status=in_progress with a FRESH heartbeat
// (well within the 1h TTL) must block with ALREADY_CLAIMED. Regression for
// grava-985d acceptance criterion: "Existing already-claimed-by-active-agent
// path still blocks correctly".
func TestClaimIssue_AlreadyClaimed(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	freshHeartbeat := time.Now().Add(-1 * time.Minute) // 1m ago, definitely not stale

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, assignee, wisp_heartbeat_at FROM issues WHERE id").
		WithArgs("grava-abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee", "wisp_heartbeat_at"}).AddRow("in_progress", "actor1", freshHeartbeat))
	mock.ExpectRollback()

	store := dolt.NewClientFromDB(db)
	_, err = claimIssue(context.Background(), store, "grava-abc123def456", "actor1", "model1")
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "ALREADY_CLAIMED", gravaErr.Code)
}

func TestClaimIssue_InvalidTransition(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, assignee, wisp_heartbeat_at FROM issues WHERE id").
		WithArgs("grava-abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee", "wisp_heartbeat_at"}).AddRow("closed", nil, nil))
	mock.ExpectRollback()

	store := dolt.NewClientFromDB(db)
	_, err = claimIssue(context.Background(), store, "grava-abc123def456", "actor1", "model1")
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "INVALID_STATUS_TRANSITION", gravaErr.Code)
}

// TestClaimIssue_AssignedOpenIssue_Succeeds: exact repro for grava-985d.
// `grava assign X --actor alice` leaves status=open with assignee=alice. A
// subsequent `grava claim X` (by any actor) must succeed — assignee is metadata,
// not a claim guard. Equivalent to TestClaimIssue_AlreadyClaimed_OpenWithAssignee
// but named per the bug's TDD plan.
func TestClaimIssue_AssignedOpenIssue_Succeeds(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, assignee, wisp_heartbeat_at FROM issues WHERE id").
		WithArgs("grava-abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee", "wisp_heartbeat_at"}).AddRow("open", "alice", nil))
	mock.ExpectExec("UPDATE issues SET").
		WithArgs("bob", "model1", "bob", "grava-abc123def456").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := claimIssue(context.Background(), store, "grava-abc123def456", "bob", "model1")
	require.NoError(t, err)
	assert.Equal(t, "in_progress", result.Status)
	assert.Equal(t, "bob", result.Actor, "claimer must overwrite the prior assignee")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestClaimIssue_StaleClaim_RecoverableSucceeds: status=in_progress with a
// heartbeat older than the 1h TTL must be treated as a crashed agent and
// allowed to be re-claimed. Regression guard for the stale-recovery path that
// the grava-985d guard refactor must preserve.
func TestClaimIssue_StaleClaim_RecoverableSucceeds(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	staleHeartbeat := time.Now().Add(-2 * time.Hour) // 2h ago, well past 1h TTL

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, assignee, wisp_heartbeat_at FROM issues WHERE id").
		WithArgs("grava-stale-claim").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee", "wisp_heartbeat_at"}).AddRow("in_progress", "crashed-actor", staleHeartbeat))
	mock.ExpectExec("UPDATE issues SET").
		WithArgs("recovery-actor", "model1", "recovery-actor", "grava-stale-claim").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := claimIssue(context.Background(), store, "grava-stale-claim", "recovery-actor", "model1")
	require.NoError(t, err)
	assert.Equal(t, "in_progress", result.Status)
	assert.Equal(t, "recovery-actor", result.Actor)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestClaimIssue_AlreadyClaimed_OpenWithAssignee: status is "open" with an existing
// assignee. Assignees no longer block claims — only in_progress status does.
// An open issue with an assignee should be successfully claimed.
func TestClaimIssue_AlreadyClaimed_OpenWithAssignee(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, assignee, wisp_heartbeat_at FROM issues WHERE id").
		WithArgs("grava-abc123def456").
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee", "wisp_heartbeat_at"}).AddRow("open", "sneaky-actor", nil))
	mock.ExpectExec("UPDATE issues SET").
		WithArgs("actor1", "model1", "actor1", "grava-abc123def456").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := claimIssue(context.Background(), store, "grava-abc123def456", "actor1", "model1")
	require.NoError(t, err)
	assert.Equal(t, "grava-abc123def456", result.IssueID)
	assert.Equal(t, "in_progress", result.Status)
	assert.Equal(t, "actor1", result.Actor)
	require.NoError(t, mock.ExpectationsWereMet())
}


// TestRollbackClaimDB verifies that rollback restores DB state on worktree failure.
func TestRollbackClaimDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	issueID := "grava-rollback-test"

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE issues SET status='open'").
		WithArgs(issueID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	err = rollbackClaimDB(context.Background(), store, issueID)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestRollbackClaimDB_DBError verifies error handling when rollback fails.
func TestRollbackClaimDB_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	issueID := "grava-rollback-error"

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE issues SET status='open'").
		WithArgs(issueID).
		WillReturnError(sql.ErrConnDone)

	store := dolt.NewClientFromDB(db)
	err = rollbackClaimDB(context.Background(), store, issueID)
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "DB_UNREACHABLE", gravaErr.Code)
}
