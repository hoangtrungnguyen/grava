---
issue: TASK-1-6-EPHEMERAL-WISP-SUPPORT
status: todo
Description: Create temporary "scratchpad" issues and safely delete old ones to prevent project history pollution.
---

**Timestamp:** 2026-02-17 17:55:00
**Affected Modules:**
  - lib/
  - bin/
  - .grava/dolt/

---

## User Story
**As an** AI agent
**I want to** create temporary "scratchpad" issues and safely delete old ones
**So that** I don't pollute the project history with intermediate reasoning

## Acceptance Criteria
- [ ] `create --ephemeral` flag sets `ephemeral=true`
- [ ] Ephemeral issues are excluded from `issues.jsonl` export
- [ ] `grava list --wisp` filters for ephemeral issues
- [ ] `deletions.jsonl` manifest created to track deleted IDs
- [ ] Import logic checks `deletions.jsonl` to prevent resurrection of deleted issues
