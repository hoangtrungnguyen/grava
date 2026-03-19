# Scenario 7: Large File Concurrent Edits — File Reservations

**Status**: Edge case validation
**Source**: [edge-case-resolution-strategy.md](../../_bmad-output/planning-artifacts/edge-case-resolution-strategy.md) — Edge Case 2
**Priority**: HIGH (prevents merge conflicts on shared large files)

---

## Overview

Multiple agents attempt sweeping changes to the same large files simultaneously. File reservations prevent concurrent edits by blocking commits to paths held by another agent.

## Setup

### Prerequisites
- `file_reservations` table in database
- Git pre-commit hook checking leases: `hooks.d/pre-commit/50-agent-mail.py`
- TTL-based auto-release on agent crash
- Agent-to-agent messaging (mcp_agent_mail integration)

### Steps

1. **Create two tasks modifying overlapping files**:
   - Task H: "Refactor authentication system (touches `pkg/auth/**`)"
   - Task I: "Add session management (touches `pkg/auth/session.go`)"

2. **Agent H declares file reservation**:
   ```bash
   grava claim task-h
   # Calls: file_reservation_paths(path="pkg/auth/**", exclusive=true, ttl=3600)
   ```
   - DB records: Agent H has exclusive lock on `pkg/auth/**`
   - Git artifact created: `file_reservations/<sha1(pkg/auth/)>.json`
   - TTL: 3600 seconds

3. **Agent H makes changes**:
   ```bash
   # Modifies: pkg/auth/handler.go, pkg/auth/middleware.go
   git add pkg/auth/*.go
   git commit -m "refactor: authentication system"
   # Pre-commit hook checks leases → OK (Agent H owns lease)
   ```

4. **Agent I attempts changes** (simultaneously):
   ```bash
   grava claim task-i
   # Calls: file_reservation_paths(path="pkg/auth/session.go", exclusive=true, ttl=3600)
   # ❌ CONFLICT: path overlaps with Agent H's reservation
   # Error: FILE_RESERVATION_CONFLICT
   ```

5. **Agent I's options**:
   - Option A: Wait for Agent H to release lease
   - Option B: Message Agent H directly (auto-allow contact via overlapping leases)
   - Option C: Choose different files (negotiate task split)

6. **Agent H releases lease**:
   ```bash
   grava close task-h
   # Calls: release_file_reservations(task_id='task-h')
   # DB removes reservation
   # Artifact deleted from Git
   ```

7. **Agent I can now claim**:
   ```bash
   # After Agent H releases, reservation is gone
   grava claim task-i
   # Calls: file_reservation_paths(path="pkg/auth/session.go", exclusive=true, ttl=3600)
   # ✅ OK (no conflict now)
   ```

---

## Expected Behavior

✅ Agent H declares file reservation before modifying files
✅ Reservation is recorded in DB + Git artifact
✅ Agent I attempts to reserve overlapping path
✅ System detects conflict: `FILE_RESERVATION_CONFLICT`
✅ Agent I **cannot commit** to reserved paths:
   - Pre-commit hook blocks commit
   - Error message: "Path `pkg/auth/**` is reserved by Agent H"
✅ Agents can auto-contact (shared file lease triggers `auto-allow`)
✅ Agent H releases lease on task close
✅ Lease auto-releases if Agent H crashes (TTL expiry)
✅ Agent I can then claim and proceed

---

## Validation

**Success Criteria**:
1. ✅ Agent H declares reservation:
   ```bash
   # Reservation recorded
   dolt sql "SELECT * FROM file_reservations WHERE agent_id='agent-h' AND path_pattern='pkg/auth/**'"
   # Should return 1 row
   ```

2. ✅ Git artifact created:
   ```bash
   ls file_reservations/*.json | wc -l
   # Should be > 0
   ```

3. ✅ Agent H can commit (owns lease):
   ```bash
   git commit -m "refactor: auth system"
   # ✅ Pre-commit hook allows it
   ```

4. ✅ Agent I blocked from overlapping path:
   ```bash
   # Attempt to commit to reserved path
   echo "code" >> pkg/auth/session.go
   git add pkg/auth/session.go
   git commit -m "feat: session" 2>&1 | grep -i "FILE_RESERVATION_CONFLICT\|reserved"
   # Should fail
   ```

