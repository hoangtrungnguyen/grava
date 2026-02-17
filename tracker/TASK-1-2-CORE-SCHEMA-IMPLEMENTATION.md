---
issue: TASK-1-2-CORE-SCHEMA-IMPLEMENTATION
status: done
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
- [x] `issues` table created with extended columns: `ephemeral` (BOOLEAN), `await_type` (VARCHAR), `await_id` (VARCHAR)
- [x] `dependencies` table supports 19 semantic types
- [x] `events` table created for audit trail (id, issue_id, event_type, actor, old_value, new_value, timestamp)
- [x] `child_counters` table created to track hierarchical ID suffixes
- [x] Foreign key constraints properly enforced
- [x] Default values and NOT NULL constraints working
- [x] JSON metadata field validated and functional

## Session Details - 2026-02-17
### Summary
Implemented the core database schema for Grava using Dolt. Created SQL definition files and applied them to the local Dolt instance. Verified schema constraints and data integrity.

### Decisions
1.  **Schema Definition (`scripts/schema/001_initial_schema.sql`)**:
    *   Defined strict types for `issues` status and type but used `VARCHAR` with `CHECK` constraints for flexibility and compatibility.
    *   Implemented `dependencies` table with composite primary key `(from_id, to_id, type)` to allow multiple relationship types between detailed tasks.
    *   Added `deletions` table as requested in Epic 1, though not explicitly in the task initial description, to support tombstoning.

2.  **Automation (`scripts/apply_schema.sh`)**:
    *   Created a script to apply schema changes reliably.
    *   Added `scripts/test_schema.sh` for smoke testing basic constraints.

### Artifacts Created
- `scripts/schema/001_initial_schema.sql`: Core schema definition.
- `scripts/apply_schema.sh`: Schema application script.
- `scripts/test_schema.sh`: Validation script.
- `.grava/dolt/`: Updated database state commit.

### Status
Task is **DONE**. Ready for Hierarchical ID Generator implementation (Task 1-3).
