# Story 1.2: Concurrency Primitives & Coordinator Error Channel (Story 0b)

Status: done

## Story

As a Grava developer,
I want deadlock-retry logic, a safe coordinator error channel, and a Notifier interface,
So that concurrent writes never deadlock and goroutines never call `os.Exit` directly.

## Acceptance Criteria

1. `pkg/dolt/retry.go` exports `WithDeadlockRetry(fn func() error) error` that retries `fn` on MySQL deadlock error (1213) up to 3 times with 10ms backoff, using `range` integer syntax (Go 1.22+).
2. `pkg/notify/notifier.go` defines `Notifier` interface with `Send(title, body string) error` method and `ConsoleNotifier` struct as the default implementation (writes to stderr with `[GRAVA ALERT]` prefix).
3. `pkg/coordinator/coordinator.go` (or `pkg/cmd/coordinator/coordinator.go`) exports `Coordinator` struct with `Start(ctx context.Context) <-chan error` — the coordinator sends errors to the channel instead of calling `log.Fatal` or `os.Exit`.
4. `pkg/cmd/` is reorganized into `pkg/cmd/issues/`, `pkg/cmd/graph/`, `pkg/cmd/maintenance/`, `pkg/cmd/sync/` — all commands compile and pass existing tests after the move.
5. No goroutine in production code calls `log.Fatal`, `os.Exit`, or `panic` directly.
6. `var Notifier notify.Notifier = notify.NewConsoleNotifier()` is declared as a package-level var in `pkg/cmd/root.go`; all commands use `cmd.Notifier.Send(...)` — never instantiate notifier directly in command code.
7. `pkg/notify/mock/notifier_mock.go` provides `MockNotifier` for test injection.
8. All existing tests pass after the refactor (`go test ./...`).

## Tasks / Subtasks

