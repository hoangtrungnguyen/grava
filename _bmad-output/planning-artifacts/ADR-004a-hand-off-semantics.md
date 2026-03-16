---
title: "ADR-004a: Concurrent Agent Hand-off and Worktree Management"
date: 2026-03-16
author: Winston (Architect)
status: Approved
phase: Phase 1
related: ADR-004, ADR-FM4, ADR-FM5
---

# ADR-004a: Concurrent Agent Hand-off and Worktree Management

## Problem Statement

ADR-004 establishes the redirect architecture for worktree isolation, but leaves **unspecified** how agents hand off work when:
1. Agent A claims an issue and crashes without cleanup
2. Agent B claims the same issue
3. Orphaned worktrees (`.worktrees/agent-a/`) still exist on disk
4. Multiple agents may have branches for the same issue

This creates ambiguity in:
- Worktree collision handling
- Wisps ownership during hand-off
- Git branch naming when agents rotate
- Cleanup semantics for orphaned state

This ADR resolves these tensions with explicit decisions.

---

## Core Decisions

### Decision 1: Single-Branch-Per-Issue Model

**Choice:** One Git branch per issue, shared across all agents who work on it.

**Branch naming:** `grava/{issue-id}` (not `grava/{agent-id}/{issue-id}`)

**Rationale:**
- Simplifies merge driver logic — one branch = one merge resolution path
- Avoids branch proliferation (N agents × M issues = NM branches; now just M)
- When Agent B takes over from Agent A, they check out the **same branch** with all prior commits
- Git history shows the full chain of work, not per-agent silos
- Easier to audit ("who worked on this issue and when") via `git log grava/{issue-id}`

**Implication:** Git worktree paths stay per-agent (`.worktrees/agent-a/`, `.worktrees/agent-b/`), but they all point to the same branch. This is safe and intended.

---

### Decision 2: Hand-off with Orphaned Worktree Detection

**Choice:** Option C (from architect review) — Explicit detection + warning + `--force-claim` override.

**Workflow:**

```
Agent B runs: grava claim issue-123

Step 1: Check DB lock
  └─ If issue is in_progress with actor=agent-a, proceed (no error)

Step 2: Check for orphaned worktrees
  └─ Scan .worktrees/ for any {actor}/ directory with a branch for issue-123
  └─ If found:
     ├─ Surface warning: "⚠️  issue-123 has an active worktree at .worktrees/agent-a/
     │   (branch: grava/issue-123). Agent A may still be working or has crashed.
     │   Override with --force-claim to take over. Existing work will be preserved in Git history."
     └─ PAUSE and wait for user input

Step 3 (if --force-claim passed or no orphaned worktree found):
  └─ Create new worktree: git worktree add .worktrees/agent-b/ grava/issue-123
  └─ Update DB: issue.actor → agent-b, issue.status → in_progress
  └─ Write .grava/redirect in .worktrees/agent-b/
```

**Rationale:**
- **Safety:** Never silently delete an agent's worktree. Explicit override prevents accidental loss.
- **Visibility:** User sees the warning and understands they're taking over from Agent A.
- **Recoverability:** If Agent A is actually still alive (slow/blocked), warning gives them a chance to finish.
- **Auditability:** `--force-claim` in command line history shows explicit intent to hand off.

**Non-goal:** We do NOT automatically clean up orphaned worktrees. That's a manual `git worktree remove` operation, which is safe and reversible.

---

### Decision 3: Wisps as Shared Activity Log

**Choice:** Wisps table is shared across all agents on an issue. New agent appends their activities.

**Schema implications:**

```sql
CREATE TABLE wisps (
  id           VARCHAR(12) PRIMARY KEY,
  issue_id     VARCHAR(12) NOT NULL,
  actor        VARCHAR(255) NOT NULL,  ← Agent who wrote this wisp entry
  activity     TEXT NOT NULL,           ← What the agent did
  created_at   TIMESTAMP DEFAULT NOW(),
  FOREIGN KEY (issue_id) REFERENCES issues(id)
);
```

**Behavior:**

1. Agent A works on issue-123, writes wisps:
   ```
   issue-123, agent-a, "Added comment: needs review"
   issue-123, agent-a, "Ran tests: 5 passed, 1 skipped"
   issue-123, agent-a, "Crash! Process killed."
   ```

2. Agent B claims issue-123, sees wisps via `grava show issue-123`:
   ```
   [WISPS LOG]
   agent-a: Added comment: needs review
   agent-a: Ran tests: 5 passed, 1 skipped
   agent-a: (last entry 4 hours ago)
   ```

3. Agent B continues, appends new wisps:
   ```
   issue-123, agent-b, "Resuming. Agent A left off 4h ago. Checking test status."
   ```

4. On issue `closed`: all wisps for that issue are deleted (ADR-FM5 cleanup).

