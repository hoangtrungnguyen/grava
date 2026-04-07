# Story 2.6: Archive and Purge Issues

Status: ready-for-dev

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer or agent,
I want to safely remove or archive issues from the active tracking space,
so that completed or cancelled work does not pollute the active queue.

## Acceptance Criteria

1. **AC#1 — Soft-Delete (Archive)**
   Given issue `abc123def456` exists with `status=done`,
   When I run `grava drop abc123def456`,
   Then `status` transitions to `archived`; the issue is excluded from `grava list` and `grava ready` by default but retrievable via `grava list --include-archived`.

2. **AC#2 — Hard-Delete (Purge)**
   Given one or more issues exist with `status=archived`,
   When I run `grava clear`,
   Then all `archived` issues are permanently deleted from the database (hard delete) and the command returns a count: `{"purged": 3}`.

3. **AC#3 — Force Flag for Active Issues**
   Given issue `abc123def456` exists with `status=in_progress`,
   When I run `grava drop abc123def456` (without `--force`),
   Then the command returns: `{"error": {"code": "ISSUE_IN_PROGRESS", "message": "Cannot drop an active issue. Use --force to override."}}`.
   When I run `grava drop abc123def456 --force`,
   Then `status` transitions to `archived`.

4. **AC#4 — Atomicity**
   All drop/clear operations are wrapped in `WithAuditedTx` for atomicity and audit trail.

## Tasks / Subtasks

