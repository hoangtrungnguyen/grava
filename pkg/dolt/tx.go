package dolt

import (
	"context"
	"database/sql"

	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
)

// AuditEvent describes a state transition to be recorded in the audit log.
// Always populate EventType using Event* constants from events.go.
type AuditEvent struct {
	IssueID   string
	EventType string // use Event* constants
	Actor     string
	Model     string
	OldValue  any
	NewValue  any
}

// WithAuditedTx executes fn inside a DB transaction, then records all audit events
// atomically before committing. On any error the transaction is rolled back and no
// audit events are persisted.
//
// Rules:
//   - All DB mutations must happen inside fn via the provided *sql.Tx.
//   - Audit events are logged after fn succeeds but before Commit, ensuring atomicity.
//   - Do NOT wrap WithAuditedTx in WithDeadlockRetry (Story 1.2) — that would
//     duplicate audit entries on retry.
func WithAuditedTx(ctx context.Context, store Store, events []AuditEvent, fn func(tx *sql.Tx) error) error {
	tx, err := store.BeginTx(ctx, nil)
	if err != nil {
		return gravaerrors.New("DB_UNREACHABLE", "failed to start transaction", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if err := fn(tx); err != nil {
		return err
	}

	for _, evt := range events {
		if err := store.LogEventTx(ctx, tx, evt.IssueID, evt.EventType, evt.Actor, evt.Model, evt.OldValue, evt.NewValue); err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to log audit event", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return gravaerrors.New("DB_COMMIT_FAILED", "failed to commit transaction", err)
	}
	return nil
}
