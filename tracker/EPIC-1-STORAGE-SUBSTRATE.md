---
issue: EPIC-1-STORAGE-SUBSTRATE
status: todo
Description: Establish the version-controlled Dolt database foundation with core schema and basic CRUD operations.
---

**Timestamp:** 2026-02-17 17:55:00
**Affected Modules:**
  - .grava/dolt/
  - lib/
  - bin/
  - tests/

---

## Task Details

### 1-1 Dolt Database Initialization
- [x] Install Dolt and setup documentation
- [x] Create database initialization scripts
- [x] Establish `.grava/dolt/` directory structure
- [x] Verify basic `dolt` commands (init, status, log)
- [x] Create documentation for rollback and recovery procedures

### 1-2 Core Schema Implementation
- [x] Create `issues` table with extended columns: `ephemeral`, `await_type`, `await_id`
- [x] Create `dependencies` table supporting 19 semantic types
- [x] Create `events` table for audit trail
- [x] Create `child_counters` table for hierarchical ID suffixes
- [x] Create `deletions` table for tombstone tracking
- [x] Enforce foreign key constraints
- [x] Implement default values and NOT NULL constraints
- [x] Validate JSON metadata field functionality

### 1-3 Hierarchical ID Generator
- [x] Implement generator for `grava-XXXX` (hash-based) top-level issues
- [x] Implement atomic increment for child issues (`.1`, `.2`) via `child_counters` table
- [x] Ensure ID uniqueness across distributed environments
- [x] Integrate generator into issue creation flow
- [x] Benchmark generation time (<1ms)
- [x] Create unit tests for collision scenarios and hierarchy depth

### 1-4 Basic CRUD CLI Tools
- [ ] Implement `grava create` command (title, description, type, priority)
- [ ] Implement `grava show <id>` command
- [ ] Implement `grava update <id>` command
- [ ] Implement `grava list` command with filtering
- [ ] Ensure proper exit codes and error messages
- [ ] Generate help documentation for all commands

### 1-5 Schema Validation and Testing
- [ ] Create unit tests for table constraints
- [ ] Create integration tests for foreign key relationships
- [ ] Create edge case tests (NULL values, boundary conditions)
- [ ] Document performance benchmarks
- [ ] Create and test schema migration scripts

### 1-6 Ephemeral "Wisp" Support and Deletion Manifests
- [ ] Implement `create --ephemeral` flag
- [ ] Exclude ephemeral issues from `issues.jsonl` export
- [ ] Implement `grava list --wisp` filter
- [ ] Create `deletions.jsonl` manifest logic
- [ ] Update import logic to check `deletions.jsonl`

