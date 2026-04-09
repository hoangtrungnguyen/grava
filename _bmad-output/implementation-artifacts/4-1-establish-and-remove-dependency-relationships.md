# Story 4.1: Establish and Remove Dependency Relationships

Status: ready-for-dev

## Story

As a developer or agent,
I want to create and remove directional blocking relationships between issues,
so that the system knows which tasks must complete before others can start.

## Acceptance Criteria

1. **AC#1 -- Add Dependency (Happy Path)**
   Given issues `fromID` (blocker) and `toID` (blocked) both exist in the `issues` table,
   When I run `grava dep fromID toID`,
   Then a row is inserted in the `dependencies` table: `{from_id: fromID, to_id: toID, type: "blocks"}`,
   And the insert is wrapped in `WithAuditedTx` with audit event `EventDependencyAdd`,
   And row locks are acquired in lexicographic order of issue IDs (ADR-H3),
   And `WithDeadlockRetry` wraps the lock-acquisition portion,
   And human output prints `🔗 Dependency created: fromID -[blocks]-> toID`,
   And `--json` output returns `{"from_id": "...", "to_id": "...", "type": "blocks", "status": "created"}`.

2. **AC#2 -- Remove Dependency (Happy Path)**
   Given a dependency exists from `fromID` to `toID`,
   When I run `grava dep --remove fromID toID`,
   Then the dependency row is deleted from `dependencies`,
   And the delete is wrapped in `WithAuditedTx` with audit event `EventDependencyRemove`,
   And human output prints `🔗 Dependency removed: fromID -/-> toID`,
   And `--json` output returns `{"from_id": "...", "to_id": "...", "status": "removed"}`.

3. **AC#3 -- Circular Dependency Rejection**
   Given A blocks B already exists,
   When I run `grava dep B A`,
   Then it returns `{"error": {"code": "CIRCULAR_DEPENDENCY", "message": "This dependency would create a cycle"}}`,
   And no row is inserted into `dependencies`.

4. **AC#4 -- Non-existent Issue**
   Given no issue with ID `nonexistent` exists,
   When I run `grava dep nonexistent someID`,
   Then it returns `{"error": {"code": "ISSUE_NOT_FOUND", "message": "issue nonexistent not found"}}`.

5. **AC#5 -- Self-loop Rejection**
   Given issue `abc123` exists,
   When I run `grava dep abc123 abc123`,
   Then it returns an error rejecting self-loops.

6. **AC#6 -- Remove Non-existent Dependency**
   Given no dependency exists from `A` to `B`,
   When I run `grava dep --remove A B`,
   Then it returns an appropriate error or no-op with message.

## Tasks / Subtasks

