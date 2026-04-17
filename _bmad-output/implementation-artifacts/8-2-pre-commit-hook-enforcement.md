# Story 8.2: Pre-Commit Hook Enforcement

Status: ready-for-dev

## Story

As an agent,
I want the pre-commit Git hook to block commits to paths held by another agent's exclusive lease,
So that concurrent file modifications are caught at commit time before they reach the repository.

## Acceptance Criteria

1. **AC#1 -- Staged File Check Against Active Leases**
   When `agent-01` runs `git commit` and `agent-02` holds an exclusive reservation on `src/cmd/issues/*.go`,
   Then the pre-commit hook (`grava hook pre-commit`) checks all staged file paths against active exclusive leases in `file_reservations`.

2. **AC#2 -- Block on Conflict**
   If any staged path overlaps with an active exclusive lease held by another agent,
   Then the commit is blocked with exit code 1 and structured output:
   `{"code": "FILE_RESERVATION_BLOCK", "message": "Path src/cmd/issues/create.go is reserved by agent-02 until <expires_ts>. Release or wait."}`

3. **AC#3 -- Allow Own Leases**
   If the committing agent holds the exclusive lease themselves,
   Then the commit proceeds normally (exit code 0).

4. **AC#4 -- Non-Exclusive Leases**
   A non-exclusive (shared) lease does NOT block commits from other agents.

5. **AC#5 -- Expired Leases**
   An expired lease (past `expires_ts`) does NOT block commits — treated as released.

6. **AC#6 -- No Active Leases**
   If no staged paths overlap with active exclusive leases, the commit proceeds normally (exit code 0).

## Tasks / Subtasks

- [ ] Task 1: Implement reservation path matching utility
  - [ ] 1.1 Create `MatchStagedPaths(paths []string, actor string) ([]Conflict, error)` in reserve package
  - [ ] 1.2 Support glob patterns from `path_pattern` column
  - [ ] 1.3 Filter: only active, exclusive, non-expired, held by OTHER agents

- [ ] Task 2: Wire into pre-commit hook handler
  - [ ] 2.1 In `runPreCommit()`, get staged files via `git diff --cached --name-only`
  - [ ] 2.2 Call `MatchStagedPaths()` and block if conflicts found
  - [ ] 2.3 Return structured error output

- [ ] Task 3: Tests
  - [ ] 3.1 Unit tests for path matching (glob, exact, expired, own lease, shared lease)
  - [ ] 3.2 Integration test for pre-commit hook enforcement flow

## Dev Notes

### Existing Infrastructure
- Pre-commit hook: `pkg/githooks/hook.go` → `runPreCommit()` (lines ~297-312)
- Reserve package: `pkg/cmd/reserve/reserve.go` — has `DeclareReservation`, `ListReservations`, etc.
- DB table: `file_reservations` with `path_pattern`, `exclusive`, `expires_ts`, `released_ts`, `agent_id`
- Hook dispatch: `grava hook pre-commit` routes to `runPreCommit()`

### Path Matching
Use `filepath.Match()` for glob patterns. The `path_pattern` column stores patterns like `src/cmd/issues/*.go`.

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-08-file-reservation.md#Story-8.2]
