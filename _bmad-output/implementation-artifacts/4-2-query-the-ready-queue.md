# Story 4.2: Query the Ready Queue

Status: ready-for-dev

## Story

As an agent or developer,
I want to query the top-priority tasks with no active blockers,
So that I can immediately identify what work is actionable without manual triage.

## Acceptance Criteria

1. **AC#1 -- Ready Queue Filtering**
   Given a workspace with issues in various states (open, in_progress, done, blocked),
   When I run `grava ready`,
   Then only issues with `status NOT IN ('done', 'archived')` are considered,
   And only issues where ALL upstream blockers have `status IN ('done', 'archived')` (or no blockers exist) are returned,
   And the result set is ordered by:
     1. Priority (Critical=0 -> Backlog=4) ascending
     2. CreatedAt ascending.

2. **AC#2 -- Limit and JSON Support**
   Given multiple ready issues exist,
   When I run `grava ready --limit 5 --json`,
   Then at most 5 issues are returned,
   And the output is a JSON array of issue objects conforming to NFR5 schema,
   And each object includes at minimum: `id`, `title`, `status`, `priority`, `assignee`.

3. **AC#3 -- Empty State**
   Given all open issues are currently blocked by other non-done tasks,
   When I run `grava ready`,
   Then a helpful message "No tasks are currently ready (all open work is blocked)" is printed to stderr,
   And the command exits with code 0 (success),
   And `--json` returns `[]`.

4. **AC#4 -- Performance (Phase 1 Baseline)**
   Given 1,000 issues in the database,
   When I run `grava ready`,
   Then the command completes in <100ms (direct SQL scan path, no caching yet).

## Tasks / Subtasks

- [ ] Task 1: Implement the Ready Queue SQL Query
  - [ ] 1.1 Construct a query that selects issues NOT done/archived where NOT EXISTS a dependency to a non-done/non-archived issue.
  - [ ] 1.2 Ensure ordering is applied: `priority ASC, created_at ASC`.
  - [ ] 1.3 Implement `--limit` flag handling in the SQL `LIMIT` clause.

- [ ] Task 2: Implement the `ready` Command CLI
  - [ ] 2.1 Add `newReadyCmd` to `pkg/cmd/graph/graph.go`.
  - [ ] 2.2 Wire the command to a `runReady` function that uses `cmddeps.Deps`.
  - [ ] 2.3 Implement standard human-readable table output (ID, Title, Priority).
  - [ ] 2.4 Implement `--json` output support using `Deps.OutputJSON`.

- [ ] Task 3: Unit and Integration Testing
  - [ ] 3.1 Write unit tests in `pkg/cmd/graph/ready_test.go` using `sqlmock`.
  - [ ] 3.2 Scenario: Independent tasks are ready.
  - [ ] 3.3 Scenario: Tasks blocked by 'done' tasks are ready.
  - [ ] 3.4 Scenario: Tasks blocked by 'open' tasks are NOT ready.
  - [ ] 3.5 Scenario: Circularly blocked tasks (if they exist) are NOT ready.

## Dev Notes

### SQL Implementation Detail
The "Ready" logic can be expressed as:
```sql
SELECT i.* FROM issues i
WHERE i.status NOT IN ('done', 'archived')
  AND NOT EXISTS (
    SELECT 1 FROM dependencies d
    JOIN issues b ON d.from_id = b.id
    WHERE d.to_id = i.id
      AND b.status NOT IN ('done', 'archived')
  )
ORDER BY i.priority ASC, i.created_at ASC
LIMIT ?;
```

### Architecture Patterns
- **Read-Only Operation:** No `WithAuditedTx` required. Use standard `store.QueryContext`.
- **JSON Schema:** Ensure the serialised `Issue` struct matches the one used in `grava list`.

### References
- [Source: pkg/cmd/graph/graph.go] -- location for the new command.
- [Source: _bmad-output/planning-artifacts/epics/epic-04-dependency-graph.md#Story-4.2] -- story spec.
