---
issue: TASK-1-6-EPHEMERAL-WISP-SUPPORT
status: done
Description: Create temporary "scratchpad" issues and safely delete old ones to prevent project history pollution.
---

**Timestamp:** 2026-02-18 15:24:00
**Affected Modules:**
  - pkg/cmd/
  - docs/
  - tracker/

---

## User Story
**As an** AI agent
**I want to** create temporary "scratchpad" issues and safely delete old ones
**So that** I don't pollute the project history with intermediate reasoning

## Acceptance Criteria
- [x] `create --ephemeral` flag sets `ephemeral=1` in DB
- [x] Ephemeral issues excluded from `grava list` by default (`WHERE ephemeral = 0`)
- [x] `grava list --wisp` filters for ephemeral issues only
- [x] `grava compact --days N` purges Wisps older than N days
- [x] Deletions recorded in `deletions` DB table (tombstone manifest) to prevent resurrection

## Subtasks
- [x] TASK-1-6.subtask-1 — Ephemeral flag handling (`create --ephemeral`, `list --wisp`)
- [x] TASK-1-6.subtask-2 — Compaction logic (`grava compact --days N`, tombstone in `deletions` table)

## Implementation Notes
- Original spec called for `deletions.jsonl` flat file; implemented as `deletions` SQL table instead (already in schema from TASK-1-2), which is more consistent with the Dolt-first architecture.
- All 9 unit tests pass. Live E2E test confirmed against running Dolt instance.

