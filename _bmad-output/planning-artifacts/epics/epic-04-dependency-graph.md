# Epic 4: Dependency Graph — Context-Aware Work Discovery

**Status:** Planned
**Matrix Score:** 4.40
**FRs covered:** FR8, FR9, FR10, FR11, FR12, FR13

## Goal

Agents and developers can establish directional blocking relationships between issues, query the ready queue (top-priority tasks with no blockers), find what's blocking a specific task, visualize or traverse the full dependency graph, filter and search issues, and view aggregated workspace metrics.

## Commands Delivered

| Command | FR | Description |
|---------|----|-------------|
| `grava dep --add <from> <to>` | FR8 | Establish directional blocking relationship |
| `grava dep --remove <from> <to>` | FR8 | Remove blocking relationship |
| `grava ready` | FR9 | Top-priority tasks with no active blockers |
| `grava blocked <id>` | FR10 | What upstream issues are blocking this task |
| `grava graph [--format=dot\|json\|ascii]` | FR11 | Visualize or traverse full dependency graph |
| `grava list [--filter] [--json]` | FR12 | Filter and list issues |
| `grava search <query>` | FR12 | Full-text search across issues |
| `grava show <id> [--json]` | FR12 | Detailed view of a single issue |
| `grava stats` | FR13 | Aggregated workspace performance and status metrics |

## Dependencies

- Epic 1 complete (named functions, pkg/cmd reorg, testutil)
- Epic 2 complete (issues must exist to have dependencies)

## Key Implementation Notes

- `grava dep --add` uses `WithDeadlockRetry` for concurrent dependency writes (ADR-H3)
- Circular dependency detection: reject cycles at write time with structured error
- `grava ready`: filters `status NOT IN ('done', 'archived')` AND no active blockers; ordered by priority
- NFR1 (<100ms for `grava ready`/`grava list`) is **deferred to Phase 2** — direct DB hydration per invocation in Phase 1 (ADR-002)
- Graph output: `--format=dot` for Graphviz; `--format=json` for machine consumption; `--format=ascii` for terminal
- `grava stats`: total issues by status, average cycle time, open blockers count, stale `in_progress` count

## NFR Validation Points

| NFR | Validation |
|-----|------------|
| NFR1 | Deferred Phase 2 — document expected latency at 1K issues in Phase 1 |
| NFR5 | `--json` outputs for `list`, `show`, `ready`, `stats` validated against schema |

## Stories

### Story 4.1: Establish and Remove Dependency Relationships

As a developer or agent,
I want to create directional "blocking" relationships between issues,
So that the system knows which tasks must complete before others can start.

**Acceptance Criteria:**

**Given** issues `abc123def456` (blocker) and `def456abc123` (blocked) both exist
**When** I run `grava dep --add abc123def456 def456abc123`
**Then** a row is inserted in the `dependencies` table: `{blocker_id: "abc123def456", blocked_id: "def456abc123"}`
**And** the operation uses `WithDeadlockRetry` to handle concurrent dep writes (ADR-H3)
**And** `grava dep --remove abc123def456 def456abc123` deletes the dependency row
**And** attempting to create a circular dependency (A blocks B, B blocks A) returns `{"error": {"code": "CIRCULAR_DEPENDENCY", "message": "This dependency would create a cycle"}}`
**And** `grava dep --add` on a non-existent issue ID returns `{"error": {"code": "ISSUE_NOT_FOUND", ...}}`

---

### Story 4.2: Query the Ready Queue

As an agent,
I want to query the top-priority tasks with no active blockers,
So that I can immediately identify what work is actionable without manual triage.

**Acceptance Criteria:**

**Given** a workspace with 10 issues, some blocked and some not
**When** I run `grava ready`
**Then** only issues with `status NOT IN ('done', 'archived')` AND no active (non-done) blockers are returned
**And** results are ordered by priority (high → medium → low) then by `created_at` ascending
**And** `grava ready --json` returns a JSON array conforming to NFR5 schema
**And** `grava ready --limit 5` returns at most 5 issues
**And** if all open issues are blocked, returns empty array `[]` with exit code 0 (not an error)

---

### Story 4.3: Query Blockers for a Specific Issue

As a developer or agent,
I want to see exactly what upstream issues are blocking a specific task,
So that I can resolve blockers in the right order to unblock work.

**Acceptance Criteria:**

**Given** issue `def456abc123` is blocked by `abc123def456` and `xyz789abc123`
**When** I run `grava blocked def456abc123`
**Then** a list is returned showing each blocker: `{id, title, status, assignee}` for `abc123def456` and `xyz789abc123`
**And** `grava blocked def456abc123 --json` returns a JSON array of blocker objects conforming to NFR5 schema
**And** if the issue has no blockers, returns empty array `[]`
**And** `grava blocked` on a non-existent issue returns `{"error": {"code": "ISSUE_NOT_FOUND", ...}}`

---

### Story 4.4: Visualize the Full Dependency Graph

As a developer or agent,
I want to visualize or traverse the overarching dependency structure,
So that I can understand the full project topology and identify critical paths.

**Acceptance Criteria:**

**Given** a workspace with interconnected dependency relationships
**When** I run `grava graph`
**Then** ASCII-art dependency tree is printed to stdout showing all non-archived issues and their relationships
**And** `grava graph --format=dot` outputs a Graphviz DOT-format graph for external rendering
**And** `grava graph --format=json` outputs a JSON adjacency list: `{"nodes": [...], "edges": [...]}` conforming to NFR5 schema
**And** `grava graph --root abc123def456` outputs only the subgraph reachable from that issue
**And** isolated issues (no deps) appear as standalone nodes, not omitted

---

### Story 4.5: Filter, Search and View Issues

As a developer or agent,
I want to filter the issue list, search by keyword, and view full details of a single issue,
So that I can quickly locate and inspect any work item in the tracker.

**Acceptance Criteria:**

**Given** a workspace with multiple issues across statuses, priorities, and assignees
**When** I run `grava list --status in_progress --assignee agent-01`
**Then** only issues matching all provided filters are returned
**And** `grava search "login"` performs a case-insensitive full-text search across `title`, `description`, and `comments` fields
**And** `grava show abc123def456` displays full issue detail: title, status, priority, assignee, labels, comments, subtasks, and dependency info
**And** `grava show abc123def456 --json` returns the complete issue object conforming to NFR5 schema
**And** `grava list --json` returns a JSON array of issue summaries (id, title, status, priority, assignee)
**And** all commands return empty results (not errors) when no issues match the filter

---

### Story 4.6: View Workspace Metrics and Stats

As a developer,
I want to view aggregated workspace performance and status metrics,
So that I can assess project health and identify bottlenecks at a glance.

**Acceptance Criteria:**

**Given** a workspace with issues in various states
**When** I run `grava stats`
**Then** output includes: total issues by status (open/in_progress/paused/done/archived), count of blocked issues, count of stale `in_progress` issues (no heartbeat > 1h), average cycle time for done issues (created_at → done_at)
**And** `grava stats --json` returns a structured JSON object conforming to NFR5 schema
**And** `grava stats` completes in <100ms at 1K issues (Phase 1 baseline; NFR1 deferred to Phase 2 at 10K)
