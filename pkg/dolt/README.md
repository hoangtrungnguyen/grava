# Package: dolt

Path: `github.com/hoangtrungnguyen/grava/pkg/dolt`

## Purpose

SQL client wrapper for Grava's Dolt-backed shared state database. Provides
the `Store` abstraction, audit-aware transactions, deadlock retries, and
the atomic per-parent counter used for hierarchical issue IDs.

## Key Types & Functions

- `Store` — interface implemented by both `Client` and test mocks. Exposes
  `Exec/Query/QueryRow` (with context variants), `BeginTx`, connection-pool
  setters, `Close`, plus `LogEvent` / `LogEventTx` for audit writes and
  `GetNextChildSequence` for ID generation.
- `Client` — `database/sql` MySQL-driver implementation of `Store`.
  - `NewClient(dsn)` opens a pooled connection (max 20) and pings the DB.
  - `NewClientFromDB(*sql.DB)` wraps an existing handle (used with sqlmock).
- `AuditEvent` — descriptor for audit log entries.
- `WithAuditedTx(ctx, store, events, fn)` — runs `fn` in a transaction,
  appends `events` to the audit log, then commits atomically.
- `WithDeadlockRetry(fn)` — retries on MySQL error 1213 up to 3 times with
  10ms backoff. Must wrap only idempotent operations; never wrap
  `WithAuditedTx`.
- `Event*` string constants (events.go) — canonical event types
  (`EventCreate`, `EventClaim`, `EventStop`, `EventWispWrite`, etc.).
- `MockStore` — deprecated; use `internal/testutil.MockStore` instead.

## Dependencies

- `github.com/go-sql-driver/mysql` — MySQL wire protocol driver (Dolt is
  wire-compatible).
- `github.com/hoangtrungnguyen/grava/pkg/errors` — `GravaError` codes
  returned by `WithAuditedTx` (`DB_UNREACHABLE`, `DB_COMMIT_FAILED`).

## How It Fits

`pkg/dolt` is the substrate for every other Grava package that reads or
mutates shared state. Commands in `pkg/cmd/...` accept a `*dolt.Store`
through `pkg/cmddeps.Deps`; the orchestrator and coordinator share the same
handle. Audit events written via `WithAuditedTx` power `grava history` and
the undo/replay machinery.

## Usage

```go
store, _ := dolt.NewClient("root@tcp(127.0.0.1:3306)/grava?parseTime=true")
defer store.Close()

err := dolt.WithAuditedTx(ctx, store, []dolt.AuditEvent{{
    IssueID:   id,
    EventType: dolt.EventClaim,
    Actor:     actor,
    NewValue:  map[string]any{"status": "in_progress"},
}}, func(tx *sql.Tx) error {
    _, err := tx.ExecContext(ctx, `UPDATE issues SET status='in_progress' WHERE id=?`, id)
    return err
})
```
