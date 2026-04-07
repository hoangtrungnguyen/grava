# Grava Multi-Agent System Benchmarks

> Evaluation criteria tailored to the Grava distributed, agent-centric issue tracker.

## 1. Ready Engine Benchmarks

The Ready Engine is Grava's core DAG-based task selector. It must be fast and correct.

| Benchmark | Metric | Target | Critical |
|-----------|--------|--------|----------|
| **Ready query latency** | Time to evaluate DAG and return next available issue | < 50ms (p95) | Yes |
| **Ready query throughput** | Issues evaluated per second under load | > 100 ops/s | Yes |
| **DAG correctness** | No blocked issue returned as ready; no ready issue returned as blocked | 100% correctness | Yes |
| **Dependency cycle detection** | Circular dependencies detected and rejected | 100% detection | Yes |
| **DAG depth scaling** | Performance with dependency chains of depth 1, 5, 10, 20 | Linear degradation acceptable | No |

### Test Scenarios
- **Simple DAG**: 10 issues, 2 dependencies → verify correct order
- **Wide DAG**: 100 issues, no dependencies → all should be ready
- **Deep DAG**: 50 issues in a chain → only leaf should be ready
- **Blocked DAG**: 50 issues, all blocked → none should be ready
- **Diamond DAG**: A→B, A→C, B→D, C→D → D blocked until A, B, C done

## 2. Agent Coordination Benchmarks

Grava coordinates multiple autonomous agents working on issues simultaneously.

| Benchmark | Metric | Target | Critical |
|-----------|--------|--------|----------|
| **Claim contention rate** | % of claims that race/conflict under concurrent agents | < 1% | Yes |
| **Claim throughput** | Issues claimed per second | > 50 ops/s | Yes |
| **Atomic claim** | No two agents can claim the same issue simultaneously | Guaranteed | Yes |
| **State transition correctness** | Issue state transitions follow valid lifecycle (open→in_progress→done) | 100% | Yes |
| **Progression history accuracy** | Every state change is recorded with correct agent ID and timestamp | 100% | Yes |
| **Agent isolation** | Agents never see each other's in-flight work until committed | Guaranteed | Yes |

### Test Scenarios
- **Concurrent claim**: 10 agents simultaneously claim from pool of 100 issues → no duplicates
- **Rapid progression**: Agent opens→claims→completes 100 issues in sequence → all history correct
- **Agent crash recovery**: Agent claims issue then disconnects → issue becomes reclaimable after timeout

## 3. MCP Integration Benchmarks

Agents interact with Grava through the Model Context Protocol server.

| Benchmark | Metric | Target | Critical |
|-----------|--------|--------|----------|
| **MCP tool latency** | Response time per tool call | < 100ms (p95) | Yes |
| **MCP tool throughput** | Concurrent tool calls handled | > 50 concurrent | Yes |
| **Tool error rate** | % of tool calls returning errors | < 0.1% | Yes |
| **Context window efficiency** | Tokens consumed per tool response | Minimize | No |
| **Tool coverage** | % of Grava operations exposed as MCP tools | 100% | Yes |

### Required MCP Tools to Benchmark
- `claim_next_issue` - Ready Engine integration
- `update_issue_status` - State transitions
- `get_issue` / `list_issues` - Read operations
- `get_issue_history` - Progression history
- `add_comment` / `add_label` - Issue annotation

## 4. Dolt Database Benchmarks

Dolt provides version-controlled SQL storage. Benchmarks must account for its unique characteristics.

| Benchmark | Metric | Target | Critical |
|-----------|--------|--------|----------|
| **Read query latency** | SELECT queries under load | < 10ms (p95) | Yes |
| **Write query latency** | INSERT/UPDATE queries | < 20ms (p95) | Yes |
| **Dolt commit latency** | Time to `CALL DOLT_COMMIT()` | < 100ms | Yes |
| **Dolt diff latency** | Time to `CALL DOLT_DIFF()` | < 500ms | Yes |
| **Concurrent read/write** | Reads during active writes with no dirty reads | Guaranteed | Yes |
| **Database size scaling** | Performance with 1K, 10K, 100K issues | Linear degradation | No |
| **Branch/merge overhead** | Time to create branches and merge | < 200ms | Yes |

