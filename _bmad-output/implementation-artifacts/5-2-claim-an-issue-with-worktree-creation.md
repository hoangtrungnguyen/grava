# Story 5.2: Claim an Issue with Worktree Creation

Status: ready-for-dev

## Story

As an agent,
I want `grava claim` to automatically create a dedicated Git worktree and branch for the claimed issue,
So that my work is isolated from other agents and I have a clean directory to work in.

## Acceptance Criteria

1. **AC#1 -- Worktree Mode Detection**
   When `grava claim <id>` is run, the command MUST check `.grava/config` for `worktrees_enabled`.
   If `false` (default), follow standard claim logic.
   If `true`, proceed with worktree orchestration.

2. **AC#2 -- Atomic Claim + Worktree Creation**
   The process MUST be atomic:
     1. `SELECT FOR UPDATE` on the issue to acquire the DB lock.
     2. Verify issue is `open` and has no assignee.
     3. Create a Git branch: `grava/<actor>/<issue-id>`.
     4. Create a Git worktree at `.grava/worktrees/<actor>/<issue-id>/`.
     5. If branch/worktree creation succeeds, `UPDATE` the issue status to `in_progress` and set the `assignee`.
     6. If any Git operation fails, ROLLBACK the DB transaction and cleanup any partial directory.

3. **AC#3 -- Branch Naming Convention**
   The branch MUST follow the format: `grava/<agent-id>/<issue-id>`.
   Example: `grava/agent-01/grava-123`.

4. **AC#4 -- JSON Output**
   `grava claim --json` MUST include:
     - `worktree_path`: Absolute or relative path to the new worktree.
     - `branch`: The name of the checked-out branch.

5. **AC#5 -- Conflict Handling**
   If two agents claim the same issue concurrently, exactly one MUST succeed at the DB level (Epic 3 NFR3).
   The "loser" MUST receive a `CLAIM_CONFLICT` error and NO worktree should be created for them.

6. **AC#6 -- Rollback Cleanup**
   If worktree creation fails after the DB claim succeeds but before commit:
   Any partially-created directory MUST be deleted.
   If cleanup fails, log a `WORKTREE_CLEANUP_FAILED` warning (for `grava doctor` to handle later).

## Tasks / Subtasks

- [ ] Task 1: Extend `grava claim` for Worktree Mode
  - [ ] 1.1 Read `worktrees_enabled` from config.
  - [ ] 1.2 Implement `git` command wrappers for `worktree add` and `branch`.
  - [ ] 1.3 Update `claimIssue` function to handle the two-phase commit (DB + FS).

- [ ] Task 2: Implement Rollback Logic
  - [ ] 2.1 Use a `defer` block or `if err != nil` cleanup to remove the directory on failure.

- [ ] Task 3: Testing
  - [ ] 3.1 Scenario: Happy path worktree claim.
  - [ ] 3.2 Scenario: Concurrent claim conflict (Dolt transaction failure).
  - [ ] 3.3 Scenario: Git failure (e.g. branch exists) causes DB rollback.

## Dev Notes

### Git Commands
```bash
git branch grava/<actor>/<id> <start-point>
git worktree add .grava/worktrees/<actor>/<id>/ grava/<actor>/<id>
```

### Path Resolution
Use the `grava.Resolver` to ensure paths are always relative to the `.grava` root.

### References
- [Source: pkg/cmd/issues/claim.go]
- [Source: _bmad-output/planning-artifacts/epics/epic-05-worktree.md#Story-5.2]
