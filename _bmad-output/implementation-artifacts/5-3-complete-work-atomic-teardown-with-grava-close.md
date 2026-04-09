# Story 5.3: Complete Work — Atomic Teardown with `grava close`

Status: ready-for-dev

## Story

As an agent,
I want to atomically tear down my worktree when I complete an issue,
So that the branch and directory are cleaned up without leaving orphaned state.

## Acceptance Criteria

1. **AC#1 -- Dirtiness Check**
   When `grava close <id>` is run, the system MUST check the associated worktree for uncommitted changes using `git status --porcelain`.
   If dirty, the command MUST block with `UNCOMMITTED_CHANGES` error.
   If clean (or `--force` is used), proceed.

2. **AC#2 -- Atomic Teardown Protocol**
   The teardown MUST follow this order:
     1. Transition issue to `done` in the database.
     2. Release any active file reservations for the actor on this issue.
     3. Remove the Git worktree: `git worktree remove --force <path>`.
     4. Delete the dedicated branch: `git branch -D grava/<actor>/<id>`.

3. **AC#3 -- JSON Output**
   `grava close --json` MUST return:
     - `status`: "done"
     - `worktree_removed`: true
     - `branch_deleted`: true
     - `reservations_released`: count of rows updated.

4. **AC#4 -- Dependency Check**
   If an issue has subtasks that are NOT `done`, `grava close` SHOULD warn the user but proceed if `--force` is given. (Alignment with standard PM logic).

## Tasks / Subtasks

- [ ] Task 1: Create the `close` Command CLI
  - [ ] 1.1 Add `newCloseCmd` to `pkg/cmd/issues/`.
  - [ ] 1.2 Implement the dirtiness check (exec `git status --porcelain`).

- [ ] Task 2: Implement the Teardown Logic
  - [ ] 2.1 Update status to `done` in `WithAuditedTx`.
  - [ ] 2.2 Release reservations (set `released_ts` in `file_reservations` - wait, check table name).
  - [ ] 2.3 Exec `git worktree remove` and `git branch -D`.

- [ ] Task 3: Testing
  - [ ] 3.1 Scenario: Close clean worktree.
  - [ ] 3.2 Scenario: Block on dirty worktree.
  - [ ] 3.3 Scenario: Force close dirty worktree.

## Dev Notes

### Reservations Table
Check migration `003_add_affected_files.sql` or similar for reservation schema.

### Git Worktree Removal
`git worktree remove` handles directory deletion. Force is needed if the worktree is modified but we want to discard changes (though AC#1 handles the guard).

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-05-worktree.md#Story-5.3]
