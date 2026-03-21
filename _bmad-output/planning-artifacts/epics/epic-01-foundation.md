# Epic 1: Foundation & Scaffold

**Status:** Planned
**Matrix Score:** 3.80
**FRs covered:** FR23 (partial — basic init/config scaffold), FR17 (partial — schema version check)

## Goal

Developers and agents have a correctly structured, fully operational Grava binary with all shared infrastructure in place — command groups reorganized, structured logging, structured error types, migration ownership fixed, notifier interface, and the `.grava/` resolution chain working. No feature story can begin until this epic is complete.

## Story 0 Decomposition

Story 0 is decomposed into 3 sequenced sub-stories to prevent single-point-of-failure blocking:

**Story 0a — Core error/logging/tx infra:**
- GravaError (`pkg/errors/`) with `Code`, `Message`, `Cause`
- zerolog structured logging setup
- WithAuditedTx transaction wrapper
- Migration ownership fix: remove `migrate.Run()` from `PersistentPreRunE`; run only during `grava init`
- JSON Error Envelope: all `--json` error paths return `{"error": {"code": "...", "message": "..."}}`

**Story 0b — Concurrency & coordination:**
- WithDeadlockRetry (max 3 attempts, 10ms backoff)
- Coordinator Error Channel pattern: `Start(ctx) <-chan error`; no goroutine calls `log.Fatal`/`os.Exit`
- Notifier interface: `pkg/notify/notifier.go` + `ConsoleNotifier`
- `pkg/cmd/` reorganization: `pkg/cmd/issues/`, `pkg/cmd/graph/`, `pkg/cmd/maintenance/`, `pkg/cmd/sync/`

**Story 0c — Integration harness:**
- Worktree resolver: `GRAVA_DIR` env → redirect file → CWD walk (ADR-004)
- testutil: `testify` + hand-written interface mocks
- Named-function extraction: `claimIssue`, `importIssues`, `readyQueue` with explicit signatures (ADR-003)

## Parallel Track Unlock

- **Epics 2 and 3 may begin after Story 0a is merged** — they only require GravaError, zerolog, and WithAuditedTx
- Story 0b and 0c can proceed in parallel with Epic 2 work
- **Full Epic 1 must be complete before Epics 5, 7, 8, 9**

## NFR Ownership

| NFR | Role |
|-----|------|
| NFR2 (<15ms writes) | *Owned* — WithAuditedTx establishes baseline |
| NFR5 (JSON schema versioning) | *Owned* — JSON Error Envelope (Story 0a) |
| NFR6 (single binary) | *Owned* — no separate runtime allowed in any subsequent epic |

## Key Architecture References

- ADR-001: Git hook binary (one-liner shell stubs)
- ADR-003: Named functions
- ADR-004: Worktree redirect
- ADR-FM6: Migrations only in init
- ADR-H3: Deadlock prevention
- ADR-N1: Notifier interface
- C2/C3: Critical gaps from implementation readiness report

## Dependencies

None — this is the root epic.

## Stories

### Story 1.1: Core Error Types, Logging & Transaction Infrastructure (Story 0a)

As a Grava developer,
I want structured error types, zerolog, and audited transaction wrappers in place,
So that all subsequent feature work has a consistent foundation for error reporting, logging, and safe writes.

**Acceptance Criteria:**

**Given** the codebase has ad-hoc `fmt.Errorf` and `log.Printf` usage
**When** Story 1.1 is complete
**Then** `pkg/errors/gravaerror.go` exists with `GravaError{Code string, Message string, Cause error}` and implements the `error` interface
**And** all `--json` error paths return `{"error": {"code": "...", "message": "..."}}` — no raw Go error strings in JSON output
**And** `zerolog` is wired as the sole logger; no `log.Printf`, `fmt.Println`, or `log.Fatal` remain in production code paths
**And** `pkg/store/tx.go` exports `WithAuditedTx(ctx, db, fn)` that wraps `fn` in a DB transaction, rolls back on error, and logs the transaction boundary at debug level
**And** `migrate.Run()` is removed from `PersistentPreRunE`; a `.grava/schema_version` file is checked instead; migration runs only during `grava init`
**And** all existing tests pass after the refactor

---

### Story 1.2: Concurrency Primitives & Coordinator Error Channel (Story 0b)

As a Grava developer,
I want deadlock-retry logic, a safe coordinator error channel, and a Notifier interface,
So that concurrent writes never deadlock and goroutines never call `os.Exit` directly.

**Acceptance Criteria:**

**Given** Story 1.1 is complete
**When** Story 1.2 is complete
**Then** `pkg/store/retry.go` exports `WithDeadlockRetry(ctx, maxAttempts int, backoff time.Duration, fn)` that retries `fn` on MySQL deadlock error (1213) up to 3 times with 10ms backoff
**And** `pkg/notify/notifier.go` defines `Notifier` interface with `Notify(event NotifyEvent)` method and `ConsoleNotifier` struct as the default implementation
**And** `pkg/coordinator/coordinator.go` exports `Start(ctx context.Context) <-chan error` — the coordinator sends errors to the channel instead of calling `log.Fatal` or `os.Exit`
**And** `pkg/cmd/` is reorganized into `pkg/cmd/issues/`, `pkg/cmd/graph/`, `pkg/cmd/maintenance/`, `pkg/cmd/sync/` — all commands compile and pass existing tests
**And** no goroutine in production code calls `log.Fatal`, `os.Exit`, or `panic` directly

---

### Story 1.3: Worktree Resolver, Named Functions & Test Harness (Story 0c)

As a Grava developer,
I want a testable named-function architecture with a worktree resolver and testutil package,
So that all core commands can be unit-tested in isolation without starting a real Dolt server.

**Acceptance Criteria:**

**Given** Stories 1.1 and 1.2 are complete
**When** Story 1.3 is complete
**Then** `.grava/` resolution follows priority chain: `GRAVA_DIR` env var → `.grava/redirect` file → CWD walk up to filesystem root; resolution logic lives in `pkg/grava/resolver.go`
**And** `grava claim`, `grava import`, and `grava ready` are extracted into named functions: `claimIssue(ctx, store, actorID, issueID)`, `importIssues(ctx, store, r io.Reader)`, `readyQueue(ctx, store)` — each returning `(result, error)`
**And** `internal/testutil/` package provides: `MockStore` (implements `dolt.Store` interface), `NewTestDB(t)` helper returning an in-memory test DB, and `AssertGravaError(t, err, code)` assertion helper
**And** at least one unit test per named function uses `MockStore` and passes without a Dolt process running
**And** `grava init` writes `.git/info/exclude` entry for `.grava/` (not `.gitignore`) and migrates existing `.gitignore` `.grava/` entries (ADR-H5)