5. ✅ Reservation conflict detected:
   ```bash
   dolt sql "SELECT COUNT(*) FROM file_reservations WHERE path_pattern LIKE 'pkg/auth/%' AND released_ts IS NULL"
   # Should be 1 (Agent H's active lease)
   ```

6. ✅ Auto-allow contact enabled:
   ```bash
   # Agent I can message Agent H directly (overlapping lease auto-allows)
   # Message thread ID = Task ID for zero mapping
   grava message agent-h "Can you expedite auth task?"
   # ✅ Allowed (no handshake needed)
   ```

7. ✅ Lease auto-releases on crash:
   ```bash
   # Simulate TTL expiry
   dolt sql "UPDATE file_reservations SET expires_ts = NOW() - INTERVAL '1 hour' WHERE agent_id='agent-h'"

   # Cleanup job detects expired lease
   grava doctor --fix

   # Lease is released
   dolt sql "SELECT released_ts FROM file_reservations WHERE agent_id='agent-h'" | grep -v NULL
   ```

8. ✅ Agent I can proceed after release:
   ```bash
   # After release, Agent I can claim
   grava claim task-i
   dolt sql "SELECT * FROM file_reservations WHERE agent_id='agent-i' AND path_pattern LIKE 'pkg/auth/%'"
   # ✅ Reservation succeeds
   ```

**Test Assertions**:
```bash
# Agent H declares
AGENT_H_LEASE=$(dolt sql -r csv "SELECT COUNT(*) FROM file_reservations WHERE agent_id='agent-h'")
[ "$AGENT_H_LEASE" -eq 1 ]

# Git artifact exists
[ $(ls file_reservations/*.json 2>/dev/null | wc -l) -gt 0 ]

# Agent H can commit
echo "code" >> pkg/auth/handler.go
git add pkg/auth/handler.go
git commit -m "refactor: auth" && SUCCESS=1 || SUCCESS=0
[ $SUCCESS -eq 1 ]

# Agent I blocked
echo "code" >> pkg/auth/session.go
git add pkg/auth/session.go
git commit -m "feat: session" 2>&1 | grep -q "reserved" || BLOCKED=1
[ $BLOCKED -eq 1 ]

# TTL cleanup works
dolt sql "UPDATE file_reservations SET expires_ts = NOW() - INTERVAL '1 hour'"
grava doctor --fix
dolt sql -r csv "SELECT COUNT(*) FROM file_reservations WHERE released_ts IS NOT NULL" | grep -q "[1-9]"
```

---

## Cleanup

```bash
# Release remaining reservations
grava close task-h
grava close task-i

# Verify all leases released
dolt sql "SELECT COUNT(*) FROM file_reservations WHERE released_ts IS NULL" | grep -q "^0"
```

---

## Data Structures

From [edge-case-resolution-strategy.md](../../_bmad-output/planning-artifacts/edge-case-resolution-strategy.md):

```sql
file_reservations (
  id           TEXT PRIMARY KEY,
  project_id   TEXT,
  agent_id     TEXT,
  path_pattern TEXT,
  exclusive    BOOLEAN,
  reason       TEXT,        -- maps to task ID
  created_ts   TIMESTAMP,
  expires_ts   TIMESTAMP,   -- TTL-based auto-expiry
  released_ts  TIMESTAMP
)
```

**Git Artifact**:
```json
-- file_reservations/<sha1(path)>.json
{
  "path_pattern": "pkg/auth/**",
  "agent_id": "agent-h",
  "task_id": "task-h",
  "exclusive": true,
  "created_ts": "2026-03-19T10:00:00Z",
  "expires_ts": "2026-03-19T11:00:00Z"
}
```

---

## Cross-Cutting Patterns

From [edge-case-resolution-strategy.md](../../_bmad-output/planning-artifacts/edge-case-resolution-strategy.md):

| Pattern | Effect |
|---------|--------|
| **Thread ID = Task ID** | Mail thread for task-h matches task-h (zero mapping) |
| **Auto-Allow Contact** | Agents sharing overlapping leases can message freely |
| **Overlapping Lease Negotiation** | Agent I can ask Agent H to expedite |

---

## Notes

- **Duration**: ~5-10 seconds
- **Reservation pattern**: "Declare intent before modifying"
- **TTL-based cleanup**: Prevents ghost leases after crash
- **Pre-commit enforcement**: Blocks unauthorized commits
- **Human control**: No automatic task reordering (agents decide)