**Rationale:**
- Provides crash-safe **context handoff** without polluting permanent `events` table
- Each wisp records the actor, so you can see who did what
- Natural append-only log mirrors Git commit history
- Cleaned up on close (bounded storage)

**Implication:** `grava show <id>` must include recent wisps summary in output (JSON + human-readable).

---

### Decision 4: Marker File Protocol for Partial Teardown

**Choice:** Use `.grava/teardown-in-progress/` directory with per-issue-actor marker files.

**File structure:**

```
.grava/
  └── teardown-in-progress/
      ├── agent-a-issue-123.marker     ← Plain text, contains phase name
      └── agent-b-issue-456.marker
```

**Marker file content (one line):**

```
phase=git-worktree-removed
or
phase=wisps-marked-for-deletion
or
phase=complete
```

**Two-phase teardown with recovery:**

```
Phase 1: Git cleanup (worktree + branch)
  1. Write .grava/teardown-in-progress/{actor}-{issue-id}.marker → "phase=git-cleanup"
  2. Check for uncommitted changes in .worktrees/{actor}/
  3. Run: git worktree remove .worktrees/{actor}/
  4. Run: git branch -D grava/{issue-id}
  5. On success: Update marker → "phase=git-cleanup-done"
  6. On failure: Surface error, marker stays at "git-cleanup", abort before DB write

Phase 2: DB update
  1. Update marker → "phase=db-update"
  2. BEGIN TRANSACTION
  3. Update issue.status → closed/open, issue.actor → NULL
  4. Mark wisps for deletion (if close) or leave untouched (if stop)
  5. COMMIT TRANSACTION
  6. On success: Delete marker file
  7. On failure: Marker stays at "db-update", transaction rolls back
```

**Crash recovery (next agent or same agent):**

1. `grava doctor` detects marker files and reports:
   ```
   ⚠️  Partial teardown detected:
       .grava/teardown-in-progress/agent-a-issue-123.marker (phase: db-update)
       Issue: issue-123 still in_progress, worktree already removed.
       Action: Run 'grava recover' to complete.
   ```

2. `grava recover` (new command) resumes from the last complete phase:
   ```
   grava recover
     └─ Scans teardown-in-progress/ for all markers
     └─ For each marker:
        ├─ If phase=git-cleanup: Resume at Phase 2 (DB update)
        ├─ If phase=db-update: Resume DB transaction
        └─ On successful completion: Delete marker
   ```

**Rationale:**
- Explicit two-phase design ensures crash recovery is deterministic
- Marker files survive process death (stored on disk, readable without DB connection)
- `grava recover` is idempotent — can be run multiple times safely
- `grava doctor` surfaces the issue without requiring user to understand internals

---

### Decision 5: Cleanup of Orphaned Wisps

