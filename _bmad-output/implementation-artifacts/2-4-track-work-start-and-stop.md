# Story 2.4: Track Work Start and Stop

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer or agent,
I want to explicitly record when I start and stop working on an issue,
so that cycle time and work-in-progress can be measured accurately.

## Acceptance Criteria

1. **Given** issue `abc123def456` exists with `status=open`
   **When** I run `grava start abc123def456`
   **Then** `status` transitions to `in_progress`, `started_at=NOW()`, `actor=current_actor_id` recorded in tracking, and the operation audits the state change

2. **Given** issue `abc123def456` exists with `status=in_progress`
   **When** I run `grava stop abc123def456`
   **Then** `status` transitions back to `open`, `stopped_at=NOW()` recorded, and the operation audits the state change

3. **Given** issue `abc123def456` is already `in_progress` by `agent-01`
   **When** I (a different agent) run `grava start abc123def456`
   **Then** it returns `{"error": {"code": "ALREADY_IN_PROGRESS", "message": "Issue is already being worked on by agent-01"}}`

4. **Given** issue `abc123def456` has `status=open` (not in progress)
   **When** I run `grava stop abc123def456`
   **Then** it returns `{"error": {"code": "NOT_IN_PROGRESS", "message": "Cannot stop work on an issue not in progress"}}`

5. **Given** issue `abc123def456` does not exist
   **When** I run `grava start abc123def456` or `grava stop abc123def456`
   **Then** it returns `{"error": {"code": "ISSUE_NOT_FOUND", "message": "Issue abc123def456 not found"}}`

6. **Given** any `grava start` or `grava stop` operation
   **Then** both operations complete within <15ms (NFR2 requirement)

## Tasks / Subtasks

