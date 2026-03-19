# Scenario 1: Happy Path — Parallel Execution Without Conflicts

**Status**: Happy path validation
**Source**: Phase 2 baseline requirement
**Priority**: CRITICAL (all other scenarios depend on this working)

---

## Overview

Two agents simultaneously claim two different branches, complete independent work, and merge cleanly without conflicts.

## Setup

### Prerequisites
- Grava running with Dolt database initialized
- 2 worktrees available
- Network connection for agent coordination

### Steps

1. **Create two independent tasks** in backlog:
   - Task A: "Update README in docs/"
   - Task B: "Add new command in cmd/"

2. **Start Agent 1**:
   ```bash
   grava claim task-a
   # Agent claims branch: grava/agent-1/task-a
   ```

3. **Start Agent 2** (simultaneously):
   ```bash
   grava claim task-b
   # Agent claims branch: grava/agent-2/task-b
   ```

4. **Agent 1 makes changes**:
   - Modifies `docs/README.md`
   - Commits with message: "docs: update README"

5. **Agent 2 makes changes** (in parallel):
   - Creates `pkg/cmd/new-command.go`
   - Commits with message: "feat: add new command"

6. **Both agents finish**:
   ```bash
   # Agent 1
   grava close task-a

   # Agent 2
   grava close task-b
   ```

---

## Expected Behavior

✅ Agent 1 claims `task-a` successfully
✅ Agent 2 claims `task-b` simultaneously (no conflict)
✅ Both agents have isolated worktrees with separate branches
✅ Agent 1 modifications don't affect Agent 2's branch
✅ Agent 2 modifications don't affect Agent 1's branch
✅ Both agents commit and close independently
✅ When both branches merge to main: no conflicts, both changes present

---

## Validation

**Success Criteria**:
1. ✅ Both agents can claim simultaneously → DB allows dual `in_progress` states
2. ✅ Worktrees are isolated → changes in worktree-1 don't appear in worktree-2
3. ✅ Commits are independent → git logs show both commits on respective branches
4. ✅ Merge succeeds → both changes integrate cleanly into main
5. ✅ No conflict markers → `git diff main..merged` shows expected 2 files changed

**Test Assertions**:
```bash
# After both agents close:
[ $(git log main..grava/agent-1/task-a | wc -l) -eq 1 ]  # Agent 1 has 1 commit
[ $(git log main..grava/agent-2/task-b | wc -l) -eq 1 ]  # Agent 2 has 1 commit
git merge grava/agent-1/task-a --no-edit                 # Should succeed
git merge grava/agent-2/task-b --no-edit                 # Should succeed
[ $(git status --porcelain | wc -l) -eq 0 ]              # No conflicts
```

---

## Cleanup

```bash
# Delete both branches after merge
git branch -D grava/agent-1/task-a
git branch -D grava/agent-2/task-b

# Mark both tasks as closed in DB
dolt table cat issues | grep task-a | update status=closed
dolt table cat issues | grep task-b | update status=closed
```

---

## Notes

- **Duration**: ~5-10 seconds
- **Network requirements**: Local coordination only (same machine)
- **Dependencies**: All other scenarios assume this works
- **Risk**: If this fails, multi-branch orchestration is fundamentally broken
