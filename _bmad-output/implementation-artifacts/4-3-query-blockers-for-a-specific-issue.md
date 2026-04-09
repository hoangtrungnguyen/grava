# Story 4.3: Query Blockers for a Specific Issue

Status: ready-for-dev

## Story

As a developer or agent,
I want to see exactly what upstream issues are blocking a specific task,
So that I can resolve blockers in the right order to unblock work.

## Acceptance Criteria

1. **AC#1 -- Query Blockers (Happy Path)**
   Given issues `A` and `B` exist, and `A` blocks `B` exists in the `dependencies` table,
   When I run `grava blocked B`,
   Then a list showing issue `A` (ID, Title, Status, Assignee) is returned,
   And only "active" blockers (status NOT IN ('done', 'archived')) are shown by default.

2. **AC#2 -- Include Done Blockers**
   Given issue `B` was blocked by `A` (now 'done') and `C` (still 'open'),
   When I run `grava blocked B --all`,
   Then both `A` and `C` are returned in the list.

3. **AC#3 -- JSON Support**
   When I run `grava blocked B --json`,
   Then the output is a JSON array of issue objects (the blockers),
   And each object matches the NFR5 schema for issue summaries.

4. **AC#4 -- Non-existent Issue**
   When I run `grava blocked nonexistent`,
   Then it returns `{"error": {"code": "ISSUE_NOT_FOUND", "message": "issue nonexistent not found"}}`.

5. **AC#5 -- Multi-Level Blockers (Optional/Recursive)**
   Given `A` blocks `B`, and `B` blocks `C`,
   When I run `grava blocked C`,
   Then only `B` (the direct blocker) is returned by default,
   And `grava blocked C --recursive` returns both `B` and `A`.

## Tasks / Subtasks

- [ ] Task 1: Add the `blocked` Command to the CLI
  - [ ] 1.1 Add `newBlockedCmd` to `pkg/cmd/graph/graph.go`.
  - [ ] 1.2 Implement `--all` and `--recursive` flags.
  - [ ] 1.3 Wire to a `runBlocked` function.

- [ ] Task 2: Implement Blockers Retrieval Logic
  - [ ] 2.1 Fetch direct blockers: `SELECT i.* FROM issues i JOIN dependencies d ON d.from_id = i.id WHERE d.to_id = ?`.
  - [ ] 2.2 If `--recursive`, implement a recursive CTE or iterative BFS/DFS to find all ancestors in the DAG.
  - [ ] 2.3 Filter by status unless `--all` is provided.

- [ ] Task 3: Testing
  - [ ] 3.1 Unit tests in `pkg/cmd/graph/blocked_test.go`.
  - [ ] 3.2 Test scenarios: direct blocker, multiple blockers, no blockers, recursive blockers.

## Dev Notes

### SQL for Recursive Blockers (MariaDB/Dolt compatible)
```sql
WITH RECURSIVE blockers AS (
  -- Seed: direct blockers of target
  SELECT d.from_id
  FROM dependencies d
  WHERE d.to_id = ?
  
  UNION
  
  -- Recurse: blockers of the blockers
  SELECT d.from_id
  FROM dependencies d
  JOIN blockers b ON d.to_id = b.from_id
)
SELECT i.* FROM issues i
JOIN blockers b ON i.id = b.from_id;
```

### Architecture Patterns
- Import `cmddeps`.
- Use `Store.QueryContext`.

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-04-dependency-graph.md#Story-4.3]
