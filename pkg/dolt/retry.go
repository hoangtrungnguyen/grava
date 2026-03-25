package dolt

import (
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
)

// WithDeadlockRetry retries fn on MySQL deadlock error (1213) up to 3 times with 10ms backoff.
//
// RESTRICTION: Use only around SELECT ... FOR UPDATE + counter increment operations.
// DO NOT wrap WithAuditedTx in WithDeadlockRetry — audit log duplication on retry.
// All operations inside fn MUST be idempotent.
func WithDeadlockRetry(fn func() error) error {
	const maxRetries = 3
	for attempt := range maxRetries {
		err := fn()
		if err == nil {
			return nil
		}
		if isMySQLDeadlock(err) && attempt < maxRetries-1 {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		return err
	}
	return nil // unreachable; satisfies compiler
}

func isMySQLDeadlock(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1213
}
