# Agent Issue Tracker - MVP Epics

Project name: Grava

## Overview

This document defines the epics for building the Minimum Viable Product (MVP) of a Dolt-backed AI Agent Issue Tracker. The system enables autonomous AI agents to coordinate work through a version-controlled, graph-aware database with deterministic task selection and multi-agent synchronization.

---

## Epic 1: Storage Substrate and Schema Implementation

**Goal:** Establish the version-controlled Dolt database foundation with core schema and basic CRUD operations.

**Success Criteria:**
- Functional Dolt database with proper version control
- Complete schema implementation with enforced constraints
- Hash-based ID generation working without collisions
- Basic CLI tools for manual database operations
- Comprehensive test coverage of schema integrity

### User Stories

#### 1.1 Dolt Database Initialization
**As a** developer  
**I want to** initialize a Dolt database in the project workspace  
**So that** we have a version-controlled storage substrate for issues

**Acceptance Criteria:**
- Dolt installation and setup documentation complete
- Database initialization scripts created
- `.grava/dolt/` directory structure established
- Basic `dolt` commands (init, status, log) functional
- Documentation includes rollback and recovery procedures

#### 1.2 Core Schema Implementation
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

#### 1.3 Hash-Based ID Generator
**As a** developer  
**I want to** generate cryptographic hash-based IDs for issues  
**So that** concurrent issue creation across branches never causes ID collisions

**Acceptance Criteria:**
- ID generator produces format `grava-XXXX` (alphanumeric)
- IDs are guaranteed unique across distributed environments
- Generator integrated into issue creation flow
- Performance: <1ms generation time
- Unit tests cover collision scenarios

#### 1.4 Basic CRUD CLI Tools
**As a** developer  
**I want to** create, read, update, and show issues via CLI  
**So that** I can manually interact with the database during development

**Acceptance Criteria:**
- `create` command accepts title, description, type, priority
- `show <id>` displays complete issue details
- `update <id>` modifies specific fields without overwriting entire row
- `list` command displays all issues with filtering options
- All commands return proper exit codes and error messages
- Help documentation available for all commands

#### 1.5 Schema Validation and Testing
**As a** QA engineer  
**I want to** comprehensive test coverage of the schema  
**So that** data integrity is guaranteed

**Acceptance Criteria:**
- Unit tests for all table constraints
- Integration tests for foreign key relationships
- Edge case testing (NULL values, boundary conditions)
- Performance benchmarks documented
- Schema migration scripts tested and versioned

---

## Epic 2: Graph Mechanics and Blocked Issue Tracking

**Goal:** Build the dependency graph engine and "Ready Engine" that computes actionable work.

**Success Criteria:**
- Four semantic dependency types fully functional
- Ready Engine computes unblocked tasks in <10ms
- Priority-based sorting operational
- Graph traversal prevents circular dependencies
- Accurate blocked task identification

### User Stories

#### 2.1 Semantic Dependency Implementation
**As a** developer  
**I want to** create and manage four types of issue dependencies  
**So that** the system understands different relationship semantics

**Acceptance Criteria:**
- `blocks` relationship enforces temporal prerequisites
- `parent-child` relationship establishes Epic → Task hierarchy
- `related` relationship creates informational links
- `discovered-from` relationship tracks issue provenance
- CLI command `dep add <from_id> <to_id> <type>` functional
- CLI command `dep remove <from_id> <to_id>` functional
- Validation prevents invalid relationship types

#### 2.2 Ready Engine Core Algorithm
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

#### 2.3 Priority-Based Task Sorting
**As an** AI agent  
**I want to** receive ready tasks sorted by priority  
**So that** I work on the most critical items first

**Acceptance Criteria:**
- Priority 0 (Critical) tasks returned first
- Priority 4 (Backlog) tasks returned last
- Ties broken by creation timestamp (oldest first)
- `ready` command accepts `--limit N` parameter
- `ready --priority 0` filters by specific priority level

#### 2.4 Blocked Task Analysis
**As a** project manager  
**I want to** identify which issues are blocked and why  
**So that** I can understand and resolve bottlenecks

**Acceptance Criteria:**
- `blocked` command lists all blocked issues
- Output shows blocking issue IDs for each blocked task
- Supports `--depth` parameter to show transitive blockers
- Visual indicator of blocker chain length
- Export to JSON for external analysis

