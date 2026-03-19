# Failure Recovery Strategy — Multi-Agent Orchestrator

> Sources: Internal ADRs (ADR-004, ADR-FM5, ADR-FM7) + mcp_agent_mail patterns

---

## Overview

Three failure scenarios require dedicated recovery strategies:

1. **Agent crashes mid-execution** — partial work is left in an unknown state
2. **Worktree cleanup after a crash** — ghost directories and stale DB records
3. **Orphaned branches left behind** — dangling Git branches with no active agent

---

## Failure 1: Agent Crashes Mid-Execution

### Detection
- Monitor for **expired TTL** on file reservations (stale leases indicate agent death)
- Detect stale system lock files: `.archive.lock`, `.commit.lock`
- `in_progress` issues with no recent Wisp activity past a configurable heartbeat threshold

### Recovery Algorithm
1. TTL expiry triggers automatic lease release via background cleanup job
2. Issue remains in `in_progress` state — preserved for next claimant
3. Next agent to claim reads **Wisps** (ephemeral activity logs) to reconstruct prior state
4. Agent resumes from last known good checkpoint — no work duplication
5. On successful task close, Wisps are deleted

### Data Structures
```sql
-- Wisps: ephemeral per-task activity log (ADR-FM5)
wisps (
  id         TEXT PRIMARY KEY,
  issue_id   TEXT REFERENCES issues(id),
  agent_id   TEXT,
  log_entry  TEXT,
  created_ts TIMESTAMP
  -- deleted atomically on task close
)
```

### mcp_agent_mail Integration
- `doctor` utility (`notebooklm doctor`) heals stale process locks left by crashed agents
- Semi-automatic repair: detects orphaned records, creates backup, purges safely
- Crashed agent's file leases auto-expire; overlapping-lease auto-allow lets next agent contact the prior one's thread for context

---

## Failure 2: Worktree Cleanup After a Crash

### Detection
`grava doctor` runs the following checks (ADR-FM7 Phase 1 Minimum Check Set):

| Check | Signal |
|-------|--------|
| `in_progress` issue exists but `.worktrees/{actor}/` directory is missing | Ghost worktree in DB |
| `.worktrees/{actor}/` directory exists but issue is `closed` | Orphaned directory on disk |
| Leftover directories from past sessions with no DB record | Stale filesystem artifact |

### Recovery Algorithm
1. **Idempotent teardown:** `grava close` / `grava stop` checks if worktree directory exists before attempting removal — skips silently if already gone, proceeds to update DB state
2. **Ghost resume:** If agent calls `grava claim` and a worktree for that issue already exists on disk → system automatically resumes and reuses the existing worktree (CM-5 pattern)
3. **Two-phase atomic teardown:**
   - Phase 1: Check for uncommitted changes → abort if found (prevent silent data loss)
   - Phase 2: Remove directory + update DB status atomically
4. `grava doctor --fix` runs semi-automatic repair: backs up orphaned state, then purges

### Branch Naming Convention
```
grava/{agent-id}/{issue-id}
```
Strict naming enables `doctor` to cross-reference branches against DB records and identify orphans programmatically.

### mcp_agent_mail Integration
- `doctor` pattern borrowed directly: diagnostic checks → backup → safe purge
- Human Overseer can broadcast emergency halt if `doctor` detects widespread corruption

---

## Failure 3: Orphaned Branches Left Behind

### Detection
- Scan all branches matching `grava/*/*` pattern
- Cross-reference against `issues` table: branch exists but issue is `closed` or `open` (never `in_progress`) → orphan
- Check for uncommitted changes inside branch worktree before any deletion

### Recovery Algorithm
1. `grava doctor` enumerates all `grava/{agent-id}/{issue-id}` branches
2. For each branch not linked to an `in_progress` issue:
   - If worktree has **uncommitted changes** → flag for human review, skip deletion
   - If worktree is clean → delete branch + remove worktree directory
3. Emit structured report: `{branch, status, action_taken, skipped_reason}`

### Safeguards
- **Never silent-delete**: always check for uncommitted changes first
- **Dry-run mode**: `grava doctor --dry-run` shows what would be deleted without acting
- **Backup before purge**: semi-automatic repair creates snapshot before destructive action

---

## The `grava doctor` Command (Unified Entry Point)

Borrowed from mcp_agent_mail's `doctor` utility — a single command that heals all three failure types:

```
grava doctor [--fix] [--dry-run]
```

| Flag | Behavior |
|------|----------|
| *(none)* | Diagnostic only — report issues, take no action |
| `--fix` | Semi-automatic repair with backup-before-purge |
| `--dry-run` | Show what `--fix` would do without executing |

### Phase 1 Minimum Check Set (ADR-FM7)

```
[ ] in_progress issues with expired Wisps heartbeat     → stale agent
[ ] in_progress issues with no worktree directory       → ghost DB record
[ ] worktree directories with no in_progress issue      → orphaned directory
[ ] branches grava/*/* with no matching issue           → orphaned branch
[ ] stale lock files (.archive.lock, .commit.lock)      → crashed mid-commit
[ ] expired file reservations not yet released          → TTL cleanup needed
```

---

## Recovery Decision Tree

```
Agent stops responding
        │
        ▼
TTL on file lease expires?
   YES → auto-release lease, leave issue in_progress
   NO  → wait for heartbeat timeout
        │
        ▼
grava doctor detects stale state
        │
        ├─ Ghost worktree (DB says in_progress, no dir) → update DB to open
        ├─ Orphaned dir (dir exists, issue closed)      → backup + remove dir
        ├─ Orphaned branch (branch, no active issue)    → check uncommitted → delete or flag
        └─ Stale lock file                              → doctor heals lock
        │
        ▼
Next agent claims issue
        │
        ▼
Read Wisps → resume from checkpoint
        │
        ▼
Task completes → Wisps deleted → branch deleted → worktree removed
```

---

## Cross-Cutting Patterns

| Pattern | Source | Purpose |
|---------|--------|---------|
| **Wisps** (ADR-FM5) | Internal | Ephemeral crash-resume logs, deleted on close |
| **TTL leases** | mcp_agent_mail | Auto-release file reservations after agent death |
| **`doctor` utility** | mcp_agent_mail | Unified diagnostic + repair entry point |
| **Semi-automatic repair** | mcp_agent_mail | Backup-before-purge for orphaned records |
| **Idempotent teardown** | Internal (CM-5) | `close`/`stop` safe to re-run after partial failure |
| **Uncommitted change guard** | Internal | Never delete branch with unsaved work |
