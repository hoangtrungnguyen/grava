# Story 6.1: ⚠️ SPIKE — Validate `grava-merge` Git Driver Invocation

Status: ready-for-dev

## Story

As a developer,
I want to validate that a custom Git merge driver can access Dolt SQL state during invocation,
So that we have proof-of-concept evidence before committing the remaining merge driver stories to sprint.

## Acceptance Criteria

1. **AC#1 -- Git Driver Invocation**
   Given a test Git repository with `issues.jsonl` tracked and `*.jsonl merge=grava-merge` in `.gitattributes`,
   And a registered merge driver `grava-merge` in `.git/config` pointing to the project binary,
   When a conflict is created on `issues.jsonl` and `git merge` is run,
   Then Git MUST invoke the driver with three relative paths: `%O` (ancestor), `%A` (ours), `%B` (theirs).

2. **AC#2 -- Environment and Connectivity**
   During invocation, the `grava-merge` process MUST:
     1. Print its invocation arguments to a debug log (e.g. `.grava/merge-debug.log`).
     2. Attempt to connect to the Dolt database (via the URL configured in the workspace).
     3. Execute `SELECT NOW()` and log the result to confirm DB accessibility while Git is in the middle of a merge operation.

3. **AC#3 -- Deterministic Success/Exit**
   The spike MUST be able to either:
     - Write a merged file to `%A` and exit 0 (Git marks conflict resolved).
     - Exit 1 (Git marks conflict as unresolvable).
   The spike MUST NOT corrupt the `%A` file on failure; it should be left as the "ours" version or replaced with Git's default markers.

4. **AC#4 -- Hard Gate: Sandbox Scenario**
   The spike MUST include a runnable sandbox scenario: `grava sandbox run --scenario=spike-merge-driver`.
   This scenario MUST pass (exit 0) in CI for the spike to be considered complete.

5. **AC#5 -- Spike Report**
   A report MUST be written to `.grava/spike-reports/merge-driver-poc.md` summarizing:
     - Invocation confirmation.
     - DB accessibility findings (any transaction locking issues?).
     - Performance overhead observation.

## Tasks / Subtasks

- [ ] Task 1: Setup Spike Harness
  - [ ] 1.1 Create a test harness that initializes a Git repo and injects `.gitattributes`.
  - [ ] 1.2 Implement a minimal `merge-driver` command in the `grava` binary that just logs its arguments and exits.

- [ ] Task 2: Validate DB Access
  - [ ] 2.1 Attempt to run a SQL query from within the driver process.
  - [ ] 2.2 Verify if the shared Dolt server (Epic 5) must be running or if direct file access is possible/preferred.

- [ ] Task 3: Create Sandbox Scenario
  - [ ] 3.1 Implement `spike-merge-driver` scenario in JSON.
  - [ ] 3.2 Verify CI execution.

## Dev Notes

### Git Configuration
To register locally:
```bash
git config merge.grava-merge.name "Grava Schema-Aware Merge Driver"
git config merge.grava-merge.driver "grava merge-driver %O %A %B"
```

### Critical Question
During `git merge`, does Dolt hold any locks on the database files that prevent the `grava-merge` process (which might start a new SQL connection) from executing? The spike MUST answer this.

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-06-advanced-merge-driver.md#Story-6.1]
