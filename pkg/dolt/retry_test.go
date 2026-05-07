package dolt

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeDeadlockErr() error {
	return &mysql.MySQLError{Number: 1213, Message: "Deadlock found when trying to get lock; try restarting transaction"}
}

func TestWithDeadlockRetry_SuccessFirstAttempt(t *testing.T) {
	calls := 0
	err := WithDeadlockRetry(func() error {
		calls++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "fn should be called exactly once on success")
}

func TestWithDeadlockRetry_DeadlockThenSuccess(t *testing.T) {
	calls := 0
	err := WithDeadlockRetry(func() error {
		calls++
		if calls < 3 {
			return makeDeadlockErr()
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 3, calls, "fn should be called 3 times (2 deadlocks then success)")
}

func TestWithDeadlockRetry_AlwaysDeadlock(t *testing.T) {
	calls := 0
	dlErr := makeDeadlockErr()
	err := WithDeadlockRetry(func() error {
		calls++
		return dlErr
	})
	require.Error(t, err)
	assert.Equal(t, 3, calls, "fn should be called exactly 3 times before giving up")
	var mysqlErr *mysql.MySQLError
	assert.True(t, errors.As(err, &mysqlErr), "returned error should be MySQL deadlock error")
	assert.Equal(t, uint16(1213), mysqlErr.Number)
}

func TestWithDeadlockRetry_NonDeadlockError_NoRetry(t *testing.T) {
	calls := 0
	nonDeadlock := errors.New("some other error")
	err := WithDeadlockRetry(func() error {
		calls++
		return nonDeadlock
	})
	require.Error(t, err)
	assert.Equal(t, 1, calls, "fn should be called once; no retry on non-deadlock error")
	assert.Equal(t, nonDeadlock, err)
}

func TestIsMySQLDeadlock_True(t *testing.T) {
	assert.True(t, isMySQLDeadlock(makeDeadlockErr()))
}

func TestIsMySQLDeadlock_False_WrongCode(t *testing.T) {
	assert.False(t, isMySQLDeadlock(&mysql.MySQLError{Number: 1062}))
}

func TestIsMySQLDeadlock_False_NonMySQL(t *testing.T) {
	assert.False(t, isMySQLDeadlock(errors.New("not mysql")))
}
