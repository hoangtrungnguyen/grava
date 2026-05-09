package reserve

import (
	"context"
	"database/sql"
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
	qInsertReservation = `INSERT INTO file_reservations`
	qCheckConflict     = `SELECT agent_id, path_pattern, expires_ts FROM file_reservations`
	qListReservations  = `SELECT id, project_id, agent_id, path_pattern, .exclusive., COALESCE`
	qReleaseQuery      = `UPDATE file_reservations`
)

// TestDeclareReservation_ExclusiveSuccess verifies that declaring an exclusive
// lease inserts a row when no conflicting lease exists.
func TestDeclareReservation_ExclusiveSuccess(t *testing.T) {
	store, mock := newMock(t)

	// Transaction: BEGIN, conflict check, INSERT, COMMIT.
	mock.ExpectBegin()
	mock.ExpectQuery(qCheckConflict).
		WithArgs("default", "agent-01").
		WillReturnRows(sqlmock.NewRows([]string{"agent_id", "path_pattern", "expires_ts"}))
	mock.ExpectExec(qInsertReservation).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// Re-read after insert (GetReservation).
	future := time.Now().Add(30 * time.Minute)
	mock.ExpectQuery(qGetReservation).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "project_id", "agent_id", "path_pattern", "exclusive", "reason",
			"created_ts", "expires_ts", "released_ts",
		}).AddRow("res-mock", "default", "agent-01", "src/cmd/issues/*.go", true, "", time.Now(), future, nil))

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
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestDeclareReservation_NonExclusiveNoConflictCheck verifies that a non-exclusive
// lease skips the conflict query and inserts directly.
func TestDeclareReservation_NonExclusiveNoConflictCheck(t *testing.T) {
	store, mock := newMock(t)

	// Non-exclusive: conflict check still runs (shared must respect existing exclusive).
	mock.ExpectBegin()
	mock.ExpectQuery(qCheckConflict).
		WithArgs("default", "agent-02").
		WillReturnRows(sqlmock.NewRows([]string{"agent_id", "path_pattern", "expires_ts"}))
	mock.ExpectExec(qInsertReservation).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// Re-read after insert (GetReservation).
	future := time.Now().Add(15 * time.Minute)
	mock.ExpectQuery(qGetReservation).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "project_id", "agent_id", "path_pattern", "exclusive", "reason",
			"created_ts", "expires_ts", "released_ts",
		}).AddRow("res-mock", "default", "agent-02", "src/cmd/issues/*.go", false, "", time.Now(), future, nil))

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
	mock.ExpectBegin()
	mock.ExpectQuery(qCheckConflict).
		WithArgs("default", "agent-01").
		WillReturnRows(sqlmock.NewRows([]string{"agent_id", "path_pattern", "expires_ts"}).
			AddRow("agent-99", "src/cmd/issues/*.go", expiresAt))
	mock.ExpectRollback()

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

	mock.ExpectBegin()
	mock.ExpectQuery(qCheckConflict).
		WithArgs("default", "agent-01").
		WillReturnError(assert.AnError)
	mock.ExpectRollback()

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

	// Transaction: BEGIN, conflict check (no conflict), INSERT fails.
	mock.ExpectBegin()
	mock.ExpectQuery(qCheckConflict).
		WithArgs("default", "agent-01").
		WillReturnRows(sqlmock.NewRows([]string{"agent_id", "path_pattern", "expires_ts"}))
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