#### 2.5 Cycle Detection and Validation
**As a** system administrator  
**I want to** prevent circular dependencies in the task graph  
**So that** the DAG structure remains valid

**Acceptance Criteria:**
- Tarjan's SCC algorithm detects cycles
- Dependency creation fails if it would create a cycle
- Error message clearly identifies the circular path
- Background job runs nightly to scan for cycles
- Cycle detection completes in <100ms for graphs with 10,000 nodes

---

## Epic 3: Dedicated Sync Server and Daemon Orchestration

**Goal:** Deploy centralized synchronization infrastructure for multi-agent coordination.

**Success Criteria:**
- Central Dolt SQL server operational
- Workspace daemon with batching logic functional
- Automated pull-merge-commit-push cycle working
- Git hooks export to issues.jsonl
- Multiple clients can sync without conflicts

### User Stories

#### 3.1 Central Dolt SQL Server Deployment
**As a** DevOps engineer  
**I want to** deploy a centralized Dolt SQL server  
**So that** multiple agents can synchronize to a single source of truth

**Acceptance Criteria:**
- `dolt sql-server` deployed on cloud infrastructure
- RemotesAPI endpoint exposed on port 50051
- Server configured with appropriate memory/CPU resources
- Monitoring and health checks implemented
- Backup strategy documented and automated
- Connection pooling configured for 50+ concurrent clients

#### 3.2 Workspace Daemon Engineering
**As a** developer  
**I want to** run a background daemon in each workspace  
**So that** synchronization happens automatically without manual intervention

**Acceptance Criteria:**
- Daemon communicates via Unix domain socket (`.grava/bd.sock`)
- Daemon maintains persistent connection to local Dolt replica
- Process management (start, stop, restart, status) implemented
- Daemon auto-starts on workspace initialization
- Graceful shutdown preserves uncommitted changes
- Daemon logs to `.grava/daemon.log`

#### 3.3 Debouncing and Batching Logic
**As a** developer  
**I want to** batch rapid local mutations before syncing  
**So that** network traffic is minimized and performance is optimized

**Acceptance Criteria:**
- 5-second debounce window after last local write
- Multiple rapid updates batched into single sync operation
- Configurable debounce interval via config file
- Manual sync available via `bd sync --force`
- Batch size limits prevent memory overflow

#### 3.4 Automated Synchronization Protocol
**As an** AI agent  
**I want to** automatic state synchronization with the central server  
**So that** my local view of issues is always up-to-date

**Acceptance Criteria:**
- `dolt pull` retrieves latest commits from server
- Cell-level three-way merge handles concurrent updates
- Conflict resolution strategy applied (newest timestamp wins)
- `dolt commit` creates immutable local snapshot
- `dolt push` sends local commits to server
- Full sync cycle completes in <2 seconds for typical workloads

#### 3.5 Git Hook Integration and JSONL Export
**As a** developer  
**I want to** export database state to human-readable format  
**So that** I can review issues in Git and have fallback recovery

**Acceptance Criteria:**
- Pre-commit hook triggers JSONL export
- `issues.jsonl` contains complete database dump
- Post-merge hook updates local database from JSONL if needed
- Export includes all tables (issues, dependencies, events)
- JSONL format validated and parseable
- Git commit includes both .dolt/ and issues.jsonl changes

---

## Epic 4: Security Implementation and Access Control

**Goal:** Implement zero-trust security and granular permissions before exposing to autonomous agents.

**Success Criteria:**
- mTLS authentication enforced on server
- RBAC configured with human and agent roles
- Agents cannot execute destructive operations
- Complete audit trail of all mutations
- Failed authentication logged and alerted

### User Stories

#### 4.1 Mutual TLS Configuration
**As a** security engineer  
**I want to** enforce mutual TLS on the Dolt server  
**So that** only authorized clients can connect

**Acceptance Criteria:**
- Server requires client certificates (listener.require_client_cert: true)
- CA certificate authority set up for signing client certs
- Client certificates distributed to authorized agents
- Unauthorized connection attempts rejected with clear error
- Certificate rotation procedure documented
- TLS 1.3 minimum version enforced

#### 4.2 Role-Based Access Control (RBAC)
**As a** system administrator  
**I want to** define granular database permissions  
**So that** agents cannot accidentally destroy data

