# Story 1.3: Worktree Resolver, Named Functions & Test Harness (Story 0c)

Status: done

## Story

As a Grava developer,
I want a testable named-function architecture with a worktree resolver and testutil package,
So that all core commands can be unit-tested in isolation without starting a real Dolt server.

## Acceptance Criteria

1. `.grava/` resolution follows priority chain: `GRAVA_DIR` env var → `.grava/redirect` file → CWD walk up to filesystem root; resolution logic lives in `pkg/grava/resolver.go`
2. `grava claim`, `grava import`, and `grava ready` are extracted into named functions: `claimIssue(ctx, store, actorID, issueID)`, `importIssues(ctx, store, r io.Reader)`, `readyQueue(ctx, store)` — each returning `(result, error)`
3. `internal/testutil/` package provides: `MockStore` (implements `dolt.Store` interface), `NewTestDB(t)` helper returning an in-memory test DB (backed by go-sqlmock), and `AssertGravaError(t, err, code)` assertion helper
4. At least one unit test per named function uses `MockStore`/sqlmock and passes without a Dolt process running
5. `grava init` writes `.grava/` to `.git/info/exclude` (not `.gitignore`) and migrates existing `.gitignore` `.grava/` entries (ADR-H5)

## Tasks / Subtasks

- [x] Task 1: Create `pkg/grava/resolver.go` — Full ADR-004 priority chain (AC: #1)
  - [x] Create directory `pkg/grava/`
  - [x] Implement `ResolveGravaDir() (string, error)` with three-step priority chain:
    1. `GRAVA_DIR` env var (if set, validate it exists and is a directory; error `NOT_INITIALIZED` if not)
    2. Walk CWD upward checking for `.grava/redirect` file; if found, read its content (trimmed), resolve relative to the redirect file's parent directory, validate resulting path is a directory; error `REDIRECT_STALE` if path invalid
    3. Walk CWD upward looking for `.grava/` directory; error `NOT_INITIALIZED` if not found
  - [x] One-level redirect only — no chained redirects (never follow redirect inside a redirect target)
  - [x] Write `pkg/grava/resolver_test.go` with table-driven tests:
    - GRAVA_DIR set to valid dir → returns it
    - GRAVA_DIR set to non-existent path → NOT_INITIALIZED error
    - Redirect file in CWD → follows relative path to real `.grava/`
    - Redirect file with stale/invalid path → REDIRECT_STALE error
    - CWD walk finds `.grava/` directory in parent → returns it
    - No .grava/ anywhere → NOT_INITIALIZED error
    - Apply `filepath.EvalSymlinks` on both sides of path assertions (macOS /var → /private/var)
  - [x] Update `pkg/cmd/root.go` PersistentPreRunE: replace `utils.ResolveGravaDir()` with `grava.ResolveGravaDir()` (import `pkg/grava`)
  - [x] Add deprecation comment to `utils.ResolveGravaDir()`: `// Deprecated: use pkg/grava.ResolveGravaDir() for full ADR-004 redirect chain. Kept for existing tests.`

- [x] Task 2: Create `grava claim` command with `claimIssue` named function (AC: #2, #4)
  - [x] Define `ClaimResult struct { IssueID, Status, Actor string }` in `pkg/cmd/issues/claim.go` (new file)
  - [x] Implement `claimIssue(ctx context.Context, store dolt.Store, issueID, actor, model string) (ClaimResult, error)`:
    - Use `dolt.WithAuditedTx` (no `WithDeadlockRetry` wrapping — single row lock, audit log must not duplicate)
    - Inside transaction: `SELECT status FROM issues WHERE id=? FOR UPDATE`
    - If no rows: `gravaerrors.New("ISSUE_NOT_FOUND", fmt.Sprintf("issue %s not found", issueID), sql.ErrNoRows)`
    - If status == `"in_progress"`: `gravaerrors.New("ALREADY_CLAIMED", fmt.Sprintf("issue %s is already claimed", issueID), nil)`
    - If status != `"open"`: `gravaerrors.New("INVALID_STATUS_TRANSITION", fmt.Sprintf("cannot claim issue %s: status is %q (must be open)", issueID, status), nil)`
    - `UPDATE issues SET status='in_progress', assignee=?, agent_model=?, updated_at=NOW(), updated_by=? WHERE id=?`
    - Audit event: `dolt.AuditEvent{IssueID: issueID, EventType: dolt.EventClaim, Actor: actor, Model: model, OldValue: map[string]any{"status": "open"}, NewValue: map[string]any{"status": "in_progress", "actor": actor}}`
    - Return `ClaimResult{IssueID: issueID, Status: "in_progress", Actor: actor}` on success
  - [x] Implement `newClaimCmd(d *cmddeps.Deps) *cobra.Command`:
    - `Use: "claim <issue-id>"`, require exactly 1 arg
    - Call `claimIssue(ctx, *d.Store, args[0], *d.Actor, *d.AgentModel)`
    - Human output: `fmt.Fprintf(cmd.OutOrStdout(), "✅ Claimed %s (status: in_progress, actor: %s)\n", result.IssueID, result.Actor)`
    - JSON output: `json.NewEncoder(cmd.OutOrStdout()).Encode(result)` (fields: `id`, `status`, `actor`)
    - On error: use `writeJSONError(cmd, err)` pattern from `pkg/cmd/util.go` if `*d.OutputJSON`
  - [x] Register `newClaimCmd` in `issues.AddCommands(root, d)` in `pkg/cmd/issues/issues.go`
  - [x] Write `pkg/cmd/issues/claim_test.go`:
    - Use `DATA-DOG/go-sqlmock` to set expectations: `ExpectBegin()`, `ExpectQuery("SELECT status FROM issues")`, `ExpectExec("UPDATE issues")`, `ExpectExec` for events table, `ExpectCommit()`
    - Create `dolt.NewClientFromDB(db)` with the mock DB (already exists in `pkg/dolt/client.go`)
    - Test happy path: open issue → claims successfully → returns ClaimResult
    - Test ISSUE_NOT_FOUND: no rows returned from SELECT
    - Test ALREADY_CLAIMED: status = `"in_progress"` returned from SELECT
    - Test INVALID_STATUS_TRANSITION: status = `"closed"` returned from SELECT

- [x] Task 3: Extract `importIssues` named function from `pkg/cmd/sync/sync.go` (AC: #2, #4)
  - [x] Define `ImportResult struct { Imported, Updated, Skipped int }` in `pkg/cmd/sync/sync.go`
  - [x] Implement `importIssues(ctx context.Context, store dolt.Store, r io.Reader, overwrite, skipExisting bool) (ImportResult, error)`:
    - Extract the scanner loop from `newImportCmd.RunE` into this function
    - Begin TX, scan JSONL lines, upsert issues/dependencies, commit; return ImportResult
    - Any `fmt.Errorf` for user-facing errors must be wrapped in `gravaerrors.New("IMPORT_ROLLED_BACK", ...)` with appropriate code
    - Preserve existing behavior for all flag combinations (skip/overwrite)
  - [x] Simplify `newImportCmd.RunE`: open file, call `importIssues(ctx, *d.Store, f, overwrite, skipExisting)`, format output
  - [x] Write `pkg/cmd/sync/sync_test.go`:
    - Use `strings.NewReader("...")` with valid JSONL input — no file I/O needed
    - Use sqlmock: `ExpectBegin()`, `ExpectExec` for INSERT/UPDATE, `ExpectCommit()`
    - Test happy path: 2 issues imported, result.Imported == 2
    - Test skip-existing: query returns row → result.Skipped incremented

- [x] Task 4: Extract `readyQueue` named function from `pkg/cmd/graph/graph.go` (AC: #2, #4)
  - [x] Implement `readyQueue(ctx context.Context, store dolt.Store, limit int) ([]*graph.ReadyTask, error)`:
    - Call `graph.LoadGraphFromDB(store)` → returns `*graph.AdjacencyDAG`
    - Call `graph.NewReadyEngine(dag, graph.DefaultReadyEngineConfig()).ComputeReady(limit)`
    - Return `([]*graph.ReadyTask, error)` directly
    - Wrap errors with `gravaerrors.New("DB_UNREACHABLE", "failed to load graph", err)` for DB errors
  - [x] Simplify `newReadyCmd.RunE`: call `readyQueue(ctx, *d.Store, readyLimit)`, then handle priority filtering and formatting
  - [x] Write `pkg/cmd/graph/graph_test.go` (or `ready_test.go`):
    - Use sqlmock to set up expected DB queries that `graph.LoadGraphFromDB` will execute
    - Verify that `readyQueue` with 0 issues returns empty slice without error
    - This test passes without a running Dolt server

- [x] Task 5: Create `internal/testutil/` package (AC: #3, #4)
  - [x] Create directory `internal/testutil/`
  - [x] Create `internal/testutil/testutil.go`:
    - `package testutil`
    - **MockStore** — complete implementation of `dolt.Store` with configurable fn fields and call recording:
      ```go
      type ExecContextCall struct {
          Query string
          Args  []any
      }
      type MockStore struct {
          // Configurable responses
          ExecContextFn     func(ctx context.Context, query string, args ...any) (sql.Result, error)
          QueryRowContextFn func(ctx context.Context, query string, args ...any) *sql.Row
          QueryContextFn    func(ctx context.Context, query string, args ...any) (*sql.Rows, error)
          LogEventTxFn     func(ctx context.Context, tx *sql.Tx, issueID, eventType, actor, model string, old, new any) error
          BeginTxFn        func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
          // Call recording
          ExecContextCalls []ExecContextCall
      }
      ```
    - All `dolt.Store` interface methods must be implemented; default behavior (when Fn fields are nil): return `nil, nil` or zero value — mirrors existing `pkg/dolt/mock_client.go`
    - `func NewMockStore() *MockStore` constructor
    - **NewTestDB** — creates sqlmock-backed `*sql.DB`:
      ```go
      func NewTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
          t.Helper()
          db, mock, err := sqlmock.New()
          require.NoError(t, err)
          t.Cleanup(func() { _ = db.Close() })
          return db, mock
      }
      ```
    - **AssertGravaError** — testify assertion helper:
      ```go
      func AssertGravaError(t *testing.T, err error, code string) {
          t.Helper()
          require.Error(t, err)
          var gravaErr *gravaerrors.GravaError
          require.True(t, errors.As(err, &gravaErr), "expected *GravaError, got: %T: %v", err, err)
          assert.Equal(t, code, gravaErr.Code, "GravaError code mismatch")
      }
      ```
  - [x] Write `internal/testutil/testutil_test.go`:
    - Test `NewTestDB` returns valid DB and mock (sqlmock.ExpectPing passes)
    - Test `AssertGravaError` catches correct code, fails on wrong code
    - Test `MockStore` satisfies `dolt.Store` at compile time: `var _ dolt.Store = (*MockStore)(nil)`

- [x] Task 6: Update `grava init` for `.git/info/exclude` — ADR-H5 (AC: #5)
  - [x] Add `writeGitExclude(repoRoot string) (migrated bool, err error)` to `pkg/cmd/init.go` or new `pkg/utils/gitexclude.go`:
    - Locate `.git/` directory: `filepath.Join(repoRoot, ".git")`; if not found, return nil (no git repo — graceful skip)
    - Create `.git/info/` directory if not exists (`os.MkdirAll`)
    - Read `.git/info/exclude` (if exists); if `.grava/` already present (exact line match), return immediately (idempotent)
    - Append `.grava/\n` to `.git/info/exclude`
    - Check `.gitignore` for `.grava/` entry: read line by line; if found, remove the line, write updated `.gitignore` back, set `migrated = true`
  - [x] Call `writeGitExclude(cwd)` in `initCmd.RunE` after `.grava/` directory is created (step 2)
  - [x] Print migration message if `migrated == true`: `_, _ = fmt.Fprintln(cmd.OutOrStdout(), "📝 Migrated .grava/ exclusion from .gitignore to .git/info/exclude")`
  - [x] Write `TestWriteGitExclude_*` in `pkg/cmd/init_test.go` (or `pkg/utils/gitexclude_test.go`):
    - Create `t.TempDir()` with fake `.git/info/` and `.gitignore` containing `.grava/`
    - Call `writeGitExclude`, verify `.git/info/exclude` contains `.grava/`, `.gitignore` does not
    - Call `writeGitExclude` again — verify idempotent (only one `.grava/` line in exclude)
    - Call `writeGitExclude` with no `.gitignore` — no error, exclude file written

- [x] Task 7: Final verification
  - [x] `go test ./...` — all non-integration tests pass (integration tests auto-skipped)
  - [x] `go vet ./...` — zero warnings
  - [x] `go build -ldflags="-s -w" ./...` — single binary, no new runtime deps

## Dev Notes

### Critical: Resolver Priority Chain — ADR-004

The full resolution priority (replace the current 2-step `utils.ResolveGravaDir()`):

```go
// pkg/grava/resolver.go
package grava

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"

    gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
)

func ResolveGravaDir() (string, error) {
    // 1. GRAVA_DIR env var override
    if dir := os.Getenv("GRAVA_DIR"); dir != "" {
        if info, err := os.Stat(dir); err == nil && info.IsDir() {
            return dir, nil
        }
        return "", gravaerrors.New("NOT_INITIALIZED",
            fmt.Sprintf("GRAVA_DIR=%q does not exist or is not a directory", dir), nil)
    }

    cwd, err := os.Getwd()
    if err != nil {
        return "", fmt.Errorf("failed to get working directory: %w", err)
    }

    // 2. Walk upward checking for .grava/redirect file first
    dir := cwd
    for {
        redirectPath := filepath.Join(dir, ".grava", "redirect")
        if data, err := os.ReadFile(redirectPath); err == nil {
            target := strings.TrimSpace(string(data))
            if !filepath.IsAbs(target) {
                target = filepath.Join(dir, ".grava", target)
            }
            target = filepath.Clean(target)
            if info, err := os.Stat(target); err == nil && info.IsDir() {
                return target, nil
            }
            return "", gravaerrors.New("REDIRECT_STALE",
                fmt.Sprintf(".grava/redirect points to %q which does not exist", target), nil)
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            break
        }
        dir = parent
    }

    // 3. Walk upward looking for .grava/ directory
    dir = cwd
    for {
        candidate := filepath.Join(dir, ".grava")
        if info, err := os.Stat(candidate); err == nil && info.IsDir() {
            return candidate, nil
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            break
        }
        dir = parent
    }

    return "", gravaerrors.New("NOT_INITIALIZED",
        "no .grava/ directory found — run 'grava init' to initialise", nil)
}
```

**Critical notes:**
- Redirect file is `.grava/redirect` (inside `.grava/` dir, NOT alongside it)
- Redirect path is relative to the `.grava/` directory (e.g., `"../../.grava"` means parent-parent of `.grava/`)
- One-level redirect only — never follow redirect inside redirect target
- Error codes: `REDIRECT_STALE` (not `NOT_INITIALIZED`) for broken redirect path
- `utils.ResolveGravaDir()` in `pkg/utils/schema.go` MUST NOT be deleted — existing tests cover it; add deprecation comment and leave in place

### Critical: pkg/cmd/root.go Import Change

```go
// pkg/cmd/root.go — import update
import (
    // ... existing imports ...
    "github.com/hoangtrungnguyen/grava/pkg/grava"  // ADD
    // Remove: utils package is still needed for CheckSchemaVersion, WriteSchemaVersion
)

// In PersistentPreRunE:
// CHANGE: gravaDir, err := utils.ResolveGravaDir()
// TO:
gravaDir, err := grava.ResolveGravaDir()
if err != nil {
    return gravaerrors.New("NOT_INITIALIZED", "grava is not initialized in this directory; run 'grava init' first", err)
}
```

### Critical: claimIssue — Exact SQL and Error Codes

```go
// pkg/cmd/issues/claim.go
package issues

import (
    "context"
    "database/sql"
    "errors"
    "fmt"

    "github.com/hoangtrungnguyen/grava/pkg/dolt"
    gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
)

type ClaimResult struct {
    IssueID string `json:"id"`
    Status  string `json:"status"`
    Actor   string `json:"actor"`
}

func claimIssue(ctx context.Context, store dolt.Store, issueID, actor, model string) (ClaimResult, error) {
    var currentStatus string

    err := dolt.WithAuditedTx(ctx, store, []dolt.AuditEvent{
        {
            IssueID:   issueID,
            EventType: dolt.EventClaim,
            Actor:     actor,
            Model:     model,
            OldValue:  map[string]any{"status": "open"},
            NewValue:  map[string]any{"status": "in_progress", "actor": actor},
        },
    }, func(tx *sql.Tx) error {
        // Lock the row exclusively
        row := tx.QueryRowContext(ctx,
            "SELECT status FROM issues WHERE id = ? FOR UPDATE", issueID)
        if err := row.Scan(&currentStatus); err != nil {
            if errors.Is(err, sql.ErrNoRows) {
                return gravaerrors.New("ISSUE_NOT_FOUND",
                    fmt.Sprintf("issue %s not found", issueID), err)
            }
            return gravaerrors.New("DB_UNREACHABLE",
                fmt.Sprintf("failed to read issue %s", issueID), err)
        }
        if currentStatus == "in_progress" {
            return gravaerrors.New("ALREADY_CLAIMED",
                fmt.Sprintf("issue %s is already claimed by another actor", issueID), nil)
        }
        if currentStatus != "open" {
            return gravaerrors.New("INVALID_STATUS_TRANSITION",
                fmt.Sprintf("cannot claim issue %s: status is %q (must be \"open\")", issueID, currentStatus), nil)
        }
        _, err := tx.ExecContext(ctx,
            "UPDATE issues SET status='in_progress', assignee=?, agent_model=?, updated_at=NOW(), updated_by=? WHERE id=?",
            actor, model, actor, issueID)
        if err != nil {
            return gravaerrors.New("DB_UNREACHABLE", "failed to update issue status", err)
        }
        return nil
    })
    if err != nil {
        return ClaimResult{}, err
    }
    return ClaimResult{IssueID: issueID, Status: "in_progress", Actor: actor}, nil
}
```

**Anti-patterns to prevent:**
- ❌ Do NOT wrap `WithAuditedTx` in `WithDeadlockRetry` — audit log duplicates on retry
- ❌ Do NOT return raw `sql` errors — always wrap in `GravaError`
- ❌ Do NOT call `context.Background()` inside the function — `ctx` is passed from `RunE`

### Critical: importIssues — Named Function Extraction

Extract from `pkg/cmd/sync/sync.go:newImportCmd.RunE`. The function signature:

```go
// pkg/cmd/sync/sync.go
type ImportResult struct {
    Imported int `json:"imported"`
    Updated  int `json:"updated"`
    Skipped  int `json:"skipped"`
}

func importIssues(ctx context.Context, store dolt.Store, r io.Reader, overwrite, skipExisting bool) (ImportResult, error) {
    tx, err := store.BeginTx(ctx, nil)
    if err != nil {
        return ImportResult{}, gravaerrors.New("DB_UNREACHABLE", "failed to start transaction", err)
    }
    defer tx.Rollback() //nolint:errcheck

    scanner := bufio.NewScanner(r)
    // ... existing scanner loop logic moved here verbatim ...
    // For user-facing import errors, wrap in gravaerrors.New("IMPORT_ROLLED_BACK", ..., err)

    if err := scanner.Err(); err != nil {
        return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK", "error reading import data", err)
    }
    if err := tx.Commit(); err != nil {
        return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK", "failed to commit import transaction", err)
    }
    return ImportResult{Imported: count, Updated: updated, Skipped: skipped}, nil
}
```

**newImportCmd.RunE simplification:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    // ... flag validation ...
    f, err := os.Open(importFile)
    if err != nil { return fmt.Errorf("failed to open import file: %w", err) }
    defer f.Close()

    result, err := importIssues(ctx, *d.Store, f, importOverwrite, importSkipExisting)
    if err != nil {
        if *d.OutputJSON { return writeJSONError(cmd, err) }
        return err
    }
    cmd.Printf("✅ Imported %d items (Updated: %d, Skipped: %d)\n", result.Imported, result.Updated, result.Skipped)
    return nil
},
```

### Critical: readyQueue — Named Function Extraction

Extract from `pkg/cmd/graph/graph.go:newReadyCmd.RunE`:

```go
// pkg/cmd/graph/graph.go
func readyQueue(ctx context.Context, store dolt.Store, limit int) ([]*graph.ReadyTask, error) {
    dag, err := graph.LoadGraphFromDB(store)
    if err != nil {
        return nil, gravaerrors.New("DB_UNREACHABLE", "failed to load graph", err)
    }
    engine := graph.NewReadyEngine(dag, graph.DefaultReadyEngineConfig())
    tasks, err := engine.ComputeReady(limit)
    if err != nil {
        return nil, gravaerrors.New("DB_UNREACHABLE", "failed to compute ready queue", err)
    }
    return tasks, nil
}
```

**newReadyCmd.RunE simplification:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    tasks, err := readyQueue(ctx, *d.Store, readyLimit)
    if err != nil {
        if *d.OutputJSON { return writeJSONError(cmd, err) }
        return err
    }
    // priority filtering and tabwriter output stays in RunE
},
```

Note: `ctx` must come from `cmd.Context()` inside RunE, not `context.Background()`.

### Critical: internal/testutil — Complete MockStore Contract

The `internal/testutil.MockStore` must implement ALL methods of `dolt.Store` interface. Verify the interface at compile time with:

```go
var _ dolt.Store = (*MockStore)(nil)
```

Current `dolt.Store` interface (from `pkg/dolt/client.go`) requires:
- `GetNextChildSequence(parentID string) (int, error)`
- `BeginTx(ctx, opts) (*sql.Tx, error)`
- `Exec(query string, args ...any) (sql.Result, error)`
- `ExecContext(ctx, query, args...) (sql.Result, error)`
- `QueryRow(query, args...) *sql.Row`
- `QueryRowContext(ctx, query, args...) *sql.Row`
- `Query(query, args...) (*sql.Rows, error)`
- `QueryContext(ctx, query, args...) (*sql.Rows, error)`
- `SetMaxOpenConns(n int)`
- `SetMaxIdleConns(n int)`
- `DB() *sql.DB`
- `Close() error`
- `LogEvent(issueID, eventType, actor, model string, old, new any) error`
- `LogEventTx(ctx, tx, issueID, eventType, actor, model string, old, new any) error`

**Do NOT delete or modify `pkg/dolt/mock_client.go`** — it is used by existing tests.

The `internal/testutil.MockStore` is a more sophisticated version with configurable fn fields, used specifically for testing named functions.

### Critical: .git/info/exclude Migration — ADR-H5

**ADR-H5 exact requirement:** `grava init` writes `.grava/` to `.git/info/exclude` (local-only, never committed) — NOT to `.gitignore`. On init, if `.gitignore` has `.grava/` entry, remove it and print migration message.

```go
// pkg/utils/gitexclude.go (preferred location for testability)
package utils

import (
    "bufio"
    "os"
    "path/filepath"
    "strings"
)

const gravaExcludeEntry = ".grava/"

// WriteGitExclude adds ".grava/" to .git/info/exclude and migrates from .gitignore.
// Returns (migrated=true, nil) if .gitignore migration was performed.
// Idempotent: safe to call multiple times.
// Gracefully skips if .git directory is absent.
func WriteGitExclude(repoRoot string) (migrated bool, err error) {
    gitDir := filepath.Join(repoRoot, ".git")
    if _, err := os.Stat(gitDir); os.IsNotExist(err) {
        return false, nil // not a git repo, skip gracefully
    }

    infoDir := filepath.Join(gitDir, "info")
    if err := os.MkdirAll(infoDir, 0755); err != nil {
        return false, fmt.Errorf("failed to create .git/info/: %w", err)
    }

    excludeFile := filepath.Join(infoDir, "exclude")
    // Read existing content
    existing, _ := os.ReadFile(excludeFile) // ignore error if not exists
    if strings.Contains(string(existing), gravaExcludeEntry) {
        // Already present — check gitignore migration only
        migrated, err = migrateGitignore(repoRoot)
        return
    }

    // Append to exclude file
    f, err := os.OpenFile(excludeFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        return false, fmt.Errorf("failed to open .git/info/exclude: %w", err)
    }
    defer f.Close()
    if _, err := f.WriteString("\n" + gravaExcludeEntry + "\n"); err != nil {
        return false, fmt.Errorf("failed to write .git/info/exclude: %w", err)
    }

    migrated, err = migrateGitignore(repoRoot)
    return
}

func migrateGitignore(repoRoot string) (bool, error) {
    gitignorePath := filepath.Join(repoRoot, ".gitignore")
    data, err := os.ReadFile(gitignorePath)
    if os.IsNotExist(err) {
        return false, nil
    }
    if err != nil {
        return false, fmt.Errorf("failed to read .gitignore: %w", err)
    }

    lines := strings.Split(string(data), "\n")
    var filtered []string
    removed := false
    for _, line := range lines {
        if strings.TrimSpace(line) == ".grava/" || strings.TrimSpace(line) == ".grava" {
            removed = true
            continue
        }
        filtered = append(filtered, line)
    }
    if !removed {
        return false, nil
    }

    if err := os.WriteFile(gitignorePath, []byte(strings.Join(filtered, "\n")), 0644); err != nil {
        return false, fmt.Errorf("failed to write .gitignore: %w", err)
    }
    return true, nil
}
```

**In `pkg/cmd/init.go` RunE** (after step 2 — MkdirAll for .grava/):
```go
migrated, err := utils.WriteGitExclude(cwd)
if err != nil {
    if !outputJSON {
        _, _ = fmt.Fprintf(cmd.OutOrStdout(), "⚠️  Warning: could not update .git/info/exclude: %v\n", err)
    }
    // non-fatal — continue init
}
if migrated && !outputJSON {
    _, _ = fmt.Fprintln(cmd.OutOrStdout(), "📝 Migrated .grava/ exclusion from .gitignore to .git/info/exclude")
}
```

### Story 1.1 & 1.2 Learnings — Apply to This Story

1. **macOS `filepath.EvalSymlinks`**: all tests using `t.TempDir()` path comparisons must apply `filepath.EvalSymlinks` on both sides. `/var` → `/private/var` on macOS causes false mismatches otherwise.

2. **`gravaerrors` alias**: any file importing both `pkg/errors` and stdlib `errors` must use alias:
   ```go
   import (
       "errors"
       gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
   )
   ```

3. **`pkg/dolt/mock_client.go`**: do NOT delete or move. Leave it in place — existing tests depend on it.

4. **`zerolog.Nop()`**: any function accepting `zerolog.Logger` as a parameter should receive `zerolog.Nop()` in tests to suppress output.

5. **`context.Background()` only at RunE entry points**: named functions receive `ctx` from the caller; never use `context.Background()` inside business logic packages.

6. **TDD cycle**: write failing test first, implement, see it pass. Use table-driven tests for all error paths.

7. **`pkg/cmd/util.go`**: `writeJSONError(cmd *cobra.Command, err error) error` is the helper for JSON error output in RunE. Import and use it from all command packages — do not duplicate.

8. **`context.TODO()` is forbidden** per architecture enforcement guidelines — always use `ctx` passed from the caller.

9. **`dolt.NewClientFromDB(db)`**: exists in `pkg/dolt/client.go` — use this to wrap sqlmock `*sql.DB` into a `dolt.Store` for integration-style unit tests that need real `*sql.Tx` handling.

10. **Story 1.2 pkg/cmd reorganization is complete**: `claim` goes into `pkg/cmd/issues/` (same package as other issue management commands). File `pkg/cmd/issues/claim.go` is a new file in the existing package.

### Architecture Compliance Checklist

- [ ] `pkg/grava/resolver.go` has exactly 3 resolution steps matching ADR-004 priority chain
- [ ] Redirect file path is resolved relative to `.grava/` directory, not CWD
- [ ] Error code `REDIRECT_STALE` used for broken redirect (not `NOT_INITIALIZED`)
- [ ] `claimIssue` uses `WithAuditedTx` — NOT wrapped in `WithDeadlockRetry`
- [ ] `ClaimResult`, `ImportResult` JSON fields use `snake_case` tags
- [ ] All named functions receive `ctx context.Context` as first parameter
- [ ] Named functions do NOT call `context.Background()` internally
- [ ] `internal/testutil.MockStore` compile-time check: `var _ dolt.Store = (*MockStore)(nil)`
- [ ] `WriteGitExclude` is idempotent (safe to call twice)
- [ ] `WriteGitExclude` non-fatal on error (init continues with warning, not abort)
- [ ] `gravaerrors` alias used consistently
- [ ] All tests use `testify/require` (fatal) and `testify/assert` (non-fatal)

### File Structure Requirements

**Files to create (new):**
- `pkg/grava/resolver.go`
- `pkg/grava/resolver_test.go`
- `pkg/cmd/issues/claim.go`
- `pkg/cmd/issues/claim_test.go`
- `pkg/cmd/sync/sync_test.go`
- `pkg/cmd/graph/ready_test.go` (or `graph_test.go`)
- `internal/testutil/testutil.go`
- `internal/testutil/testutil_test.go`
- `pkg/utils/gitexclude.go`
- `pkg/utils/gitexclude_test.go`

**Files to modify (existing):**
- `pkg/cmd/root.go` — replace `utils.ResolveGravaDir()` with `grava.ResolveGravaDir()`, add `pkg/grava` import
- `pkg/cmd/issues/issues.go` — register `newClaimCmd` in `AddCommands`
- `pkg/cmd/sync/sync.go` — extract `importIssues` named function, add `ImportResult` type
- `pkg/cmd/graph/graph.go` — extract `readyQueue` named function
- `pkg/cmd/init.go` — add `WriteGitExclude` call after `.grava/` creation
- `pkg/utils/schema.go` — add deprecation comment to `ResolveGravaDir()`

**Files NOT to touch:**
- `pkg/dolt/mock_client.go` — do NOT move, rename, or delete
- `pkg/dolt/tx.go` — already complete from Story 1.1
- `pkg/dolt/retry.go` — already complete from Story 1.2
- `pkg/errors/errors.go` — already complete from Story 1.1
- `pkg/log/log.go` — already complete from Story 1.1
- `pkg/notify/` — already complete from Story 1.2
- `pkg/coordinator/` — already complete from Story 1.2

### Technical Stack Confirmation (from go.mod)

- Go: **1.24.0** (module `github.com/hoangtrungnguyen/grava`)
- DB driver: `github.com/go-sql-driver/mysql v1.9.3`
- CLI: `github.com/spf13/cobra v1.10.2`
- Testing: `github.com/stretchr/testify v1.11.1`
- Mock DB: `github.com/DATA-DOG/go-sqlmock v1.5.2` — already in go.mod, use for `NewTestDB`
- Logging: `github.com/rs/zerolog v1.34.0` — already in go.mod

**No new `go get` commands needed** — all dependencies already present.

### Project Structure Notes

**Current state BEFORE Story 1.3:**

| Package | Current State | Story 1.3 Change |
|---|---|---|
| `pkg/grava/` | Does not exist | Create with `ResolveGravaDir()` full ADR-004 chain |
| `pkg/utils/schema.go` | Has `ResolveGravaDir()` (2-step) | Deprecate; keep for existing tests |
| `pkg/cmd/issues/claim.go` | Does not exist | Create `claimIssue` + `newClaimCmd` |
| `pkg/cmd/sync/sync.go` | Has anonymous `importIssues` logic in RunE | Extract to named `importIssues` function |
| `pkg/cmd/graph/graph.go` | Has anonymous `readyQueue` logic in RunE | Extract to named `readyQueue` function |
| `internal/testutil/` | Does not exist | Create `MockStore`, `NewTestDB`, `AssertGravaError` |
| `pkg/utils/gitexclude.go` | Does not exist | Create `WriteGitExclude` |
| `pkg/cmd/init.go` | No `.git/info/exclude` handling | Add `WriteGitExclude` call |

### References

- [Epic 1 Story 1.3 spec: _bmad-output/planning-artifacts/epics/epic-01-foundation.md](_bmad-output/planning-artifacts/epics/epic-01-foundation.md) — "Story 1.3: Worktree Resolver, Named Functions & Test Harness"
- [Architecture ADR-003 (Named functions): _bmad-output/planning-artifacts/architecture.md] — section "ADR-003: pkg/ops Interface Preparation"
- [Architecture ADR-004 (Worktree redirect): _bmad-output/planning-artifacts/architecture.md] — section "ADR-004: Worktree-Aware Redirect Architecture"
- [Architecture ADR-H5 (.git/info/exclude): _bmad-output/planning-artifacts/architecture.md] — section "ADR-H5"
- [Architecture Testing: _bmad-output/planning-artifacts/architecture.md] — section "Testing"
- [Previous story 1.1: _bmad-output/implementation-artifacts/1-1-core-error-types-logging-and-transaction-infrastructure.md](_bmad-output/implementation-artifacts/1-1-core-error-types-logging-and-transaction-infrastructure.md)
- [Previous story 1.2: _bmad-output/implementation-artifacts/1-2-concurrency-primitives-and-coordinator-error-channel.md](_bmad-output/implementation-artifacts/1-2-concurrency-primitives-and-coordinator-error-channel.md)
- [pkg/utils/schema.go](pkg/utils/schema.go) — `ResolveGravaDir()` to deprecate; `CheckSchemaVersion()`, `WriteSchemaVersion()` to keep
- [pkg/dolt/client.go](pkg/dolt/client.go) — `Store` interface definition; `NewClientFromDB(db)` for sqlmock tests
- [pkg/dolt/mock_client.go](pkg/dolt/mock_client.go) — existing `MockStore` (do NOT modify)
- [pkg/dolt/tx.go](pkg/dolt/tx.go) — `WithAuditedTx` reference implementation
- [pkg/dolt/retry.go](pkg/dolt/retry.go) — `WithDeadlockRetry` (do NOT wrap WithAuditedTx in this)
- [pkg/dolt/events.go](pkg/dolt/events.go) — `EventClaim` and all event constants
- [pkg/cmd/root.go](pkg/cmd/root.go) — PersistentPreRunE; add `grava.ResolveGravaDir()` import
- [pkg/cmd/issues/issues.go](pkg/cmd/issues/issues.go) — `AddCommands` to update
- [pkg/cmd/sync/sync.go](pkg/cmd/sync/sync.go) — `newImportCmd` to extract `importIssues` from
- [pkg/cmd/graph/graph.go](pkg/cmd/graph/graph.go) — `newReadyCmd` to extract `readyQueue` from
- [pkg/cmd/init.go](pkg/cmd/init.go) — add `WriteGitExclude` call
- [pkg/cmd/util.go](pkg/cmd/util.go) — `writeJSONError` helper
- [pkg/cmddeps/deps.go](pkg/cmddeps/deps.go) — `Deps` struct for command injection
- [go.mod](go.mod) — Go 1.24.0; `go-sqlmock v1.5.2` for `NewTestDB`
- Go module: `github.com/hoangtrungnguyen/grava`

## Dev Agent Record

### Agent Model Used

claude-sonnet-4-6

### Debug Log References

- `dolt.WithAuditedTx` sequence: Begin → fn(tx) → LogEventTx (INSERT events) → Commit; sqlmock expectations must follow this exact order
- `graph.LoadGraphFromDB` runs 2 queries (issues + dependencies); sqlmock for `readyQueue` must expect both: `SELECT id, title, ... FROM issues` and `SELECT from_id, to_id, type FROM dependencies`
- `importIssues` extraction: scanner loop uses `tx.QueryRowContext` to check existing IDs before insert/update; mock must expect `ExpectQuery` before `ExpectExec`
- macOS symlink: `resolver_test.go` uses `filepath.EvalSymlinks` on both sides of CWD-walk assertions

### Completion Notes List

- All 5 ACs satisfied
- Task 1: `pkg/grava/resolver.go` implements full ADR-004 three-step chain (GRAVA_DIR → redirect → CWD walk); 7 table-driven tests pass; `root.go` updated to use new resolver; `utils.ResolveGravaDir()` deprecated with comment
- Task 2: `pkg/cmd/issues/claim.go` — `claimIssue` named function with `WithAuditedTx`, `ClaimResult` type, `newClaimCmd`; 4 sqlmock tests (HappyPath, NotFound, AlreadyClaimed, InvalidTransition); registered in `AddCommands`
- Task 3: `importIssues(ctx, store, r, overwrite, skipExisting) (ImportResult, error)` extracted from `sync.go`; `newImportCmd.RunE` simplified; 4 sqlmock tests in `sync_test.go`
- Task 4: `readyQueue(ctx, store, limit) ([]*graph.ReadyTask, error)` extracted from `graph.go`; `newReadyCmd.RunE` simplified; 2 sqlmock tests in `ready_test.go`
- Task 5: `internal/testutil/testutil.go` — complete `MockStore` (compile-time interface check), `NewTestDB` (go-sqlmock backed), `AssertGravaError`; 11 tests pass
- Task 6: `pkg/utils/gitexclude.go` — `WriteGitExclude` idempotent, ADR-H5 compliant; `init.go` updated to call it; 8 tests covering all branches
- Task 7: `go build ./...` clean, `go vet ./...` clean, `go test ./... -count=1` — all 21 packages pass, zero failures

### File List

**Created:**
- `pkg/grava/resolver.go`
- `pkg/grava/resolver_test.go`
- `pkg/cmd/issues/claim.go`
- `pkg/cmd/issues/claim_test.go`
- `pkg/cmd/sync/sync_test.go`
- `pkg/cmd/graph/ready_test.go`
- `internal/testutil/testutil.go`
- `internal/testutil/testutil_test.go`
- `pkg/utils/gitexclude.go`
- `pkg/utils/gitexclude_test.go`

**Modified:**
- `pkg/cmd/root.go` — switched to `grava.ResolveGravaDir()`, added `pkg/grava` import
- `pkg/cmd/issues/issues.go` — added `newClaimCmd(d)` to `AddCommands`
- `pkg/cmd/sync/sync.go` — extracted `ImportResult` + `importIssues()` named function, simplified `newImportCmd.RunE`
- `pkg/cmd/graph/graph.go` — added `readyQueue()` named function, updated `newReadyCmd.RunE`
- `pkg/cmd/init.go` — added `utils.WriteGitExclude(cwd)` call after `.grava/` creation
- `pkg/utils/schema.go` — added `Deprecated:` comment to `ResolveGravaDir()`
- `pkg/cmd/ready_test.go` — tests `TestReadyCmd`/`TestBlockedCmd` through `rootCmd` (already existed, not in original File List)

**Modified (review follow-up fixes):**
- `pkg/utils/gitexclude.go` — replaced `strings.Contains` with line-exact `hasExactLine` check (AI-H2)
- `pkg/utils/gitexclude_test.go` — added `TestWriteGitExclude_CommentContainingGravaNotFalsePositive` (AI-H2)
- `pkg/cmd/sync/sync.go` — added `overwrite && skipExisting` mutual-exclusion guard (AI-M1)
- `pkg/cmd/graph/graph.go` — added `ctx.Err()` pre-flight check in `readyQueue` (AI-M2)
- `pkg/cmd/issues/claim.go` — added ✅ emoji to claim output (AI-L1)

**Deleted (review follow-up):**
- `pkg/cmd/ready.go` — dead code, orphan from Story 1.2 reorganization (AI-H1)


## Code Review Action Items

Adversarial code review performed 2026-03-29 (claude-sonnet-4-6). 2 HIGH, 3 MEDIUM, 2 LOW findings. Must resolve HIGH and MEDIUM before marking `done`.

- [x] [AI-H1] **Delete `pkg/cmd/ready.go`** — declares `readyCmd` (106 lines) with no `func init()`, never registered; dead code since Story 1.2 reorganization moved `ready` to `pkg/cmd/graph/`. `TestReadyCmd` in `pkg/cmd/ready_test.go` already tests the real implementation via `rootCmd`. File must be deleted to prevent future duplicate-registration confusion.

- [x] [AI-H2] **Fix `WriteGitExclude` idempotency check** (`pkg/utils/gitexclude.go:29`) — `strings.Contains(string(existing), ".grava/")` matches `.grava/` as substring (e.g., `# .grava/` comment in exclude file triggers false-positive, real entry never added). Replace with line-exact check: `strings.TrimSpace(line) == ".grava/"` matching pattern used in `migrateGitignore`. Add test case: exclude file has `# .grava/` comment → entry still added.

- [x] [AI-M1] **Add mutual-exclusion guard inside `importIssues`** (`pkg/cmd/sync/sync.go:247`) — `overwrite=true, skipExisting=true` together produce silent wrong behavior (MySQL INSERT IGNORE suppresses ON DUPLICATE KEY UPDATE). Validate at function entry: `if overwrite && skipExisting { return ImportResult{}, gravaerrors.New("INVALID_ARGS", ...) }`. Epic 7 hook pipeline will call this directly, bypassing `newImportCmd` validation.

- [x] [AI-M2] **Pre-flight context check in `readyQueue`** (`pkg/cmd/graph/graph.go:418`) — `_ = ctx` silently discards context; Ctrl+C / timeouts never abort graph loading. Add `if err := ctx.Err(); err != nil { return nil, gravaerrors.New("CANCELLED", "readyQueue cancelled", err) }` before calling `graph.LoadGraphFromDB`.

- [x] [AI-M3] **Add `pkg/cmd/ready_test.go` to File List** — `TestReadyCmd` / `TestBlockedCmd` test the `graph/` implementation through `rootCmd` but were not listed in modified files. No code change needed; update File List below.

- [x] [AI-L1] **Add ✅ emoji to `claimCmd` output** (`pkg/cmd/issues/claim.go:87`) — story spec says `"✅ Claimed %s..."` but implementation outputs `"Claimed %s..."`. One-character fix for consistency with other commands.

- [x] [AI-L2] **Update story `Status` field to `done`** — sprint-status.yaml was updated to `done` by commit `d0744d4` but story file header still says `review`. Update after all other action items resolved.

## Change Log

- Initial implementation of all 7 tasks: full ADR-004 resolver, claimIssue named function + grava claim command, importIssues/readyQueue extractions, internal/testutil harness, WriteGitExclude ADR-H5 (Date: 2026-03-27)
- Code review performed, 7 action items added (Date: 2026-03-29)
- All 7 review action items resolved: deleted dead code ready.go (H1), fixed WriteGitExclude substring bug (H2), added importIssues mutual-exclusion guard (M1), added readyQueue context pre-flight (M2), updated File List (M3), added claim emoji (L1), status → done (L2) (Date: 2026-03-29)
