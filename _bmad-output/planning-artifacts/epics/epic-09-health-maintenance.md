# Epic 9: Workspace Health & Maintenance

**Status:** Planned
**Grava ID:** grava-46af
**Matrix Score:** 4.15
**FRs covered:** FR14, FR15, FR16, FR17

## Goal

Developers can audit the full command history, safely undo recent state changes, compact stale data, and run `grava doctor` (with `--fix` and `--dry-run`) to diagnose and repair ghost worktrees, orphaned branches, stale agents, expired file leases, and lock file corruption — with backup-before-purge safety.

## Commands Delivered

| Command | FR | Description |
|---------|----|-------------|
| `grava cmd_history [--limit N]` | FR14 | Retrieve ledger of executed system commands |
| `grava undo [--steps N]` | FR15 | Revert recent state-altering commands |
| `grava compact` | FR16 | Prune expired/deleted data for query performance |
| `grava doctor` | FR17 | Diagnostic health report — no action |
| `grava doctor --fix` | FR17 | Semi-automatic repair with backup-before-purge |
| `grava doctor --dry-run` | FR17 | Show what `--fix` would do without executing |

## Doctor Check Set (Phase 1 — 12 checks total)

**From ADR-FM7 (7 mandatory):**
1. Stale `in_progress` issues (no heartbeat update in N hours)
2. Orphaned branch detection (ADR-H6)
3. Schema version mismatch (`.grava/schema_version` vs DB)
4. Coordinator process health (if `--enable-worktrees` active)
5. `.git/info/exclude` entry presence (ADR-H5)
6. Hook stub integrity (`grava init` stubs match expected content)
7. Dolt server connectivity (if coordinator active)

**Extended checks (from Failure Recovery Strategy):**
8. `in_progress` issues with expired Wisp heartbeat → stale agent
9. Worktree directories with no corresponding `in_progress` issue → orphaned directory
10. `grava/*/*` branches with no matching active issue → orphaned branch (ADR-H6)
11. Stale lock files (`.archive.lock`, `.commit.lock`) → crashed mid-commit
12. Expired file reservations not yet released → TTL cleanup needed

## `--fix` Safety Protocol

- **backup-before-purge:** Creates state snapshot before any destructive action
- Structured repair report: `{check, status, action_taken, snapshot_path}`
- `--dry-run`: identical report but `action_taken = "skipped (dry-run)"`
- Orphaned branch recovery: check for uncommitted changes first — if dirty, flag for human review; if clean, delete + remove worktree directory

## Dependencies

