package testutil

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time check: MockStore implements dolt.Store
var _ dolt.Store = (*MockStore)(nil)

// ExecContextCall records a single ExecContext call for assertion.
type ExecContextCall struct {
	Query string
	Args  []any
}

// MockStore is a configurable mock implementing dolt.Store.
// Use configurable Fn fields to control behavior; ExecContextCalls records calls.
type MockStore struct {
	ExecContextFn     func(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContextFn func(ctx context.Context, query string, args ...any) *sql.Row
	QueryContextFn    func(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	LogEventTxFn      func(ctx context.Context, tx *sql.Tx, issueID, eventType, actor, model string, old, new any) error
	BeginTxFn         func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	ExecFn            func(query string, args ...any) (sql.Result, error)
	QueryRowFn        func(query string, args ...any) *sql.Row
	QueryFn           func(query string, args ...any) (*sql.Rows, error)
	GetNextChildSequenceFn func(parentID string) (int, error)

	ExecContextCalls []ExecContextCall
}

// NewMockStore creates a MockStore with safe default (nil return) implementations.
func NewMockStore() *MockStore {
	return &MockStore{}
}

func (m *MockStore) GetNextChildSequence(parentID string) (int, error) {
	if m.GetNextChildSequenceFn != nil {
		return m.GetNextChildSequenceFn(parentID)
	}
	return 0, nil
}

func (m *MockStore) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if m.BeginTxFn != nil {
		return m.BeginTxFn(ctx, opts)
	}
	return nil, nil
}

func (m *MockStore) Exec(query string, args ...any) (sql.Result, error) {
	if m.ExecFn != nil {
		return m.ExecFn(query, args...)
	}
	return nil, nil
}

func (m *MockStore) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	m.ExecContextCalls = append(m.ExecContextCalls, ExecContextCall{Query: query, Args: args})
	if m.ExecContextFn != nil {
		return m.ExecContextFn(ctx, query, args...)
	}
	return nil, nil
}

func (m *MockStore) QueryRow(query string, args ...any) *sql.Row {
	if m.QueryRowFn != nil {
		return m.QueryRowFn(query, args...)
	}
	return nil
}

func (m *MockStore) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	if m.QueryRowContextFn != nil {
		return m.QueryRowContextFn(ctx, query, args...)
	}
	return nil
}

func (m *MockStore) Query(query string, args ...any) (*sql.Rows, error) {
	if m.QueryFn != nil {
		return m.QueryFn(query, args...)
	}
	return nil, nil
}

func (m *MockStore) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if m.QueryContextFn != nil {
		return m.QueryContextFn(ctx, query, args...)
	}
	return nil, nil
}

func (m *MockStore) SetMaxOpenConns(n int) {}
func (m *MockStore) SetMaxIdleConns(n int) {}
func (m *MockStore) DB() *sql.DB           { return nil }
func (m *MockStore) Close() error          { return nil }

func (m *MockStore) LogEvent(issueID, eventType, actor, model string, old, new any) error {
	return nil
}

func (m *MockStore) LogEventTx(ctx context.Context, tx *sql.Tx, issueID, eventType, actor, model string, old, new any) error {
	if m.LogEventTxFn != nil {
		return m.LogEventTxFn(ctx, tx, issueID, eventType, actor, model, old, new)
	}
	return nil
}

// NewTestDB creates a sqlmock-backed *sql.DB for unit tests (no real database needed).
func NewTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

// AssertGravaError asserts that err is a *GravaError with the given error code.
func AssertGravaError(t *testing.T, err error, code string) {
	t.Helper()
	require.Error(t, err)
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gravaErr),
		"expected *gravaerrors.GravaError, got %T: %v", err, err)
	assert.Equal(t, code, gravaErr.Code, "GravaError code mismatch")
}
