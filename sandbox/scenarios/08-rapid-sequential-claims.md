# Scenario 8: Rapid Sequential Claims — `SELECT FOR UPDATE` Locks

**Status**: Edge case validation
**Source**: [edge-case-resolution-strategy.md](../../_bmad-output/planning-artifacts/edge-case-resolution-strategy.md) — Edge Case 3
**Priority**: CRITICAL (ensures at most one agent per task)

---

## Overview

Two agents attempt to claim the same task milliseconds apart. Row-level `SELECT FOR UPDATE` lock ensures only one agent succeeds; second agent reads updated state and aborts with "already claimed" error.

## Setup

### Prerequisites
- Dolt database with `issues` table
- `SELECT FOR UPDATE` support (row-level locking)
- Wisps table for crash recovery
- Agent claiming logic with transactional atomicity

### Steps

1. **Create single task**:
   ```bash
   # DB: INSERT INTO issues (id, status) VALUES ('task-j', 'open')
   ```

2. **Agent J queries backlog** (high-priority first):
   ```bash
   grava backlog --robot-priority
   # Returns: task-j (highest priority)
   ```

3. **Agent K queries backlog** (same time):
   ```bash
   grava backlog --robot-priority
   # Returns: task-j (same task, milliseconds later)
   ```

4. **Both agents issue claim simultaneously**:
   ```bash
   # Agent J
   grava claim task-j
   # Executes: SELECT * FROM issues WHERE id='task-j' FOR UPDATE
   # ← Locks the row

   # Agent K (microseconds later)
   grava claim task-j
   # Waits for lock...
   ```

5. **Agent J's transaction succeeds**:
   ```
   - Acquires lock on task-j row
   - Updates: status='in_progress', assignee='agent-j'
   - Commits transaction
   - Lock released
   ```

6. **Agent K's transaction reads updated state**:
   ```
   - Lock acquired (Agent J released it)
   - Reads: status='in_progress', assignee='agent-j'
   - Recognizes: "already claimed by agent-j"
   - Aborts with error: "Task task-j already claimed"
   ```

7. **Agent K picks next task** from backlog

---

## Expected Behavior

✅ Both agents query same backlog
✅ Both see task-j (highest priority)
✅ Both attempt claim within milliseconds
✅ **Agent J's claim succeeds**:
   - `SELECT FOR UPDATE` lock acquired
   - DB state updated atomically
   - Worktree created
✅ **Agent K's claim fails safely**:
   - Waits for lock (not deadlock)
   - Reads Agent J's committed state
   - Returns: "task-j already claimed by agent-j"
   - No data corruption
   - No duplicate work
✅ Wisps recorded for Agent J:
   - Task start logged
   - Checkpoint data recorded
   - Allows crash recovery
✅ No orphaned work or duplicated claims

---

## Validation

**Success Criteria**:
1. ✅ Both agents see same task:
   ```bash
   # Two agents query backlog
   AGENT_J_BACKLOG=$(grava backlog --robot-priority | head -1)
   AGENT_K_BACKLOG=$(grava backlog --robot-priority | head -1)
   [ "$AGENT_J_BACKLOG" == "$AGENT_K_BACKLOG" ]
   # Should be task-j
   ```

2. ✅ Agent J claim succeeds:
   ```bash
   grava claim task-j > /tmp/agent_j.log 2>&1
   AGENT_J_EXIT=$?
   [ $AGENT_J_EXIT -eq 0 ]
   grep -q "claimed\|in_progress" /tmp/agent_j.log
   ```

3. ✅ Agent K claim fails (already claimed):
   ```bash
   grava claim task-j > /tmp/agent_k.log 2>&1
   AGENT_K_EXIT=$?
   [ $AGENT_K_EXIT -ne 0 ]
   grep -q "already claimed\|in_progress" /tmp/agent_k.log
   ```

4. ✅ DB state is consistent:
   ```bash
   # Only one agent is assignee
   dolt sql "SELECT assignee FROM issues WHERE id='task-j'" | grep -q "agent-j"

   # Status is in_progress (only once)
   dolt sql "SELECT COUNT(DISTINCT assignee) FROM issues WHERE id='task-j' AND status='in_progress'" | grep -q "^1"
   ```