### Test Scenarios
- **Bulk insert**: 1000 issues inserted → all queryable immediately
- **Concurrent read/write**: 10 agents reading while 5 writing → no stale reads
- **History query**: Query issue progression history across 100 commits → results correct

## 5. Scalability Benchmarks

How Grava performs as the system grows.

| Benchmark | Metric | Target | Critical |
|-----------|--------|--------|----------|
| **Agent count scaling** | Performance with 1, 5, 10, 20, 50 concurrent agents | < 20% degradation at 20 agents | Yes |
| **Issue count scaling** | Performance with 100, 1K, 10K, 100K issues | < 30% degradation at 10K issues | Yes |
| **Throughput at scale** | Issues resolved per minute at target scale | > 10 issues/min at 10 agents | Yes |
| **Memory footprint** | Server memory usage at target scale | < 512MB at 10 concurrent agents | No |
| **Connection pool** | Database connections under load | No exhaustion at 50 concurrent agents | Yes |

## 6. Robustness & Safety Benchmarks

| Benchmark | Metric | Target | Critical |
|-----------|--------|--------|----------|
| **Error recovery** | System state after agent crash | All in-flight issues become reclaimable | Yes |
| **No orphan issues** | Issues never stuck in permanent in_progress | Guaranteed (with timeout) | Yes |
| **No duplicate processing** | Same issue never processed by 2 agents | Guaranteed | Yes |
| **Data integrity** | All state changes are ACID | Guaranteed | Yes |
| **Graceful shutdown** | All in-flight work tracked during shutdown | Zero data loss | Yes |

### Test Scenarios
- **Kill mid-process**: Kill agent during issue processing → issue reclaimable, no data corruption
- **Network partition**: Simulate network loss → system remains consistent
- **Double claim attempt**: Two agents claim same issue simultaneously → exactly one succeeds

## 7. End-to-End Workflow Benchmarks

Full workflow from issue creation to completion.

| Benchmark | Metric | Target | Critical |
|-----------|--------|--------|----------|
| **Issue lifecycle time** | Time from creation to done | Varies by complexity | No |
| **Agent iteration efficiency** | Turns (claim→work→test→commit) per issue | < 5 turns avg | No |
| **Test pass rate** | % of issues that pass on first agent attempt | > 70% | No |
| **Rework rate** | % of issues requiring re-opening after completion | < 10% | No |
| **Coordination overhead** | % of total time spent on coordination vs actual work | < 20% | No |

### Test Scenarios
- **"Are you ready" workflow**: Agent checks readiness, claims issue, implements, tests, commits → end-to-end
- **Multi-agent project**: 10 agents resolve 50 interdependent issues → all completed, no conflicts
- **Blocked chain**: 5 issues in dependency chain resolved sequentially → correct order enforced

## 8. Cost Efficiency Benchmarks

| Benchmark | Metric | Target | Critical |
|-----------|--------|--------|----------|
| **Tokens per issue** | LLM tokens consumed per resolved issue | Track and minimize | No |
| **Wasted computation** | % of agent turns that produce no useful progress | < 20% | No |
| **Context efficiency** | Ratio of relevant context to total context loaded | > 70% | No |

## Running Benchmarks

### Prerequisites
- Dolt database running at `.grava/dolt/`
- MCP server compiled and running
- Go test environment configured

### Commands
```bash
# Run all benchmarks
go test ./internal/benchmarks/... -bench=. -benchmem

# Run specific benchmark category
go test ./internal/benchmarks/... -bench=Ready -benchmem

# Run with CPU profiling
go test ./internal/benchmarks/... -bench=. -cpuprofile cpu.prof

# Generate benchmark comparison
go test ./internal/benchmarks/... -bench=. -count=5
```

### Benchmark Report Template

```
## Benchmark Report - [Date]

### Environment
- Go version: [version]
- Dolt version: [version]
- Hardware: [specs]
- Agent count: [N]
- Issue count: [N]

### Results Summary
| Category | Benchmark | Result | Target | Pass/Fail |
|----------|-----------|--------|--------|-----------|
| ... | ... | ... | ... | ... |

### Regressions
- [Any benchmarks that regressed from previous run]

### Recommendations
- [Actions based on results]
```
