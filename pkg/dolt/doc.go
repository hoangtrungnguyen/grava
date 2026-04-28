// Package dolt is the SQL client wrapper Grava uses to talk to its Dolt
// (Git-for-data) database at .grava/dolt/.
//
// The package defines the Store interface — the abstraction every other
// Grava package uses to read and mutate shared agent state — and a Client
// implementation backed by the standard database/sql MySQL driver. Beyond
// the basic Exec/Query passthroughs, the package provides three pieces of
// machinery that callers should prefer over hand-rolled SQL:
//
//   - WithAuditedTx executes a function inside a transaction and atomically
//     records a batch of AuditEvent rows in the events table before commit,
//     guaranteeing that mutations and their audit log live or die together.
//   - WithDeadlockRetry retries idempotent operations on MySQL/Dolt error
//     1213 with a small backoff. It must NOT wrap WithAuditedTx because that
//     would duplicate audit entries on retry.
//   - GetNextChildSequence atomically increments per-parent counters in the
//     child_counters table using LAST_INSERT_ID, retrying on serialization
//     failures; this is the foundation for the hierarchical issue ID scheme.
//
// The events.go file declares the canonical Event* string constants
// (EventCreate, EventClaim, EventStop, etc.) — callers must use these
// instead of raw string literals when emitting audit rows.
//
// MockStore in this package is deprecated; prefer internal/testutil.MockStore
// in tests. NewClientFromDB is provided for sqlmock-based unit tests.
package dolt