- Epic 1 complete
- Epic 2 complete (issues table must exist for `undo`)
- Epic 3 complete (Wisp heartbeat required for check #8)

## NFR Ownership

| NFR | Role |
|-----|------|
| NFR2 (<15ms writes) | *Validation point* — `compact` must not degrade write throughput; benchmark before/after |

## Key Architecture References

- ADR-FM5: Wisp lifecycle — issue stays `in_progress` after lease expiry for next agent resume
- ADR-FM7: Doctor Phase 1 mandatory checks
- ADR-H6: Orphaned branch detection
- ADR-H5: `.git/info/exclude` (not `.gitignore`)

## Downstream Dependencies

- Epic 10 (Sandbox Validation)

## Stories

### Story 9.1: Command History Ledger *(grava-25ea)*

As a developer or agent,
I want to retrieve a ledger of all previously executed Grava commands,
So that I can audit what actions were taken and by whom in the workspace.

**Acceptance Criteria:**

**Given** several `grava` commands have been run in the workspace
**When** I run `grava cmd_history`
**Then** an ordered list of past commands is returned: `{command, actor, timestamp, args, exit_code}` — most recent first
**And** `grava cmd_history --limit 20` returns the 20 most recent entries
**And** `grava cmd_history --actor agent-01` filters to only commands run by `agent-01`
**And** `grava cmd_history --json` returns a JSON array conforming to NFR5 schema
**And** every write command (create, update, claim, drop, etc.) is automatically recorded to the `cmd_audit_log` table as part of `WithAuditedTx`

---

### Story 9.2: Undo Recent State Changes *(grava-cdd0)*

As a developer,
I want to safely revert the most recent state-altering commands,
So that I can recover from accidental or incorrect operations without manual DB intervention.

**Acceptance Criteria:**

**Given** the last command was `grava update abc123def456 --status done`
**When** I run `grava undo`
**Then** the issue reverts to its previous status (`in_progress`) using the snapshot stored in `cmd_audit_log`
**And** `grava undo --steps 3` reverts the last 3 undoable commands in reverse order
**And** `grava undo --dry-run` shows what would be reverted without executing
**And** commands that cannot be undone (e.g., `grava clear` — hard delete) display `{"error": {"code": "NOT_UNDOABLE", "message": "clear operations cannot be undone — data was permanently deleted"}}`
**And** undo itself is recorded in `cmd_audit_log` as an `undo` event

---

### Story 9.3: Compact Stale and Deleted Data *(grava-213e)*

As a developer or agent,
I want to prune expired and soft-deleted data from the database,
So that query performance is maintained as the workspace grows.

**Acceptance Criteria:**

**Given** the workspace has archived issues and expired Wisp entries older than 30 days
**When** I run `grava compact`
**Then** archived issues older than the configured retention period are hard-deleted from the DB
**And** `wisp_entries` for done/archived issues older than the retention period are purged
**And** expired `cmd_audit_log` entries (beyond retention window) are pruned
**And** `grava compact` returns a summary: `{"issues_purged": 5, "wisp_entries_purged": 42, "audit_log_entries_purged": 100}`
**And** `grava compact --dry-run` shows what would be purged without executing
**And** write throughput benchmark before and after compact shows no regression on NFR2 (<15ms per write)

---

### Story 9.4: Doctor — Diagnostic Health Report *(grava-490e)*

As a developer,
I want to run `grava doctor` to get a full health report of my workspace,
So that I can proactively detect issues before they become failures.

**Acceptance Criteria:**

**Given** a Grava workspace that may have stale state
**When** I run `grava doctor`
**Then** all 12 health checks execute and produce a structured report (diagnostic only — no mutations)
**And** checks include: (1) stale `in_progress` > 1h, (2) orphaned branch, (3) schema version mismatch, (4) coordinator health, (5) `.git/info/exclude` entry, (6) hook stub integrity, (7) Dolt connectivity, (8) expired Wisp heartbeat, (9) orphaned worktree directory, (10) orphaned branch cross-ref, (11) stale lock files, (12) expired file reservations
**And** `grava doctor --json` outputs `{"checks": [{"name": "...", "status": "pass|warn|fail", "detail": "..."}]}`
**And** exit code is `0` if all checks pass, `1` if any check is `warn`, `2` if any check is `fail`
**And** `grava doctor` makes no modifications to the workspace (read-only)

---

### Story 9.5: Doctor — Semi-Automatic Repair with Backup Safety *(grava-c472)*

As a developer,
I want to run `grava doctor --fix` to semi-automatically repair detected issues,
So that I can resolve workspace problems safely without risking data loss.

**Acceptance Criteria:**

**Given** `grava doctor` has identified fixable issues (e.g., stale lock file, expired reservation)
**When** I run `grava doctor --dry-run`
**Then** a report shows exactly what `--fix` would do for each detected issue, with no changes made
**And** when I run `grava doctor --fix`, a state snapshot is created at `.grava/backups/doctor-<timestamp>/` before any destructive action (backup-before-purge)
**And** for each orphaned branch: if dirty → flagged for human review and skipped; if clean → branch deleted and worktree directory removed
**And** for each expired file reservation → TTL auto-release executed; issue status preserved as `in_progress`
**And** for each stale lock file → lock cleared with structured log entry
**And** `grava doctor --fix` emits a structured report: `{check, status, action_taken, snapshot_path, skipped_reason}`
**And** `grava doctor --fix` on an already-clean workspace completes with `{"actions_taken": 0, "message": "Workspace is healthy"}`