- [x] **Task 1: Design and Create Work Session Tracking** (AC: #1, #2, #3, #4, #5)
  - [x]1.1 Create `work_sessions` table migration (if not already present): `id (PK), issue_id (FK), actor, model, started_at, stopped_at, created_at`
  - [x]1.2 Verify current `issues` table schema supports `started_at`, `stopped_at` columns (or add them via migration)
  - [x]1.3 Design the logical flow: `grava start` updates issue status → `in_progress`, records `started_at`, logs audit event; `grava stop` updates status → `open`, records `stopped_at`

- [x] **Task 2: Implement `grava start` Command** (AC: #1, #3, #5, #6)
  - [x]2.1 Create `pkg/cmd/issues/start.go` — extract `startIssue` named function: `startIssue(ctx context.Context, store dolt.Store, params StartParams) (StartResult, error)`
  - [x]2.2 Define `StartParams` struct: `ID string; Actor string; Model string`
  - [x]2.3 Define `StartResult` struct with JSON tags: `ID string; Status string; StartedAt string`
  - [x]2.4 Pre-read current issue status (before mutation) to detect if already `in_progress` — raise `ALREADY_IN_PROGRESS` GravaError with message including current actor
  - [x]2.5 Validate issue existence: if not found, return `ISSUE_NOT_FOUND` GravaError
  - [x]2.6 Use `dag.SetNodeStatus` to transition status from `open` → `in_progress` (routes through graph layer)
  - [x]2.7 Record `started_at = time.Now()` and `actor` in the issue row (via same mutation or separate operation)
  - [x]2.8 Emit `EventStart` audit event via `WithAuditedTx`: `OldValue: {status: "open"}; NewValue: {status: "in_progress", started_at: ...}`
  - [x]2.9 Return `StartResult{ID, Status: "in_progress", StartedAt: timestamp}`
  - [x]2.10 Create `newStartCmd` in `issues.go` to delegate to `startIssue` with proper CLI flags: `grava start <id>`

- [x] **Task 3: Implement `grava stop` Command** (AC: #2, #4, #5, #6)
  - [x]3.1 Create `pkg/cmd/issues/stop.go` — extract `stopIssue` named function: `stopIssue(ctx context.Context, store dolt.Store, params StopParams) (StopResult, error)`
  - [x]3.2 Define `StopParams` struct: `ID string; Actor string; Model string`
  - [x]3.3 Define `StopResult` struct with JSON tags: `ID string; Status string; StoppedAt string`
  - [x]3.4 Pre-read current issue status: if not `in_progress`, return `NOT_IN_PROGRESS` GravaError
  - [x]3.5 Validate issue existence: if not found, return `ISSUE_NOT_FOUND` GravaError
  - [x]3.6 Use `dag.SetNodeStatus` to transition status from `in_progress` → `open` (back to ready queue)
  - [x]3.7 Record `stopped_at = time.Now()` in the issue row
  - [x]3.8 Emit `EventStop` audit event via `WithAuditedTx`: `OldValue: {status: "in_progress"}; NewValue: {status: "open", stopped_at: ...}`
  - [x]3.9 Return `StopResult{ID, Status: "open", StoppedAt: timestamp}`
  - [x]3.10 Create `newStopCmd` in `issues.go` to delegate to `stopIssue`: `grava stop <id>`

- [x] **Task 4: Unit Tests for `startIssue`** (AC: #1, #3, #5)
  - [x]4.1 `TestStartIssue_HappyPath`: valid open issue, transitions to `in_progress`, returns correct `StartResult`
  - [x]4.2 `TestStartIssue_AlreadyInProgress`: issue already `in_progress` by same or different actor → `ALREADY_IN_PROGRESS` error with actor name
  - [x]4.3 `TestStartIssue_IssueNotFound`: non-existent issue → `ISSUE_NOT_FOUND` error
  - [x]4.4 Use `mockStoreForStart` helper + sqlmock for pre-read and status transition mocking

- [x] **Task 5: Unit Tests for `stopIssue`** (AC: #2, #4, #5)
  - [x]5.1 `TestStopIssue_HappyPath`: valid `in_progress` issue, transitions to `open`, returns correct `StopResult`
  - [x]5.2 `TestStopIssue_NotInProgress`: issue has `status=open` (not in progress) → `NOT_IN_PROGRESS` error
  - [x]5.3 `TestStopIssue_IssueNotFound`: non-existent issue → `ISSUE_NOT_FOUND` error
  - [x]5.4 Use `mockStoreForStop` helper + sqlmock for pre-read and status transition mocking

- [x] **Task 6: Integration Tests** (AC: #1, #2, #3, #4, #5, #6)
  - [x]6.1 `TestStartCmd`: `grava start <id>` with valid open issue → verifies output includes "Started work on"
  - [x]6.2 `TestStartCmd_AlreadyInProgress`: `grava start <id>` on `in_progress` issue → error message includes actor
  - [x]6.3 `TestStopCmd`: `grava stop <id>` on valid `in_progress` issue → verifies output includes "Stopped work on"
  - [x]6.4 `TestStopCmd_NotInProgress`: `grava stop <id>` on `open` issue → error message
  - [x]6.5 `TestStart_Stop_Cycle`: Full cycle — create issue → start → verify status → stop → verify status

- [x] **Task 7: Schema Verification and Migration** (AC: #1, #2)
  - [x]7.1 Check if `started_at`, `stopped_at` columns exist in `issues` table; add migration if missing
  - [x]7.2 Check if `work_sessions` table is needed (per acceptance criteria); if AC requires it but not used, document decision

- [x] **Task 8: Final Verification**
  - [x]8.1 `go test ./...` — all packages pass, including new start/stop tests
  - [x]8.2 `golangci-lint run ./...` — no new linting issues
  - [x]8.3 `go build ./...` — clean compile
  - [x]8.4 Verify NFR2 <15ms performance target met (benchmark if needed)
  - [x]8.5 Update CLI docs in `docs/guides/CLI_REFERENCE.md` for new `grava start` and `grava stop` commands

### Review Follow-ups (AI)

- [ ] [AI-Review][MEDIUM] Add `--json` test coverage for `grava start` and `grava stop` — the JSON output contract (NFR5) is untested. Add `TestStartCmd_JSON` and `TestStopCmd_JSON` to `commands_test.go` verifying the `{"id","status","started_at"}` and `{"id","status","stopped_at"}` shapes. [`start.go:119-123`, `stop.go:116-120`] (grava-1073.9)
- [ ] [AI-Review][MEDIUM] Add unit test for `DB_UNREACHABLE` pre-read error path in `startIssue` and `stopIssue` — when `tx.QueryRowContext` returns a non-`ErrNoRows` error, the `DB_UNREACHABLE` branch is never exercised by current tests. Add one test each to `start_test.go` / `stop_test.go` with `mock.ExpectQuery(...).WillReturnError(errors.New("db down"))`. [`start.go:37-42`, `stop.go:37-41`] (grava-1073.10)
- [ ] [AI-Review][MEDIUM] Document `sprint-status.yaml` in the Dev Agent Record File List — the file was modified (status `in-progress` → `review`) but is absent from the File List section, breaking the change traceability contract. (grava-1073.11)
- [ ] [AI-Review][LOW] Guard `grava start` against invalid source statuses — currently any status (done, archived, closed) can be transitioned to `in_progress`. Add a `currentStatus != "open"` check after the `in_progress` check, returning `INVALID_STATUS_TRANSITION`. [`start.go:47-52`] (grava-1073.12)
- [ ] [AI-Review][LOW] Add index on `stopped_at` column in migration 005 — `started_at` has `idx_started_at` but `stopped_at` has no index. Cycle time range queries (`WHERE stopped_at BETWEEN ? AND ?`) will full-scan as the table grows. [`005_add_work_session_columns.sql`] (grava-1073.13)
- [ ] [AI-Review][LOW] Document Epic 2 vs Story 2.4 divergence: `grava stop` was specified as `→ paused` in the epic (and in grava-1073 description) but Story AC#2 and architecture both use `→ open`. Add a note in Dev Notes clarifying that `paused` status does not exist in the current schema and the architecture table is authoritative. (grava-1073.14)

## Dev Notes

### Critical: Understand Existing Graph/DAG Architecture

The `updateIssue` function (Story 2.3) uses `dag.SetNodeStatus()` to manage status transitions through the dependency graph. This story **must follow the same pattern**:
- `startIssue` calls `dag.SetNodeStatus(id, "in_progress")` (not direct DB UPDATE)
- `stopIssue` calls `dag.SetNodeStatus(id, "open")` (returns to ready queue)

The DAG layer handles audit logging internally, so coordinate with architecture to understand if we wrap DAG calls in `WithAuditedTx` or let DAG handle it exclusively.

**Key dependency**: Read how Story 2.3 (`updateIssue` lines 105-114) integrates status updates via the graph layer.

### Critical: Status Columns vs. `work_sessions` Table

**Current State (from schema check):**
- `issues` table exists with columns: `id, title, description, issue_type, priority, status, assignee, created_at, updated_at, updated_by, agent_model, created_by, affected_files, created_by_model`
- `work_sessions` table **does NOT exist** in current schema
- No `started_at`, `stopped_at` columns visible in `issues` table

**Design Decision Needed:**
1. **Option A (Recommended for Phase 1):** Add `started_at`, `stopped_at` columns to `issues` table via migration. Record timestamps on each `start`/`stop` operation. `work_sessions` table is deferred to Phase 2 (full session tracking with detailed timing).
2. **Option B:** Create `work_sessions` table immediately (per AC spec) and link `started_at`, `stopped_at` there. More robust for Phase 2 but adds DB design complexity now.

**Recommendation:** Go with **Option A** — minimal schema change, aligns with Phase 1 pragmatism. Document that `work_sessions` table is a Phase 2 enhancement.

### Architecture: Timing and Audit Events

**For `startIssue`:**
```go
startTime := time.Now()
// Call dag.SetNodeStatus(...) with audit event
// OldValue: {"status": "open"}
// NewValue: {"status": "in_progress", "started_at": startTime.String()}
```

**For `stopIssue`:**
```go
stopTime := time.Now()
// Call dag.SetNodeStatus(...) with audit event
// OldValue: {"status": "in_progress"}
// NewValue: {"status": "open", "stopped_at": stopTime.String()}
```

Ensure `started_at` and `stopped_at` are ISO8601 formatted strings (same as audit event timestamps).

### Error Codes for this Story

| Scenario | Code | Message | HTTP-like |
|---|---|---|---|
| Issue not found | `ISSUE_NOT_FOUND` | `Issue {id} not found` | 404 |
| Already in progress | `ALREADY_IN_PROGRESS` | `Issue is already being worked on by {actor}` | 409 |
| Not in progress (can't stop) | `NOT_IN_PROGRESS` | `Cannot stop work on an issue not in progress` | 409 |
| DB error | `DB_UNREACHABLE` | `failed to read/update issue` | 500 |

### JSON Output Contract (NFR5)

**`grava start --json` success:**
```json
{
  "id": "abc123def456",
  "status": "in_progress",
  "started_at": "2026-04-01T10:30:45Z"
}
```

**`grava stop --json` success:**
```json
{
  "id": "abc123def456",
  "status": "open",
  "stopped_at": "2026-04-01T10:45:30Z"
}
```

**Error response (same as Story 2.3):**
```json
{"error": {"code": "ALREADY_IN_PROGRESS", "message": "Issue is already being worked on by agent-01"}}
```

### CLI Interface

```bash
# Start work on an issue (transitions open → in_progress)
grava start <id>

# Stop work on an issue (transitions in_progress → open)
grava stop <id>
```

Both commands inherit `--actor` and `--agent-model` from global flags (set via `cmddeps.Deps`).

### Architecture: Pre-Read Pattern (from Story 2.3)

Before calling `dag.SetNodeStatus`, pre-read the issue to:
1. Verify existence (`ISSUE_NOT_FOUND` if no row)
2. Check current status for conflict detection (`ALREADY_IN_PROGRESS`, `NOT_IN_PROGRESS`)
3. Capture old status for audit event

```go
// Pre-read current status
var currentStatus string
row := store.QueryRow("SELECT status, assignee FROM issues WHERE id = ?", params.ID)
if err := row.Scan(&currentStatus, &_); err != nil {
    if errors.Is(err, sql.ErrNoRows) {
        return StartResult{}, gravaerrors.New("ISSUE_NOT_FOUND", ...)
    }
    return StartResult{}, gravaerrors.New("DB_UNREACHABLE", ...)
}

// Check status before transition
if currentStatus == "in_progress" {
    // Determine who's working on it (need assignee or work_session lookup)
    return StartResult{}, gravaerrors.New("ALREADY_IN_PROGRESS", ...)
}
```

### Performance Requirement (NFR2)

Both `grava start` and `grava stop` must complete within <15ms. This includes:
- Pre-read query
- Status validation
- `dag.SetNodeStatus` call (which does DB write + audit)
- JSON marshaling + output

Monitor with a simple benchmark or integration test that measures end-to-end time.

### References

- **Story 2.3 (Update & Assign):** [2-3-update-issue-fields-and-assign-actors.md](2-3-update-issue-fields-and-assign-actors.md) — Review the `updateIssue` and `assignIssue` implementations for named function pattern, pre-read strategy, and audit event structure.
- **Architecture:** [_bmad-output/planning-artifacts/architecture.md](../planning-artifacts/architecture.md) — Sections on DAG/graph layer (lines ~160-200), audit event types (EventStart, EventStop), and performance constraints.
- **Epic 2 Full Spec:** [_bmad-output/planning-artifacts/epics/epic-02-issue-lifecycle.md](../planning-artifacts/epics/epic-02-issue-lifecycle.md) — Lines 107-122 define Story 2.4 acceptance criteria and dependencies.

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (claude-opus-4-6)

### Debug Log References

### Completion Notes List

- Story created via create-story workflow (2026-04-01 09:10 UTC)
- Ultimate context engine analysis completed
- Ready for dev-story implementation
- Grava Tracking: story=grava-1073
- Grava Tasks: task1=grava-1073.1, task2=grava-1073.2, task3=grava-1073.3, task4=grava-1073.4, task5=grava-1073.5, task6=grava-1073.6, task7=grava-1073.7, task8=grava-1073.8
- Implementation complete (2026-04-02): All 8 tasks done, all tests pass
- Fixed command name collision: server start/stop renamed to db-start/db-stop to avoid conflict with issue start/stop
- Fixed AC#3: ALREADY_IN_PROGRESS error now includes actor name from assignee column
- Fixed pre-existing compilation error: TestStartCmd naming conflict resolved
- Migration 005 adds started_at/stopped_at columns (Option A: column-based, work_sessions deferred to Phase 2)
- SchemaVersion updated from 4 to 5 to match migration count

### File List

- `pkg/cmd/issues/start.go` (created: startIssue named function + newStartCmd)
- `pkg/cmd/issues/start_test.go` (created: 3 unit tests with sqlmock)
- `pkg/cmd/issues/stop.go` (created: stopIssue named function + newStopCmd)
- `pkg/cmd/issues/stop_test.go` (created: 3 unit tests with sqlmock)
- `pkg/cmd/issues/issues.go` (modified: added newStartCmd and newStopCmd to AddCommands)
- `pkg/cmd/commands_test.go` (modified: fixed integration tests, added TestStart_Stop_Cycle)
- `pkg/cmd/start.go` (modified: renamed server command to db-start)
- `pkg/cmd/stop.go` (modified: renamed server command to db-stop)
- `pkg/cmd/root.go` (modified: updated PersistentPreRunE skip list)
- `pkg/cmd/start_stop_test.go` (modified: updated test for renamed command)
- `pkg/utils/schema.go` (modified: SchemaVersion 4 → 5)
- `pkg/migrate/migrations/005_add_work_session_columns.sql` (created: adds started_at/stopped_at)
- `docs/guides/CLI_REFERENCE.md` (modified: documented start/stop commands, renamed db-start/db-stop)
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (modified: status in-progress → review)

### Change Log

- 2026-04-02: Implemented Story 2.4 — grava start/stop commands for work session tracking
