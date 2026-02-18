---
issue: TASK-1-4.subtask-2
status: done
Description: Implement the 'create' command.
---

## User Story
**As a** user
**I want to** run `grava create`
**So that** I can add new issues to the database

## Acceptance Criteria
- [x] Accepts title, description, priority, type
- [x] Generates ID using `idgen` package
- [x] Inserts row into `issues` table
- [x] Prints the new ID to stdout
