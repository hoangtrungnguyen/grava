# Story 3.1: Atomic Issue Claim

Status: done

## Story

As an agent,
I want to atomically claim an unassigned issue under concurrent conditions,
So that exactly one agent owns the issue and no two agents work on the same task simultaneously.

## Acceptance Criteria

1. **AC#1 — Happy Path Claim**
   Given issue `abc123def456` exists with `assignee=NULL` and `status=open`,
   When I run `grava claim abc123def456 --actor agent-01`,
   Then a single DB transaction issues `SELECT FOR UPDATE` on the issues row, verifies `assignee IS NULL`, then sets `assignee=agent-01` and `status=in_progress`,
   And `grava claim --json` returns `{"id": "abc123def456", "status": "in_progress", "actor": "agent-01"}`,
   And the claim operation completes in <15ms (NFR2).

2. **AC#2 — Concurrent Claim Rejection**
   Given issue `abc123def456` has just been claimed by `agent-01`,
   When a second concurrent `grava claim abc123def456 --actor agent-02` executes simultaneously,
   Then it returns `{"error": {"code": "ALREADY_CLAIMED", "message": "Issue abc123def456 is already claimed"}}` — no polluted row, no deadlock.

3. **AC#3 — Non-Concurrent Re-claim**
   Given issue `abc123def456` is already in `in_progress` state,
   When I run `grava claim abc123def456 --actor agent-02` (non-concurrent),
   Then it returns the same `ALREADY_CLAIMED` error immediately.

4. **AC#4 — Invalid Status Rejection**
   Given issue `abc123def456` has `status=closed`,
   When I run `grava claim abc123def456 --actor agent-01`,
   Then it returns `{"error": {"code": "INVALID_STATUS_TRANSITION", "message": "cannot claim issue abc123def456: status is \"closed\" (must be \"open\")"}}`.

5. **AC#5 — Not Found**
   Given no issue with ID `nonexistent` exists,
   When I run `grava claim nonexistent --actor agent-01`,
   Then it returns `{"error": {"code": "ISSUE_NOT_FOUND", "message": "issue nonexistent not found"}}`.

## Tasks / Subtasks

