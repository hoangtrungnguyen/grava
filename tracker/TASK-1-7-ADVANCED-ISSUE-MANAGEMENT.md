---
issue: TASK-1-7-ADVANCED-ISSUE-MANAGEMENT
status: done
Description: Implement advanced issue management commands including comments, dependencies, and labels.
---

**Timestamp:** 2026-02-18 15:28:00
**Affected Modules:**
  - bin/
  - lib/commands/
  - pkg/core/
  - pkg/dolt/

---

## User Story
**As a** developer
**I want to** add comments, dependencies, and labels to issues
**So that** I can enrich the issue tracking data and manage complex relationships

## Acceptance Criteria
- [x] `grava comment <id> "text"` appends a comment to an issue
- [x] `grava dep <parent_id> <child_id>` creates a dependency relationship
- [x] `grava label <id> <label>` adds a label to an issue
- [x] `grava assign <id> <user>` assigns an issue to a user
- [x] All commands return proper exit codes and error messages
- [x] Help documentation available for all commands
