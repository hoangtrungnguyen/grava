---
issue: TASK-1-6.subtask-2
status: todo
Description: Create compaction logic for clearing old Wisps.
---

## User Story
**As a** system
**I want to** delete old ephemeral issues
**So that** the database stays clean

## Acceptance Criteria
- [ ] `grava compact` command implemented
- [ ] deletes ephemeral issues older than X days
- [ ] Records deletions in `deletions` table