**Acceptance Criteria:**
- `human_admin` role with full schema control (DDL/DML)
- `ai_agent` role with restricted permissions (SELECT, INSERT, UPDATE only)
- `ai_agent` role explicitly denied DELETE, DROP, TRUNCATE
- `read_only` role for monitoring and reporting tools
- Roles enforced at database connection level
- Role assignment tracked in privileges.db

#### 4.3 Immutable Audit Logging
**As a** compliance officer  
**I want to** log every database mutation with actor identity  
**So that** I can trace who made what changes and when

**Acceptance Criteria:**
- Events table captures all INSERT/UPDATE/DELETE operations
- Actor field populated with authenticated identity (human or agent ID)
- Old and new values stored as JSON for complete diff
- Audit log is append-only (DELETE disabled)
- Timestamp precision to millisecond level
- Query interface for audit trail search

#### 4.4 Privilege Enforcement Testing
**As a** QA engineer  
**I want to** verify that permission boundaries are respected  
**So that** security cannot be bypassed

**Acceptance Criteria:**
- Test suite verifies agent role cannot DELETE
- Test suite verifies agent role cannot DROP tables
- Test suite verifies agent role cannot ALTER schema
- Test suite verifies unauthorized certificates are rejected
- Penetration testing completed with clean results
- Security vulnerability scan passed

#### 4.5 Incident Response and Rollback
**As a** system administrator  
**I want to** quickly rollback malicious or erroneous changes  
**So that** data integrity can be restored immediately

**Acceptance Criteria:**
- `DOLT_REVERT()` procedure tested and documented
- Rollback can target specific commit hashes
- Rollback preserves audit trail of the revert itself
- Alert system notifies admins of suspicious activity patterns
- Recovery time objective (RTO) <5 minutes documented

---

## Epic 5: MCP Integration and Agent Onboarding

**Goal:** Bridge the database to AI agents via Model Context Protocol and complete end-to-end testing.

**Success Criteria:**
- MCP server exposes typed JSON RPC tools
- Agents can create, update, query issues without raw SQL
- AGENTS.md documentation guides agent behavior
- Multi-agent concurrent testing passed
- Production-ready MVP deployed

### User Stories

#### 5.1 MCP Server Wrapper Development
**As a** developer  
**I want to** expose database operations as MCP tools  
**So that** AI agents can interact without writing SQL

**Acceptance Criteria:**
- MCP server implements `init`, `create`, `update`, `ready`, `dep` tools
- All tools accept and return strictly typed JSON
- Error responses include descriptive messages and error codes
- Server handles concurrent requests from multiple agents
- Server implements rate limiting to prevent abuse
- API documentation generated from schema

#### 5.2 Typed JSON RPC Tool Definitions
**As an** AI agent  
**I want to** invoke tools with clear input/output schemas  
**So that** I can reliably interact with the issue tracker

**Acceptance Criteria:**
- `create(title, description, priority, type)` → returns issue_id
- `update(issue_id, field, value)` → returns updated issue
- `ready(limit, priority_filter)` → returns array of ready issues
- `dep(from_id, to_id, dep_type)` → creates dependency
- `show(issue_id)` → returns complete issue with dependencies
- JSON schemas validated on every request/response

#### 5.3 Agent Context Injection (AGENTS.md)
**As a** prompt engineer  
**I want to** provide clear instructions to AI agents  
**So that** they use the issue tracker correctly

**Acceptance Criteria:**
- AGENTS.md template created with workflow guidelines
- Instructions mandate using `ready` for task discovery
- Instructions require status updates via `update` tool
- Instructions enforce dependency linking for discovered work
- Examples of correct tool usage included
- Anti-patterns and common mistakes documented

#### 5.4 Multi-Agent Concurrent Testing
**As a** QA engineer  
**I want to** test multiple agents working simultaneously  
**So that** I can verify the system handles concurrency correctly

**Acceptance Criteria:**
- Test harness spawns 5+ concurrent agent processes
- Agents create, update, and claim issues simultaneously
- No merge conflicts or data corruption observed
- Ready Engine correctly handles concurrent task claims
- Cell-level merge tested with overlapping updates
- Performance under load documented (throughput, latency)

#### 5.5 End-to-End Integration Validation
**As a** product manager  
**I want to** validate the complete agent workflow  
**So that** the MVP is production-ready

**Acceptance Criteria:**
- Agent queries `ready`, receives unblocked task
- Agent claims task by updating assignee to its ID
- Agent updates status to `in_progress`
- Agent completes work, updates status to `closed`
- Agent discovers new bug, creates issue with `discovered-from` link
- Dependency graph updates correctly trigger new ready tasks
- All operations sync to central server within 10 seconds

