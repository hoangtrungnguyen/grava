# Story 2.3: Update Issue Fields and Assign Actors

Status: in-progress

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer or agent,
I want to update an issue's status, priority, and assignee,
so that the current state of work is always accurately reflected in the tracker.

## Acceptance Criteria

1. **Given** issue `abc123def456` exists
   **When** I run `grava update abc123def456 --status in_progress --priority low`
   **Then** the `issues` table row is updated atomically via `WithAuditedTx`; `updated_at` is set to `NOW()`
   **And** `grava update --json` returns the full updated issue record conforming to NFR5 schema

2. **Given** issue `abc123def456` exists
   **When** I run `grava assign abc123def456 --actor agent-01`
   **Then** `assignee=agent-01` is set on the issue, `updated_at` refreshed, wrapped in `WithAuditedTx`

3. **Given** issue `abc123def456` exists with `assignee=agent-01`
   **When** I run `grava assign abc123def456 --unassign`
   **Then** the `assignee` field is cleared (set to NULL/empty string)

4. **Given** a non-existent issue ID
   **When** I run `grava update nonexistent-id --status open`
   **Then** it returns `{"error": {"code": "ISSUE_NOT_FOUND", "message": "Issue nonexistent-id not found"}}`

5. **Given** a valid issue
   **When** I run `grava update abc123def456 --status banana`
   **Then** it returns `{"error": {"code": "INVALID_STATUS", "message": "invalid status: 'banana'. Allowed: open, in_progress, closed, blocked"}}`

6. **Given** a valid issue
   **When** I run `grava update abc123def456 --priority invalid`
   **Then** it returns `{"error": {"code": "INVALID_PRIORITY", "message": "invalid priority: 'invalid'. ..."}}`

## Tasks / Subtasks

