package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMockStore_ImplementsStore(t *testing.T) {
	// Compile-time check is in testutil.go; this test documents the contract at runtime.
	store := NewMockStore()
	assert.NotNil(t, store)
}

func TestMockStore_ExecContext_RecordsCalls(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	_, _ = store.ExecContext(ctx, "INSERT INTO foo VALUES (?)", "bar")
	_, _ = store.ExecContext(ctx, "UPDATE foo SET x=?", 42)

	require.Len(t, store.ExecContextCalls, 2)
	assert.Equal(t, "INSERT INTO foo VALUES (?)", store.ExecContextCalls[0].Query)
	assert.Equal(t, "UPDATE foo SET x=?", store.ExecContextCalls[1].Query)
}

func TestMockStore_ExecContext_UsesFn(t *testing.T) {
	store := NewMockStore()
	called := false
	store.ExecContextFn = func(ctx context.Context, query string, args ...any) (sql.Result, error) {
		called = true
		return nil, fmt.Errorf("injected error")
	}

	ctx := context.Background()
	_, err := store.ExecContext(ctx, "SELECT 1")
	assert.True(t, called)
	assert.EqualError(t, err, "injected error")
}

func TestMockStore_LogEventTx_DefaultNoError(t *testing.T) {
	store := NewMockStore()
	err := store.LogEventTx(context.Background(), nil, "issue-1", "claim", "actor", "model", nil, nil)
	assert.NoError(t, err)
}

func TestMockStore_GetNextChildSequence_Default(t *testing.T) {
	store := NewMockStore()
	seq, err := store.GetNextChildSequence("parent-1")
	assert.NoError(t, err)
	assert.Equal(t, 0, seq)
}

func TestMockStore_GetNextChildSequence_UseFn(t *testing.T) {
	store := NewMockStore()
	store.GetNextChildSequenceFn = func(parentID string) (int, error) {
		return 42, nil
	}
	seq, err := store.GetNextChildSequence("parent-1")
	assert.NoError(t, err)
	assert.Equal(t, 42, seq)
}

func TestNewTestDB_CreatesDB(t *testing.T) {
	db, mock := NewTestDB(t)
	assert.NotNil(t, db)
	assert.NotNil(t, mock)
}

func TestAssertGravaError_MatchingCode(t *testing.T) {
	err := gravaerrors.New("NOT_INITIALIZED", "test message", nil)
	// Use a sub-test to run AssertGravaError without failing the parent test.
	t.Run("sub", func(t *testing.T) {
		AssertGravaError(t, err, "NOT_INITIALIZED")
	})
}

func TestAssertGravaError_WrappedError(t *testing.T) {
	// Verify that AssertGravaError works with wrapped errors (errors.As traversal).
	inner := gravaerrors.New("INNER_CODE", "inner message", nil)
	outer := fmt.Errorf("wrapped: %w", inner)
	t.Run("sub", func(t *testing.T) {
		AssertGravaError(t, outer, "INNER_CODE")
	})
}

func TestMockStore_Close_NoError(t *testing.T) {
	store := NewMockStore()
	assert.NoError(t, store.Close())
}

func TestMockStore_DB_ReturnsNil(t *testing.T) {
	store := NewMockStore()
	assert.Nil(t, store.DB())
}
