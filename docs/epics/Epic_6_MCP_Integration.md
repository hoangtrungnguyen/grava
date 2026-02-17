# Epic 6: MCP Integration and Agent Onboarding

**Goal:** Bridge the database to AI agents via Model Context Protocol and complete end-to-end testing.

**Success Criteria:**
- MCP server exposes typed JSON RPC tools
- Agents can create, update, query issues without raw SQL
- AGENTS.md documentation guides agent behavior
- Multi-agent concurrent testing passed
- Production-ready MVP deployed

## User Stories

### 6.1 MCP Server Wrapper Development
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

### 6.2 Typed JSON RPC Tool Definitions
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

### 6.3 Agent Context Injection (AGENTS.md)
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

### 6.4 Multi-Agent Concurrent Testing
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

### 6.5 End-to-End Integration Validation
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