---

## Epic 6 (Optional): Advanced Graph Analytics

**Goal:** Implement sophisticated graph analysis for project optimization (post-MVP enhancement).

**Success Criteria:**
- PageRank identifies foundational blocker tasks
- Betweenness centrality detects bottlenecks
- Critical path analysis calculates longest dependency chains

### User Stories

#### 6.1 PageRank Implementation
**As a** project manager  
**I want to** identify which tasks are blocking the most downstream work  
**So that** I can prioritize foundational issues

**Acceptance Criteria:**
- PageRank algorithm runs on dependency graph
- Results cached and refreshed every 30 minutes
- `analytics pagerank --top 10` shows most critical blockers
- Visual output shows PageRank score distribution

#### 6.2 Critical Path Analysis
**As a** project manager  
**I want to** calculate the longest chain of dependencies  
**So that** I understand the minimum project completion time

**Acceptance Criteria:**
- Critical path algorithm identifies longest unbrokable chain
- Path displayed with issue IDs and estimated durations
- Tasks on critical path flagged in `ready` output
- Critical path recomputed on dependency changes

#### 6.3 Bottleneck Detection
**As a** team lead  
**I want to** identify tasks that bridge major work clusters  
**So that** I can assign senior engineers to critical junctures

**Acceptance Criteria:**
- Betweenness centrality calculated for all nodes
- High-centrality tasks flagged as bottlenecks
- Visualization shows graph with bottlenecks highlighted
- Alerts when new bottleneck tasks are created

---

## Implementation Timeline

### Phase 1: Foundation (Weeks 1-2)
- Epic 1: Storage Substrate and Schema Implementation

### Phase 2: Intelligence (Weeks 3-4)
- Epic 2: Graph Mechanics and Blocked Issue Tracking

### Phase 3: Distribution (Weeks 5-6)
- Epic 3: Dedicated Sync Server and Daemon Orchestration

### Phase 4: Security (Week 7)
- Epic 4: Security Implementation and Access Control

### Phase 5: Integration (Week 8)
- Epic 5: MCP Integration and Agent Onboarding

### Phase 6: Enhancement (Post-MVP, Optional)
- Epic 6: Advanced Graph Analytics

---

## Definition of Done

An epic is considered complete when:
1. All user stories within the epic are implemented
2. Acceptance criteria for each story are met
3. Unit and integration tests achieve >85% coverage
4. Code review approved by at least 2 team members
5. Documentation updated (README, API docs, AGENTS.md)
6. Performance benchmarks meet stated requirements
7. Security review passed (for Epics 4-5)
8. Demo completed to stakeholders

---

## Risk Mitigation

| Risk | Impact | Mitigation Strategy |
|------|--------|-------------------|
| Dolt performance degradation at scale | High | Implement caching layer, optimize queries, conduct load testing early |
| Cell-level merge conflicts in practice | Medium | Define clear conflict resolution strategies, extensive testing with concurrent agents |
| MCP integration breaking changes | Medium | Pin MCP server version, maintain compatibility layer |
| Certificate management complexity | Low | Automate cert generation, clear rotation procedures |
| Agent misuse of tracker | High | Strict RBAC, comprehensive audit logging, automated anomaly detection |

---

## Success Metrics

### Technical Metrics
- Ready Engine query time: <10ms (p95)
- Sync cycle completion: <2 seconds (p95)
- System uptime: >99.9%
- Concurrent agent capacity: 50+ agents
- Zero data loss incidents

### Product Metrics
- Agent adoption rate: 80% of agents use tracker within 2 weeks
- Task completion rate improvement: 30% increase vs. baseline
- Blocking issue resolution time: 40% reduction
- Agent confusion/error rate: <5% of operations

---

## Appendix: Epic Dependencies

```
Epic 1 (Foundation)
    ↓
Epic 2 (Graph Mechanics) ← requires Epic 1 schema
    ↓
Epic 3 (Sync Server) ← requires Epic 1 & 2 core functionality
    ↓
Epic 4 (Security) ← requires Epic 3 server deployment
    ↓
Epic 5 (MCP Integration) ← requires Epic 1-4 complete
    ↓
Epic 6 (Analytics) ← optional, requires Epic 2 graph engine
```

