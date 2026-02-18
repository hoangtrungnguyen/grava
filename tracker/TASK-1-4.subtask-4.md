---
issue: TASK-1-4.subtask-4
status: done
Description: Implement the 'update' command.
---

## User Story
**As a** user
**I want to** run `grava update <id>`
**So that** I can modify an issue's status or details

## Acceptance Criteria
- [x] Accepts flags for fields to update (e.g., `--status`, `--priority`)
- [x] Updates only specified fields
- [x] Updates `updated_at` timestamp
