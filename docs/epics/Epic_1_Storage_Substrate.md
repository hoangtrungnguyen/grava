# Epic 1: Storage Substrate and Schema Implementation

**Goal:** Establish the version-controlled Dolt database foundation with core schema and basic CRUD operations.

**Success Criteria:**
- Functional Dolt database with proper version control
- Complete schema implementation with enforced constraints
- Hash-based ID generation working without collisions
- Basic CLI tools for manual database operations
- Comprehensive test coverage of schema integrity

## User Stories

### 1.1 Dolt Database Initialization
**As a** developer  
**I want to** initialize a Dolt database in the project workspace  
**So that** we have a version-controlled storage substrate for issues

**Acceptance Criteria:**
- Dolt installation and setup documentation complete
- Database initialization scripts created
- `.grava/dolt/` directory structure established
- Basic `dolt` commands (init, status, log) functional
- Documentation includes rollback and recovery procedures

### 1.2 Core Schema Implementation
**As a** system architect  
**I want to** implement the issues, dependencies, and events tables  
**So that** we have a structured foundation for task tracking

**Acceptance Criteria:**
- `issues` table created with extended columns: `ephemeral` (BOOLEAN), `await_type` (VARCHAR), `await_id` (VARCHAR)
- `dependencies` table supports 19 semantic types
- `events` table created for audit trail (id, issue_id, event_type, actor, old_value, new_value, timestamp)
- `child_counters` table created to track hierarchical ID suffixes
- Foreign key constraints properly enforced
- Default values and NOT NULL constraints working
- JSON metadata field validated and functional

### 1.3 Hierarchical ID Generator
   **As a** developer
   **I want to** generate atomic, hierarchical IDs (e.g., `grava-a1b2.1`)
   **So that** I can break down tasks recursively without ID collisions

   **Acceptance Criteria:**
   - Generator produces `grava-XXXX` (hash-based) for top-level issues
   - Generator supports atomic increment for child issues (`.1`, `.2`) via `child_counters` table
   - IDs are guaranteed unique across distributed environments
   - Generator integrated into issue creation flow
   - Performance: <1ms generation time
   - Unit tests cover collision scenarios and hierarchy depth (parent.child.grandchild)

### 1.4 Basic CRUD CLI Tools
**As a** developer  
**I want to** create, read, update, and show issues via CLI  
**So that** I can manually interact with the database during development

**Acceptance Criteria:**
- `grava create` command accepts title, description, type, priority
- `grava show <id>` displays complete issue details
- `grava update <id>` modifies specific fields without overwriting entire row
- `grava list` command displays all issues with filtering options
- All commands return proper exit codes and error messages
- Help documentation available for all commands

### 1.5 Schema Validation and Testing
**As a** QA engineer  
**I want to** comprehensive test coverage of the schema  
**So that** data integrity is guaranteed

**Acceptance Criteria:**
- Unit tests for all table constraints
- Integration tests for foreign key relationships
- Edge case testing (NULL values, boundary conditions)
- Performance benchmarks documented
- Performance benchmarks documented
- Schema migration scripts tested and versioned

### 1.6 Ephemeral "Wisp" Support and Deletion Manifests
**As an** AI agent
**I want to** create temporary "scratchpad" issues and safely delete old ones
**So that** I don't pollute the project history with intermediate reasoning

**Acceptance Criteria:**
- `create --ephemeral` flag sets `ephemeral=true`
- Ephemeral issues are excluded from `issues.jsonl` export
- `grava list --wisp` filters for ephemeral issues
- `deletions.jsonl` manifest created to track deleted IDs
- Import logic checks `deletions.jsonl` to prevent resurrection of deleted issues
