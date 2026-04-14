package reserve

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMock returns a dolt.Store backed by sqlmock and the mock controller.
func newMock(t *testing.T) (dolt.Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() }) //nolint:errcheck
	return dolt.NewClientFromDB(db), mock
}

var (
	qInsertReservation = regexp.QuoteMeta(`INSERT INTO file_reservations`)
	qCheckConflict     = regexp.QuoteMeta(`SELECT agent_id, expires_ts FROM file_reservations`)
	qListReservations  = regexp.QuoteMeta(`SELECT id, project_id, agent_id, path_pattern, exclusive, COALESCE(reason,''), created_ts, expires_ts`)
	qReleaseQuery      = regexp.QuoteMeta(`UPDATE file_reservations`)
)

// TestDeclareReservation_ExclusiveSuccess verifies that declaring an exclusive
// lease inserts a row when no conflicting lease exists.
func TestDeclareReservation_ExclusiveSuccess(t *testing.T) {
	store, mock := newMock(t)

	// No conflict found.
	mock.ExpectQuery(qCheckConflict).
		WillReturnRows(sqlmock.NewRows([]string{"agent_id", "expires_ts"}))
	// INSERT succeeds.
	mock.ExpectExec(qInsertReservation).
		WillReturnResult(sqlmock.NewResult(1, 1))

	p := DeclareParams{
		PathPattern: "src/cmd/issues/*.go",
		AgentID:     "agent-01",
		ProjectID:   "default",
		Exclusive:   true,
		TTLMinutes:  30,
	}
	result, err := DeclareReservation(context.Background(), store, p)

	require.NoError(t, err)
	assert.Equal(t, "agent-01", result.Reservation.AgentID)
	assert.Equal(t, "src/cmd/issues/*.go", result.Reservation.PathPattern)
	assert.True(t, result.Reservation.Exclusive)
	assert.True(t, result.Reservation.ExpiresTS.After(time.Now()))
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestDeclareReservation_NonExclusiveNoConflictCheck verifies that a non-exclusive
// lease skips the conflict query and inserts directly.
func TestDeclareReservation_NonExclusiveNoConflictCheck(t *testing.T) {
	store, mock := newMock(t)

	// Non-exclusive: no conflict query expected.
	mock.ExpectExec(qInsertReservation).
		WillReturnResult(sqlmock.NewResult(1, 1))

	p := DeclareParams{
		PathPattern: "src/cmd/issues/*.go",
		AgentID:     "agent-02",
		Exclusive:   false,
		TTLMinutes:  15,
	}
	result, err := DeclareReservation(context.Background(), store, p)

	require.NoError(t, err)
	assert.False(t, result.Reservation.Exclusive)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestDeclareReservation_ExclusiveConflict verifies that declaring an exclusive
// lease when another agent holds one returns FILE_RESERVATION_CONFLICT.
func TestDeclareReservation_ExclusiveConflict(t *testing.T) {
	store, mock := newMock(t)

	expiresAt := time.Now().Add(20 * time.Minute)
	mock.ExpectQuery(qCheckConflict).
		WillReturnRows(sqlmock.NewRows([]string{"agent_id", "expires_ts"}).
			AddRow("agent-99", expiresAt))

	p := DeclareParams{
		PathPattern: "src/cmd/issues/*.go",
		AgentID:     "agent-01",
		Exclusive:   true,
		TTLMinutes:  30,
	}
	_, err := DeclareReservation(context.Background(), store, p)

	require.Error(t, err)
	var gerr *gravaerrors.GravaError
	require.ErrorAs(t, err, &gerr)
	assert.Equal(t, "FILE_RESERVATION_CONFLICT", gerr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestDeclareReservation_ConflictCheckDBError verifies that a DB error during
// the conflict check is propagated — not silently treated as "no conflict".
func TestDeclareReservation_ConflictCheckDBError(t *testing.T) {
	store, mock := newMock(t)

	mock.ExpectQuery(qCheckConflict).
		WillReturnError(assert.AnError)

	p := DeclareParams{
		PathPattern: "src/cmd/issues/*.go",
		AgentID:     "agent-01",
		Exclusive:   true,
		TTLMinutes:  30,
	}
	_, err := DeclareReservation(context.Background(), store, p)

	require.Error(t, err, "DB error during conflict check must not be swallowed")
	// Must NOT proceed to INSERT — no expectations left unfulfilled.
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestDeclareReservation_InsertDBError verifies that an INSERT failure is propagated.
func TestDeclareReservation_InsertDBError(t *testing.T) {
	store, mock := newMock(t)

	// No conflict.
	mock.ExpectQuery(qCheckConflict).
		WillReturnRows(sqlmock.NewRows([]string{"agent_id", "expires_ts"}))
	// INSERT fails.
	mock.ExpectExec(qInsertReservation).
		WillReturnError(assert.AnError)

	p := DeclareParams{
		PathPattern: "src/cmd/issues/*.go",
		AgentID:     "agent-01",
		Exclusive:   true,
		TTLMinutes:  30,
	}
	_, err := DeclareReservation(context.Background(), store, p)

	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestDeclareReservation_MissingPathPattern verifies that missing path returns an error.
func TestDeclareReservation_MissingPathPattern(t *testing.T) {
	store, _ := newMock(t)

	_, err := DeclareReservation(context.Background(), store, DeclareParams{AgentID: "a"})
	require.Error(t, err)
	var gerr *gravaerrors.GravaError
	require.ErrorAs(t, err, &gerr)
	assert.Equal(t, "MISSING_REQUIRED_FIELD", gerr.Code)
}

// TestListReservations_ReturnsActiveOnly verifies that ListReservations only
// returns non-expired, non-released rows.
func TestListReservations_ReturnsActiveOnly(t *testing.T) {
	store, mock := newMock(t)

	now := time.Now()
	expires := now.Add(25 * time.Minute)
	mock.ExpectQuery(qListReservations).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "project_id", "agent_id", "path_pattern", "exclusive", "reason", "created_ts", "expires_ts",
		}).AddRow("res-aabbcc", "default", "agent-01", "src/*.go", true, "", now, expires))

	reservations, err := ListReservations(context.Background(), store, "default")

	require.NoError(t, err)
	require.Len(t, reservations, 1)
	assert.Equal(t, "res-aabbcc", reservations[0].ID)
	assert.Equal(t, "agent-01", reservations[0].AgentID)
	assert.True(t, reservations[0].Exclusive)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestListReservations_Empty verifies that an empty result returns nil slice.
func TestListReservations_Empty(t *testing.T) {
	store, mock := newMock(t)

	mock.ExpectQuery(qListReservations).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "project_id", "agent_id", "path_pattern", "exclusive", "reason", "created_ts", "expires_ts",
		}))

	reservations, err := ListReservations(context.Background(), store, "default")

	require.NoError(t, err)
	assert.Empty(t, reservations)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestReleaseReservation_Success verifies that releasing an active reservation
// sets released_ts and reports success.
func TestReleaseReservation_Success(t *testing.T) {
	store, mock := newMock(t)

	mock.ExpectExec(qReleaseQuery).
		WithArgs("res-aabbcc").
		WillReturnResult(sqlmock.NewResult(1, 1)) // 1 row affected

	err := ReleaseReservation(context.Background(), store, "res-aabbcc")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestReleaseReservation_NotFound verifies that releasing a non-existent or
// already-released reservation returns RESERVATION_NOT_FOUND.
func TestReleaseReservation_NotFound(t *testing.T) {
	store, mock := newMock(t)

	mock.ExpectExec(qReleaseQuery).
		WithArgs("res-missing").
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected

	err := ReleaseReservation(context.Background(), store, "res-missing")
	require.Error(t, err)
	var gerr *gravaerrors.GravaError
	require.ErrorAs(t, err, &gerr)
	assert.Equal(t, "RESERVATION_NOT_FOUND", gerr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestReleaseReservation_RowsAffectedError verifies graceful handling when
// RowsAffected() itself returns an error.
func TestReleaseReservation_RowsAffectedError(t *testing.T) {
	store, mock := newMock(t)

	mock.ExpectExec(qReleaseQuery).
		WithArgs("res-bad").
		WillReturnResult(sqlmock.NewErrorResult(sql.ErrNoRows))

	err := ReleaseReservation(context.Background(), store, "res-bad")
	require.Error(t, err)
}

// TestGenerateReservationID verifies the ID format "res-XXXXXX".
func TestGenerateReservationID(t *testing.T) {
	id := generateReservationID()
	assert.Regexp(t, `^res-[0-9a-f]{6}$`, id)
}
