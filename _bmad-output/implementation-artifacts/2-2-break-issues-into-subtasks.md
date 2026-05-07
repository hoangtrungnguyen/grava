# Story 2.2: Break Issues into Subtasks

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer or agent,
I want to decompose an existing issue into numbered subtasks,
so that large issues can be tracked at a granular level with parent-child relationships.

## Acceptance Criteria

1. **Given** issue `abc123def456` exists with `status=open`
   **When** I run `grava subtask abc123def456 --title "Write unit tests"`
   **Then** a new subtask is created with ID format `abc123def456.1` (parent.sequence) using atomic ID generation on `child_counters`
   **And** `grava subtask --json` returns `{"id": "abc123def456.1", "title": "Write unit tests", "status": "open", "priority": "medium"}` conforming to NFR5 schema

2. **Given** a parent issue exists
   **When** `grava show <parent_id> --json` is called
   **Then** the response includes a `"subtasks"` array listing child issue IDs (e.g., `["abc123def456.1", "abc123def456.2"]`)

3. **Given** two concurrent `grava subtask abc123def456` calls
   **When** both execute simultaneously
   **Then** they produce subtasks `.1` and `.2` with no ID collision (atomic `child_counters` via `INSERT ... ON DUPLICATE KEY UPDATE`)

4. **Given** a non-existent parent ID
   **When** I run `grava subtask nonexistent-id --title "..."`
   **Then** it returns `{"error": {"code": "ISSUE_NOT_FOUND", "message": "Issue nonexistent-id not found"}}`

5. **Given** a missing `--title`
   **When** I run `grava subtask abc123def456`
   **Then** it returns `{"error": {"code": "MISSING_REQUIRED_FIELD", "message": "title is required"}}`

## Tasks / Subtasks

