package maintenance

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	qSelectExpired = regexp.QuoteMeta(`SELECT id FROM file_reservations WHERE expires_ts < NOW() AND released_ts IS NULL`)
	qUpdateRelease = regexp.QuoteMeta(`UPDATE file_reservations SET released_ts = NOW() WHERE id = ? AND released_ts IS NULL`)
)

func newDoctorMock(t *testing.T) (dolt.Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() }) //nolint:errcheck
	return dolt.NewClientFromDB(db), mock
}

// --- queryExpiredLeases ---

func TestQueryExpiredLeases_None(t *testing.T) {
	store, mock := newDoctorMock(t)
	mock.ExpectQuery(qSelectExpired).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	ids, err := queryExpiredLeases(context.Background(), store)
	require.NoError(t, err)
	assert.Empty(t, ids)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryExpiredLeases_Found(t *testing.T) {
	store, mock := newDoctorMock(t)
	mock.ExpectQuery(qSelectExpired).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow("res-aaa111").
			AddRow("res-bbb222"))

	ids, err := queryExpiredLeases(context.Background(), store)
	require.NoError(t, err)
	require.Len(t, ids, 2)
	assert.Equal(t, "res-aaa111", ids[0])
	assert.Equal(t, "res-bbb222", ids[1])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryExpiredLeases_DBError(t *testing.T) {
	store, mock := newDoctorMock(t)
	mock.ExpectQuery(qSelectExpired).
		WillReturnError(fmt.Errorf("connection lost"))

	ids, err := queryExpiredLeases(context.Background(), store)
	require.Error(t, err)
	assert.Nil(t, ids)
	assert.Contains(t, err.Error(), "connection lost")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- releaseExpiredLeases ---

func TestReleaseExpiredLeases_EmptyInput(t *testing.T) {
	store, mock := newDoctorMock(t)
	// No DB calls expected for empty input.
	n, err := releaseExpiredLeases(context.Background(), store, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseExpiredLeases_Success(t *testing.T) {
	store, mock := newDoctorMock(t)
	mock.ExpectExec(qUpdateRelease).
		WithArgs("res-aaa111").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(qUpdateRelease).
		WithArgs("res-bbb222").
		WillReturnResult(sqlmock.NewResult(1, 1))

	n, err := releaseExpiredLeases(context.Background(), store, []string{"res-aaa111", "res-bbb222"})
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseExpiredLeases_AlreadyReleased(t *testing.T) {
	// Row already released: RowsAffected=0 — not counted but no error.
	store, mock := newDoctorMock(t)
	mock.ExpectExec(qUpdateRelease).
		WithArgs("res-gone").
		WillReturnResult(sqlmock.NewResult(0, 0))

	n, err := releaseExpiredLeases(context.Background(), store, []string{"res-gone"})
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseExpiredLeases_DBError(t *testing.T) {
	store, mock := newDoctorMock(t)
	mock.ExpectExec(qUpdateRelease).
		WithArgs("res-fail").
		WillReturnError(fmt.Errorf("deadlock"))

	_, err := releaseExpiredLeases(context.Background(), store, []string{"res-fail"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadlock")
	assert.NoError(t, mock.ExpectationsWereMet())
}
