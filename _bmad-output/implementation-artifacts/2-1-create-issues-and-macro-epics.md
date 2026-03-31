# Story 2.1: Create Issues and Macro-Epics

Status: review

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer or agent,
I want to create a new issue or macro-epic with a title, description, and priority,
So that work items are tracked in the Grava database from the moment they are identified.

## Acceptance Criteria

1. **Given** `grava init` has been run and `.grava/` exists
   **When** I run `grava create --title "Fix login bug" --priority high`
   **Then** a new issue record is inserted in the `issues` table with a unique 12-char hex ID, `status=open`, `created_at=NOW()`, and the provided fields
   **And** `grava create --json` returns `{"id": "abc123def456", "title": "Fix login bug", "status": "open", "priority": "high"}` conforming to NFR5 schema

2. **Given** `grava init` has been run
   **When** I run `grava quick "Fix login bug"`
   **Then** a new issue is created with defaults (`priority=medium`, no description) in one command

3. **Given** `grava init` has been run
   **When** I run `grava create` without `--title`
   **Then** it returns `{"error": {"code": "MISSING_REQUIRED_FIELD", "message": "title is required"}}`

4. **Given** any create operation
   **Then** the operation completes in <15ms (NFR2 baseline)

## Tasks / Subtasks

- [x] Task 1: Refactor `grava create` to use `WithAuditedTx`, `GravaError`, and enhanced `--json` output (AC: #1, #3)
  - [x] 1.1 Extract `createIssue` named function with signature `createIssue(ctx context.Context, store dolt.Store, params CreateParams) (CreateResult, error)`
  - [x] 1.2 Define `CreateParams` struct: `Title, Description, IssueType, Priority string; ParentID string; Ephemeral bool; AffectedFiles []string; Actor, Model string`
  - [x] 1.3 Define `CreateResult` struct with JSON tags: `ID string "json:\"id\""; Title string "json:\"title\""; Status string "json:\"status\""; Priority string "json:\"priority\""; Ephemeral bool "json:\"ephemeral,omitempty\""`
  - [x] 1.4 Wrap all DB mutations in `dolt.WithAuditedTx` (replace manual tx.BeginTx / tx.Commit pattern)
  - [x] 1.5 Use `GravaError` for all user-facing errors: `MISSING_REQUIRED_FIELD` for missing title, `PARENT_NOT_FOUND` for invalid parent, `DB_UNREACHABLE` for connection errors
  - [x] 1.6 Enhanced `--json` output: return full `CreateResult` (not just `{"id", "status"}`) including title, priority label, and ephemeral flag
  - [x] 1.7 Return priority as string label (e.g. `"high"`) in JSON output, not integer
  - [x] 1.8 Update `newCreateCmd` to call `createIssue` named function via `d` deps

- [x] Task 2: Refactor `grava quick` from list-command to quick-create command (AC: #2)
  - [x] 2.1 Change `newQuickCmd` to accept a positional arg: `Use: "quick <title>"`, `Args: cobra.ExactArgs(1)`
  - [x] 2.2 Implementation: call `createIssue(ctx, store, CreateParams{Title: args[0], IssueType: "task", Priority: "medium", Actor: actor, Model: model})`
  - [x] 2.3 Same JSON/human output format as `grava create`
  - [x] 2.4 Remove the old list-based quick command implementation entirely

- [x] Task 3: Write unit tests for `createIssue` named function (AC: #1, #2, #3)
  - [x] 3.1 Test happy path: create issue → returns CreateResult with 12-char hex ID, status=open
  - [x] 3.2 Test missing title: returns `MISSING_REQUIRED_FIELD` GravaError
  - [x] 3.3 Test invalid parent: parent ID not found → returns `PARENT_NOT_FOUND` GravaError
  - [x] 3.4 Test quick create: title only, defaults applied (priority=medium, type=task)
  - [x] 3.5 Test JSON output structure conforms to NFR5 (flat object, snake_case fields)
  - [x] 3.6 Use sqlmock pattern established in `claim_test.go`: `sqlmock.New()` → `dolt.NewClientFromDB(db)` → set expectations → call function → assert

- [x] Task 4: Update existing tests to match refactored create/quick commands
  - [x] 4.1 Update or remove old `pkg/cmd/create.go` tests if any reference the legacy flat-`pkg/cmd` create command
  - [x] 4.2 Update old `pkg/cmd/quick.go` tests — the quick command is no longer a list command
  - [x] 4.3 Ensure `go test ./...` passes (non-integration tests)

- [x] Task 5: Performance baseline (AC: #4)
  - [x] 5.1 Add benchmark test `BenchmarkCreateIssue` using sqlmock (measures Go-side overhead, not DB)
  - [x] 5.2 Document expected <15ms in code comment referencing NFR2

- [x] Task 6: Final verification
  - [x] 6.1 `go test ./...` — all non-integration tests pass
  - [x] 6.2 `go vet ./...` — zero warnings
  - [x] 6.3 `go build -ldflags="-s -w" ./...` — compiles, no new runtime deps

## Dev Notes

### Critical: This story modifies EXISTING code — not greenfield

The `grava create` and `grava quick` commands **already exist** in `pkg/cmd/issues/issues.go`. This story is a **refactor + enhancement**, not a fresh implementation. The dev agent MUST:

1. Read the existing `newCreateCmd` in `pkg/cmd/issues/issues.go` (line ~130) before making any changes
2. Read the existing `newQuickCmd` in `pkg/cmd/issues/issues.go` (line ~1053) before making any changes
3. Preserve all existing flags on `create`: `--title`, `--desc`, `--type`, `--priority`, `--parent`, `--ephemeral`, `--files`
4. NOT break any existing functionality that other stories or tests depend on

### Critical: `grava quick` semantic change

The current `grava quick` is a **list command** that shows high-priority issues. The Epic 2 spec and Story 2.1 AC require `grava quick "Fix login bug"` to be a **create command** that creates an issue with defaults. This is a breaking change to the `quick` command's semantics.

**Decision needed:** The old list behavior is duplicated by `grava list --sort priority:asc --limit 10`. The dev should:
- Replace the quick command body entirely with the create-with-defaults logic
- The old quick-list behavior is accessible via `grava list` with sort/filter flags

### Architecture Compliance: Named Function Pattern (ADR-003)

Extract `createIssue` as a named function per ADR-003:

```go
// pkg/cmd/issues/create.go (new file — extract from issues.go)
func createIssue(ctx context.Context, store dolt.Store, params CreateParams) (CreateResult, error) {
    // Validate required fields
    if params.Title == "" {
        return CreateResult{}, gravaerrors.New("MISSING_REQUIRED_FIELD", "title is required", nil)
    }
    // ... validation, ID generation, WithAuditedTx ...
}
```

### Architecture Compliance: WithAuditedTx Pattern

The existing `newCreateCmd` manually manages transactions (`BeginTx` → `defer Rollback` → `Commit`). Refactor to use `dolt.WithAuditedTx` as established in Story 1.3:

```go
auditEvents := []dolt.AuditEvent{{
    IssueID:   id,
    EventType: dolt.EventCreate,
    Actor:     params.Actor,
    Model:     params.Model,
    OldValue:  nil,
    NewValue:  map[string]any{"title": params.Title, "type": issueType, "priority": pInt, "status": "open"},
}}

err := dolt.WithAuditedTx(ctx, store, auditEvents, func(tx *sql.Tx) error {
    _, err := tx.ExecContext(ctx, insertQuery, ...)
    return err
})
```

### Architecture Compliance: GravaError for User-Facing Errors

All user-facing errors MUST use `gravaerrors.New()` with domain-prefixed codes:

| Scenario | Code | Message |
|---|---|---|
| Missing title | `MISSING_REQUIRED_FIELD` | `title is required` |
| Invalid parent ID | `PARENT_NOT_FOUND` | `parent issue {id} not found` |
| Invalid issue type | `INVALID_ISSUE_TYPE` | `invalid issue type: '{type}'` |
| Invalid priority | `INVALID_PRIORITY` | `invalid priority: '{priority}'` |
| DB connection failure | `DB_UNREACHABLE` | `failed to start transaction` |

### Architecture Compliance: JSON Output Contract (NFR5)

**Success output** (flat object, no `data:` wrapper):
```json
{
  "id": "abc123def456",
  "title": "Fix login bug",
  "status": "open",
  "priority": "high",
  "ephemeral": false
}
```

**Error output** (via `writeJSONError` in `pkg/cmd/util.go`):
```json
{"error": {"code": "MISSING_REQUIRED_FIELD", "message": "title is required"}}
```

The current create command returns `{"id": "...", "status": "created"}` — this must be updated to match the AC which specifies `"status": "open"` (the actual DB status, not a verb).

### Architecture Compliance: ID Generation

- Uses existing `idgen.NewStandardGenerator(store)` in `pkg/idgen/generator.go`
- Base IDs: currently 4-char hex (architecture says 12-char). **Note:** The architecture ADR says 12-char hex but the current `GenerateBaseID()` generates shorter IDs. Do NOT change the ID generator in this story — that's a separate concern. Use whatever the generator produces.
- Child IDs (subtasks): `parentID.N` via `SELECT FOR UPDATE` on `child_counters`

### Schema Awareness: Existing `issues` Table

```sql
CREATE TABLE issues (
    id VARCHAR(32) NOT NULL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description LONGTEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'open',
    priority INT NOT NULL DEFAULT 4,        -- 0=Critical, 4=Backlog
    issue_type VARCHAR(32) NOT NULL DEFAULT 'task',
    assignee VARCHAR(128),
    metadata JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    ephemeral BOOLEAN DEFAULT FALSE,
    await_type VARCHAR(32),
    await_id VARCHAR(128),
    created_by VARCHAR(128) DEFAULT 'unknown',
    updated_by VARCHAR(128) DEFAULT 'unknown',
    agent_model VARCHAR(128),
    affected_files TEXT,                     -- JSON array stored as string
    CONSTRAINT check_priority CHECK (priority BETWEEN 0 AND 4),
    CONSTRAINT check_status CHECK (status IN ('open','in_progress','blocked','closed','tombstone','deferred','pinned')),
    CONSTRAINT check_issue_type CHECK (issue_type IN ('bug','feature','task','epic','chore','message','story'))
);
```

**Important schema-vs-AC discrepancy:** The AC mentions statuses `paused`, `done`, `archived` but the DB constraint only allows `open`, `in_progress`, `blocked`, `closed`, `tombstone`, `deferred`, `pinned`. For this story, use the EXISTING schema statuses. Status enum changes are a separate migration concern for later stories (2.3, 2.4, 2.6).

### Validation Package

Use existing validators in `pkg/validation/validation.go`:
- `validation.ValidateIssueType(t string) error` — allowed: task, bug, epic, story, feature, chore
- `validation.ValidatePriority(p string) (int, error)` — maps: critical=0, high=1, medium=2, low=3, backlog=4
- Wrap validation errors in `GravaError` at the `createIssue` boundary (current validators return `fmt.Errorf`)

### Priority Encoding

Priority is stored as INT (0-4) in the DB but must be displayed as a string label in JSON output. Use the reverse mapping:

```go
var PriorityToString = map[int]string{0: "critical", 1: "high", 2: "medium", 3: "low", 4: "backlog"}
```

This mapping already exists informally in `newShowCmd`. Consider adding it to `pkg/validation/` if not already exported.

### Test Patterns (from Story 1.3)

Follow the established sqlmock pattern from `pkg/cmd/issues/claim_test.go`:

```go
func TestCreateIssue_HappyPath(t *testing.T) {
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    defer db.Close() //nolint:errcheck

    mock.ExpectBegin()
    mock.ExpectExec("INSERT INTO issues").WillReturnResult(sqlmock.NewResult(1, 1))
    mock.ExpectExec("INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
    mock.ExpectCommit()

    store := dolt.NewClientFromDB(db)
    result, err := createIssue(context.Background(), store, CreateParams{
        Title: "Test issue", IssueType: "task", Priority: "medium",
        Actor: "test-actor", Model: "test-model",
    })
    require.NoError(t, err)
    assert.Equal(t, "open", result.Status)
    assert.NotEmpty(t, result.ID)
    require.NoError(t, mock.ExpectationsWereMet())
}
```

### Project Structure Notes

- **New file:** `pkg/cmd/issues/create.go` — extract `createIssue` named function, `CreateParams`, `CreateResult` from the monolithic `issues.go`
- **Modified file:** `pkg/cmd/issues/issues.go` — update `newCreateCmd` and `newQuickCmd` to call extracted `createIssue`
- **New file:** `pkg/cmd/issues/create_test.go` — unit tests for `createIssue`
- **Potentially modified:** `pkg/validation/validation.go` — add `PriorityToString` map if not already exported
- **Legacy files to check:** `pkg/cmd/create.go` and `pkg/cmd/quick.go` still exist in the flat `pkg/cmd/` — these are pre-reorganization legacy files. Do NOT modify them. The active versions are in `pkg/cmd/issues/`.

### References

- [Source: _bmad-output/planning-artifacts/epics/epic-02-issue-lifecycle.md#Story 2.1]
- [Source: _bmad-output/planning-artifacts/architecture.md#ADR-003 — named function pattern]
- [Source: _bmad-output/planning-artifacts/architecture.md#Error Handling — GravaError]
- [Source: _bmad-output/planning-artifacts/architecture.md#Implementation Patterns — Transaction pattern]
- [Source: _bmad-output/planning-artifacts/architecture.md#Implementation Patterns — JSON Output Fields]
- [Source: _bmad-output/planning-artifacts/prd.md#FR1 — create/quick commands]
- [Source: _bmad-output/planning-artifacts/prd.md#NFR2 — <15ms write latency]
- [Source: _bmad-output/planning-artifacts/prd.md#NFR5 — JSON schema versioning contract]
- [Source: pkg/cmd/issues/issues.go — existing create and quick implementations]
- [Source: pkg/cmd/issues/claim_test.go — sqlmock test pattern]
- [Source: pkg/dolt/tx.go — WithAuditedTx]
- [Source: pkg/dolt/events.go — Event constants]
- [Source: pkg/errors/errors.go — GravaError]

### Previous Story Intelligence (Story 1.3)

**Key learnings from Story 1.3 that apply:**
- Named functions must accept `context.Context` as first param, `dolt.Store` as second — never use package-level `Store` global
- Use `dolt.NewClientFromDB(db)` for sqlmock-based tests (not MockStore)
- `WithAuditedTx` handles begin/commit/rollback — do NOT double-manage transactions
- sqlmock regex patterns: use `ExpectExec("INSERT INTO issues")` (substring match), not full SQL
- `//nolint:errcheck` on `defer db.Close()` and `defer rows.Close()` per project convention
- `filepath.EvalSymlinks` was needed for macOS `/var` → `/private/var` in resolver tests — not relevant here but illustrates the level of OS-awareness expected
- Review findings from 1.3: multiple rounds of code review caught missing error wrapping and inconsistent patterns — be thorough on first pass

**Files created/modified in Story 1.3 that this story builds on:**
- `pkg/grava/resolver.go` — resolver pattern (not directly used but establishes package structure)
- `pkg/cmd/issues/claim.go` + `claim_test.go` — the template for named function + test pattern
- `internal/testutil/testutil.go` — MockStore and AssertGravaError helpers (available but sqlmock preferred for DB tests)
- `pkg/dolt/tx.go` — `WithAuditedTx` (MUST use this)

### Git Intelligence Summary

Recent commit patterns:
- `fix(review-1.3): resolve 7 code review findings` — code review produces multiple rounds of fixes
- `feat(story-1.3): worktree resolver, named functions, and test harness` — feature commits use `feat(story-X.Y):` prefix
- Convention: `feat(story-2.1): ...` for the main implementation commit

## Dev Agent Record

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Grava Tracking: story=grava-2735, tasks=task1=grava-2735.1, task2=grava-2735.2, task3=grava-2735.3, task4=grava-2735.4, task5=grava-2735.5, task6=grava-2735.6
- Extracted `createIssue` named function into `pkg/cmd/issues/create.go` with `CreateParams` / `CreateResult` structs
- Replaced manual `BeginTx/Commit` with `dolt.WithAuditedTx` — all errors wrapped as `GravaError`
- JSON output now returns `{id, title, status, priority, ephemeral}` — status is `"open"` (not the old `"created"`)
- `grava quick` rewritten from list-command to quick-create: `grava quick "<title>"` creates with defaults (type=task, priority=medium)
- Old `newCreateCmd` in `issues.go` replaced by new `newCreateCmd` in `create.go`; `quickPriority`/`quickLimit` vars removed
- 9 unit tests + 1 benchmark added in `create_test.go`; 3 legacy quick tests in `commands_test.go` updated
- `go test ./...` ✅ `go vet ./...` ✅ `go build -ldflags="-s -w" ./...` ✅

### File List

- pkg/cmd/issues/create.go (new)
- pkg/cmd/issues/create_test.go (new)
- pkg/cmd/issues/issues.go (modified — removed newCreateCmd, refactored newQuickCmd, removed quickPriority/quickLimit vars)
- pkg/cmd/commands_test.go (modified — updated TestQuickCmd, TestQuickCmdAllCaughtUp, TestQuickCmdCustomPriority)
