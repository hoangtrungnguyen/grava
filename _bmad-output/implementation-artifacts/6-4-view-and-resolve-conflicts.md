# Story 6.4: View and Resolve Conflicts

Status: ready-for-dev

## Story

As a developer,
I want to view and dismiss unresolvable merge conflicts,
So that I can inspect what was isolated and manually resolve or accept the outcome.

## Acceptance Criteria

1. **AC#1 -- List Pending Conflicts**
   `grava conflicts list` MUST display all records from `conflict_records` where `resolved_status='pending'`.
   Output columns: ID, Issue ID, Field, Ours, Theirs, Detected At.

2. **AC#2 -- JSON Output**
   `grava conflicts list --json` MUST return a JSON array conforming to NFR5 schema.

3. **AC#3 -- Manual Resolution**
   `grava conflicts resolve <conflict-id> --accept=ours|theirs` MUST:
     - Update the specified field on the issue in the database.
     - Mark the conflict record as `resolved_status='resolved'` and set `resolved_at=NOW()`.

4. **AC#4 -- Dismissing Conflicts**
   `grava conflicts dismiss <conflict-id>` MUST:
     - Mark the conflict record as `resolved_status='dismissed'`.
     - NOT modify the issue (the human accepts the current file state).

5. **AC#5 -- Multi-Issue Filtering**
   `grava conflicts list --issue <issue-id>` MUST filter the list to a specific issue.

## Tasks / Subtasks

- [ ] Task 1: Implement `conflicts` Command Group
  - [ ] 1.1 Add `newConflictsCmd` to the CLI.
  - [ ] 1.2 Implement `list`, `resolve`, and `dismiss` subcommands.

- [ ] Task 2: Implement Table Display
  - [ ] 2.1 Use a terminal table library to show side-by-side diffs if possible, or simple list.

- [ ] Task 3: Implement Database Logic
  - [ ] 3.1 Implement status updates for `conflict_records`.
  - [ ] 3.2 Wire into the audit trail so resolutions are recorded as events.

- [ ] Task 4: Testing
  - [ ] 4.1 Integration test: Merge conflict -> List -> Resolve -> Verify issue state.

## Dev Notes

### Audit Trail
Resolving a conflict is a significant state change. Use `WithAuditedTx` to record the change as an `EventUpdate`.

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-06-advanced-merge-driver.md#Story-6.4]
