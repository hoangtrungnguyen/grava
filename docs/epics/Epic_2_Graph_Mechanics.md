,# Epic 2: Graph Mechanics and Blocked Issue Tracking

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
- 19 semantic dependency types implemented (blocks, waits-for, supersedes, etc.)
- `grava dep` supports extended types
- Gates (`await_type`) integrated into blocking logic
- Cycle detection handles all dependency types

### 2.2 Ready Engine Core Algorithm
**As an** AI agent  
**I want to** query for actionable tasks that are not blocked  
**So that** I only work on tasks I can actually complete

**Acceptance Criteria:**
- SQL query filters issues by status = 'open'
- Topological analysis calculates indegree on `blocks` edges
- Issues blocked by open "Gate" conditions (`await_type` != NULL) excluded
- Issues with `waits-for` dependencies de-prioritized but not strictly blocked
- Returns empty array when no work is ready
- Handles graphs with 10k+ nodes efficiently

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

### 2.6 Gate Evaluation Logic
**As an** AI agent
**I want to** wait for external events (PRs, Timers) before starting a task
**So that** I don't waste time polling or failing early

**Acceptance Criteria:**
- Ready engine checks `await_type='gh:pr'` status via GitHub API (mocked for MVP)
- Ready engine checks `await_type='timer'` against current timestamp
- Gates that are "closed" (condition met) unblock dependent tasks
- `grava gate list` shows pending external dependencies