- [ ] Task 1: Add `archived` status to validation and schema (AC: #1)
  - [ ] 1.1 Add `"archived": true` to `AllowedStatuses` map in `pkg/validation/validation.go`
  - [ ] 1.2 Add `StatusArchived` constant to `pkg/graph/types.go` (or equivalent graph status file)
  - [ ] 1.3 Create migration `007_add_archived_status.sql` to update the CHECK constraint on `issues.status` to include `'archived'`
  - [ ] 1.4 Update `SchemaVersion` in `pkg/utils/schema.go` from 6 to 7
  - [ ] 1.5 Unit test: verify `ValidateStatus("archived")` returns nil

- [ ] Task 2: Implement `dropIssue` named function and `grava drop` command (AC: #1, #3, #4)
  - [ ] 2.1 Define `DropParams` struct: `ID`, `Force` (bool), `Actor`, `Model`
  - [ ] 2.2 Define `DropResult` struct: `ID` (json:"id"), `Status` (json:"status")
  - [ ] 2.3 Implement `dropIssue(ctx, store, params)` named function following established pattern:
    - Pre-read: SELECT issue to validate existence and current status
    - If `status == "in_progress"` and `!params.Force` → return `ISSUE_IN_PROGRESS` error
    - Use graph layer `dag.SetNodeStatus(id, StatusArchived)` for status transition (graph manages its own tx + audit)
    - Return `DropResult{ID: id, Status: "archived"}`
  - [ ] 2.4 Build `newDropCmd(d)` Cobra command — NOTE: existing `newDropCmd` in `issues.go` is a nuclear-reset command (delete ALL data). Must reconcile: rename existing to `newDropAllCmd` / `grava drop --all` and make `grava drop <id>` the per-issue archive. See Dev Notes for resolution strategy.
  - [ ] 2.5 Register updated command in `AddCommands()` in `issues.go`

- [ ] Task 3: Implement `clearArchived` named function and `grava clear` command (AC: #2, #4)
  - [ ] 3.1 Define `ClearParams` struct: `Actor`, `Model`
  - [ ] 3.2 Define `ClearResult` struct: `Purged` (json:"purged")
  - [ ] 3.3 Implement `clearArchived(ctx, store, params)` named function:
    - Count archived issues: `SELECT COUNT(*) FROM issues WHERE status = 'archived'`
    - If count == 0, return `ClearResult{Purged: 0}` (not an error — idempotent)
    - Wrap in `WithAuditedTx`: `DELETE FROM issues WHERE status = 'archived'` (FK CASCADE handles dependencies, events, labels, comments, work_sessions)
    - Record tombstones in `deletions` table for each purged issue ID
    - Audit event per purged issue with `EventType: dolt.EventClear`
    - Return `ClearResult{Purged: count}`
  - [ ] 3.4 Build `newClearCmd(d)` Cobra command — NOTE: existing `newClearCmd` in `issues.go` is a date-range purge command. Must reconcile: `grava clear` (no flags) = purge archived (Story 2.6), `grava clear --from DATE --to DATE` = date-range purge (Epic 1.1, deferred). See Dev Notes.
  - [ ] 3.5 Register in `AddCommands()`

- [ ] Task 4: Update `grava list` to exclude archived by default (AC: #1)
  - [ ] 4.1 Modify list query in `issues.go` to add `WHERE status != 'archived'` as default filter
  - [ ] 4.2 Add `--include-archived` flag to list command
  - [ ] 4.3 When `--include-archived` is set, remove the archived exclusion filter
  - [ ] 4.4 Unit test: verify `grava list` excludes archived; `grava list --include-archived` includes them

- [ ] Task 5: Unit tests for `dropIssue` (AC: #1, #3)
  - [ ] 5.1 Test happy path: drop a `done` issue → status becomes `archived`
  - [ ] 5.2 Test happy path: drop an `open` issue → status becomes `archived`
  - [ ] 5.3 Test error: drop `in_progress` issue without `--force` → `ISSUE_IN_PROGRESS`
  - [ ] 5.4 Test happy path: drop `in_progress` issue with `--force` → `archived`
  - [ ] 5.5 Test error: drop non-existent issue → `ISSUE_NOT_FOUND`
  - [ ] 5.6 Test JSON output structure matches `DropResult` schema

- [ ] Task 6: Unit tests for `clearArchived` (AC: #2)
  - [ ] 6.1 Test happy path: 3 archived issues → all deleted, `{"purged": 3}`
  - [ ] 6.2 Test idempotent: 0 archived issues → `{"purged": 0}`
  - [ ] 6.3 Test: non-archived issues are NOT deleted
  - [ ] 6.4 Test: tombstone records created in `deletions` table
  - [ ] 6.5 Test JSON output structure matches `ClearResult` schema

- [ ] Task 7: Integration tests (AC: #1, #2, #3)
  - [ ] 7.1 End-to-end: create issue → drop → verify excluded from list → clear → verify gone
  - [ ] 7.2 End-to-end: create issue → start (in_progress) → drop (fails) → drop --force (succeeds)
  - [ ] 7.3 Verify `--include-archived` flag works in list
  - [ ] 7.4 Verify `--json` output for both drop and clear commands

- [ ] Task 8: Update CLI reference docs and final verification
  - [ ] 8.1 Update `docs/guides/CLI_REFERENCE.md` with `grava drop` and `grava clear` documentation
  - [ ] 8.2 Run `go test ./...` — all tests pass
  - [ ] 8.3 Run `go vet ./...` — no issues
  - [ ] 8.4 Run `go build ./...` — clean build

## Dev Notes

### Command Conflict Resolution (CRITICAL)

**Existing commands that conflict with Story 2.6 naming:**

1. **`grava drop` (existing in `issues.go`)** — Currently a nuclear-reset command that deletes ALL data from ALL tables. This is from Epic 1.1 Section 1.1.
   - **Resolution:** Refactor existing nuclear-reset `drop` into `grava drop --all --force` (requires both flags for safety). Make `grava drop <id>` the per-issue archive command (Story 2.6). When `grava drop` is called with no arguments and no `--all` flag, print usage help.
   - **Location:** Check `issues.go` for `newDropCmd` — refactor in-place.

2. **`grava clear` (existing in `issues.go`)** — Currently a date-range purge command from Epic 1.1 Section 1.2.
   - **Resolution:** `grava clear` with no date flags = purge all archived (Story 2.6). `grava clear --from DATE --to DATE` = date-range purge (Epic 1.1, can be deferred or kept if already implemented). Check if the date-range variant is actually implemented or just stubbed.

**Action:** Read the existing `newDropCmd` and `newClearCmd` implementations BEFORE writing new code. Understand current behavior, then refactor to accommodate both use cases.

### Architecture Patterns (MUST FOLLOW)

**Named Function Pattern** (established Stories 2.1-2.5):
```
func dropIssue(ctx context.Context, store dolt.Store, params DropParams) (DropResult, error)
func clearArchived(ctx context.Context, store dolt.Store, params ClearParams) (ClearResult, error)
```
- All validation upfront before DB access
- Pre-read current state for audit event old values
- All mutations inside `WithAuditedTx`
- Return `*gravaerrors.GravaError` on all user-facing errors

**Status Transition via Graph Layer** (from `update.go` lines 104-114):
- Status changes MUST go through `graph.LoadGraphFromDB()` + `dag.SetNodeStatus()` — NOT direct SQL UPDATE
- The graph layer manages its own `WithAuditedTx` for status transitions
- Do NOT nest status updates inside another `WithAuditedTx`

**JSON Output Contract (NFR5):**
- Success: `json.MarshalIndent(result, "", "  ")` → stdout
- Error: `writeJSONError(cmd, err)` → stderr
- All result structs must have explicit `json:"field"` tags

**Event Types** (already defined in `pkg/dolt/events.go`):
- `dolt.EventDrop` — for archive (soft-delete)
- `dolt.EventClear` — for purge (hard-delete)

**Error Codes:**
| Scenario | Code | Message |
|---|---|---|
| Issue not found | `ISSUE_NOT_FOUND` | `Issue {id} not found` |
| In-progress without --force | `ISSUE_IN_PROGRESS` | `Cannot drop an active issue. Use --force to override.` |
| DB error | `DB_UNREACHABLE` | `failed to read/write issue` |

### Database Schema Notes

**Tables affected by `grava drop` (status update):**
- `issues` — status column updated to `archived`

**Tables affected by `grava clear` (hard delete via FK CASCADE):**
- `issues` — rows deleted WHERE status = 'archived'
- `dependencies` — CASCADE delete (from_id and to_id FKs)
- `events` — CASCADE delete (issue_id FK)
- `work_sessions` — CASCADE delete (issue_id FK)
- `issue_labels` — CASCADE delete (issue_id FK)
- `issue_comments` — CASCADE delete (issue_id FK)
- `deletions` — INSERT tombstone records before delete

**Migration 007** must add `'archived'` to the status CHECK constraint:
```sql
ALTER TABLE issues DROP CONSTRAINT check_status;
ALTER TABLE issues ADD CONSTRAINT check_status
  CHECK (status IN ('open', 'in_progress', 'closed', 'blocked', 'tombstone', 'deferred', 'pinned', 'archived'));
```
Note: Dolt uses MySQL syntax. Verify ALTER TABLE constraint syntax works with Dolt.

### Testing Patterns (from Story 2.5)

- Use `sqlmock.New()` to mock SQL driver
- Use `testutil.MockStore` with `BeginTxFn` and `LogEventTxFn`
- Mock expectations: `ExpectBegin()`, `ExpectQuery()`, `ExpectExec()`, `ExpectCommit()`
- Use `errors.As(err, &gravaErr)` for error code assertions
- Use `testify/assert` and `testify/require`
- Test file location: `pkg/cmd/issues/drop_test.go`, `pkg/cmd/issues/clear_test.go`

### Previous Story Learnings (Story 2.5)

- Story 2.5 review found exported mutable state (`LabelAddFlags`) — avoid package-level mutable state; read flags via `cmd.Flags().GetXxx()` inside RunE
- Story 2.5 review found `drop` command table list missing `issue_labels` and `issue_comments` — the nuclear-reset drop MUST include these tables. When refactoring the existing drop command, ensure ALL tables are covered in FK-safe order
- Story 2.5 review found audit events logging intended changes rather than actual — ensure audit events reflect what actually happened (check affected rows)
- Story 2.5 review found `LastInsertId()` error silently discarded — always check errors from DB operations

### Project Structure Notes

- All issue commands: `pkg/cmd/issues/*.go`
- Database layer: `pkg/dolt/` (Store interface, tx.go for WithAuditedTx, events.go for event constants)
- Error types: `pkg/errors/errors.go` (GravaError)
- Validation: `pkg/validation/validation.go` (AllowedStatuses, ValidateStatus)
- Graph layer: `pkg/graph/` (status transitions, DAG operations)
- Migrations: `pkg/migrate/migrations/NNN_description.sql`
- Schema version: `pkg/utils/schema.go`
- Test helpers: `internal/testutil/`
- CLI reference: `docs/guides/CLI_REFERENCE.md`

### References

- [Source: _bmad-output/planning-artifacts/epics/epic-02-issue-lifecycle.md#Story 2.6]
- [Source: docs/epics/Epic_1.1_additional_commands.md#grava drop, #grava clear]
- [Source: pkg/cmd/issues/update.go#lines 104-114 — graph layer status transition pattern]
- [Source: pkg/cmd/issues/create.go — named function + Cobra command pattern]
- [Source: pkg/cmd/issues/start.go — status pre-check pattern (in_progress guard)]
- [Source: pkg/dolt/tx.go#WithAuditedTx — transaction pattern]
- [Source: pkg/dolt/events.go — EventDrop, EventClear constants]
- [Source: pkg/validation/validation.go#AllowedStatuses — status validation]
- [Source: pkg/migrate/migrations/001_initial_schema.sql — table schemas, FK CASCADE]
- [Source: _bmad-output/implementation-artifacts/2-5-label-and-comment-on-issues.md — previous story patterns and review findings]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- Grava Tracking: epic=grava-8f07, story=grava-8f07.3
- Ultimate context engine analysis completed — comprehensive developer guide created (2026-04-05)

### File List