- [ ] Task 1: Add `--remove` flag and `EventDependencyRemove` constant (AC: #2)
  - [ ] 1.1 Add `EventDependencyRemove = "dependency_remove"` to `pkg/dolt/events.go`
  - [ ] 1.2 Add `--remove` boolean flag to `newDepCmd` in `pkg/cmd/graph/graph.go`
  - [ ] 1.3 Route `--remove` to a `removeDependency` function; keep `addDependency` for the default path

- [ ] Task 2: Refactor `addDependency` to use `WithAuditedTx` + `WithDeadlockRetry` + issue existence validation (AC: #1, #3, #4, #5)
  - [ ] 2.1 Before graph load: validate both `fromID` and `toID` exist in `issues` table via `SELECT id FROM issues WHERE id IN (?, ?)` -- return `GravaError("ISSUE_NOT_FOUND", ...)` if either is missing
  - [ ] 2.2 Acquire row locks in lexicographic order: `sort.Strings([]string{fromID, toID})` then `SELECT id FROM issues WHERE id = ? FOR UPDATE` for each (ADR-H3)
  - [ ] 2.3 Wrap lock acquisition in `WithDeadlockRetry`
  - [ ] 2.4 Load graph, run `AddEdgeWithCycleCheck` for blocking types -- map `graph.ErrCycleDetected` to `GravaError("CIRCULAR_DEPENDENCY", "This dependency would create a cycle")`
  - [ ] 2.5 Wrap the DB INSERT + audit log in `WithAuditedTx` with `EventDependencyAdd`
  - [ ] 2.6 Add `--json` output support: return `{"from_id": "...", "to_id": "...", "type": "...", "status": "created"}`

- [ ] Task 3: Implement `removeDependency` with `WithAuditedTx` + audit log (AC: #2, #6)
  - [ ] 3.1 Validate both issues exist (same as add path)
  - [ ] 3.2 Check dependency exists before deleting: `SELECT 1 FROM dependencies WHERE from_id = ? AND to_id = ?`
  - [ ] 3.3 Delete row within `WithAuditedTx` with `EventDependencyRemove`
  - [ ] 3.4 Add `--json` output support: return `{"from_id": "...", "to_id": "...", "status": "removed"}`

- [ ] Task 4: Write unit tests (AC: #1-#6)
  - [ ] 4.1 Test add dependency happy path (sqlmock: issue exists, insert succeeds, audit logged)
  - [ ] 4.2 Test add dependency with non-existent issue (ISSUE_NOT_FOUND error)
  - [ ] 4.3 Test circular dependency rejection (CIRCULAR_DEPENDENCY error via graph engine)
  - [ ] 4.4 Test self-loop rejection
  - [ ] 4.5 Test remove dependency happy path
  - [ ] 4.6 Test remove non-existent dependency
  - [ ] 4.7 Test `--json` output format for both add and remove

- [ ] Task 5: Verify no regressions (AC: all)
  - [ ] 5.1 Run `go test ./pkg/cmd/graph/...` -- all existing tests pass
  - [ ] 5.2 Run `go test ./pkg/graph/...` -- all graph engine tests pass
  - [ ] 5.3 Run `go vet ./...` -- no warnings

## Dev Notes

### Existing Implementation Analysis

**The `dep` command already exists** in `pkg/cmd/graph/graph.go` with significant functionality:
- `addDependency` (line ~100): Creates deps via direct `Store.Exec` INSERT + `Store.LogEvent` (non-transactional)
- `newDepBatchCmd`: Batch dep creation from JSON
- `newDepClearCmd`: Clear all deps for an issue
- `newDepTreeCmd`: Show ancestry tree
- `newDepPathCmd`: Show blocking path
- `newDepImpactCmd`: Show downstream impact

**What's MISSING (this story's scope):**
1. **No `--remove` flag** -- only `dep clear` (removes ALL deps) exists. No targeted single-dep removal.
2. **No `WithAuditedTx`** -- current `addDependency` uses raw `Store.Exec` + `Store.LogEvent` separately (non-atomic).
3. **No `WithDeadlockRetry`** -- no deadlock protection on concurrent dep writes.
4. **No issue existence validation** -- `addDependency` does not check if `fromID`/`toID` exist before INSERT. The FK constraint will reject, but the error is a raw MySQL error, not a structured `GravaError`.
5. **No `--json` output** -- current output is human-only emoji format.
6. **No `EventDependencyRemove` constant** -- only `EventDependencyAdd` exists in `pkg/dolt/events.go`.
7. **Lock ordering (ADR-H3) not implemented** -- no lexicographic lock acquisition.

### Architecture Patterns (MUST FOLLOW)

- **Transaction Pattern:** `dolt.WithAuditedTx(ctx, store, []AuditEvent{...}, func(tx *sql.Tx) error { ... })` -- see `pkg/dolt/tx.go`
- **Deadlock Retry:** `dolt.WithDeadlockRetry(func() error { ... })` -- see `pkg/dolt/retry.go`. Only wrap lock acquisition, NOT `WithAuditedTx`.
- **Lock Ordering (ADR-H3):** `sort.Strings([]string{fromID, toID})` before `SELECT ... FOR UPDATE`. This prevents deadlocks when two agents add reciprocal deps simultaneously.
- **Error Types:** Use `gravaerrors.New(code, message, cause)` from `pkg/errors`. Import as `gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"`.
- **Audit Events:** Use `dolt.EventDependencyAdd` and new `dolt.EventDependencyRemove` constants. Never raw strings.
- **Graph Cycle Check:** `dag.AddEdgeWithCycleCheck(edge)` returns `graph.ErrCycleDetected` (sentinel) or `*graph.CycleError` (detailed). Map to `CIRCULAR_DEPENDENCY` GravaError.
- **JSON output:** Check `*d.OutputJSON` flag. Success: flat object `{"from_id": ..., "status": "created"}`. Error: `{"error": {"code": "...", "message": "..."}}`.
- **Human output:** Use `cmd.OutOrStdout()` for stdout, `cmd.ErrOrStderr()` for errors inside RunE.
- **Context propagation:** Thread `cmd.Context()` through to `WithAuditedTx`. Never `context.Background()` inside `pkg/`.

### DB Schema Reference

```sql
-- dependencies table (001_initial_schema.sql)
CREATE TABLE dependencies (
    from_id VARCHAR(32) NOT NULL,
    to_id VARCHAR(32) NOT NULL,
    type VARCHAR(32) NOT NULL,
    metadata JSON,
    PRIMARY KEY (from_id, to_id, type),
    FOREIGN KEY (from_id) REFERENCES issues(id) ON DELETE CASCADE,
    FOREIGN KEY (to_id) REFERENCES issues(id) ON DELETE CASCADE,
    INDEX idx_to_id (to_id)
);
```

Note: `created_by`, `updated_by`, `agent_model` columns are added by migration `002_audit_columns.sql`.

### Dependencies Injection Pattern

All commands receive `*cmddeps.Deps` containing `Store`, `Actor`, `AgentModel`, `OutputJSON` pointers. See `pkg/cmddeps/deps.go`.

### Testing Pattern

- Use `sqlmock` for unit tests (see `pkg/cmd/graph/ready_test.go` for reference).
- `dolt.NewClientFromDB(db)` wraps a `*sql.DB` into a `Store`.
- `testify/require` for fatal assertions, `testify/assert` for non-fatal.
- Test file: `pkg/cmd/graph/dep_test.go` (new file, co-located).

### Critical Anti-Patterns (DO NOT)

- Do NOT use `fmt.Errorf` for user-facing errors -- use `gravaerrors.New()`
- Do NOT call `Store.LogEvent` outside of `WithAuditedTx` for write operations
- Do NOT wrap `WithAuditedTx` inside `WithDeadlockRetry` -- audit duplication risk
- Do NOT use `fmt.Println` -- use `cmd.OutOrStdout()` or `cmd.ErrOrStderr()`
- Do NOT add new event constants outside `pkg/dolt/events.go`
- Do NOT skip lock ordering -- always sort IDs lexicographically before `FOR UPDATE`

### Project Structure Notes

- Command file: `pkg/cmd/graph/graph.go` (existing, modify in place)
- Test file: `pkg/cmd/graph/dep_test.go` (new)
- Events file: `pkg/dolt/events.go` (add `EventDependencyRemove`)
- Graph engine: `pkg/graph/` -- DO NOT modify; use existing `AddEdgeWithCycleCheck`, `RemoveEdge`, `ErrCycleDetected`

### References

- [Source: pkg/cmd/graph/graph.go] -- existing dep command implementation
- [Source: pkg/dolt/tx.go] -- WithAuditedTx pattern
- [Source: pkg/dolt/retry.go] -- WithDeadlockRetry pattern
- [Source: pkg/dolt/events.go] -- audit event constants
- [Source: pkg/graph/dag.go#AddEdgeWithCycleCheck] -- cycle detection
- [Source: pkg/graph/errors.go] -- ErrCycleDetected, ErrNodeNotFound sentinels
- [Source: _bmad-output/planning-artifacts/architecture.md#ADR-H3] -- lock ordering decision
- [Source: _bmad-output/planning-artifacts/epics/epic-04-dependency-graph.md#Story-4.1] -- story spec

## Dev Agent Record

### Agent Model Used

### Debug Log References

### Completion Notes List

### File List

### Change Log
