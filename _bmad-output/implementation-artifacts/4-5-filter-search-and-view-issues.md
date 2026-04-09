# Story 4.5: Filter, Search and View Issues

Status: ready-for-dev

## Story

As a developer or agent,
I want to filter the issue list, search by keyword, and view full details of a single issue,
So that I can quickly locate and inspect any work item in the tracker.

## Acceptance Criteria

1. **AC#1 -- Filtered Listing**
   When I run `grava list --status in_progress --priority 0 --assignee agent-01`,
   Then ONLY issues matching ALL provided filters are returned,
   And the output is a formatted table including ID, Title, Status, Priority, and Assignee.

2. **AC#2 -- Full-Text Search**
   When I run `grava search "login"`,
   Then the system performs a case-insensitive search across `title`, `description`, AND `issue_comments.message`,
   And returns matching issues as a list.

3. **AC#3 -- Detailed Show**
   When I run `grava show abc123def456`,
   Then all available metadata for the issue is displayed:
     - Header: ID, Title, Status, Priority, Type
     - Detailed Description
     - Assignee and Actor History
     - Labels
     - Comments (timestamped)
     - Subtasks (hierarchy)
     - Dependencies (Inbound/Outbound)
   And `grava show --json` returns the complete issue object.

4. **AC#4 -- JSON Schema Compliance**
   Both `list --json` and `show --json` MUST conform to the NFR5 schema contract.

## Tasks / Subtasks

- [ ] Task 1: Update `grava list`
  - [ ] 1.1 Add flags for `--status`, `--priority`, `--assignee`, `--type`.
  - [ ] 1.2 Implement dynamic SQL query building (AND logic).
  - [ ] 1.3 Add `--include-archived` flag (defaults to false).

- [ ] Task 2: Implement `grava search`
  - [ ] 2.1 Implement SQL `LIKE` search across multiple columns (optimized for MariaDB).
  - [ ] 2.2 Include a JOIN with `issue_comments` to search inside discussions.

- [ ] Task 3: Enhance `grava show`
  - [ ] 3.1 Aggregate data from `issues`, `issue_labels`, `issue_comments`, and `dependencies`.
  - [ ] 3.2 Implement rich terminal formatting for the detailed view.
  - [ ] 3.3 Ensure the `Issue` Go struct includes all related slices and serialises correctly to JSON.

- [ ] Task 4: Testing
  - [ ] 4.1 SQL mapping tests.
  - [ ] 4.2 Integration tests for search across comments.

## Dev Notes

### Search SQL
```sql
SELECT DISTINCT i.* FROM issues i
LEFT JOIN issue_comments c ON i.id = c.issue_id
WHERE (i.title LIKE ? OR i.description LIKE ? OR c.message LIKE ?)
```

### Architecture Patterns
- Use `Store.QueryContext`.
- Use `Deps.OutputJSON`.

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-04-dependency-graph.md#Story-4.5]