- [x] Task 1: Refactor `grava subtask` to use named function, `WithAuditedTx`, and `GravaError` (AC: #1, #3, #4, #5)
  - [x] 1.1 Create `pkg/cmd/issues/subtask.go` — extract `subtaskIssue` named function with signature `subtaskIssue(ctx context.Context, store dolt.Store, params SubtaskParams) (SubtaskResult, error)`
  - [x] 1.2 Define `SubtaskParams` struct: `ParentID, Title, Description, IssueType, Priority string; Ephemeral bool; AffectedFiles []string; Actor, Model string`
  - [x] 1.3 Define `SubtaskResult` struct with JSON tags: `ID, Title, Status, Priority string; Ephemeral bool` — same shape as `CreateResult` (NFR5 flat object)
  - [x] 1.4 Validate `Title != ""` → `MISSING_REQUIRED_FIELD` GravaError before any DB work
  - [x] 1.5 Validate issue type and priority using existing `validation.ValidateIssueType` and `validation.ValidatePriority`; wrap in GravaError (`INVALID_ISSUE_TYPE`, `INVALID_PRIORITY`)
  - [x] 1.6 Replace manual `BeginTx` / `tx.Commit` in `newSubtaskCmd` with `dolt.WithAuditedTx` — include both `EventSubtask` (create) and `EventDependencyAdd` (subtask-of) audit events
  - [x] 1.7 Parent existence check: use `tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM issues WHERE id = ?", parentID)` — if count=0, return `ISSUE_NOT_FOUND` GravaError
  - [x] 1.8 ID generation: call `idgen.NewStandardGenerator(store).GenerateChildID(parentID)` BEFORE opening the transaction — on failure return `DB_UNREACHABLE`
  - [x] 1.9 Update `newSubtaskCmd` in `issues.go` to call `subtaskIssue` named function (same pattern as `newCreateCmd` → `createIssue`)
  - [x] 1.10 JSON success output: return full `SubtaskResult` (id, title, status, priority, ephemeral) — NOT the old minimal `{"id":"...", "status":"created"}`
  - [x] 1.11 Error output via `writeJSONError` (already defined in `create.go`, same package — reuse it)

- [x] Task 2: Enhance `grava show --json` to include `subtasks` array (AC: #2)
  - [x] 2.1 In `newShowCmd`, query dependencies table: `SELECT d.from_id FROM dependencies d WHERE d.to_id = ? AND d.type = 'subtask-of' ORDER BY d.from_id`
  - [x] 2.2 Extend `IssueDetail` struct (in `issues.go`) with `Subtasks []string \`json:"subtasks,omitempty"\`` field
  - [x] 2.3 Populate `Subtasks` field in both JSON and human-readable output paths
  - [x] 2.4 Human output: print `Subtasks: [id1, id2, ...]` line only when subtasks exist

- [x] Task 3: Write unit tests for `subtaskIssue` named function (AC: #1, #3, #4, #5)
  - [x] 3.1 `TestSubtaskIssue_HappyPath`: parent exists, returns SubtaskResult with `parentID.N` ID, status=open, priority=medium
  - [x] 3.2 `TestSubtaskIssue_MissingTitle`: returns `MISSING_REQUIRED_FIELD` GravaError without hitting DB
  - [x] 3.3 `TestSubtaskIssue_ParentNotFound`: parent count=0 → returns `ISSUE_NOT_FOUND` GravaError; transaction rolls back
  - [x] 3.4 `TestSubtaskIssue_JSONOutputStructure`: verifies NFR5 — flat object, snake_case, string priority label not integer
  - [x] 3.5 `TestSubtaskIssue_InvalidPriority`: bad priority string → `INVALID_PRIORITY` GravaError
  - [x] 3.6 Use `testutil.MockStore` with `GetNextChildSequenceFn`, `BeginTxFn`, `LogEventTxFn` — same pattern as `TestCreateIssue_WithParent`

- [x] Task 4: Update `TestSubtaskCmd` integration smoke test (AC: #1)
  - [x] 4.1 Rewrote `TestSubtaskCmd` to use `testutil.MockStore` (fixes nested-tx conflict with sqlmock)
  - [x] 4.2 Added `TestSubtaskCmd_ParentNotFound`: count=0 → error contains "grava-missing not found"
  - [x] 4.3 Added `TestShowCmd_WithSubtasks`: dependencies rows returned → output contains subtask ID

- [x] Task 5: Final verification
  - [x] 5.1 `go test ./...` — all 17 packages pass
  - [x] 5.2 `go vet ./...` — zero warnings
  - [x] 5.3 `go build -ldflags="-s -w" ./...` — compiles clean

### Review Follow-ups (AI)
- [ ] [AI-Review][MEDIUM] `mockStoreForSubtask` QueryRowFn creates a new `*sql.DB` per call via `sqlmock.New()` that is never closed — leaks goroutines/FDs at scale. Fix: create mockDB once outside the closure, close in `t.Cleanup`. [subtask_test.go:37-39, commands_test.go:371-373, commands_test.go:410-412]
- [ ] [AI-Review][LOW] `SubtaskAffectedFiles` is a package-level var bound via `StringSliceVar` — if two tests instantiate `newSubtaskCmd` in the same binary, second run inherits state from the first. [subtask.go:207]
- [ ] [AI-Review][LOW] Human output `Subtasks: %v` prints Go slice syntax `[id1 id2]` — consider comma-separated or one-per-line format for readability. [issues.go:237]

## Dev Notes

### Critical: This story modifies EXISTING code — not greenfield

`newSubtaskCmd` **already exists** in `pkg/cmd/issues/issues.go` at line 788. Read it completely before touching anything. The refactor must:
1. Extract logic to a named `subtaskIssue` function (new file `subtask.go`) — following the exact same pattern as `createIssue` in `create.go`
2. Update `newSubtaskCmd` in `issues.go` to delegate to `subtaskIssue`
3. NOT break `TestSubtaskCmd` in `commands_test.go` — the test exercises the full cobra path

The `newShowCmd` in `issues.go` also exists and **must be enhanced** to return a `subtasks` array. Read `newShowCmd` (line 128) and `IssueDetail` struct (line 52) before modifying.

### Critical: ID Generation Ordering

The existing `newSubtaskCmd` calls `generator.GenerateChildID(parentID)` **after** opening the transaction at line 827. This is wrong — `GenerateChildID` opens its own internal transaction on the same `db` connection pool, causing nested transaction conflicts in tests.

**Fix:** Generate the child ID **before** calling `dolt.WithAuditedTx`. See how `createIssue` handles it: ID generation happens at lines 73-82, then `WithAuditedTx` is called at line 124. Follow this same ordering.

### Critical: Parent Existence Check Pattern

AC#4 requires `ISSUE_NOT_FOUND` (not `DB_UNREACHABLE` or a raw `fmt.Errorf`). The current `newSubtaskCmd` at line 822 returns a raw `fmt.Errorf("parent issue %s not found: %w", ...)` — this must become a `GravaError`.

Pattern to use (inside the `fn(tx *sql.Tx)` closure):
```go
var count int
if err := tx.QueryRowContext(ctx,
    "SELECT COUNT(*) FROM issues WHERE id = ?", params.ParentID).Scan(&count); err != nil {
    return gravaerrors.New("DB_UNREACHABLE", "failed to check parent existence", err)
}
if count == 0 {
    return gravaerrors.New("ISSUE_NOT_FOUND",
        fmt.Sprintf("Issue %s not found", params.ParentID), nil)
}
```

### Architecture: Named Function Pattern (ADR-003)

Extract `subtaskIssue` to `pkg/cmd/issues/subtask.go` with full signature:
```go
func subtaskIssue(ctx context.Context, store dolt.Store, params SubtaskParams) (SubtaskResult, error)
```

Do NOT extract to `pkg/ops` yet — ADR-003 says extraction to a separate package is deferred until daemon mode is built. Keep it in `pkg/cmd/issues/`.

### Architecture: WithAuditedTx Audit Events

Two audit events are needed (same as what the current code logs via `LogEventTx`):
```go
auditEvents := []dolt.AuditEvent{
    {
        IssueID:   id,             // the subtask ID
        EventType: dolt.EventSubtask,
        Actor:     params.Actor,
        Model:     params.Model,
        OldValue:  nil,
        NewValue:  map[string]any{
            "title":     params.Title,
            "type":      params.IssueType,
            "priority":  pInt,
            "parent_id": params.ParentID,
        },
    },
    {
        IssueID:   id,
        EventType: dolt.EventDependencyAdd,
        Actor:     params.Actor,
        Model:     params.Model,
        OldValue:  nil,
        NewValue:  map[string]any{"to_id": params.ParentID, "type": "subtask-of"},
    },
}
```
`dolt.EventSubtask = "subtask"` and `dolt.EventDependencyAdd = "dependency_add"` are defined in `pkg/dolt/events.go`.

### Architecture: `grava show` Subtasks Query

Two options for querying subtasks — prefer the `dependencies` table approach for correctness:
```sql
-- Option A: dependencies table (canonical — preferred)
SELECT d.from_id FROM dependencies d
WHERE d.to_id = ? AND d.type = 'subtask-of'
ORDER BY d.from_id

-- Option B: ID prefix (simpler but fragile if IDs ever change format)
SELECT id FROM issues WHERE id LIKE ? ORDER BY id
-- with arg: parentID + ".%"
```

**Use Option A** — the dependencies table is the authoritative source for parent-child relationships. Querying it is consistent with how the graph engine works.

### JSON Output Contract (NFR5)

**`grava subtask --json` success:**
```json
{
  "id": "abc123def456.1",
  "title": "Write unit tests",
  "status": "open",
  "priority": "medium"
}
```

**`grava subtask --json` error (parent not found):**
```json
{"error": {"code": "ISSUE_NOT_FOUND", "message": "Issue abc123def456 not found"}}
```

**`grava show <parent> --json` with subtasks:**
```json
{
  "id": "abc123def456",
  "title": "...",
  "subtasks": ["abc123def456.1", "abc123def456.2"]
}
```
`subtasks` is `omitempty` — omitted when empty.

### GravaError Codes for this Story

| Scenario | Code | Message |
|---|---|---|
| Missing title | `MISSING_REQUIRED_FIELD` | `title is required` |
| Parent not found | `ISSUE_NOT_FOUND` | `Issue {id} not found` |
| Invalid issue type | `INVALID_ISSUE_TYPE` | `invalid issue type: '{type}'` |
| Invalid priority | `INVALID_PRIORITY` | `invalid priority: '{priority}'` |
| DB connection failure | `DB_UNREACHABLE` | `failed to start transaction` |
| Child ID generation failure | `DB_UNREACHABLE` | `failed to generate child ID` |

### Test Patterns — MockStore for Parent-Child Tests

The `TestCreateIssue_WithParent` test in `create_test.go` (lines 132–173) established the correct pattern for testing parent-child ID generation. Use EXACTLY this pattern for `subtaskIssue` tests:

```go
store := testutil.NewMockStore()
store.GetNextChildSequenceFn = func(parentID string) (int, error) { return 1, nil }
store.BeginTxFn = func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
    return dolt.NewClientFromDB(db).BeginTx(ctx, nil)
}
store.LogEventTxFn = func(ctx context.Context, tx *sql.Tx, issueID, eventType, actor, model string, old, new interface{}) error {
    _, err := tx.ExecContext(ctx, "INSERT INTO events VALUES ()")
    return err
}
```

**Why:** `GetNextChildSequence` opens its own internal transaction on the `db` connection. If you use `dolt.NewClientFromDB(db)` directly, the nested transaction calls conflict with sqlmock's single-connection expectation. The MockStore isolates this: `GetNextChildSequenceFn` returns `1` directly (no DB call), while actual SQL operations route through the sqlmock-backed `dolt.Client`.

### Project Structure Notes

- **New file:** `pkg/cmd/issues/subtask.go` — `subtaskIssue` named function, `SubtaskParams`, `SubtaskResult`
- **New file:** `pkg/cmd/issues/subtask_test.go` — unit tests for `subtaskIssue`
- **Modified:** `pkg/cmd/issues/issues.go` — `newSubtaskCmd` delegates to `subtaskIssue`; `IssueDetail` struct gets `Subtasks []string`; `newShowCmd` queries subtasks
- **Modified:** `pkg/cmd/commands_test.go` — update `TestSubtaskCmd`; add `TestSubtaskCmd_ParentNotFound`, `TestShowCmd_WithSubtasks`
- **Do NOT modify:** `pkg/cmd/issues/create.go` or `pkg/cmd/issues/claim.go` — these are complete

### Key Files to Read Before Starting

1. `pkg/cmd/issues/issues.go` lines 788-909 — existing `newSubtaskCmd` (full implementation to replace)
2. `pkg/cmd/issues/issues.go` lines 128-223 — existing `newShowCmd` (needs subtasks augmentation)
3. `pkg/cmd/issues/issues.go` lines 52-66 — `IssueDetail` struct (needs `Subtasks` field)
4. `pkg/cmd/issues/create.go` — full file, use as template for `subtask.go`
5. `pkg/cmd/issues/create_test.go` lines 132-173 — `TestCreateIssue_WithParent` (MockStore pattern)
6. `pkg/cmd/commands_test.go` lines 345-417 — existing `TestSubtaskCmd` (to preserve and augment)
7. `pkg/dolt/client.go` lines 109-164 — `GetNextChildSequence` (understand why MockStore is needed)

### Previous Story Intelligence (Story 2.1)

**Key learnings that apply directly:**

1. **`GetNextChildSequence` + sqlmock conflict**: The root cause was that `GetNextChildSequence` opens an internal `db.BeginTx()`. When using `dolt.NewClientFromDB(db)` in tests, this internal tx conflicts with the outer sqlmock expectations. **Solution confirmed**: use `testutil.MockStore` with `GetNextChildSequenceFn` returning a hardcoded sequence value, while routing actual tx operations through `dolt.NewClientFromDB(db).BeginTx`.

2. **`writeJSONError` is already available**: Defined in `pkg/cmd/issues/create.go` (same package). The `subtaskIssue` command can call `writeJSONError(cmd, err)` directly — no need to duplicate or import.

3. **Audit events pre-built before TX**: Pre-building `auditEvents` before `WithAuditedTx` is fine — events only log after `fn(tx)` succeeds (see `tx.go:37-45`). But parent-not-found is returned from inside `fn(tx)`, which causes rollback before events fire, so no phantom audit entries.

4. **`json.Marshal` errors should not be silently dropped**: Use `//nolint:errcheck` + comment, or handle explicitly. This was a M2 finding in the 2.1 review — apply the fix proactively in `subtask.go`.

5. **Mock `ExpectClose()`**: `PersistentPostRunE` in `root.go` calls `Store.Close()` after every cobra command. Add `mock.ExpectClose()` to integration tests in `commands_test.go`.

### Schema Reference

```sql
-- issues table (relevant columns for subtask creation)
CREATE TABLE issues (
    id VARCHAR(32) NOT NULL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description LONGTEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'open',
    priority INT NOT NULL DEFAULT 4,  -- 0=Critical, 4=Backlog
    issue_type VARCHAR(32) NOT NULL DEFAULT 'task',
    ephemeral BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    created_by VARCHAR(128) DEFAULT 'unknown',
    updated_by VARCHAR(128) DEFAULT 'unknown',
    agent_model VARCHAR(128),
    affected_files TEXT  -- JSON array stored as string
);

-- child_counters table (atomic subtask ID generation)
CREATE TABLE child_counters (
    parent_id VARCHAR(32) NOT NULL PRIMARY KEY,
    next_child INT NOT NULL DEFAULT 1,
    updated_by VARCHAR(128)
);

-- dependencies table (parent-child relationships)
CREATE TABLE dependencies (
    from_id VARCHAR(32) NOT NULL,   -- the subtask
    to_id   VARCHAR(32) NOT NULL,   -- the parent
    type    VARCHAR(32) NOT NULL,   -- "subtask-of"
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    agent_model VARCHAR(128),
    PRIMARY KEY (from_id, to_id, type)
);
```

### References

- [Source: _bmad-output/planning-artifacts/epics/epic-02-issue-lifecycle.md#Story 2.2]
- [Source: _bmad-output/planning-artifacts/architecture.md#ADR-003 — named function pattern]
- [Source: _bmad-output/planning-artifacts/architecture.md#ADR-H3 — dep deadlock prevention]
- [Source: _bmad-output/planning-artifacts/architecture.md#Technical Constraints — SELECT FOR UPDATE subtask ID atomicity]
- [Source: pkg/cmd/issues/issues.go#newSubtaskCmd — existing implementation to refactor]
- [Source: pkg/cmd/issues/issues.go#newShowCmd — show command to enhance with subtasks]
- [Source: pkg/cmd/issues/create.go — template for subtask.go named function pattern]
- [Source: pkg/cmd/issues/create_test.go#TestCreateIssue_WithParent — MockStore test pattern]
- [Source: pkg/dolt/client.go#GetNextChildSequence — atomic ID generation implementation]
- [Source: pkg/dolt/tx.go#WithAuditedTx — transaction wrapper]
- [Source: pkg/dolt/events.go — EventSubtask, EventDependencyAdd constants]
- [Source: pkg/errors/errors.go — GravaError, ISSUE_NOT_FOUND code]
- [Source: internal/testutil/testutil.go — MockStore with GetNextChildSequenceFn]

## Dev Agent Record

### Agent Model Used

claude-sonnet-4-6

### Debug Log References

### Completion Notes List

Grava Tracking: epic=grava-8f07, story=grava-baa2, tasks=grava-baa2.1–grava-baa2.5

Implementation summary:
- Extracted `subtaskIssue(ctx, store, SubtaskParams) (SubtaskResult, error)` named function to new `subtask.go` (ADR-003)
- Replaced manual BeginTx/Commit in old `newSubtaskCmd` with `dolt.WithAuditedTx`; all errors now GravaError
- ID generation moved BEFORE WithAuditedTx to avoid nested-tx conflict with sqlmock in tests
- Parent existence check inside tx returns `ISSUE_NOT_FOUND` GravaError (not raw fmt.Errorf)
- `IssueDetail` struct extended with `Subtasks []string` (omitempty); `newShowCmd` queries `dependencies` table
- 6 unit tests in `subtask_test.go`; 3 integration tests updated/added in `commands_test.go`
- `TestSubtaskCmd` rewritten to use `testutil.MockStore` (same pattern as `TestCreateIssue_WithParent`)
- Removed old `newSubtaskCmd` from `issues.go`; removed unused `idgen` import

### File List

- pkg/cmd/issues/subtask.go (new)
- pkg/cmd/issues/subtask_test.go (new)
- pkg/cmd/issues/issues.go (modified — removed old newSubtaskCmd, added Subtasks to IssueDetail, enhanced newShowCmd)
- pkg/cmd/commands_test.go (modified — rewrote TestSubtaskCmd, added TestSubtaskCmd_ParentNotFound, TestShowCmd_WithSubtasks)
