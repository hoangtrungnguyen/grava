# Scenario 4: Worktree Ghost State — `grava doctor` Detection & Healing

**Status**: Failure recovery validation
**Source**: [failure-recovery-strategy.md](../../_bmad-output/planning-artifacts/failure-recovery-strategy.md) — Failure 2
**Priority**: HIGH (prevents orphaned state corruption)

---

## Overview

Agent crashes after partially tearing down. DB says task is `in_progress` but worktree directory is missing (ghost state). `grava doctor` detects mismatch and heals it.

## Setup

### Prerequisites
- Dolt database with issues and file_reservations tables
- `.worktrees/{agent}` directory structure exists
- `grava doctor` command implemented
- Worktree cleanup logic in `grava close/stop`

### Steps

1. **Create task and start agent**:
   ```bash
   grava claim task-d
   # Creates .worktrees/agent-3/task-d/
   # DB: INSERT INTO issues (id, status, assignee) VALUES ('task-d', 'in_progress', 'agent-3')
   ```

2. **Agent starts work**:
   - Makes some commits to branch
   - Modifies files in worktree

3. **Agent crashes during cleanup**:
   - Simulated: `rm -rf .worktrees/agent-3/task-d/` (directory deleted)
   - BUT DB still says `in_progress`
   - **Ghost state created**: DB ≠ Filesystem

4. **Human/system runs diagnostic**:
   ```bash
   grava doctor
   # Scans for mismatches
   ```

---

## Expected Behavior

✅ Agent crashes, worktree directory deleted
✅ DB still has task in `in_progress` state
✅ `grava doctor` detects mismatch:
   - DB says `in_progress`
   - Filesystem has no `.worktrees/agent-3/task-d/` directory
✅ `grava doctor --fix` heals the ghost state:
   - Updates DB: `status = 'open'` (released back to backlog)
   - No data loss (branch still exists in Git)
   - Next agent can claim and resume
✅ No cascading errors from ghost state

---

## Validation

**Success Criteria**:
1. ✅ Ghost state exists:
   ```bash
   # DB says in_progress
   dolt sql "SELECT status FROM issues WHERE id='task-d'"
   # Output: in_progress

   # But directory is gone
   [ ! -d ".worktrees/agent-3/task-d" ]
   ```

2. ✅ `grava doctor` detects it:
   ```bash
   grava doctor
   # Output should include:
   # "Ghost worktree: in_progress issue 'task-d' has no directory"
   ```

3. ✅ `grava doctor --fix` repairs it:
   ```bash
   grava doctor --fix
   # Backs up any related state
   # Updates DB: status = 'open'
   ```

4. ✅ DB is healed:
   ```bash
   dolt sql "SELECT status FROM issues WHERE id='task-d'"
   # Output: open (no longer ghost)
   ```

5. ✅ Branch still exists:
   ```bash
   git branch | grep grava/agent-3/task-d
   # Branch is NOT deleted (work is preserved)
   ```

6. ✅ Next agent can claim:
   ```bash
   grava claim task-d  # Works without error
   # Can resume from branch work
   ```

**Test Assertions**:
```bash
# Create ghost state
rm -rf .worktrees/agent-3/task-d/

# Doctor detects it
grava doctor 2>&1 | grep -i "ghost\|mismatch"

# Doctor fixes it
grava doctor --fix

# Verify fix
dolt sql -r csv "SELECT status FROM issues WHERE id='task-d'" | grep -q "open"

# Branch preserved
git branch | grep -q "grava/agent-3/task-d"

# Next agent can claim
grava claim task-d > /tmp/claim.log
grep -q "claimed" /tmp/claim.log
```

---

## Cleanup

```bash
# Delete branch after testing
git branch -D grava/agent-3/task-d

# Mark issue as closed
dolt sql "UPDATE issues SET status='closed' WHERE id='task-d'"
```

---

## Critical Safeguards

From [failure-recovery-strategy.md](../../_bmad-output/planning-artifacts/failure-recovery-strategy.md):

```
[ ] in_progress issues with no worktree directory       → ghost DB record
```

**Two-Phase Atomic Teardown**:
1. Check for uncommitted changes → abort if found (prevent silent data loss)
2. Remove directory + update DB status atomically

**Idempotent Teardown**: `grava close/stop` safe to re-run after partial failure

---

## Notes

- **Duration**: ~2-3 seconds
- **Severity**: Ghost state can cause cascading errors
- **Recovery pattern**: Detect + backup + heal
- **Data safety**: Branch preserved, DB recovered, no loss
- **Dry-run mode**: `grava doctor --dry-run` shows what would happen
