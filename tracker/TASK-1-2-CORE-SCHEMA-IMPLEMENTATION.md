---
issue: TASK-1-2-CORE-SCHEMA-IMPLEMENTATION
status: todo
Description: Implement the issues, dependencies, and events tables to have a structured foundation for task tracking.
---

**Timestamp:** 2026-02-17 17:55:00
**Affected Modules:**
  - .grava/dolt/
  - lib/

---

## User Story
**As a** system architect  
**I want to** implement the issues, dependencies, and events tables  
**So that** we have a structured foundation for task tracking

## Acceptance Criteria
- [ ] `issues` table created with extended columns: `ephemeral` (BOOLEAN), `await_type` (VARCHAR), `await_id` (VARCHAR)
- [ ] `dependencies` table supports 19 semantic types
- [ ] `events` table created for audit trail (id, issue_id, event_type, actor, old_value, new_value, timestamp)
- [ ] `child_counters` table created to track hierarchical ID suffixes
- [ ] Foreign key constraints properly enforced
- [ ] Default values and NOT NULL constraints working
- [ ] JSON metadata field validated and functional
