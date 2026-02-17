---
issue: TASK-1-8-SEARCH-AND-MAINTENANCE
status: todo
Description: Implement search, discovery, and maintenance commands for the Grava system.
---

**Timestamp:** 2026-02-17 18:30:00
**Affected Modules:**
  - bin/
  - lib/commands/

---

## User Story
**As a** user
**I want to** search for issues and perform system maintenance
**So that** I can find relevant work and ensure the system remains healthy

## Acceptance Criteria
- [ ] `grava search "query"` finds issues matching the text
- [ ] `grava quick` lists high-priority or quick tasks
- [ ] `grava doctor` diagnoses and reports system issues
- [ ] `grava sync` synchronizes the local database with remote
- [ ] `grava compact` performs database maintenance and compression
- [ ] All commands return proper exit codes and error messages
- [ ] Help documentation available for all commands
