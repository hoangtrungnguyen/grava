# Scenario 3: Agent Crash + Resume — Wisps Recovery

**Status**: Failure recovery validation
**Source**: [failure-recovery-strategy.md](../../_bmad-output/planning-artifacts/failure-recovery-strategy.md) — Failure 1
**Priority**: CRITICAL (prevents work duplication, enables resilience)

---

## Overview

Agent crashes mid-execution, leaving partial work in unknown state. TTL lease expires, next agent reads Wisps (ephemeral activity log) to resume without duplicating work.

## Setup

### Prerequisites
- Dolt database with TTL lease support
- Wisps table exists
- File reservation system active
- Agent can be killed/crashed during execution

### Steps

1. **Create task that takes multiple commits**:
   - Task C: "Refactor multiple modules" (10+ step process)

2. **Agent 1 starts work**:
   ```bash
   grava claim task-c
   # Obtains file lease with TTL=3600s
   # Branch: grava/agent-1/task-c
   ```

3. **Agent 1 makes partial progress** (steps 1-5 of 10):
   - Modifies `pkg/module-a/handler.go`
   - Writes Wisp entry: "Completed module-a refactor"
   - Commits: "refactor: module-a handler"
   - **Agent process crashes** (simulated):
     ```bash
     kill -9 <agent-pid>
     ```

4. **Wait for TTL expiry**:
   ```bash
   # Simulate TTL expiry (or wait 3600+ seconds)
   # Database cleanup job detects stale lease
   # File reservations for task-c are auto-released
   ```

5. **Agent 2 claims same task**:
   ```bash
   grava claim task-c
   # Reads Wisps for task-c
   # Sees: "Completed module-a refactor"
   # Resumes from checkpoint (step 6 of 10)
   ```

6. **Agent 2 continues work** (steps 6-10):
   - Modifies `pkg/module-b/service.go`
   - Modifies `pkg/module-c/store.go`
   - Commits: "refactor: module-b and module-c"
   - Closes task

7. **Cleanup Wisps**:
   ```bash
   # On task close, Wisps entries are deleted
   # (Prevents stale data for next claim)
   ```

---

## Expected Behavior

✅ Agent 1 obtains file lease with TTL
✅ Agent 1 writes Wisps throughout execution
✅ Agent 1 crashes (or is killed)
✅ TTL expires → lease auto-released
✅ File reservations for task-c are cleared
✅ Agent 2 can claim task-c
✅ Agent 2 reads Wisps → understands Agent 1's progress
✅ Agent 2 resumes from checkpoint (no duplication)
✅ Agent 2 completes remaining work
✅ Task closes → Wisps deleted
✅ No orphaned leases or ghost DB records

---

## Validation

**Success Criteria**:
1. ✅ Agent 1 crash detected:
   ```bash
   # TTL lease exists but agent is dead
   dolt table cat file_reservations | grep task-c
   # Shows: expires_ts < now() (expired)
   ```

2. ✅ TTL cleanup auto-runs:
   ```bash
   # After TTL expires, lease is released
   dolt table cat file_reservations | grep task-c | wc -l
   # Should be 0 (or shows released_ts is set)
   ```

3. ✅ Wisps exist with checkpoint data:
   ```bash
   dolt table cat wisps | grep task-c
   # Shows: agent_id=agent-1, log_entry="Completed module-a refactor"
   ```

4. ✅ Agent 2 can claim task:
   ```bash
   [ $(grava claim task-c 2>&1 | grep "claimed") ]
   ```

5. ✅ Agent 2 reads Wisps:
   ```bash
   dolt table cat wisps | grep task-c
   # Agent 2 can read this data
   ```

6. ✅ No work duplication:
   ```bash
   git log grava/agent-1/task-c..grava/agent-2/task-c
   # Shows only Agent 2's commits (Agent 1's not repeated)
   ```

7. ✅ Wisps deleted on close:
   ```bash
   grava close task-c
   dolt table cat wisps | grep task-c | wc -l
   # Should be 0 (Wisps deleted)
   ```

**Test Assertions**:
```bash
# TTL expired
[ $(dolt sql -r csv "SELECT COUNT(*) FROM file_reservations WHERE id='task-c' AND expires_ts < NOW()") -eq 1 ]

# Wisps exist
[ $(dolt sql -r csv "SELECT COUNT(*) FROM wisps WHERE issue_id='task-c'") -gt 0 ]

# Agent 2 can claim
grava claim task-c > /tmp/claim.log
grep -q "claimed" /tmp/claim.log

# No duplication in branch history
AGENT1_COMMITS=$(git log grava/agent-1/task-c | wc -l)
AGENT2_COMMITS=$(git log grava/agent-2/task-c | wc -l)
[ $AGENT2_COMMITS -eq $AGENT1_COMMITS ]  # Not doubled

# Wisps cleaned up
grava close task-c
[ $(dolt sql -r csv "SELECT COUNT(*) FROM wisps WHERE issue_id='task-c'") -eq 0 ]
```

---

## Cleanup

```bash
# Mark task as closed in DB
dolt sql "UPDATE issues SET status='closed' WHERE id='task-c'"

# Delete branch (if not auto-deleted)
git branch -D grava/agent-2/task-c
```

---

## Data Structures

From [failure-recovery-strategy.md](../../_bmad-output/planning-artifacts/failure-recovery-strategy.md):

```sql
-- File leases with TTL
file_reservations (
  id           TEXT PRIMARY KEY,
  agent_id     TEXT,
  expires_ts   TIMESTAMP,
  released_ts  TIMESTAMP
)

-- Ephemeral crash-resume log
wisps (
  id         TEXT PRIMARY KEY,
  issue_id   TEXT REFERENCES issues(id),
  agent_id   TEXT,
  log_entry  TEXT,
  created_ts TIMESTAMP
)
```

---

## Notes

- **Duration**: ~5-15 seconds (or 3600+ if testing real TTL)
- **TTL simulation**: Can mock `now()` in DB for faster testing
- **Recovery pattern**: Avoids "start from scratch" after crash
- **Silent failure prevention**: Wisps prove work was done (no silent loss)
- **Test ownership**: Agent 2 doesn't modify Agent 1's commits
