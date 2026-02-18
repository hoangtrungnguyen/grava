---
issue: TASK-1-4-BASIC-CRUD-CLI-TOOLS
status: done
Description: Create, read, update, and show issues via CLI to manually interact with the database during development.
---

**Timestamp:** 2026-02-18 10:20:00
**Affected Modules:**
  - pkg/cmd/
  - cmd/grava/

---

## User Story
**As a** developer  
**I want to** create, read, update, and show issues via CLI  
**So that** I can manually interact with the database during development

## Acceptance Criteria
- [x] `grava init` initializes a new repository
- [x] `grava create` command accepts title, description, type, priority
- [x] `grava show <id>` displays complete issue details
- [x] `grava update <id>` modifies specific fields without overwriting entire row
- [x] `grava close <id>` transitions issue status to closed (via update --status closed)
- [ ] `grava delete <id>` permanently removes an issue (not implemented, using update status)
- [x] `grava list` command displays all issues with filtering options
- [x] `grava subtask <parent_id>` creates hierarchical subtasks (e.g., `grava-123.1`)
- [x] All commands return proper exit codes and error messages
- [x] Help documentation available for all commands
- [x] Unit tests implemented for all commands