**Choice:** Wisps are preserved during hand-off (Agent B sees Agent A's wisps). On issue `close`, all wisps are deleted.

**Behavior:**

```
grava close issue-123
  1. Check for uncommitted changes (abort if any)
  2. Run git worktree remove + branch deletion
  3. BEGIN TRANSACTION
  4. UPDATE issues SET status='closed' WHERE id='issue-123'
  5. DELETE FROM wisps WHERE issue_id='issue-123'  ← Clean slate
  6. COMMIT
  7. Delete marker file
```

**Vs `grava stop` (abandon without closing):**

```
grava stop issue-123
  1. Check for uncommitted changes (abort if any)
  2. Run git worktree remove + branch deletion
  3. BEGIN TRANSACTION
  4. UPDATE issues SET status='open', actor=NULL WHERE id='issue-123'
  5. DO NOT delete wisps  ← Preserve for next agent
  6. COMMIT
  7. Delete marker file
```

**Rationale:**
- `close` = final decision, clean up ephemeral state (wisps)
- `stop` = temporary abandon, preserve context for next agent
- Wisps table self-bounds (only active/in_progress issues have wisps)
- On periodic `grava compact`, delete wisps for closed issues older than retention period (e.g., 7 days)

---

### Decision 6: `issues.jsonl` as Read-Only Export

**Choice:** `issues.jsonl` is **NOT** the source of truth. Dolt DB is.

**`issues.jsonl` lifecycle:**

1. **Exported** by `grava export` command or Git post-merge hook
2. **Contents:** Snapshot of all open + in_progress issues at export time (JSON Lines format, one issue per line)
3. **Commit:** Committed to Git history for version control and offline reference
4. **Read:** Agents can `cat issues.jsonl | jq` for quick reference, but must not assume it's current
5. **Sync:** On `grava sync`, issues.jsonl is re-generated from DB and committed

**Why not source of truth?**
- Git worktrees on different branches may have divergent `issues.jsonl` files
- Merge conflicts on .jsonl are hard to resolve automatically
- DB is the single atomic source; .jsonl is a convenience view
- Prevents the footgun of "I edited issues.jsonl and expected the DB to change"

**Implication:** CLI always reads from Dolt DB, never from `issues.jsonl`. The file is for human reference and Git history only.

**Implementation:** Add validation in `grava doctor`:
```
grava doctor
  └─ Check: Are open/in_progress issues in DB consistent with issues.jsonl?
  └─ If diverged: ⚠️  warning: "issues.jsonl is out of sync. Run 'grava sync' to update."
```

---

## Implementation Checklist

- [ ] **Branch naming:** Change from `grava/{agent-id}/{issue-id}` to `grava/{issue-id}`
- [ ] **Worktree paths:** Keep `.worktrees/{actor}/` (per-agent directories)
- [ ] **Claim command:** Add `--force-claim` flag; detect orphaned worktrees; surface warning + pause
- [ ] **Wisps schema:** Add `actor` column; include in `grava show` output
- [ ] **Marker file protocol:** Implement `.grava/teardown-in-progress/` directory and two-phase teardown
- [ ] **`grava recover` command:** New command to resume partial teardowns
- [ ] **`grava doctor` enhancements:** Detect marker files, report orphaned worktrees, validate issues.jsonl sync
- [ ] **`grava close` vs `grava stop`:** Both trigger two-phase teardown; `close` deletes wisps, `stop` preserves
- [ ] **`grava compact` enhancements:** Delete wisps for closed issues older than retention period
- [ ] **Git hook for export:** Post-merge hook regenerates issues.jsonl from DB and commits
- [ ] **CLI validation:** Reject any attempt to mutate `issues.jsonl` directly; raise `error: issues.jsonl is read-only. Use 'grava sync' to update.`

---

## Alternatives Considered

### Alt 1: Multiple Branches Per Agent (Rejected)
- **Branch model:** `grava/{agent-id}/{issue-id}`
- **Pro:** Explicit separation, no branch contention
- **Con:** NM branch explosion; merge driver must handle N parallel branches per issue; harder to audit full work history
- **Decision:** Rejected in favor of single-branch-per-issue (Decision 1)

### Alt 2: Silent Orphaned Worktree Cleanup (Rejected)
- **Behavior:** When Agent B claims, automatically delete Agent A's worktrees
- **Pro:** No user interaction required
- **Con:** Dangerous if Agent A is still alive; silent data loss if Agent A has uncommitted work
- **Decision:** Rejected in favor of explicit warning + `--force-claim` (Decision 2)

### Alt 3: Per-Agent Wisps Tables (Rejected)
- **Model:** `wisps_agent_a`, `wisps_agent_b`, etc.
- **Pro:** Clear isolation per agent
- **Con:** Schema fragmentation; hard to audit full issue history; hand-off requires joining tables; schema changes require new tables
- **Decision:** Rejected in favor of single shared wisps table with actor column (Decision 3)

### Alt 4: Synchronous Marker File Cleanup (Rejected)
- **Behavior:** Immediately delete marker file on teardown success
- **Pro:** No recovery code needed
- **Con:** If deletion fails (permissions), system is left in unknown state; markers valuable for post-mortem
- **Decision:** Rejected in favor of explicit recovery protocol (Decision 4)

---

## Risk Mitigation

| Risk | Mitigation |
|---|---|
| Agent B accidentally overwrites Agent A's work | `--force-claim` requires explicit intent; warning surfaces risk |
| Orphaned worktrees accumulate on disk | `grava doctor` detects; user can manually clean with `git worktree remove` |
| Partial teardown leaves DB in inconsistent state | Marker file protocol + `grava recover` command ensures deterministic recovery |
| `issues.jsonl` diverges from DB | `grava doctor` warning on mismatch; `grava sync` refreshes |
| Wisps table grows unbounded | `grava compact` deletes old wisps; cleanup on close removes per-issue wisps |

---

## Operational Guidance

### For Single-Agent Workflows
- No change. `grava claim`, work, `grava close` — marker files never created.

### For Multi-Agent Swarms
- **Monitor:** Set up cron job to run `grava doctor` periodically (e.g., every 5 min)
- **Alert on marker files:** If `grava doctor` reports partial teardowns, run `grava recover` immediately
- **Monitor stale claims:** Set threshold (e.g., 24h) for issues stuck in `in_progress`; run `grava stop <id>` to reset
- **Audit hand-offs:** Query wisps table to see which agents worked on each issue: `dolt sql "SELECT DISTINCT actor, issue_id FROM wisps ORDER BY issue_id"`

### For Crash Recovery
- User should run `grava recover` after unexpected outages
- `grava doctor` surfaces the issue; user runs `grava recover` to resume
- No data loss — marker files ensure recovery is always possible

---

## Approval

- [x] Architect (Winston): Approved
- [ ] Dev Lead: Pending
- [ ] QA: Pending
- [ ] Product: Pending

---

**Next Step:** Hand off to dev team with implementation checklist. ADR-004a is production-ready.