5. ✅ Worktree created for Agent J:
   ```bash
   [ -d ".worktrees/agent-j/task-j" ]
   ```

6. ✅ Wisps recorded:
   ```bash
   dolt sql "SELECT COUNT(*) FROM wisps WHERE issue_id='task-j' AND agent_id='agent-j'" | grep -q "[1-9]"
   # Start Wisp should exist
   ```

7. ✅ No deadlock occurs:
   ```bash
   # Both transactions should complete in reasonable time (< 5 seconds)
   time grava claim task-j
   time grava claim task-j
   ```

**Test Assertions**:
```bash
# Both query same backlog
TASK_J_1=$(grava backlog --robot-priority | head -1)
TASK_J_2=$(grava backlog --robot-priority | head -1)
[ "$TASK_J_1" == "$TASK_J_2" ] && [ "$TASK_J_1" == "task-j" ]

# Agent J succeeds
grava claim task-j > /tmp/j.log && J_EXIT=0 || J_EXIT=$?
[ $J_EXIT -eq 0 ]

# Agent K fails (simulate sequential claim)
grava claim task-j > /tmp/k.log && K_EXIT=0 || K_EXIT=$?
[ $K_EXIT -ne 0 ]
grep -q "already claimed" /tmp/k.log

# DB is consistent
dolt sql -r csv "SELECT COUNT(*) FROM issues WHERE id='task-j' AND status='in_progress' AND assignee='agent-j'" | grep -q "^1"

# Worktree exists
[ -d ".worktrees/agent-j/task-j" ]

# Wisps logged
dolt sql -r csv "SELECT COUNT(*) FROM wisps WHERE issue_id='task-j'" | grep -q "[1-9]"
```

---

## Cleanup

```bash
# Agent J completes task
grava close task-j

# Verify cleanup
[ ! -d ".worktrees/agent-j/task-j" ]
dolt sql "SELECT status FROM issues WHERE id='task-j'" | grep -q "closed"

# Wisps deleted on close
dolt sql "SELECT COUNT(*) FROM wisps WHERE issue_id='task-j'" | grep -q "^0"
```

---

## Data Structures

From [edge-case-resolution-strategy.md](../../_bmad-output/planning-artifacts/edge-case-resolution-strategy.md):

```sql
issues (
  id       TEXT PRIMARY KEY,
  status   TEXT,   -- open | in_progress | closed
  assignee TEXT    -- agent_id
)

wisps (
  id         TEXT PRIMARY KEY,
  issue_id   TEXT REFERENCES issues(id),
  agent_id   TEXT,
  log_entry  TEXT,
  created_ts TIMESTAMP
)
```

---

## Critical Pattern: Row-Level Locking

**Transaction sequence for Agent J**:
```sql
BEGIN TRANSACTION;
  SELECT * FROM issues WHERE id='task-j' FOR UPDATE;
  -- ← Locks row (Agent K waits here)

  UPDATE issues SET status='in_progress', assignee='agent-j';
  INSERT INTO wisps (...) VALUES (...);
COMMIT;
-- ← Lock released, Agent K proceeds
```

**Why this works**:
- `FOR UPDATE` prevents second agent from proceeding
- No polling/retry loop (clean lock wait)
- First writer wins (Agent J)
- Second reader sees clean state (Agent K reads committed update)
- No race conditions or duplicate work

---

## Cross-Cutting Patterns

From [edge-case-resolution-strategy.md](../../_bmad-output/planning-artifacts/edge-case-resolution-strategy.md):

| Pattern | Implementation |
|---------|-----------------|
| **Thread ID = Task ID** | Wisps thread matches task ID (task-j) |
| **Unified thread notification** | Both agents notified when task claimed |
| **Summarize thread** | Agent K can query why claim failed |

---

## Notes

- **Duration**: ~1-3 seconds
- **Concurrency pattern**: Optimistic read, pessimistic write (row lock on claim)
- **Fairness**: First-to-lock wins (not random)
- **Deadlock prevention**: Single resource per transaction (task row)
- **Recovery**: Wisps preserve checkpoint if Agent J crashes
- **Scale limit**: Thousands of concurrent claims (DB row locking scales)
