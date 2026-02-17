---
issue: TASK-1-3.subtask-3
status: done
Description: Implement hierarchical child ID generation using the database.
---

## User Story
**As a** developer
**I want to** generate atomic child IDs (e.g. .1, .2)
**So that** I can create subtasks with a clear ancestry

## Acceptance Criteria
- [x] `GenerateChildID(parentID)` increments `child_counters` table
- [x] Returns correct format `parentID.N`
- [x] Handles concurrency (atomic updates in SQL)
