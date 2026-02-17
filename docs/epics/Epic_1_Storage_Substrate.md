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
- `issues` table created with all required columns (id, title, description, status, priority, issue_type, assignee, metadata)
- `dependencies` table created with semantic edge types (from_id, to_id, type)
- `events` table created for audit trail (id, issue_id, event_type, actor, old_value, new_value, timestamp)
- Foreign key constraints properly enforced
- Default values and NOT NULL constraints working
- JSON metadata field validated and functional

### 1.3 Hash-Based ID Generator
**As a** developer  
**I want to** generate cryptographic hash-based IDs for issues  
**So that** concurrent issue creation across branches never causes ID collisions

**Acceptance Criteria:**
- ID generator produces format `grava-XXXX` (alphanumeric)
- IDs are guaranteed unique across distributed environments
- Generator integrated into issue creation flow
- Performance: <1ms generation time
- Unit tests cover collision scenarios

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
- Schema migration scripts tested and versioned