- [x] Task 1: Create `pkg/dolt/retry.go` — WithDeadlockRetry (AC: #1)
  - [x] Create `pkg/dolt/retry.go` with `WithDeadlockRetry(fn func() error) error`
  - [x] Implement `isMySQLDeadlock(err error) bool` using `errors.As` + `*mysql.MySQLError{Number: 1213}`
  - [x] Use `range maxRetries` integer syntax (requires Go 1.22+; already `go 1.24.0` in go.mod — OK)
  - [x] Sleep 10ms between retries; do NOT sleep after final attempt
  - [x] Write tests in `pkg/dolt/retry_test.go`

- [x] Task 2: Create `pkg/notify/notifier.go` — Notifier interface + ConsoleNotifier (AC: #2, #6, #7)
  - [x] Create directory `pkg/notify/`
  - [x] Define `Notifier` interface: `Send(title, body string) error`
  - [x] Define `ConsoleNotifier` struct implementing `Notifier`
  - [x] Implement `NewConsoleNotifier() *ConsoleNotifier` constructor
  - [x] Create `pkg/notify/mock/notifier_mock.go` with `MockNotifier`
  - [x] Write tests in `pkg/notify/notifier_test.go`

- [x] Task 3: Create `pkg/coordinator/coordinator.go` — error channel pattern (AC: #3, #5)
  - [x] Create `pkg/coordinator/coordinator.go` with `Coordinator` struct
  - [x] Implement `Start(ctx context.Context) <-chan error` (buffered chan, no log.Fatal/os.Exit)
  - [x] Write tests in `pkg/coordinator/coordinator_test.go`
  - [x] Add `var Notifier notify.Notifier = notify.NewConsoleNotifier()` to `pkg/cmd/root.go`

- [x] Task 4: Reorganize `pkg/cmd/` into sub-packages (AC: #4, #8)
  - [x] Create `pkg/cmd/issues/issues.go` (package issues) — 10 commands
  - [x] Create `pkg/cmd/graph/graph.go` (package cmdgraph) — 6 commands
  - [x] Create `pkg/cmd/maintenance/maintenance.go` (package maintenance) — 5 commands
  - [x] Create `pkg/cmd/sync/sync.go` (package synccmd) — 3 commands
  - [x] Create `pkg/cmddeps/deps.go` — circular-import-safe dependency injection
  - [x] Update `pkg/cmd/root.go` to import and register all sub-package commands
  - [x] Remove `init()` from all 24 old command files (prevent double-registration)
  - [x] All tests compile and pass: `go test ./...`
  - [x] Binary builds: `go build -ldflags="-s -w" ./...`
  - [x] `go vet ./...` — zero warnings

- [x] Task 5: Final verification (AC: #5, #8)
  - [x] No `log.Fatal`/`os.Exit`/`panic` in production sub-packages (only CLI entry in root.go:Execute)
  - [x] `go test ./...` — all non-integration tests pass
  - [x] `go vet ./...` — zero warnings
  - [x] `go build -ldflags="-s -w" ./...` — success

## Dev Notes

### Critical: WithDeadlockRetry — Exact Signature and Constraint

```go
// pkg/dolt/retry.go
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
    return nil // unreachable but satisfies compiler
}

func isMySQLDeadlock(err error) bool {
    var mysqlErr *mysql.MySQLError
    return errors.As(err, &mysqlErr) && mysqlErr.Number == 1213
}
```

**Anti-patterns:**
- ❌ Do NOT wrap `WithAuditedTx` in `WithDeadlockRetry` — duplicate audit log on retry
- ❌ Do NOT use `WithDeadlockRetry` around non-idempotent INSERT operations
- ✅ Use `WithDeadlockRetry` for: `SELECT FOR UPDATE` + counter increment, `dep --add` multi-row locking

**Usage example (from architecture write path):**
```go
// claimIssue usage:
return dolt.WithDeadlockRetry(func() error {
    _, err := tx.ExecContext(ctx, "SELECT id FROM issues WHERE id=? FOR UPDATE", id)
    return err
})
```

### Critical: Notifier Interface Contract (ADR-N1)

**Architecture source of truth** — `Send(title, body string) error` (NOT `Notify(event)`):

```go
// pkg/notify/notifier.go
package notify

import (
    "fmt"
    "os"
)

// Notifier is the interface for system-level alerts.
// Send errors are non-fatal — the primary operation always completes regardless.
type Notifier interface {
    Send(title, body string) error
}

// ConsoleNotifier implements Notifier for Phase 1 — writes to stderr.
type ConsoleNotifier struct{}

func NewConsoleNotifier() *ConsoleNotifier {
    return &ConsoleNotifier{}
}

func (n *ConsoleNotifier) Send(title, body string) error {
    fmt.Fprintf(os.Stderr, "[GRAVA ALERT] %s: %s\n", title, body)
    return nil
}
```

**Rules:**
- `Send` errors are **non-fatal** — primary operation always completes
- On `Send` error: log locally to stderr and continue; never propagate up
- If notifier is nil: silently fall back to ConsoleNotifier (never panic on nil)
- Call site: `cmd.Notifier.Send("Coordinator Down", "grava coordinator is not running")` — never instantiate
- `Notifier.Send` errors use `fmt.Fprintf(os.Stderr, ...)` — no Cobra context available

**Mock for tests:**
```go
// pkg/notify/mock/notifier_mock.go
package mock

type MockNotifier struct {
    Calls []struct{ Title, Body string }
    Error error // Return this on Send (nil by default)
}

func (m *MockNotifier) Send(title, body string) error {
    m.Calls = append(m.Calls, struct{ Title, Body string }{title, body})
    return m.Error
}
```

**Injection in root.go:**
```go
// pkg/cmd/root.go — package-level var, default ConsoleNotifier
var Notifier notify.Notifier = notify.NewConsoleNotifier()
// Commands call: Notifier.Send(...) — never instantiate directly in command code
// Tests inject: MockNotifier from pkg/notify/mock/
```

### Critical: Coordinator Error Channel Pattern (Architecture C2)

```go
// pkg/coordinator/coordinator.go
package coordinator

import (
    "context"

    "github.com/rs/zerolog"
    "github.com/hoangtrungnguyen/grava/pkg/notify"
)

type Coordinator struct {
    notifier notify.Notifier
    log      zerolog.Logger
}

func New(n notify.Notifier, log zerolog.Logger) *Coordinator {
    return &Coordinator{notifier: n, log: log}
}

// Start launches the coordinator goroutine and returns an error channel.
// Caller MUST select on the channel or ctx.Done():
//
//   ch := coord.Start(ctx)
//   select {
//   case err := <-ch:
//       // handle coordinator error
//   case <-ctx.Done():
//       // graceful shutdown
//   }
//
// Buffer size 1 prevents goroutine leak if caller abandons the channel.
func (c *Coordinator) Start(ctx context.Context) <-chan error {
    errCh := make(chan error, 1)
    go func() {
        // coordinator work loop — ctx cancellation is the graceful shutdown signal
        // NEVER call log.Fatal, os.Exit, or panic here
        select {
        case <-ctx.Done():
            close(errCh)
        }
    }()
    return errCh
}
```

**Rules:**
- Goroutines MUST NOT call `log.Fatal`, `os.Exit`, or `panic`
- Error channel buffer size = 1 (prevents goroutine leak)
- Always `close(errCh)` before goroutine exits for clean shutdown detection
- Coordinator respects `ctx` cancellation for graceful shutdown
- Fire `Notifier.Send` before sending fatal error to channel

### pkg/cmd/ Reorganization — File Mapping

**Command groups and file destinations:**

| Current File | Destination Package | Command |
|---|---|---|
| `pkg/cmd/create.go` | `pkg/cmd/issues/` | `grava create` (FR1) |
| `pkg/cmd/show.go` | `pkg/cmd/issues/` | `grava show` (FR2) |
| `pkg/cmd/list.go` | `pkg/cmd/issues/` | `grava list` (FR3) |
| `pkg/cmd/update.go` | `pkg/cmd/issues/` | `grava update` (FR4) |
| `pkg/cmd/drop.go` | `pkg/cmd/issues/` | `grava drop` (FR5) |
| `pkg/cmd/assign.go` | `pkg/cmd/issues/` | `grava assign` |
| `pkg/cmd/label.go` | `pkg/cmd/issues/` | `grava label` |
| `pkg/cmd/comment.go` | `pkg/cmd/issues/` | `grava comment` |
| `pkg/cmd/subtask.go` | `pkg/cmd/issues/` | `grava subtask` |
| `pkg/cmd/quick.go` | `pkg/cmd/issues/` | `grava quick` |
| `pkg/cmd/dep.go` | `pkg/cmd/graph/` | `grava dep` |
| `pkg/cmd/graph.go` | `pkg/cmd/graph/` | `grava graph` |
| `pkg/cmd/ready.go` | `pkg/cmd/graph/` | `grava ready` |
| `pkg/cmd/blocked.go` | `pkg/cmd/graph/` | `grava blocked` |
| `pkg/cmd/search.go` | `pkg/cmd/graph/` | `grava search` |
| `pkg/cmd/stats.go` | `pkg/cmd/graph/` | `grava stats` |
| `pkg/cmd/compact.go` | `pkg/cmd/maintenance/` | `grava compact` |
| `pkg/cmd/doctor.go` | `pkg/cmd/maintenance/` | `grava doctor` |
| `pkg/cmd/undo.go` | `pkg/cmd/maintenance/` | `grava undo` |
| `pkg/cmd/clear.go` | `pkg/cmd/maintenance/` | `grava clear` |
| `pkg/cmd/cmd_history.go` | `pkg/cmd/maintenance/` | `grava history` |
| `pkg/cmd/commit.go` | `pkg/cmd/sync/` | `grava commit` |
| `pkg/cmd/import.go` | `pkg/cmd/sync/` | `grava import` |
| `pkg/cmd/export.go` | `pkg/cmd/sync/` | `grava export` |

**Files that STAY in `pkg/cmd/` (root level):**
- `root.go` — root command + global vars + `Execute()`
- `init.go` — `grava init`
- `version.go` — `grava version`
- `start.go` — `grava start` (dolt server)
- `stop.go` — `grava stop` (dolt server)
- `util.go` — shared helpers (`writeJSONError`, etc.)
- `config.go` — `grava config`

**⚠️ IMPORTANT — Circular Import Prevention:**
- Sub-packages (`issues/`, `graph/`, etc.) must NOT import from each other
- Sub-packages may import `pkg/dolt`, `pkg/errors`, `pkg/log`, `pkg/validation`
- Global vars (`Store`, `outputJSON`, `actor`, `agentModel`, `Notifier`) live in `pkg/cmd` root — sub-commands access via `cmd.Store` etc.
- Pattern: each sub-package exports an `AddCommands(root *cobra.Command)` function that `root.go` calls

**⚠️ IMPORTANT — Package name must match directory name:**
- Files in `pkg/cmd/issues/` → `package issues`
- Files in `pkg/cmd/graph/` → `package graph`  ← **CONFLICT** with `pkg/graph` — use `package cmdgraph` or `package graphcmd`
- Files in `pkg/cmd/maintenance/` → `package maintenance`
- Files in `pkg/cmd/sync/` → `package sync`  ← **CONFLICT** with stdlib `sync` — use `package synccmd` or `package cmdsync`

**Registration pattern in root.go:**
```go
import (
    "github.com/hoangtrungnguyen/grava/pkg/cmd/issues"
    cmdgraph "github.com/hoangtrungnguyen/grava/pkg/cmd/graph"
    "github.com/hoangtrungnguyen/grava/pkg/cmd/maintenance"
    synccmd "github.com/hoangtrungnguyen/grava/pkg/cmd/sync"
)

func init() {
    // ... existing flag setup ...
    issues.AddCommands(rootCmd)
    cmdgraph.AddCommands(rootCmd)
    maintenance.AddCommands(rootCmd)
    synccmd.AddCommands(rootCmd)
}
```

### Story 1.1 Learnings — Apply to This Story

From the Story 1.1 dev agent record:

1. **zerolog is now in go.mod** (`github.com/rs/zerolog v1.34.0`) — confirmed present. Import directly without `go get`.
2. **Import alias required for `pkg/errors`**: always use `gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"` in any file importing both stdlib `errors` and `pkg/errors`.
3. **MockStore location**: `pkg/dolt/mock_client.go` (flat, not in `mock/` subdirectory). Architecture spec says `pkg/dolt/mock/store_mock.go` — do NOT move `mock_client.go`; create `pkg/notify/mock/notifier_mock.go` for new mock (different package, new directory).
4. **Skip list in PersistentPreRunE**: `start`, `stop` commands must remain in skip list after reorganization.
5. **macOS filepath.EvalSymlinks**: tests using `t.TempDir()` on macOS may hit `/var` → `/private/var` symlink. Apply `filepath.EvalSymlinks` on both sides of path comparisons in tests.
6. **TDD cycle**: follow red-green-refactor. Write test first, see it fail, implement, see it pass.
7. **`pkg/devlog` stub**: do NOT import or use `devlog` in any new code. It is already stubbed.

### Architecture Compliance Checklist

- [ ] `WithDeadlockRetry` only wraps SELECT FOR UPDATE — NEVER wraps `WithAuditedTx`
- [ ] No `log.Fatal`, `os.Exit`, `panic` in any goroutine or production code
- [ ] `Notifier.Send` errors are non-fatal — logged to stderr, never propagated
- [ ] Coordinator `Start` returns buffered `chan error` (size 1)
- [ ] Sub-package names avoid stdlib collision (`package cmdgraph` not `package graph`)
- [ ] `gravaerrors` alias used consistently in all files importing `pkg/errors`
- [ ] All new `pkg/` business logic receives `zerolog.Logger` as parameter (not global)
- [ ] `context.Background()` only at `RunE` entry points, never in `pkg/` internals
- [ ] All test files use `require` (fatal) and `assert` (non-fatal) from testify

### File Structure Requirements

**Files to create (new):**
- `pkg/dolt/retry.go`
- `pkg/dolt/retry_test.go`
- `pkg/notify/notifier.go`
- `pkg/notify/notifier_test.go`
- `pkg/notify/mock/notifier_mock.go`
- `pkg/coordinator/coordinator.go`
- `pkg/coordinator/coordinator_test.go`
- `pkg/cmd/issues/` (directory + moved/refactored files)
- `pkg/cmd/graph/` (directory + moved/refactored files — package `cmdgraph`)
- `pkg/cmd/maintenance/` (directory + moved/refactored files)
- `pkg/cmd/sync/` (directory + moved/refactored files — package `synccmd`)

**Files to modify (existing):**
- `pkg/cmd/root.go` — add `var Notifier notify.Notifier = notify.NewConsoleNotifier()`; add sub-package command registration
- `pkg/cmd/root.go` — update `init()` to call `issues.AddCommands(rootCmd)` etc.

**Files NOT to touch:**
- `pkg/dolt/tx.go` — already complete from Story 1.1
- `pkg/errors/errors.go` — already complete from Story 1.1
- `pkg/log/log.go` — already complete from Story 1.1
- `pkg/utils/schema.go` — already complete from Story 1.1
- `pkg/dolt/mock_client.go` — do NOT move/rename (breaking change)

### Testing Requirements

**Unit test targets (no DB required):**
- `pkg/dolt/retry_test.go` — mock `fn` closures returning nil/deadlock/non-deadlock errors
- `pkg/notify/notifier_test.go` — ConsoleNotifier writes to stderr; MockNotifier captures calls
- `pkg/coordinator/coordinator_test.go` — Start returns channel; ctx cancellation exits goroutine

**Test helper patterns from Story 1.1:**
```go
// Suppress zerolog output in tests:
log := zerolog.Nop()

// Use testify:
require.NoError(t, err)   // fatal on fail
assert.Equal(t, expected, actual)  // non-fatal

// Table-driven tests:
testCases := []struct{ name string; input X; want Y }{...}
for _, tc := range testCases {
    t.Run(tc.name, func(t *testing.T) { ... })
}
```

**Integration tests**: skipped unless `GRAVA_TEST_DB=1` — mark with `//go:build integration` tag.

### Project Structure Notes

**Existing packages state BEFORE Story 1.2:**

| Package | Current State | Story 1.2 Change |
|---|---|---|
| `pkg/dolt/retry.go` | Does not exist | Create with `WithDeadlockRetry` |
| `pkg/notify/` | Does not exist | Create with `Notifier` + `ConsoleNotifier` |
| `pkg/coordinator/` | Does not exist | Create with `Coordinator.Start` |
| `pkg/cmd/` | Flat (all commands in one package) | Split into `issues/`, `graph/`, `maintenance/`, `sync/` |
| `pkg/cmd/root.go` | No Notifier var | Add `var Notifier notify.Notifier` |

**Packages already complete from Story 1.1 (do NOT recreate):**
- `pkg/errors/errors.go` — `GravaError` with constructor
- `pkg/log/log.go` — zerolog global logger
- `pkg/dolt/tx.go` — `WithAuditedTx`
- `pkg/dolt/events.go` — `Event*` constants
- `pkg/utils/schema.go` — `CheckSchemaVersion`

### References

- [Epic 1 Story 1.2 spec: _bmad-output/planning-artifacts/epics/epic-01-foundation.md](_bmad-output/planning-artifacts/epics/epic-01-foundation.md)
- [Architecture ADR-H3 (deadlock prevention): _bmad-output/planning-artifacts/architecture.md](_bmad-output/planning-artifacts/architecture.md) — section "ADR-H3"
- [Architecture ADR-N1 (Notifier interface): _bmad-output/planning-artifacts/architecture.md](_bmad-output/planning-artifacts/architecture.md) — section "ADR-N1"
- [Architecture C2 (Coordinator Error Channel): _bmad-output/planning-artifacts/architecture.md](_bmad-output/planning-artifacts/architecture.md) — section "Gap Analysis C2"
- [Architecture pkg/cmd reorganization: _bmad-output/planning-artifacts/architecture.md](_bmad-output/planning-artifacts/architecture.md) — section "pkg/cmd reorganization"
- [Architecture WithDeadlockRetry pattern: _bmad-output/planning-artifacts/architecture.md](_bmad-output/planning-artifacts/architecture.md) — section "WithDeadlockRetry — for SELECT FOR UPDATE operations only"
- [Architecture Notifier Injection: _bmad-output/planning-artifacts/architecture.md](_bmad-output/planning-artifacts/architecture.md) — section "Notifier Injection"
- [Previous story 1.1 file: _bmad-output/implementation-artifacts/1-1-core-error-types-logging-and-transaction-infrastructure.md](_bmad-output/implementation-artifacts/1-1-core-error-types-logging-and-transaction-infrastructure.md)
- [pkg/dolt/client.go](pkg/dolt/client.go) — Store interface (BeginTx, LogEventTx, etc.)
- [pkg/dolt/tx.go](pkg/dolt/tx.go) — WithAuditedTx reference implementation
- [pkg/dolt/mock_client.go](pkg/dolt/mock_client.go) — existing MockStore pattern
- [pkg/cmd/root.go](pkg/cmd/root.go) — current root command structure to modify
- [go.mod](go.mod) — Go 1.24.0; zerolog v1.34.0 present; mysql driver v1.9.3
- Go module: `github.com/hoangtrungnguyen/grava`
- `github.com/go-sql-driver/mysql v1.9.3` — use `mysql.MySQLError.Number` for deadlock check

## Dev Agent Record

### Agent Model Used

{{agent_model_name_version}}

### Debug Log References

### Completion Notes List

### File List

**New files created:**
- `pkg/dolt/retry.go` — WithDeadlockRetry + isMySQLDeadlock
- `pkg/dolt/retry_test.go` — 7 unit tests (success, deadlock retry, non-deadlock, error type)
- `pkg/notify/notifier.go` — Notifier interface + ConsoleNotifier
- `pkg/notify/notifier_test.go` — ConsoleNotifier stderr capture + nil-error contract
- `pkg/notify/mock/notifier_mock.go` — MockNotifier for test injection
- `pkg/notify/mock/notifier_mock_test.go` — MockNotifier call capture + error return tests
- `pkg/coordinator/coordinator.go` — Coordinator struct + Start(<-chan error) pattern
- `pkg/coordinator/coordinator_test.go` — buffered channel, ctx cancellation, no-os-exit contract
- `pkg/cmddeps/deps.go` — pointer-based Deps struct for circular-import-safe injection
- `pkg/cmd/issues/issues.go` — issues sub-package (create, show, list, update, drop, assign, label, comment, subtask, quick)
- `pkg/cmd/graph/graph.go` — cmdgraph sub-package (dep, graph, ready, blocked, search, stats)
- `pkg/cmd/maintenance/maintenance.go` — maintenance sub-package (compact, doctor, undo, clear, history)
- `pkg/cmd/sync/sync.go` — synccmd sub-package (commit, import, export)

**Modified files:**
- `pkg/cmd/root.go` — added Notifier var, cmddeps.Deps, sub-package AddCommands registrations
- `pkg/cmd/assign.go`, `blocked.go`, `clear.go`, `cmd_history.go`, `comment.go`, `commit.go`, `compact.go`, `create.go`, `dep.go`, `doctor.go`, `drop.go`, `export.go`, `graph.go`, `import.go`, `label.go`, `list.go`, `quick.go`, `ready.go`, `search.go`, `show.go`, `stats.go`, `subtask.go`, `undo.go`, `update.go` — removed `init()` to prevent double-registration
- `pkg/cmd/clear_test.go`, `commands_test.go` — updated to use sub-package StdinReader vars
