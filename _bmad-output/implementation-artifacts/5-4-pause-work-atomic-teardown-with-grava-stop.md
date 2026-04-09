# Story 5.4: Pause Work — Atomic Teardown with `grava stop`

Status: ready-for-dev

## Story

As an agent,
I want to pause and abandon my worktree without losing the issue state for the next agent,
So that another agent can resume the work from my last Wisp checkpoint.

## Acceptance Criteria

1. **AC#1 -- Extended Stop Logic**
   When `grava stop <id>` is run in worktree mode:
   - It MUST check for uncommitted changes.
   - If dirty, emit a `WARNING` but allow proceeding with `--force`.
   - Transition issue to `paused`.
   - Preserve **Wisp state** (do NOT delete `wisp_entries` for this issue).

2. **AC#2 -- Partial Teardown (Branch Preservation)**
   Unlike `close`, `stop` MUST:
     1. Remove the Git worktree directory (`git worktree remove --force`).
     2. **KEEP** the Git branch `grava/<actor>/<id>`.
     3. This allows the next agent to potentially merge or reference the branch.

3. **AC#3 -- JSON Output**
   `grava stop --json` MUST return:
     - `status`: "paused"
     - `worktree_removed`: true
     - `branch_kept`: "grava/agent-01/abc123def456"
     - `wisp_preserved`: true

4. **AC#4 -- Resume Capability**
   A subsequent `grava claim <id> --actor agent-02` MUST succeed.
   The new agent gets a clean worktree, but `grava history` shows the Wisp entries from the previous session.

## Tasks / Subtasks

- [ ] Task 1: Update `pkg/cmd/issues/stop.go`
  - [ ] 1.1 Detect worktree mode.
  - [ ] 1.2 Implement the uncommitted-changes check.
  - [ ] 1.3 Update status to `paused` (and ensure DB constraint allows it).

- [ ] Task 2: Implement Partial Teardown
  - [ ] 2.1 Call `git worktree remove --force`.
  - [ ] 2.2 Skip branch deletion.

- [ ] Task 3: Migration (Statuses)
  - [ ] 3.1 Create a new migration to add `paused` and `done` (if not present) to the `check_status` constraint.

- [ ] Task 4: Testing
  - [ ] 4.1 Scenario: Stop with uncommitted changes (blocked).
  - [ ] 4.2 Scenario: Force stop (worktree removed, branch kept).

## Dev Notes

### Status Constraint Update
The `check_status` constraint in `issues` table needs to be dropped and recreated to include `paused` and `done`.

### References
- [Source: pkg/cmd/issues/stop.go]
- [Source: _bmad-output/planning-artifacts/epics/epic-05-worktree.md#Story-5.4]