- [x] Task 1: Fix JSON field name alignment (AC: #1) — NO CHANGE NEEDED
  - [x] 1.1 Investigated: `json:"actor"` is the established codebase convention (used in comment.go, issues.go, maintenance.go). Epic AC's `"assignee"` was the mismatch. Story AC already correctly specifies `"actor"`.
  - [x] 1.2 No test changes needed — existing tests pass

- [x] Task 2: Add concurrent claim integration test (AC: #2)
  - [x] 2.1 Created `pkg/cmd/issues/claim_concurrent_test.go` with build tag `//go:build integration`
  - [x] 2.2 Test `TestConcurrentClaim_ExactlyOneSucceeds` asserts exactly one success and one ALREADY_CLAIMED failure
  - [x] 2.3 Test uses sync.WaitGroup + channel for synchronization, verifies DB state consistency

- [x] Task 3: Verify existing unit tests still pass (AC: #1, #3, #4, #5)
  - [x] 3.1 `go test ./pkg/cmd/issues/... -run TestClaim` — all 4 tests PASS
  - [x] 3.2 `go test ./pkg/cmd/issues/...` — PASS (pre-existing build failure in pkg/cmd unrelated)

## Dev Notes

### Existing Implementation

**The `claim` command is already fully implemented** in `pkg/cmd/issues/claim.go`. The implementation correctly uses:
- `WithAuditedTx` for atomic write + audit log
- `SELECT status FROM issues WHERE id = ? FOR UPDATE` for row-level locking
- `GravaError` for structured error responses
- Audit event logging with `dolt.EventClaim`

**Tests exist** in `pkg/cmd/issues/claim_test.go` covering:
- Happy path (open → in_progress)
- Not found (ISSUE_NOT_FOUND)
- Already claimed (ALREADY_CLAIMED)
- Invalid status transition (INVALID_STATUS_TRANSITION)

### Required Fixes (from Readiness Assessment)

1. **JSON field name mismatch:** `ClaimResult.Actor` has `json:"actor"` but the epic AC specifies `"assignee"`. Change the struct tag to `"assignee"` for NFR5 compliance. This is a schema-breaking change — verify no downstream consumers rely on `"actor"`.

2. **No concurrent test:** Unit tests use sqlmock (no real DB). A concurrent integration test against real Dolt is needed to validate NFR3.

### Architecture Patterns (MUST FOLLOW)

- **Named Function Pattern:** `func claimIssue(ctx context.Context, store dolt.Store, issueID, actor, model string) (ClaimResult, error)` — already in place
- **Transaction Pattern:** `BeginTx` → `defer Rollback` → `SELECT FOR UPDATE` → validate → `UPDATE` → audit log → `Commit` — already correct
- **DO NOT wrap `WithAuditedTx` in `WithDeadlockRetry`** — would duplicate audit logs on retry (ADR-FM4)
- **Error codes:** `ALREADY_CLAIMED`, `INVALID_STATUS_TRANSITION`, `ISSUE_NOT_FOUND`, `DB_UNREACHABLE`

### Database Schema

```sql
-- issues table (migration 001)
status VARCHAR(32) -- CHECK includes 'open', 'in_progress'
assignee VARCHAR(128) -- NULL when unclaimed

-- events table (audit trail)
event_type = 'claim' -- dolt.EventClaim constant
```

### Testing Patterns

- Unit tests: `sqlmock.New()` + `dolt.NewClientFromDB(db)`
- Integration tests: real Dolt connection, concurrent goroutines
- Assertions: `errors.As(err, &gravaErr)` for error code checks
- Framework: `testify/assert` + `testify/require`

### Project Structure Notes

- Command: `pkg/cmd/issues/claim.go`
- Tests: `pkg/cmd/issues/claim_test.go`
- New concurrent test: `pkg/cmd/issues/claim_concurrent_test.go`
- DB layer: `pkg/dolt/tx.go` (WithAuditedTx), `pkg/dolt/retry.go` (WithDeadlockRetry)
- Errors: `pkg/errors/errors.go` (GravaError)

### References

- [Source: _bmad-output/planning-artifacts/epics/epic-03-atomic-claim.md#Story 3.1]
- [Source: _bmad-output/planning-artifacts/architecture.md#ADR-FM4, ADR-003, ADR-H3]
- [Source: pkg/cmd/issues/claim.go — existing implementation]
- [Source: pkg/cmd/issues/claim_test.go — existing unit tests]
- [Source: _bmad-output/implementation-artifacts/implementation-readiness-report-2026-04-05.md — readiness findings]
- [Source: _bmad-output/implementation-artifacts/2-6-archive-and-purge-issues.md — previous story patterns]

## Dev Agent Record

### Agent Model Used

### Debug Log References

### Completion Notes List

- Story 3.1 context created — claim command already implemented, minor alignment fixes needed (2026-04-05)
- Task 1: No change needed — `json:"actor"` is established codebase convention. Epic AC was the mismatch, not the code.
- Task 2: Created concurrent integration test with `//go:build integration` tag. Validates NFR3 (exactly-one claim guarantee).
- Task 3: All 4 existing unit tests pass. No regressions.
- Code review (2026-04-06): Fixed 5 issues — H1: added assignee NULL check, M1: DSN port aligned to 3311, M2: added NFR2 benchmark, M3: updated File List, L1: error message casing. All tests pass.

### File List

- `pkg/cmd/issues/claim_concurrent_test.go` (NEW) — integration test for concurrent claim behavior
- `pkg/cmd/issues/claim.go` (MODIFIED) — added assignee NULL verification to SELECT FOR UPDATE
- `pkg/cmd/issues/claim_test.go` (MODIFIED) — updated mocks for new SELECT columns, added edge case test
- `_bmad-output/implementation-artifacts/3-1-atomic-issue-claim.md` (CREATED) — this story file
- `_bmad-output/implementation-artifacts/3-2-write-and-read-wisp-ephemeral-state.md` (CREATED) — story 3.2 spec
- `_bmad-output/implementation-artifacts/3-3-retrieve-issue-progression-history.md` (CREATED) — story 3.3 spec
- `_bmad-output/implementation-artifacts/3-4-epic-3-sandbox-integration-tests.md` (CREATED) — story 3.4 spec
- `_bmad-output/implementation-artifacts/epic-3-stories.md` (CREATED) — epic 3 story list
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (MODIFIED) — epic 3 sprint tracking
- `_bmad-output/implementation-artifacts/implementation-readiness-report-2026-04-05.md` (CREATED) — readiness report
