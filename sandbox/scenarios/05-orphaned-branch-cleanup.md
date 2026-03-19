# Scenario 5: Orphaned Branch Cleanup — `grava doctor` Safe Purge

**Status**: Failure recovery validation
**Source**: [failure-recovery-strategy.md](../../_bmad-output/planning-artifacts/failure-recovery-strategy.md) — Failure 3
**Priority**: HIGH (prevents branch proliferation, maintains repo hygiene)

---

## Overview

Agent crashes, leaving worktree directory and branch on disk but task is `closed` (never recorded as completed). `grava doctor` detects orphaned branch and safely removes it with human safeguards.

## Setup

### Prerequisites
- Dolt database with issues table
- Git repository with branch naming convention: `grava/{agent-id}/{issue-id}`
- `grava doctor` command with `--dry-run` and `--fix` flags
- Uncommitted change detection

### Steps

1. **Create task and agent work**:
   ```bash
   grava claim task-e
   # Branch: grava/agent-4/task-e
   # Creates .worktrees/agent-4/task-e/
   ```

2. **Agent makes commits**:
   - Modifies `pkg/core/cache.go`
   - Commits: "feat: improve cache performance"
   - Branch has work

3. **Simulated failure scenario**:
   - Agent process dies without calling `grava close`
   - Worktree still exists: `.worktrees/agent-4/task-e/`
   - Branch still exists: `grava/agent-4/task-e`
   - DB issue is manually marked `closed` (cleanup job misses it)
   - **Orphaned state**: Branch exists but no active issue

4. **Run diagnostic**:
   ```bash
   grava doctor --dry-run
   # Should detect orphaned branch
   ```

---

## Expected Behavior

✅ Orphaned branch detected:
   - Branch `grava/agent-4/task-e` exists
   - No matching `in_progress` issue in DB
   - `grava doctor` flags it

✅ `grava doctor --dry-run` shows what would be deleted:
   ```
   Orphaned branch: grava/agent-4/task-e
   Status: Would delete (working directory is clean)
   ```

✅ `grava doctor --fix` safely removes:
   - Checks for uncommitted changes first
   - Backs up branch if uncommitted changes found
   - Deletes branch + worktree directory
   - Removes `.worktrees/agent-4/task-e/`

✅ Safeguards prevent silent data loss:
   - Never delete if uncommitted changes exist
   - Always show what will be deleted first
   - Create backup before destructive action

---

## Validation

**Success Criteria**:
1. ✅ Orphaned branch detected:
   ```bash
   # Branch exists
   git branch | grep -q "grava/agent-4/task-e"

   # No active issue
   dolt sql "SELECT COUNT(*) FROM issues WHERE id='task-e' AND status='in_progress'" | grep -q "^0"
   ```

2. ✅ `grava doctor --dry-run` warns:
   ```bash
   grava doctor --dry-run 2>&1 | grep -i "orphaned\|grava/agent-4/task-e"
   ```

3. ✅ Uncommitted changes are detected:
   ```bash
   # Add uncommitted change
   echo "uncommitted work" >> .worktrees/agent-4/task-e/pkg/core/cache.go

   grava doctor --fix 2>&1 | grep -i "uncommitted\|skip"
   # Should NOT delete while uncommitted changes exist
   ```

4. ✅ Clean branch is deleted:
   ```bash
   # Reset uncommitted changes
   cd .worktrees/agent-4/task-e && git checkout .

   grava doctor --fix
   # Backs up, then deletes

   [ ! -d ".worktrees/agent-4/task-e" ]
   git branch | grep -q "grava/agent-4/task-e" || echo "Branch deleted"
   ```

5. ✅ Backup is created:
   ```bash
   # After fix, backup should exist
   ls -la .backups/grava-agent-4-task-e*
   ```

**Test Assertions**:
```bash
# Set up orphaned state
git branch | grep -q "grava/agent-4/task-e"

# Dry-run detects it
grava doctor --dry-run 2>&1 | grep -q "orphaned"

# Uncommitted changes block deletion
echo "work" >> .worktrees/agent-4/task-e/file.txt
grava doctor --fix 2>&1 | grep -q "uncommitted" || [ -d ".worktrees/agent-4/task-e" ]

# Clean state allows deletion
cd .worktrees/agent-4/task-e && git checkout . && cd -
grava doctor --fix
[ ! -d ".worktrees/agent-4/task-e" ]
[ ! -z "$(ls .backups/grava-agent-4-task-e* 2>/dev/null)" ]
```

---

## Cleanup

```bash
# Already cleaned by grava doctor --fix
# Verify no orphans remain
grava doctor
# Should report no issues
```

---

## Critical Safeguards

From [failure-recovery-strategy.md](../../_bmad-output/planning-artifacts/failure-recovery-strategy.md):

```
- Never silent-delete: always check for uncommitted changes first
- Dry-run mode: grava doctor --dry-run shows what would be deleted
- Backup before purge: semi-automatic repair creates snapshot
```

**Strict naming convention enables detection**:
```
grava/{agent-id}/{issue-id}
```

Cross-reference branches against DB: orphan = branch exists but no `in_progress` issue

---

## Branch Naming Convention

```bash
# Valid branch (has active issue)
grava/agent-1/task-a

# Orphaned branch (no active issue)
grava/agent-4/task-e  # But no issue in DB with status='in_progress'
```

---

## Notes

- **Duration**: ~3-5 seconds
- **Safety level**: HIGH (multiple safeguards)
- **Data preservation**: Backup created before deletion
- **User control**: Must explicitly run `--fix` (not automatic)
- **Dry-run essential**: Always run `--dry-run` before `--fix`
