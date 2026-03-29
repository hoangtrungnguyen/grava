package dolt_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
)

// auditedStore wraps sqlmock DB to implement dolt.Store for WithAuditedTx tests.
type auditedStore struct {
	db           *sql.DB
	logEventTxFn func(ctx context.Context, tx *sql.Tx, issueID, eventType, actor, model string, oldVal, newVal any) error
	logEventCalls []dolt.AuditEvent
}

func (s *auditedStore) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, opts)
}
func (s *auditedStore) Exec(q string, a ...any) (sql.Result, error)                   { return nil, nil }
func (s *auditedStore) ExecContext(ctx context.Context, q string, a ...any) (sql.Result, error) {
	return nil, nil
}
func (s *auditedStore) QueryRow(q string, a ...any) *sql.Row { return nil }
func (s *auditedStore) QueryRowContext(ctx context.Context, q string, a ...any) *sql.Row {
	return nil
}
func (s *auditedStore) Query(q string, a ...any) (*sql.Rows, error)                    { return nil, nil }
func (s *auditedStore) QueryContext(ctx context.Context, q string, a ...any) (*sql.Rows, error) {
	return nil, nil
}
func (s *auditedStore) SetMaxOpenConns(n int)                  {}
func (s *auditedStore) SetMaxIdleConns(n int)                  {}
func (s *auditedStore) DB() *sql.DB                            { return s.db }
func (s *auditedStore) Close() error                           { return s.db.Close() }
func (s *auditedStore) GetNextChildSequence(id string) (int, error) { return 0, nil }
func (s *auditedStore) LogEvent(issueID, eventType, actor, model string, oldVal, newVal any) error {
	return nil
}
func (s *auditedStore) LogEventTx(ctx context.Context, tx *sql.Tx, issueID, eventType, actor, model string, oldVal, newVal any) error {
	s.logEventCalls = append(s.logEventCalls, dolt.AuditEvent{
		IssueID:   issueID,
		EventType: eventType,
		Actor:     actor,
		Model:     model,
		OldValue:  oldVal,
		NewValue:  newVal,
	})
	if s.logEventTxFn != nil {
		return s.logEventTxFn(ctx, tx, issueID, eventType, actor, model, oldVal, newVal)
	}
	return nil
}

func TestWithAuditedTx_CommitsOnSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectCommit()

	store := &auditedStore{db: db}
	ctx := context.Background()

	called := false
	err = dolt.WithAuditedTx(ctx, store, []dolt.AuditEvent{
		{IssueID: "abc", EventType: dolt.EventCreate, Actor: "agent-01"},
	}, func(tx *sql.Tx) error {
		called = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, called)
	assert.Len(t, store.logEventCalls, 1)
	assert.Equal(t, "abc", store.logEventCalls[0].IssueID)
	assert.Equal(t, dolt.EventCreate, store.logEventCalls[0].EventType)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithAuditedTx_RollsBackOnFnError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectRollback()

	store := &auditedStore{db: db}
	ctx := context.Background()

	fnErr := errors.New("mutation failed")
	err = dolt.WithAuditedTx(ctx, store, []dolt.AuditEvent{
		{IssueID: "abc", EventType: dolt.EventCreate},
	}, func(tx *sql.Tx) error {
		return fnErr
	})

	require.Error(t, err)
	assert.Equal(t, fnErr, err)
	// audit event must NOT be logged when fn fails
	assert.Empty(t, store.logEventCalls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithAuditedTx_RollsBackOnAuditLogError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectRollback()

	auditErr := errors.New("audit log write failed")
	store := &auditedStore{
		db: db,
		logEventTxFn: func(_ context.Context, _ *sql.Tx, _, _, _, _ string, _, _ any) error {
			return auditErr
		},
	}
	ctx := context.Background()

	err = dolt.WithAuditedTx(ctx, store, []dolt.AuditEvent{
		{IssueID: "abc", EventType: dolt.EventCreate},
	}, func(tx *sql.Tx) error { return nil })

	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr))
	assert.Equal(t, "DB_UNREACHABLE", gravaErr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithAuditedTx_NoEvents_StillCommits(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectCommit()

	store := &auditedStore{db: db}
	ctx := context.Background()

	err = dolt.WithAuditedTx(ctx, store, nil, func(tx *sql.Tx) error { return nil })
	require.NoError(t, err)
	assert.Empty(t, store.logEventCalls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithAuditedTx_MultipleEvents_AllLogged(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectBegin()
	mock.ExpectCommit()

	store := &auditedStore{db: db}
	ctx := context.Background()

	events := []dolt.AuditEvent{
		{IssueID: "abc", EventType: dolt.EventCreate, Actor: "agent-01"},
		{IssueID: "abc", EventType: dolt.EventUpdate, Actor: "agent-01", OldValue: "open", NewValue: "done"},
	}

	err = dolt.WithAuditedTx(ctx, store, events, func(tx *sql.Tx) error { return nil })
	require.NoError(t, err)
	assert.Len(t, store.logEventCalls, 2)
	assert.Equal(t, dolt.EventCreate, store.logEventCalls[0].EventType)
	assert.Equal(t, dolt.EventUpdate, store.logEventCalls[1].EventType)
	require.NoError(t, mock.ExpectationsWereMet())
}