// TestDeclareReservation_RejectsTTLZero is a regression for grava-399b: passing
// TTLMinutes=0 must be rejected with INVALID_TTL rather than silently falling
// back to the 30-minute default. No DB interaction should occur — validation
// happens before BeginTx.
func TestDeclareReservation_RejectsTTLZero(t *testing.T) {
	store, mock := newMock(t)

	p := DeclareParams{
		PathPattern: "src/cmd/issues/*.go",
		AgentID:     "agent-01",
		ProjectID:   "default",
		Exclusive:   true,
		TTLMinutes:  0,
	}
	_, err := DeclareReservation(context.Background(), store, p)

	require.Error(t, err, "TTL=0 must be rejected, not silently defaulted")
	var gerr *gravaerrors.GravaError
	require.ErrorAs(t, err, &gerr)
	assert.Equal(t, "INVALID_TTL", gerr.Code)
	assert.Contains(t, gerr.Message, "TTL", "error message should mention TTL")
	// No DB calls should have been made — guard against a regression where the
	// validation moves below BeginTx and we waste a transaction on bad input.
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestDeclareReservation_RejectsTTLNegative is a regression for grava-399b:
// negative TTLMinutes must also be rejected (defensive against integer
// over/underflow paths from flag parsing).
func TestDeclareReservation_RejectsTTLNegative(t *testing.T) {
	store, mock := newMock(t)

	p := DeclareParams{
		PathPattern: "src/cmd/issues/*.go",
		AgentID:     "agent-01",
		ProjectID:   "default",
		Exclusive:   true,
		TTLMinutes:  -5,
	}
	_, err := DeclareReservation(context.Background(), store, p)

	require.Error(t, err)
	var gerr *gravaerrors.GravaError
	require.ErrorAs(t, err, &gerr)
	assert.Equal(t, "INVALID_TTL", gerr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestDeclareReservation_PositiveTTLAccepted is a regression for grava-399b:
// after the TTL<=0 rejection guard, valid positive TTL values must continue to
// reach the INSERT path. Pairs with TestDeclareReservation_RejectsTTLZero to
// ensure the validation is strictly `<= 0`, not over-broad.
func TestDeclareReservation_PositiveTTLAccepted(t *testing.T) {
	store, mock := newMock(t)

	mock.ExpectBegin()
	mock.ExpectQuery(qCheckConflict).
		WithArgs("default", "agent-01").
		WillReturnRows(sqlmock.NewRows([]string{"agent_id", "path_pattern", "expires_ts"}))
	mock.ExpectExec(qInsertReservation).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	future := time.Now().Add(5 * time.Minute)
	mock.ExpectQuery(qGetReservation).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "project_id", "agent_id", "path_pattern", "exclusive", "reason",
			"created_ts", "expires_ts", "released_ts",
		}).AddRow("res-mock", "default", "agent-01", "src/cmd/issues/*.go", true, "", time.Now(), future, nil))

	p := DeclareParams{
		PathPattern: "src/cmd/issues/*.go",
		AgentID:     "agent-01",
		ProjectID:   "default",
		Exclusive:   true,
		TTLMinutes:  5,
	}
	result, err := DeclareReservation(context.Background(), store, p)

	require.NoError(t, err)
	assert.Equal(t, "src/cmd/issues/*.go", result.Reservation.PathPattern)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestListReservations_ReturnsActiveOnly verifies that ListReservations only
// returns non-expired, non-released rows.
func TestListReservations_ReturnsActiveOnly(t *testing.T) {
	store, mock := newMock(t)

	now := time.Now()
	expires := now.Add(25 * time.Minute)
	mock.ExpectQuery(qListReservations).
		WithArgs("default").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "project_id", "agent_id", "path_pattern", "exclusive", "reason", "created_ts", "expires_ts", "remaining_seconds",
		}).AddRow("res-aabbcc", "default", "agent-01", "src/*.go", true, "", now, expires, 1500))

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
		WithArgs("default").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "project_id", "agent_id", "path_pattern", "exclusive", "reason", "created_ts", "expires_ts", "remaining_seconds",
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

// TestPatternsOverlap covers the glob-overlap detection used by findConflictTx
// to determine whether two exclusive reservations conflict. Regression tests
// for grava-f5a7 — without overlap detection, an exclusive reservation on
// 'internal/**/*.go' would not block a subsequent exclusive reservation on
// 'internal/db/store.go'. Each case below documents the expected behavior in
// both directions (existing-then-new, new-then-existing) since the conflict
// check has to be order-independent.
func TestPatternsOverlap(t *testing.T) {
	tests := []struct {
		name    string
		a, b    string
		overlap bool
	}{
		// --- AC: spec repro ---
		// Glob covers literal: internal/**/*.go matches internal/db/store.go.
		{"glob blocks literal within (grava-f5a7 repro)", "internal/**/*.go", "internal/db/store.go", true},
		// Reverse direction must also detect overlap (order-independent).
		{"literal blocks glob over it (reverse direction)", "internal/db/store.go", "internal/**/*.go", true},

		// --- AC: non-overlapping reservations both succeed ---
		// Disjoint top-level dirs: no possible path matches both.
		{"non-overlapping globs both succeed", "internal/db/**/*.go", "cmd/**/*.go", false},
		// Same prefix, different leaf dirs.
		{"sibling globs different leaf dirs", "internal/db/*.go", "internal/api/*.go", false},
		// Different extensions on same tree.
		{"different extension no conflict", "internal/**/*.go", "internal/**/*.md", false},

		// --- Overlapping globs that must conflict ---
		// One pattern is strictly contained in the other.
		{"overlapping globs (subset)", "internal/**/*.go", "internal/db/**/*.go", true},
		// Identical patterns trivially overlap.
		{"identical patterns overlap", "internal/**/*.go", "internal/**/*.go", true},
		// Doublestar at root vs literal — must match.
		{"root doublestar covers literal", "**/*.go", "internal/db/store.go", true},
		// Doublestar without suffix covers anything under prefix.
		{"open doublestar covers literal", "internal/**", "internal/db/store.go", true},

		// --- Non-overlapping when literal extension differs ---
		{"glob matches only .go, literal is .txt", "internal/**/*.go", "internal/db/store.txt", false},

		// --- filepath.Match metacharacters still work ---
		{"single-char wildcard overlaps literal", "src/?.go", "src/a.go", true},
		{"char-class wildcard overlaps literal", "src/[ab].go", "src/a.go", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := patternsOverlap(tt.a, tt.b)
			assert.Equal(t, tt.overlap, got,
				"patternsOverlap(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.overlap)
		})
	}
}

