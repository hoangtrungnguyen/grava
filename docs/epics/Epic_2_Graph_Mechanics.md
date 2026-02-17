# Epic 2: Graph Mechanics and Blocked Issue Tracking

**Goal:** Build the dependency graph engine and "Ready Engine" that computes actionable work.

**Success Criteria:**
- Four semantic dependency types fully functional
- Ready Engine computes unblocked tasks in <10ms
- Priority-based sorting operational
- Graph traversal prevents circular dependencies
- Accurate blocked task identification

## User Stories

### 2.1 Semantic Dependency Implementation
**As a** developer  
**I want to** create and manage four types of issue dependencies  
**So that** the system understands different relationship semantics

**Acceptance Criteria:**
- `blocks` relationship enforces temporal prerequisites
- `parent-child` relationship establishes Epic → Task hierarchy
- `related` relationship creates informational links
- `discovered-from` relationship tracks issue provenance
- CLI command `grava dep add <from_id> <to_id> <type>` functional
- CLI command `grava dep remove <from_id> <to_id>` functional
- Validation prevents invalid relationship types

### 2.2 Ready Engine Core Algorithm
**As an** AI agent  
**I want to** query for actionable tasks that are not blocked  
**So that** I only work on tasks I can actually complete

**Acceptance Criteria:**
- SQL query filters issues by status = 'open'
- Topological analysis calculates indegree on `blocks` edges
- Issues with any open blockers excluded from ready list
- Performance: <10ms query execution time
- Returns empty array when no work is ready
- Handles graphs with 1000+ nodes efficiently

### 2.3 Priority-Based Task Sorting
**As an** AI agent  
**I want to** receive ready tasks sorted by priority  
**So that** I work on the most critical items first

**Acceptance Criteria:**
- Priority 0 (Critical) tasks returned first
- Priority 4 (Backlog) tasks returned last
- Ties broken by creation timestamp (oldest first)
- `grava ready` command accepts `--limit N` parameter
- `grava ready --priority 0` filters by specific priority level

### 2.4 Blocked Task Analysis
**As a** project manager  
**I want to** identify which issues are blocked and why  
**So that** I can understand and resolve bottlenecks

**Acceptance Criteria:**
- `grava blocked` command lists all blocked issues
- Output shows blocking issue IDs for each blocked task
- Supports `--depth` parameter to show transitive blockers
- Visual indicator of blocker chain length
- Export to JSON for external analysis

### 2.5 Cycle Detection and Validation
**As a** system administrator  
**I want to** prevent circular dependencies in the task graph  
**So that** the DAG structure remains valid

**Acceptance Criteria:**
- Tarjan's SCC algorithm detects cycles
- Dependency creation fails if it would create a cycle
- Error message clearly identifies the circular path
- Background job runs nightly to scan for cycles
- Cycle detection completes in <100ms for graphs with 10,000 nodes
