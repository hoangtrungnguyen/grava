---
issue: TASK-1-4-BASIC-CRUD-CLI-TOOLS
status: todo
Description: Create, read, update, and show issues via CLI to manually interact with the database during development.
---

**Timestamp:** 2026-02-17 17:55:00
**Affected Modules:**
  - bin/
  - lib/commands/

---

## User Story
**As a** developer  
**I want to** create, read, update, and show issues via CLI  
**So that** I can manually interact with the database during development

## Acceptance Criteria
- [ ] `grava init` initializes a new repository
- [ ] `grava create` command accepts title, description, type, priority
- [ ] `grava show <id>` displays complete issue details
- [ ] `grava update <id>` modifies specific fields without overwriting entire row
- [ ] `grava close <id>` transitions issue status to closed
- [ ] `grava delete <id>` permanently removes an issue (or tombstones)
- [ ] `grava list` command displays all issues with filtering options
- [ ] All commands return proper exit codes and error messages
- [ ] Help documentation available for all commands