// TestDeclareReservation_GlobBlocksLiteral is the integration-level regression
// for grava-f5a7: when an existing exclusive reservation has a glob path_pattern
// (e.g. 'internal/**/*.go'), a new exclusive reservation on a literal path it
// covers (e.g. 'internal/db/store.go') from a different agent must be rejected
// with FILE_RESERVATION_CONFLICT — even though the path_pattern strings are
// not equal as strings. This guards against a future regression where the
// conflict query reverts to exact string matching.
func TestDeclareReservation_GlobBlocksLiteral(t *testing.T) {
	store, mock := newMock(t)

	expiresAt := time.Now().Add(20 * time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery(qCheckConflict).
		WithArgs("default", "agent-B").
		WillReturnRows(sqlmock.NewRows([]string{"agent_id", "path_pattern", "expires_ts"}).
			AddRow("agent-A", "internal/**/*.go", expiresAt))
	mock.ExpectRollback()

	p := DeclareParams{
		PathPattern: "internal/db/store.go",
		AgentID:     "agent-B",
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

// TestDeclareReservation_LiteralBlocksGlob verifies the reverse direction of
// grava-f5a7: existing literal reservation 'internal/db/store.go' must block a
// new exclusive reservation on the broader glob 'internal/**/*.go' that covers
// it. Order-independence is critical because real agents claim leases in
// arbitrary order.
func TestDeclareReservation_LiteralBlocksGlob(t *testing.T) {
	store, mock := newMock(t)

	expiresAt := time.Now().Add(20 * time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery(qCheckConflict).
		WithArgs("default", "agent-B").
		WillReturnRows(sqlmock.NewRows([]string{"agent_id", "path_pattern", "expires_ts"}).
			AddRow("agent-A", "internal/db/store.go", expiresAt))
	mock.ExpectRollback()

	p := DeclareParams{
		PathPattern: "internal/**/*.go",
		AgentID:     "agent-B",
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

// TestDeclareReservation_NonOverlappingGlobsSucceed verifies that disjoint glob
// reservations from different agents both succeed — the overlap check must not
// be over-eager and produce false positives (which would deadlock unrelated
// work). 'internal/db/**/*.go' and 'cmd/**/*.go' have no possible common path.
func TestDeclareReservation_NonOverlappingGlobsSucceed(t *testing.T) {
	store, mock := newMock(t)

	expiresAt := time.Now().Add(20 * time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery(qCheckConflict).
		WithArgs("default", "agent-B").
		WillReturnRows(sqlmock.NewRows([]string{"agent_id", "path_pattern", "expires_ts"}).
			AddRow("agent-A", "internal/db/**/*.go", expiresAt))
	mock.ExpectExec(qInsertReservation).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	future := time.Now().Add(30 * time.Minute)
	mock.ExpectQuery(qGetReservation).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "project_id", "agent_id", "path_pattern", "exclusive", "reason",
			"created_ts", "expires_ts", "released_ts",
		}).AddRow("res-mock", "default", "agent-B", "cmd/**/*.go", true, "", time.Now(), future, nil))

	p := DeclareParams{
		PathPattern: "cmd/**/*.go",
		AgentID:     "agent-B",
		Exclusive:   true,
		TTLMinutes:  30,
	}
	result, err := DeclareReservation(context.Background(), store, p)

	require.NoError(t, err)
	assert.Equal(t, "agent-B", result.Reservation.AgentID)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestDeclareReservation_DifferentExtensionNoConflict verifies that two globs
// rooted at the same path but matching different extensions don't collide.
// 'internal/**/*.go' and 'internal/**/*.md' share a prefix but no path can
// match both — the glob-overlap check must be precise enough to recognize this.
func TestDeclareReservation_DifferentExtensionNoConflict(t *testing.T) {
	store, mock := newMock(t)

	expiresAt := time.Now().Add(20 * time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery(qCheckConflict).
		WithArgs("default", "agent-B").
		WillReturnRows(sqlmock.NewRows([]string{"agent_id", "path_pattern", "expires_ts"}).
			AddRow("agent-A", "internal/**/*.go", expiresAt))
	mock.ExpectExec(qInsertReservation).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	future := time.Now().Add(30 * time.Minute)
	mock.ExpectQuery(qGetReservation).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "project_id", "agent_id", "path_pattern", "exclusive", "reason",
			"created_ts", "expires_ts", "released_ts",
		}).AddRow("res-mock", "default", "agent-B", "internal/**/*.md", true, "", time.Now(), future, nil))

	p := DeclareParams{
		PathPattern: "internal/**/*.md",
		AgentID:     "agent-B",
		Exclusive:   true,
		TTLMinutes:  30,
	}
	result, err := DeclareReservation(context.Background(), store, p)

	require.NoError(t, err)
	assert.Equal(t, "internal/**/*.md", result.Reservation.PathPattern)
	require.NoError(t, mock.ExpectationsWereMet())
}
