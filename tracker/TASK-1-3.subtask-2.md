---
issue: TASK-1-3.subtask-2
status: done
Description: Implement the hash-based ID generation logic.
---

## User Story
**As a** system
**I want to** generate unique hash-based IDs
**So that** we avoid collisions in a distributed environment

## Acceptance Criteria
- [x] `GenerateBaseID()` returns valid `grava-<hash>` format
- [x] Hash is derived from timestamp + randomness
- [x] No collisions detected in basic localized tests
