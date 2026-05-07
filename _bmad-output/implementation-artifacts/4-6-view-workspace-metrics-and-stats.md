# Story 4.6: View Workspace Metrics and Stats

Status: ready-for-dev

## Story

As a developer,
I want to view aggregated workspace performance and status metrics,
So that I can assess project health and identify bottlenecks at a glance.

## Acceptance Criteria

1. **AC#1 -- Aggregate Status Counts**
   When I run `grava stats`,
   Then a breakdown of total issues by status is displayed (open, in_progress, paused, done, archived).

2. **AC#2 -- Bottleneck Metrics**
   The `stats` output MUST include:
     - **Blocked Count**: Number of issues currently blocked by at least one non-done task.
     - **Stale Work Count**: Number of `in_progress` issues with no audit events/heartbeat in the last 1 hour.
     - **Ready Queue Depth**: Number of issues currently in the 'ready' state.

3. **AC#3 -- Performance Metrics**
   The `stats` output MUST include:
     - **Avg Cycle Time**: Calculated as the average of `(done_at - created_at)` for all issues in 'done' status.
     - **Lead Time**: Time from creation to completion.

4. **AC#4 -- JSON Support**
   When I run `grava stats --json`,
   Then it returns a structured JSON object conforming to NFR5 schema.

5. **AC#5 -- Performance Validation**
   The command MUST complete in <100ms with a database of 1,000 issues.

## Tasks / Subtasks

- [ ] Task 1: Create the `stats` Command CLI
  - [ ] 1.1 Add `newStatsCmd` to `pkg/cmd/graph/graph.go`.
  - [ ] 1.2 Wire to `runStats` using `cmddeps.Deps`.

- [ ] Task 2: Implement Aggregate SQL Queries
  - [ ] 2.1 Query counts by status: `SELECT status, COUNT(*) FROM issues GROUP BY status`.
  - [ ] 2.2 Query blocked count: `SELECT COUNT(DISTINCT to_id) FROM dependencies JOIN issues FROM_I ON from_id = FROM_I.id WHERE FROM_I.status NOT IN ('done', 'archived')`.
  - [ ] 2.3 Query stale work: Cross-reference `issues` with `events` timestamps.
  - [ ] 2.4 Query average cycle time.

- [ ] Task 3: Testing
  - [ ] 3.1 Unit tests in `pkg/cmd/graph/stats_test.go`.
  - [ ] 3.2 Mock varied database states to verify calculations.

## Dev Notes

### Stale Work Check
```sql
SELECT COUNT(*) FROM issues i
WHERE i.status = 'in_progress'
  AND NOT EXISTS (
    SELECT 1 FROM events e
    WHERE e.issue_id = i.id
      AND e.timestamp > NOW() - INTERVAL 1 HOUR
  )
```

### Architecture Patterns
- Use `Store.QueryContext`.
- Ensure `done_at` is sourced from `work_sessions` (if implemented) or the audit event of the last status transition to 'done'.

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-04-dependency-graph.md#Story-4.6]
