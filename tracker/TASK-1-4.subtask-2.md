---
issue: TASK-1-4.subtask-2
status: todo
Description: Implement the 'create' command.
---

## User Story
**As a** user
**I want to** run `grava create`
**So that** I can add new issues to the database

## Acceptance Criteria
- [ ] Accepts title, description, priority, type
- [ ] Generates ID using `idgen` package
- [ ] Inserts row into `issues` table
- [ ] Prints the new ID to stdout
