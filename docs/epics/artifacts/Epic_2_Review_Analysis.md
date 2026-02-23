# Epic 2: Graph Mechanics - Comprehensive Review and Analysis

**Review Date:** 2026-02-20
**Epic:** Epic 2: Graph Mechanics and Blocked Issue Tracking
**Reviewer:** Claude Code Analysis

---

## Executive Summary

This review identifies critical gaps in Epic 2's specification, provides industry best practices from research, and recommends concrete improvements for implementing a production-ready dependency graph system. Key findings include missing algorithm specifications, incomplete dependency type taxonomy, and absence of priority inversion handling.

---

## Missing Pieces

### 1. **Implementation Details Missing**

#### Algorithm Selection Not Specified
- **Gap:** No clear choice between DFS vs. BFS (Kahn's Algorithm) for topological sorting
- **Gap:** No specification of which cycle detection algorithm to use (Tarjan's SCC is mentioned but implementation approach unclear)
- **Gap:** Performance target of <10ms for Ready Engine and <100ms for cycle detection needs implementation strategy
- **Impact:** Development team will need to research and decide independently, risking inconsistent approaches

#### Data Structure Specifications
- **Gap:** No mention of in-memory graph caching strategy
- **Gap:** Missing details on how to handle graph updates and cache invalidation
- **Gap:** No specification for priority queue implementation
- **Impact:** Potential performance bottlenecks and cache coherency issues

### 2. **Incomplete Dependency Type System**

#### Inconsistency in Requirements
- **Gap:** Line 20 mentions "19 semantic dependency types" but only lists examples
- **Gap:** Line 31 in Epic 1 mentions "19 semantic types" but current implementation (dep.go:60) only has 4 types: `blocks, relates-to, duplicates, parent-child`
- **Impact:** Unclear scope, potential rework needed

#### Gate System Incomplete
User Story 2.6 mentions gates but lacks:
- Polling frequency for gate conditions
- Timeout/expiration handling for gates
- Error handling when GitHub API is unavailable
- Webhook vs. polling strategy decision
- **Impact:** Gate implementation may be unreliable or inefficient

### 3. **Performance and Scalability Gaps**

#### Missing Optimization Strategies
- **Gap:** No mention of incremental graph updates vs. full recomputation
- **Gap:** No caching strategy for frequently-queried ready tasks
- **Gap:** No discussion of batch operations for large dependency changes
- **Gap:** Performance target "10k+ nodes" needs specific optimization techniques
- **Impact:** May not scale to stated performance goals

#### Database Query Optimization
- **Gap:** No index strategy specified for dependencies table beyond `idx_to_id`
- **Gap:** Missing compound indexes for common queries (status + priority, blocking chains)
- **Gap:** No mention of materialized views or computed columns for indegree
- **Impact:** Slow queries as graph grows, especially for Ready Engine

### 4. **Transitive Dependency Handling**

#### Depth Parameter Implementation
- **Gap:** User Story 2.4 mentions `--depth` parameter but no algorithm specified
- **Gap:** No discussion of performance implications for deep transitive queries
- **Gap:** Missing BFS vs. DFS choice for transitive traversal
- **Impact:** Inconsistent or inefficient transitive dependency resolution

### 5. **Priority Inversion and Starvation**

#### Critical Gap
- **Gap:** No handling of priority inversion (when high-priority task is blocked by low-priority task)
- **Gap:** No mention of priority inheritance or boosting
- **Gap:** No starvation prevention for low-priority tasks
- **Gap:** Missing aging mechanism to prevent indefinite postponement
- **Impact:** High-priority work may be delayed indefinitely; low-priority work may never execute

### 6. **Error Handling and Edge Cases**

#### Missing Scenarios
- What happens when a dependency references a non-existent issue?
- How to handle deleted issues that are part of dependency chains?
- Concurrent dependency creation and potential race conditions
- Handling of orphaned tasks when blockers are deleted
- **Impact:** Undefined behavior in production, potential data inconsistencies

### 7. **Monitoring and Observability**

#### Missing Instrumentation
- **Gap:** No metrics for graph computation performance
- **Gap:** No logging strategy for debugging blocked states
- **Gap:** No visualization tools for dependency graphs
- **Gap:** Missing alerts for problematic graph states (too many blocked tasks, deep chains)
- **Impact:** Difficult to diagnose issues in production, no visibility into system health

---

## Recommended Improvements

### 1. **Algorithm Specification**

Add this section to Epic 2:

#### Implementation Strategy

**Cycle Detection:**
- Use **Kahn's Algorithm (BFS-based)** for primary cycle detection
  - **Rationale:** Better error reporting, easier to parallelize, more maintainable
  - **Implementation:** Track indegree, process zero-indegree nodes iteratively
  - **Fallback:** Use Tarjan's SCC for existing graphs with suspected cycles
  - **Performance:** O(V + E) time complexity, O(V) space

**Ready Engine:**
- Use **modified Kahn's Algorithm with priority queue**
  - Maintain indegree map in memory (refreshed on dependency changes)
  - Use min-heap ordered by (priority, created_at) for O(log n) extraction
  - Filter by gate conditions as final step
  - **Implementation:** Go's `container/heap` package

**Code Structure:**
```go
// Pseudocode
type ReadyEngine struct {
    indegree map[string]int
    adjList  map[string][]string
    gates    map[string]GateCondition
}

func (re *ReadyEngine) ComputeReady() []Issue {
    // 1. Load open issues
    // 2. Filter by indegree == 0
    // 3. Filter by gate conditions
    // 4. Sort by priority, created_at
    // 5. Return top N
}
```

### 2. **Complete Dependency Type Taxonomy**

Propose this standardized set of 19 dependency types:

#### Blocking Types (Hard Dependencies)
1. `blocks` - from_id blocks to_id (to_id cannot start)
2. `blocked-by` - Inverse of blocks (for bidirectional queries)

#### Soft Dependencies
3. `waits-for` - Soft dependency, can start but should defer
4. `depends-on` - General dependency (non-blocking)

#### Hierarchical
5. `parent-child` - Hierarchical decomposition
6. `child-of` - Inverse
7. `subtask-of` - Task breakdown
8. `has-subtask` - Inverse

#### Semantic Relationships
9. `duplicates` - Marks as duplicate
10. `duplicated-by` - Inverse
11. `relates-to` - General association
12. `supersedes` - Replaces older task
13. `superseded-by` - Inverse

#### Ordering
14. `follows` - Sequencing hint (not blocking)
15. `precedes` - Inverse

#### Technical
16. `caused-by` - Bug causation
17. `causes` - Inverse
18. `fixes` - Fix relationship
19. `fixed-by` - Inverse

**Update Requirements:**
- Line 20: Change "19 semantic dependency types" to list all types explicitly
- Update dep.go to support all 19 types
- Document blocking semantics for each type

### 3. **Performance Optimization Strategy**

Add new section to Epic 2:

#### Performance Architecture

**Caching Layer:**
```go
type GraphCache struct {
    adjacencyList map[string][]Dependency  // Rebuilt on change
    indegreeMap   map[string]int           // Incrementally updated
    readyList     []Issue                  // TTL: 1 minute
    lastUpdate    time.Time
}
```

**Cache Invalidation:**
- On dependency add: increment indegree of to_id only
- On dependency remove: decrement indegree of to_id only
- On issue status change: recompute affected subgraph only
- On bulk operations: full cache rebuild

**Database Indexes:**
```sql
-- For blocking chain queries
CREATE INDEX idx_from_status ON dependencies(from_id, type)
  WHERE type IN ('blocks', 'waits-for');

-- For ready engine (filter open + compute indegree)
CREATE INDEX idx_status_priority ON issues(status, priority, created_at)
  WHERE status = 'open';

-- For transitive traversal
CREATE INDEX idx_dep_type_from ON dependencies(type, from_id);

-- For reverse lookup (who blocks me?)
CREATE INDEX idx_dep_type_to ON dependencies(type, to_id);
```

**Incremental Updates:**
- Avoid full graph recomputation on every change
- Track "dirty" nodes and recompute only affected subgraphs
- Benchmark: Single dependency add should be <1ms

### 4. **Priority Inversion Mitigation**

Add new user story:

#### 2.7 Priority Inheritance and Starvation Prevention

**As an** AI agent
**I want to** high-priority tasks to inherit priority through blocking chains
**So that** critical work isn't delayed by low-priority blockers

**Acceptance Criteria:**
- When high-priority task (P0) is blocked by low-priority task (P3), blocker inherits higher priority
- Priority inheritance propagates transitively through blocking chains (max depth: 10)
- `grava ready` returns tasks with effective priority (original or inherited, whichever is higher)
- `grava ready --show-inherited` displays both original and effective priority
- Aging mechanism: tasks waiting >7 days get priority boost (+1 level)
- Dashboard shows tasks with inherited priority for visibility
- Performance: Priority inheritance calculation <5ms for graphs with 10k nodes

**Algorithm:**
```
For each task T in ready queue:
  effective_priority[T] = min(T.priority, min_priority_of_all_dependents[T])

Where dependents are tasks blocked by T (directly or transitively)
```

**Starvation Prevention:**
- Track `created_at` timestamp
- Every 7 days, boost priority by 1 level (max: P0)
- Log priority boosts in events table
- `grava list --aged` shows boosted tasks

### 5. **Enhanced Gate System**

Add implementation details to User Story 2.6:

#### Gate System Implementation Details

**GitHub PR Gate:**
- **Polling Strategy:** Query GitHub API on-demand when computing ready tasks
- **Caching:** Cache PR status for 5 minutes to reduce API calls
- **Rate Limiting:** Respect GitHub API rate limits (5000 req/hour)
- **Webhook Mode (Future):** Register webhook to clear cache on PR merge/close
- **Graceful Degradation:** If API fails, treat gate as "closed" and log warning
- **Configuration:**
  - `await_type='gh:pr'`
  - `await_id='owner/repo/pulls/123'`
  - Check: `merged_at IS NOT NULL` via GitHub API v4 GraphQL

**Timer Gate:**
- **Implementation:** Simple timestamp comparison against current time
- **Support:** ISO 8601 timestamps and relative durations
- **Configuration:**
  - `await_type='timer'`
  - `await_id='2026-03-01T00:00:00Z'` (absolute)
  - `await_id='+7d'` (relative from creation)
- **Check:** `CURRENT_TIMESTAMP >= PARSE_TIMESTAMP(await_id)`

**Human Gate:**
- **Manual Approval:** Required human intervention
- **Command:** `grava gate approve <issue_id> --reason "Review complete"`
- **Storage:** Store approval in events table
- **Configuration:**
  - `await_type='human'`
  - `await_id='approval:security-review'`
- **Check:** Query events table for approval event

**Gate API:**
```go
type Gate interface {
    IsOpen(issue Issue) (bool, error)
    GetStatus(issue Issue) (string, error) // "open", "closed", "pending", "error"
}

type GitHubPRGate struct { /* ... */ }
type TimerGate struct { /* ... */ }
type HumanGate struct { /* ... */ }
```

### 6. **Observability and Debugging**

Add new user story:

#### 2.8 Graph Health Monitoring

**As a** system operator
**I want to** monitor graph health metrics
**So that** I can identify and resolve bottlenecks

**Acceptance Criteria:**
- `grava graph stats` shows:
  - Total nodes, edges, avg indegree, max indegree
  - Percentage of blocked vs. ready tasks
  - Longest blocking chain depth
  - Tasks blocked by gates vs. dependencies
  - Cycle detection status (last run, duration)
- `grava graph visualize <issue_id>` exports DOT format for Graphviz
  - Option: `--depth N` to limit visualization depth
  - Option: `--focus <id>` to center on specific issue
- Slow query logging for Ready Engine (>50ms)
- Metrics exported via JSON for integration with monitoring systems
- `grava graph health` returns exit code 0 (healthy) or 1 (issues detected)

**Example Output:**
```
Graph Statistics:
  Total Issues:        1,247 (1,089 open, 158 closed)
  Total Dependencies:  3,421
  Average Indegree:    2.74
  Max Indegree:        12 (issue: grava-a1b2)

Ready Tasks:           47 (3.8% of open)
Blocked Tasks:         892 (72.1% of open)
  Blocked by Issues:   784
  Blocked by Gates:    108

Longest Chain:         8 levels (grava-xyz1 → ... → grava-abc9)
Cycles Detected:       0

Performance:
  Last Ready Query:    4.2ms
  Last Cycle Check:    67ms (2024-02-20 14:30:12)
```

### 7. **Enhanced User Stories**

Add these stories to Epic 2:

#### 2.9 Batch Dependency Operations

**As a** developer
**I want to** create multiple dependencies atomically
**So that** I can efficiently set up complex task structures

**Acceptance Criteria:**
- `grava dep batch --file deps.json` creates multiple deps from JSON file
- `grava dep batch --stdin` accepts JSON from stdin (for piping)
- JSON format: `[{"from": "id1", "to": "id2", "type": "blocks"}, ...]`
- Validates entire batch before applying (all-or-nothing transaction)
- Single cycle detection pass for entire batch (not per-dependency)
- Performance: 1000 deps in <500ms
- Returns summary: "Created 1000 dependencies, 0 skipped, 0 errors"
- Rollback on any error (atomic operation)

**Example:**
```json
[
  {"from": "grava-abc", "to": "grava-def", "type": "blocks"},
  {"from": "grava-abc", "to": "grava-ghi", "type": "blocks"},
  {"from": "grava-def", "to": "grava-jkl", "type": "waits-for"}
]
```

#### 2.10 Dependency Removal and Cleanup

**As a** developer
**I want to** safely remove dependencies
**So that** I can correct mistakes and adapt to changing requirements

**Acceptance Criteria:**
- `grava dep remove <from_id> <to_id>` removes edge (defaults to all types)
- `grava dep remove <from_id> <to_id> --type <type>` removes specific type
- `grava dep clear <issue_id>` removes all deps for an issue (with confirmation)
- `grava dep clear <issue_id> --force` skips confirmation
- Audit trail captures dependency removals in events table
- Cache invalidation triggered on removal
- Returns affected tasks: "Removed 1 dependency, unblocked 3 tasks"
- `--dry-run` flag shows what would be removed without applying

#### 2.11 Dependency Query and Analysis

**As a** developer
**I want to** query and analyze dependency relationships
**So that** I can understand task structure

**Acceptance Criteria:**
- `grava dep list <issue_id>` shows all deps for an issue
- `grava dep tree <issue_id>` shows dependency tree (ASCII art)
- `grava dep blockers <issue_id>` shows what blocks this issue
- `grava dep blocked-by <issue_id>` shows what this issue blocks
- `grava dep path <from_id> <to_id>` finds shortest blocking path
- All commands support `--json` output
- Performance: Queries complete in <100ms for graphs with 10k nodes

---

## Best Practices from Research

### Algorithm Selection

1. **Use Kahn's Algorithm for Production Systems**
   - Iterative, better error messages, easier to debug than DFS recursion
   - Preferred in interviews and production systems
   - Source: [Topological Sort Best Practices](https://medium.com/codetodeploy/topological-sort-cycle-detection-the-brain-behind-scheduling-problems-9f3063571e83)

2. **Separate Cycle Detection from Sorting**
   - Validate graph integrity separately for better error handling
   - Provides clearer error messages to users
   - Source: [Dependency Resolution Made Simple](https://borretti.me/article/dependency-resolution-made-simple)

### Performance Optimization

3. **Minimize API Requests**
   - Cache gate conditions aggressively
   - Use webhooks where possible instead of polling
   - Source: [Gates and Coordination](https://deepwiki.com/steveyegge/beads/9.2-claude-plugin-and-editor-integration)

4. **Implement Priority Queues with Heaps**
   - O(log n) extraction for efficient priority-based scheduling
   - Use `container/heap` in Go standard library
   - Source: [Priority Queue Data Structure](https://herovired.com/learning-hub/topics/priority-queue-in-data-structure)

5. **Use Incremental Updates**
   - Don't recompute entire graph on small changes
   - Maven's BF and Skipper algorithm reduced resolution time from 30+ minutes to seconds
   - Source: [Maven Dependency Resolution](https://blog.anilgulati.com/maven-dependency-resolution-the-bf-and-skipper-algorithm)

### Priority Handling

6. **Add Aging to Prevent Starvation**
   - Low-priority tasks should eventually get promoted
   - Common approach: boost priority every N days
   - Source: [Priority Scheduling](https://medium.com/@rudrab1914/operating-system-scheduling-algorithms-priority-based-scheduling-0d06b1ea51ed)

7. **Implement Priority Inheritance**
   - High-priority tasks should boost priority of blockers
   - Prevents priority inversion problems
   - Source: [Priority-Based Scheduling](https://www.sciencedirect.com/topics/computer-science/priority-based-scheduling)

### Dependency Management

8. **Track Transitive Dependencies Efficiently**
   - Use BFS for transitive closure calculation
   - Cache frequently-accessed transitive relationships
   - Source: [Dependency Management - Taskwarrior](https://deepwiki.com/GothenburgBitFactory/taskwarrior/3.5-dependency-management)

9. **Use SAT-Based Solvers for Complex Constraints**
   - For version solving with complex constraints, SAT solvers (PubGrub, libsolv) are proven
   - CDCL (Conflict-Driven Clause Learning) scales well
   - Source: [Dependency Resolution Methods](https://nesbitt.io/2026/02/06/dependency-resolution-methods.html)

### GitHub Integration

10. **Check PR Mergeability via API**
    - Use GraphQL v4 API: `mergeable_state == "BLOCKED"` check
    - Cache results to avoid rate limiting
    - Source: [API to check if a PR is mergeable](https://github.com/orgs/community/discussions/25284)

11. **Use Merge Gatekeeper Pattern**
    - Wait for all CI checks before considering PR "ready"
    - Fail gate if any check fails
    - Source: [Merge Gatekeeper](https://github.com/marketplace/actions/merge-gatekeeper)

---

## Implementation Priority

Recommend tackling improvements in this order:

### Phase 1: Critical Gaps (Week 1-2)
1. ✅ Define complete 19-type dependency taxonomy
2. ✅ Specify Kahn's Algorithm for cycle detection
3. ✅ Design database indexes for performance
4. ✅ Implement priority inheritance

### Phase 2: Performance (Week 3-4)
5. ✅ Build caching layer for graph data
6. ✅ Implement incremental updates
7. ✅ Add batch operations
8. ✅ Optimize Ready Engine query

### Phase 3: Features (Week 5-6)
9. ✅ Enhance gate system with GitHub API integration
10. ✅ Add dependency query commands
11. ✅ Implement starvation prevention
12. ✅ Add graph visualization

### Phase 4: Observability (Week 7-8)
13. ✅ Build monitoring dashboard
14. ✅ Add performance metrics
15. ✅ Implement slow query logging
16. ✅ Create health check command

---

## Conclusion

Epic 2 provides a solid foundation for graph mechanics, but requires significant elaboration before implementation. The most critical gaps are:

1. **Algorithm specification** - Without clear algorithm choices, implementation will be inconsistent
2. **Priority inversion handling** - Critical for AI agent workflows where priority matters
3. **Performance optimization strategy** - Required to meet stated performance goals
4. **Complete dependency taxonomy** - Current 4 types vs. stated 19 creates confusion

Implementing the recommended improvements will result in a production-ready, scalable dependency graph system that can handle complex project workflows.

---

## Sources

- [Cycle Detection and Topological Sort Algorithm](https://labuladong.online/en/algo/data-structure/topological-sort/)
- [Topological Sort & Cycle Detection - Medium](https://medium.com/codetodeploy/topological-sort-cycle-detection-the-brain-behind-scheduling-problems-9f3063571e83)
- [Dependency graph - Wikipedia](https://en.wikipedia.org/wiki/Dependency_graph)
- [Managing Dependencies with Topological Sorting in Java](https://medium.com/@AlexanderObregon/managing-dependencies-with-topological-sorting-in-java-956a026a90d3)
- [Prioritized Task Scheduling](https://wicg.github.io/scheduling-apis/)
- [Priority-Based Scheduling](https://www.sciencedirect.com/topics/computer-science/priority-based-scheduling)
- [Dependency Management - Taskwarrior](https://deepwiki.com/GothenburgBitFactory/taskwarrior/3.5-dependency-management)
- [Priority Queue in Data Structure](https://herovired.com/learning-hub/topics/priority-queue-in-data-structure)
- [Priority-Based Scheduling - Medium](https://medium.com/@rudrab1914/operating-system-scheduling-algorithms-priority-based-scheduling-0d06b1ea51ed)
- [Gates and Coordination](https://deepwiki.com/steveyegge/beads/9.2-claude-plugin-and-editor-integration)
- [GitHub Pull Request Check and Gate Design](https://github.com/BonnyCI/projman/wiki/GitHub-Pull-Request-Check-and-Gate-Design)
- [API to check if a PR is mergeable](https://github.com/orgs/community/discussions/25284)
- [Merge Gatekeeper - GitHub Marketplace](https://github.com/marketplace/actions/merge-gatekeeper)
- [Dependency Resolution Methods](https://nesbitt.io/2026/02/06/dependency-resolution-methods.html)
- [Maven: BF and Skipper Algorithm](https://blog.anilgulati.com/maven-dependency-resolution-the-bf-and-skipper-algorithm)
- [The magic of dependency resolution](https://ochagavia.nl/blog/the-magic-of-dependency-resolution/)
- [Dependency Resolution Made Simple](https://borretti.me/article/dependency-resolution-made-simple)
- [Open-Source Contribution: New Maven Dependency Resolution Algorithm](https://innovation.ebayinc.com/stories/open-source-contribution-new-maven-dependency-resolution-algorithm/)

---

**Document Version:** 1.0
**Last Updated:** 2026-02-20
**Next Review:** Before Epic 2 implementation kickoff