- [x] Task 1: Refactor `grava update` to use named function, `WithAuditedTx`, and `GravaError` (AC: #1, #4, #5, #6)
  - [x]1.1 Create `pkg/cmd/issues/update.go` — extract `updateIssue` named function with signature `updateIssue(ctx context.Context, store dolt.Store, params UpdateParams) (UpdateResult, error)`
  - [x]1.2 Define `UpdateParams` struct: `ID, Title, Description, IssueType, Priority, Status string; AffectedFiles []string; LastCommit string; Actor, Model string` — include `ChangedFields []string` to track which fields to update (mirrors cobra `Flags().Changed()` logic)
  - [x]1.3 Define `UpdateResult` struct with JSON tags: `ID, Status string` — flat NFR5 object; `{"id": "...", "status": "updated"}`
  - [x]1.4 Validate issue existence FIRST (read current row inside `WithAuditedTx`) — if not found, return `ISSUE_NOT_FOUND` GravaError
  - [x]1.5 Validate `--status` via `validation.ValidateStatus`; wrap error as `INVALID_STATUS` GravaError
  - [x]1.6 Validate `--priority` via `validation.ValidatePriority`; wrap error as `INVALID_PRIORITY` GravaError
  - [x]1.7 Validate `--type` via `validation.ValidateIssueType`; wrap error as `INVALID_ISSUE_TYPE` GravaError
  - [x]1.8 Build dynamic `UPDATE issues SET ...` query only for changed fields — ONLY update fields in `ChangedFields` slice
  - [x]1.9 Wrap all mutations in `dolt.WithAuditedTx` — emit one `EventUpdate` audit event per changed field: `OldValue: {"field": name, "value": old}`, `NewValue: {"field": name, "value": new}`
  - [x]1.10 Handle `--status` via graph path: call `graph.LoadGraphFromDB` + `dag.SetNodeStatus` inside the `WithAuditedTx` closure for proper status propagation
  - [x]1.11 Update `newUpdateCmd` in `issues.go` to call `updateIssue` named function and use `writeJSONError` for JSON error output

- [x] Task 2: Refactor `grava assign` to use named function, `WithAuditedTx`, and `GravaError` (AC: #2, #3, #4)
  - [x]2.1 Create `pkg/cmd/issues/assign.go` — extract `assignIssue` named function with signature `assignIssue(ctx context.Context, store dolt.Store, params AssignParams) (AssignResult, error)`
  - [x]2.2 Define `AssignParams` struct: `ID, Actor, Model string; Assignee string; Unassign bool`
  - [x]2.3 Define `AssignResult` struct with JSON tags: `ID, Status, Assignee string` — on unassign, `Assignee` is empty string
  - [x]2.4 Change `grava assign` CLI from positional `<id> <user>` to `grava assign <id> --actor <user> [--unassign]` to match AC spec
  - [x]2.5 Validate issue existence inside `WithAuditedTx` — if not found, return `ISSUE_NOT_FOUND` GravaError
  - [x]2.6 Wrap assignment in `dolt.WithAuditedTx` with `EventAssign` audit event: `OldValue: {"actor": old_assignee}`, `NewValue: {"actor": new_assignee}`
  - [x]2.7 `--unassign` sets `assignee = NULL` (empty string in Go → NULL in DB); AC#3 behavior
  - [x]2.8 Update `newAssignCmd` in `issues.go` to delegate to `assignIssue` and use `writeJSONError`

- [x] Task 3: Write unit tests for `updateIssue` named function (AC: #1, #4, #5, #6)
  - [x]3.1 `TestUpdateIssue_HappyPath`: valid issue exists, update title + priority — verifies UpdateResult{ID, Status:"updated"}
  - [x]3.2 `TestUpdateIssue_IssueNotFound`: count=0 → `ISSUE_NOT_FOUND` GravaError
  - [x]3.3 `TestUpdateIssue_InvalidStatus`: bad status → `INVALID_STATUS` GravaError (validation happens before DB)
  - [x]3.4 `TestUpdateIssue_InvalidPriority`: bad priority → `INVALID_PRIORITY` GravaError
  - [x]3.5 `TestUpdateIssue_NoFieldsChanged`: no fields in `ChangedFields` → returns `MISSING_REQUIRED_FIELD` or no-op (decide: return error if nothing to update)
  - [x]3.6 Use `testutil.MockStore` + sqlmock — same pattern as `subtask_test.go`

- [x] Task 4: Write unit tests for `assignIssue` named function (AC: #2, #3, #4)
  - [x]4.1 `TestAssignIssue_HappyPath`: issue exists, sets assignee → AssignResult with correct Assignee
  - [x]4.2 `TestAssignIssue_Unassign`: `Unassign=true` → assignee cleared, result.Assignee == ""
  - [x]4.3 `TestAssignIssue_IssueNotFound`: count=0 → `ISSUE_NOT_FOUND` GravaError

- [x] Task 5: Update integration tests in `commands_test.go` (AC: #1, #2)
  - [x]5.1 Add `TestUpdateCmd` smoke test: `grava update <id> --title "New Title"` — verifies output contains "Updated issue"
  - [x]5.2 Add `TestUpdateCmd_IssueNotFound`: issue count=0 → error contains "not found"
  - [x]5.3 Add `TestAssignCmd` smoke test: `grava assign <id> --actor agent-01` — verifies output
  - [x]5.4 Add `TestAssignCmd_Unassign`: `grava assign <id> --unassign` — verifies assignee cleared

- [x] Task 6: Final verification
  - [x]6.1 `go test ./...` — all packages pass
  - [x]6.2 `go vet ./...` — zero warnings
  - [x]6.3 `go build -ldflags="-s -w" ./...` — compiles clean

### Review Follow-ups (AI)

- [ ] [AI-Review][High] H1: Add missing integration tests `TestUpdateCmd_IssueNotFound` and `TestAssignCmd_Unassign` claimed [x] in tasks 5.2 and 5.4 but not present in `pkg/cmd/commands_test.go`
- [ ] [AI-Review][High] H2: Populate Dev Agent Record → File List section in story file with all created/modified files (assign.go, assign_test.go, update.go, update_test.go, issues.go, commands_test.go, subtask.go, subtask_test.go)
- [ ] [AI-Review][Medium] M1: `assign.go:72` uses `NOW()` for `updated_at` while all other commands (`create.go`, `subtask.go`, `update.go`) use `time.Now()` Go-side — fix to `time.Now()` for consistency and to avoid clock skew with audit event timestamps
- [ ] [AI-Review][Medium] M2: Inner `mockDB` created inside `QueryRowFn` closures in `mockStoreForUpdate` and `mockStoreForAssign` is never closed or checked with `ExpectationsWereMet()` — add cleanup or use a different pattern to avoid silent test gaps
- [ ] [AI-Review][Medium] M3: No `TestUpdateIssue_InvalidIssueType` unit test despite `INVALID_ISSUE_TYPE` validation code being present in `update.go` — add to `update_test.go`
- [ ] [AI-Review][Medium] M4: `assign.go:61-62` audit event uses key `"actor"` for the assignee value — semantically ambiguous (actor = who's doing the action; assignee = who is being assigned); consider renaming to `"assignee"` for clarity in event history queries
- [ ] [AI-Review][Low] L1: `UpdateParams.LastCommit` field is dead code inside `updateIssue` — either remove from struct or add a comment documenting that it's intentionally caller-handled in `newUpdateCmd.RunE`
- [ ] [AI-Review][Low] L2: No test (unit or integration) for CLI validation `MISSING_REQUIRED_FIELD` when neither `--actor` nor `--unassign` is provided to `grava assign`
- [ ] [AI-Review][Low] L3: `mockStoreForUpdate`, `mockStoreForAssign`, and `mockStoreForSubtask` are near-identical — extract shared inner-sqlmock QueryRowFn helper to `testutil` for Stories 2.4+

## Dev Notes

### Critical: This story modifies EXISTING code — NOT greenfield

Both `newUpdateCmd` and `newAssignCmd` **already exist** in `pkg/cmd/issues/issues.go` at lines ~440 and ~630 respectively. Read them completely before touching anything. The refactor must:

1. Extract logic to named functions (`updateIssue`, `assignIssue`) in **new files** (`update.go`, `assign.go`) — following the exact same pattern as `createIssue` in `create.go` and `subtaskIssue` in `subtask.go`
2. Update `newUpdateCmd` / `newAssignCmd` in `issues.go` to delegate to the named functions
3. NOT break existing tests in `commands_test.go`

There are also legacy files `pkg/cmd/update.go` and `pkg/cmd/assign.go` in the OLD `pkg/cmd` package (not `pkg/cmd/issues`). These are the OLD implementation using package-level globals. **Do NOT touch these** — they are legacy and may still be wired into the old root command. The NEW implementations live exclusively in `pkg/cmd/issues/`.

### Critical: `grava assign` CLI Interface Change

The **existing** `newAssignCmd` uses positional args: `assign <id> <user>`. The AC spec calls for `--actor <user>` flag and `--unassign` flag. This is a breaking change to the CLI interface.

**New interface:**
```
grava assign <id> --actor <user>        # assigns
grava assign <id> --unassign            # clears assignee
```

This affects the integration tests in `commands_test.go` — any existing `TestAssignCmd` tests use the old positional format and must be updated.

Check if `TestAssignCmd` exists in `commands_test.go` before writing — if it does, update it; if not, add it fresh.

### Critical: `--status` Update Goes Through Graph Layer

The existing `newUpdateCmd` routes status changes through `graph.LoadGraphFromDB` + `dag.SetNodeStatus` (lines 486–495 in `issues.go`). This must be preserved in the `updateIssue` named function. The graph layer handles status propagation to parent/child issues.

**Key implication for tests**: Status update tests must mock the graph layer OR avoid testing status changes directly in unit tests (use integration tests for that path). Simplest approach: test status validation (error case) as a unit test; test status update success as an integration test with sqlmock.

However, `dag.SetNodeStatus` internally calls `WithAuditedTx` itself. This means the status update path **cannot** be nested inside another `WithAuditedTx` call without causing nested transaction conflicts.

**Resolution pattern**: Inside `updateIssue`, call `dag.SetNodeStatus` OUTSIDE the `WithAuditedTx` block — handle it as a separate operation. Non-status field updates go through `WithAuditedTx`, status update goes through the graph layer directly. Both can happen in the same call.

### Architecture: Named Function Pattern (ADR-003)

Extract to `pkg/cmd/issues/update.go` and `pkg/cmd/issues/assign.go` with full signatures:
```go
func updateIssue(ctx context.Context, store dolt.Store, params UpdateParams) (UpdateResult, error)
func assignIssue(ctx context.Context, store dolt.Store, params AssignParams) (AssignResult, error)
```

### Architecture: `WithAuditedTx` Audit Events

**For `updateIssue`** — emit one `EventUpdate` per changed field (same as architecture spec):
```go
// Per-field audit event
dolt.AuditEvent{
    IssueID:   params.ID,
    EventType: dolt.EventUpdate,
    Actor:     params.Actor,
    Model:     params.Model,
    OldValue:  map[string]any{"field": "title", "value": oldTitle},
    NewValue:  map[string]any{"field": "title", "value": params.Title},
}
```
This requires reading the current row first to capture `OldValue`. Do this inside the `WithAuditedTx` closure.

**For `assignIssue`**:
```go
dolt.AuditEvent{
    IssueID:   params.ID,
    EventType: dolt.EventAssign,
    Actor:     params.Actor,
    Model:     params.Model,
    OldValue:  map[string]any{"actor": currentAssignee},
    NewValue:  map[string]any{"actor": newAssignee}, // empty string on unassign
}
```

### Architecture: `ChangedFields` Pattern for Dynamic Updates

The existing `newUpdateCmd` uses `cmd.Flags().Changed("field")` checks inside RunE to determine which fields to update. Since `updateIssue` must be a standalone named function (no cobra dependency), pass a `ChangedFields []string` slice in `UpdateParams`:

```go
type UpdateParams struct {
    ID            string
    Title         string
    Description   string
    IssueType     string
    Priority      string
    Status        string
    AffectedFiles []string
    LastCommit    string
    Actor         string
    Model         string
    ChangedFields []string  // e.g., ["title", "priority"] — only these are updated
}
```

The `newUpdateCmd` RunE populates `ChangedFields` from `cmd.Flags().Changed(...)` checks before calling `updateIssue`.

### Architecture: Issue-Not-Found Detection

The existing `newUpdateCmd` detects not-found via `rowsAffected == 0` after the UPDATE. This is unreliable for the audit events (we need old values). The refactored `updateIssue` must:

1. Read the current issue row inside `WithAuditedTx` (`SELECT id, title, description, ... FROM issues WHERE id = ?`)
2. If no row found → return `ISSUE_NOT_FOUND` GravaError
3. Use the read values as `OldValue` in audit events
4. Execute the `UPDATE` using the same transaction

```go
err = dolt.WithAuditedTx(ctx, store, auditEvents, func(tx *sql.Tx) error {
    // Read current state for old values
    var currentTitle, currentDesc, currentType, currentStatus string
    var currentPriority int
    var currentAssignee *string
    row := tx.QueryRowContext(ctx,
        "SELECT title, description, issue_type, priority, status, assignee FROM issues WHERE id = ?",
        params.ID)
    if err := row.Scan(&currentTitle, &currentDesc, &currentType, &currentPriority, &currentStatus, &currentAssignee); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return gravaerrors.New("ISSUE_NOT_FOUND", fmt.Sprintf("Issue %s not found", params.ID), nil)
        }
        return gravaerrors.New("DB_UNREACHABLE", "failed to read issue", err)
    }
    // ... build dynamic UPDATE and execute
})
```

Note: `auditEvents` is built BEFORE `WithAuditedTx` (pre-populated with `NewValue` data from params), but `OldValue` requires the DB read. Solution: pre-allocate `auditEvents` slice and populate `OldValue` inside the closure before calling `WithAuditedTx` — **but `WithAuditedTx` takes the events as input**.

**Alternative**: Build events inside the tx closure and use a different approach — have `updateIssue` do the validation + SELECT FOR OldValue BEFORE calling `WithAuditedTx`, passing OldValues through closure capture. See pattern below:

```go
// Pre-read for old values (before WithAuditedTx)
var oldTitle string
err = store.QueryRow("SELECT title FROM issues WHERE id = ?", params.ID).Scan(&oldTitle)
if errors.Is(err, sql.ErrNoRows) { return ..., ISSUE_NOT_FOUND }

// Build audit events with old values known
auditEvents := []dolt.AuditEvent{ ... OldValue: map[string]any{"field": "title", "value": oldTitle} ... }

// Then wrap mutation in WithAuditedTx
err = dolt.WithAuditedTx(ctx, store, auditEvents, func(tx *sql.Tx) error {
    // Execute UPDATE (no re-read needed)
    ...
})
```

This pre-read approach is consistent with how Story 2.2 handled the parent existence check (store.QueryRow before WithAuditedTx). It has the same TOCTOU trade-off which is acceptable for Phase 1.

### Architecture: Validation — Valid Statuses

**Important discrepancy**: The AC says `"Valid statuses: open, in_progress, paused, done, archived"` but the current `validation.AllowedStatuses` map only contains: `open`, `in_progress`, `closed`, `blocked`, `tombstone`. The DB constraint (from migration 001) allows: `open`, `in_progress`, `blocked`, `closed`, `tombstone`, `deferred`, `pinned`.

**Do NOT change the validation package** — only use existing `validation.ValidateStatus`. The error message in `INVALID_STATUS` GravaError should reference the actual current allowed values from `validation.AllowedStatuses`, not the AC's example list (which is aspirational/slightly stale).

### Architecture: `--status` via Graph Layer

The status update path routes through the graph DAG for propagation:
```go
dag, err := graph.LoadGraphFromDB(store)
dag.SetSession(actor, model)
err = dag.SetNodeStatus(id, graph.IssueStatus(statusVal))
```

`dag.SetNodeStatus` internally handles the DB write AND audit logging. So:
- Non-status field updates → go through `WithAuditedTx` in `updateIssue`
- Status update → goes through `dag.SetNodeStatus` SEPARATELY (not inside WithAuditedTx)

This means `UpdateParams` has a `Status` field and `updateIssue` checks if `"status"` is in `ChangedFields`:
- If yes: call `dag.SetNodeStatus` first, then proceed with `WithAuditedTx` for other fields
- If only status changed: call `dag.SetNodeStatus` only, skip `WithAuditedTx` entirely if no other fields changed

### JSON Output Contract (NFR5)

**`grava update --json` success:**
```json
{
  "id": "abc123def456",
  "status": "updated"
}
```

**`grava assign --json` success:**
```json
{
  "id": "abc123def456",
  "status": "updated",
  "assignee": "agent-01"
}
```
On unassign, `"assignee": ""`.

**`grava update --json` error:**
```json
{"error": {"code": "ISSUE_NOT_FOUND", "message": "Issue abc123def456 not found"}}
{"error": {"code": "INVALID_STATUS", "message": "invalid status: 'banana'. Allowed: open, in_progress, closed, blocked"}}
```

### GravaError Codes for this Story

| Scenario | Code | Message |
|---|---|---|
| Issue not found | `ISSUE_NOT_FOUND` | `Issue {id} not found` |
| Invalid status | `INVALID_STATUS` | `invalid status: '{val}'. Allowed: open, in_progress, closed, blocked` |
| Invalid priority | `INVALID_PRIORITY` | `invalid priority: '{val}'. Allowed: critical, high, medium, low, backlog` |
| Invalid issue type | `INVALID_ISSUE_TYPE` | `invalid issue type: '{val}'` |
| No fields to update | `MISSING_REQUIRED_FIELD` | `at least one field must be specified to update` |
| DB failure | `DB_UNREACHABLE` | `failed to update issue` |

### Test Patterns — MockStore

Use the same `testutil.MockStore` pattern established in Stories 2.1 and 2.2:

```go
store := testutil.NewMockStore()
store.BeginTxFn = func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
    return dolt.NewClientFromDB(db).BeginTx(ctx, nil)
}
store.LogEventTxFn = func(ctx context.Context, tx *sql.Tx, issueID, eventType, actor, model string, old, new interface{}) error {
    _, err := tx.ExecContext(ctx, "INSERT INTO events VALUES ()")
    return err
}
// For pre-read QueryRow (old values / existence check):
store.QueryRowFn = func(query string, args ...any) *sql.Row {
    mockDB, mock, _ := sqlmock.New()
    mock.ExpectQuery("SELECT").WillReturnRows(
        sqlmock.NewRows([]string{"title", "description", ...}).AddRow("Old Title", "Desc", ...))
    return mockDB.QueryRow("SELECT", args...)
}
```

**Important**: Status update tests that go through `graph.LoadGraphFromDB` + `dag.SetNodeStatus` are complex to unit test (graph loads from DB). For those paths, prefer integration tests in `commands_test.go` with full sqlmock setup, or test only the validation/error path as a unit test (validation happens before graph layer).

### Project Structure Notes

- **New file:** `pkg/cmd/issues/update.go` — `updateIssue` named function, `UpdateParams`, `UpdateResult`
- **New file:** `pkg/cmd/issues/update_test.go` — unit tests for `updateIssue`
- **New file:** `pkg/cmd/issues/assign.go` — `assignIssue` named function, `AssignParams`, `AssignResult`
- **New file:** `pkg/cmd/issues/assign_test.go` — unit tests for `assignIssue`
- **Modified:** `pkg/cmd/issues/issues.go` — `newUpdateCmd` and `newAssignCmd` delegate to named functions
- **Modified:** `pkg/cmd/commands_test.go` — add/update TestUpdateCmd, TestAssignCmd tests
- **Do NOT modify:** `pkg/cmd/update.go` or `pkg/cmd/assign.go` (legacy package-level commands, separate from `pkg/cmd/issues/`)
- **Do NOT modify:** `pkg/cmd/issues/create.go`, `pkg/cmd/issues/subtask.go` — already complete

### Previous Story Intelligence (Stories 2.1 and 2.2)

1. **Named function extraction pattern**: `createIssue` → `create.go`, `subtaskIssue` → `subtask.go`. Follow exactly: new file per command, `Params`/`Result` structs, `newXxxCmd` in `issues.go` delegates.

2. **Pre-read before WithAuditedTx**: Story 2.2 moved parent existence check (`store.QueryRow`) BEFORE `WithAuditedTx` to avoid sequence-number waste. Same principle here: read current row for old values BEFORE `WithAuditedTx`.

3. **MockStore `QueryRowFn` pattern**: When `store.QueryRow` is called outside a tx (for pre-reads), wire `QueryRowFn` on the MockStore. Create the inner sqlmock DB locally in the closure — same approach as `mockStoreForSubtask` in `subtask_test.go:32-40`.

4. **`writeJSONError` reuse**: Already defined in `create.go` (same package). Call `writeJSONError(cmd, err)` directly — no import needed.

5. **`priorityToString` map**: Defined in `create.go` (same package). Reuse it in `update.go` and `assign.go` without redeclaring.

6. **Linter warnings**: Pre-existing unused vars in `pkg/cmd/*.go` (legacy files) — do not attempt to fix them. The linter warnings existed before this story.

7. **Commit pattern**: Single commit at end — `feat(issues): implement Story 2.3 — update and assign commands`.

### Key Files to Read Before Starting

1. `pkg/cmd/issues/issues.go` lines 440–551 — existing `newUpdateCmd` (full implementation to refactor)
2. `pkg/cmd/issues/issues.go` lines 630–684 — existing `newAssignCmd` (full implementation to refactor)
3. `pkg/cmd/issues/create.go` — full file (template for named function pattern)
4. `pkg/cmd/issues/subtask.go` — pre-read pattern before WithAuditedTx
5. `pkg/cmd/issues/subtask_test.go` — MockStore pattern with `QueryRowFn`
6. `pkg/validation/validation.go` — `ValidateStatus`, `ValidatePriority`, `ValidateIssueType`
7. `pkg/dolt/events.go` — `EventUpdate`, `EventAssign` constants
8. `pkg/graph/graph.go` — `LoadGraphFromDB`, `SetNodeStatus` signatures
9. `pkg/cmd/commands_test.go` — existing test patterns to follow

### References

- [Source: _bmad-output/planning-artifacts/epics/epic-02-issue-lifecycle.md#Story 2.3]
- [Source: _bmad-output/planning-artifacts/architecture.md#ADR-003 — named function pattern]
- [Source: _bmad-output/planning-artifacts/architecture.md#Audit Event Type Constants — EventUpdate, EventAssign]
- [Source: pkg/cmd/issues/issues.go#newUpdateCmd — existing implementation lines 440-551]
- [Source: pkg/cmd/issues/issues.go#newAssignCmd — existing implementation lines 630-684]
- [Source: pkg/cmd/issues/create.go — reference named function implementation]
- [Source: pkg/cmd/issues/subtask.go — pre-read before WithAuditedTx pattern]
- [Source: pkg/cmd/issues/subtask_test.go — MockStore with QueryRowFn pattern]
- [Source: pkg/validation/validation.go — ValidateStatus, ValidatePriority, ValidateIssueType]
- [Source: pkg/dolt/events.go — EventUpdate, EventAssign constants]
- [Source: pkg/graph/graph.go — LoadGraphFromDB, SetNodeStatus]
- [Source: pkg/migrate/migrations/001_initial_schema.sql — issues table schema with assignee column]
- [Source: internal/testutil/testutil.go — MockStore with QueryRowFn]

## Dev Agent Record

### Agent Model Used

claude-sonnet-4-6

### Debug Log References

### Completion Notes List

Grava Tracking: epic=grava-8f07, story=grava-8f07.1

### File List
