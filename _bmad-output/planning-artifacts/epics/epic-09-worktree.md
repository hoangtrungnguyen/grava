# Epic 9: Worktree Orchestration — Multi-Agent Isolation

**Status:** Planned
**Matrix Score:** 4.05
**FRs covered:** FR23 (worktree init extension), FR5 (claim + worktree creation extended), FR4 (close/stop teardown extended)

## Goal

Developers can initialize Grava in worktree mode (`--enable-worktrees`), giving each agent an isolated Git branch and worktree directory. `grava claim` creates the worktree and checks out a dedicated branch; `grava close`/`stop` tears it down atomically with uncommitted-change guards. The coordinator manages Dolt server lifecycle and exclusive schema migration.

## Commands Delivered

| Command | FR | Description |
|---------|----|-------------|
| `grava init --enable-worktrees` | FR23 | Initialize Grava with worktree/coordinator mode |
| `grava claim <id>` (extended) | FR5 | Claim + create worktree + checkout dedicated branch `grava/<agent-id>/<issue-id>` |
| `grava close <id>` | FR4 | Complete work: atomic teardown with uncommitted-changes guard |
| `grava stop <id>` | FR4 | Pause/abandon: atomic teardown; leaves issue `in_progress` for resume |
| `grava coordinator start` | FR23 | Start long-running coordinator (Dolt server + schema migrations) |

## Branch Naming Convention

- Format: `grava/<agent-id>/<issue-id>`
- Example: `grava/claude-agent-01/abc123def456`
- Enumeration by `grava doctor` for orphaned branch detection (ADR-H6)

## Coordinator Architecture (ADR-FM3)

- Opt-in: only active when `--enable-worktrees` set during `grava init`
- Responsibilities: Dolt server lifecycle, exclusive schema migration execution
- Error channel: `coordinator.Start(ctx) <-chan error` (established in Epic 1 Story 0b)
- No goroutine calls `log.Fatal`/`os.Exit` (Epic 1 constraint)

## Teardown Protocol (two-phase atomic)

**`grava close <id>` (complete):**
1. Check for uncommitted changes in worktree → block if dirty, emit structured warning
2. Transition issue to `done`
3. Delete worktree directory
4. Delete `grava/<agent-id>/<issue-id>` branch
5. Release any active file reservations for this agent

**`grava stop <id>` (pause/abandon):**
1. Check for uncommitted changes → warn but allow with explicit `--force`
2. Transition issue to `paused` (remains `in_progress` semantically for next agent)
3. Preserve Wisp state for resume
4. Delete worktree directory
5. Keep branch for potential resume (clean up via `grava doctor --fix` if orphaned)

## NFR Ownership

| NFR | Role |
|-----|------|
| NFR3 (atomic execution) | *Validated* here — concurrent `grava claim <same-id>` by two agents in worktree mode → exactly one success, one rejection |

## Dependencies

- Epic 3 complete (base claim semantics — Epic 9 **extends**, never replaces)
- Epic 5 complete (doctor must handle orphaned worktree directories and branches)

## Parallel Track

- Can proceed in parallel with Epic 6 (Onboarding) once Epic 5 is complete

## Key Architecture References

- ADR-004: Worktree redirect (`.grava/` resolution chain)
- ADR-FM3: Coordinator opt-in
- ADR-FM5: Wisp lifecycle for resume
- ADR-H6: Orphaned branch detection

## Stories

### Story 9.1: Initialize Grava in Worktree Mode

As a developer,
I want to initialize Grava with worktree mode enabled,
So that the coordinator starts managing the Dolt server lifecycle and each agent gets an isolated branch and directory.

**Acceptance Criteria:**

**Given** a Git repository with `grava init` already run
**When** I run `grava init --enable-worktrees`
**Then** `.grava/config` is updated with `worktrees_enabled=true`
**And** `grava coordinator start` is invoked, launching the long-running coordinator process managing Dolt server lifecycle
**And** the coordinator uses the `Start(ctx) <-chan error` error channel (no `log.Fatal`/`os.Exit` from goroutines)
**And** `grava init --enable-worktrees` is idempotent: re-running when already enabled returns `{"status": "already_enabled"}` without restarting the coordinator
**And** `grava init --enable-worktrees` on a non-Git directory returns `{"error": {"code": "NOT_A_GIT_REPO", ...}}`

---

### Story 9.2: Claim an Issue with Worktree Creation

As an agent,
I want `grava claim` to automatically create a dedicated Git worktree and branch for the claimed issue,
So that my work is isolated from other agents and I have a clean directory to work in.

**Acceptance Criteria:**

**Given** Grava is initialized with `--enable-worktrees` and issue `abc123def456` exists with `status=open`
**When** I run `grava claim abc123def456 --actor agent-01`
**Then** the base claim executes atomically (Epic 3 semantics — `SELECT FOR UPDATE`, `assignee IS NULL` check)
**And** after successful claim: a Git worktree is created at `.grava/worktrees/agent-01/abc123def456/` and a branch `grava/agent-01/abc123def456` is checked out
**And** `grava claim --json` returns `{"id": "abc123def456", "status": "in_progress", "assignee": "agent-01", "worktree_path": ".grava/worktrees/agent-01/abc123def456/", "branch": "grava/agent-01/abc123def456"}`
**And** if two agents concurrently claim the same issue, exactly one succeeds (Epic 3 NFR3 guarantee); the losing agent receives `CLAIM_CONFLICT` and no worktree is created for them
**And** if worktree creation fails after the DB claim succeeds: any partially-created `.grava/worktrees/<agent>/<issue>/` directory is deleted before the DB claim rollback executes — no partial state (directory or DB)
**And** if the directory deletion itself fails during rollback, the error is logged with `WORKTREE_CLEANUP_FAILED` code and `grava doctor` will detect and clean the orphaned directory on next run

---

### Story 9.3: Complete Work — Atomic Teardown with `grava close`

As an agent,
I want to atomically tear down my worktree when I complete an issue,
So that the branch and directory are cleaned up without leaving orphaned state.

**Acceptance Criteria:**

**Given** agent `agent-01` has an active worktree for issue `abc123def456` at `.grava/worktrees/agent-01/abc123def456/`
**When** I run `grava close abc123def456`
**Then** the worktree directory is checked for uncommitted changes — if dirty, the close is blocked with `{"error": {"code": "UNCOMMITTED_CHANGES", "message": "Worktree has uncommitted changes. Commit or use --force to abandon."}}`
**And** if clean (or `--force` given): issue transitions to `done`; worktree directory is deleted; branch `grava/agent-01/abc123def456` is deleted
**And** any active file reservations held by `agent-01` for this issue are released (`released_ts=NOW()`)
**And** `grava close --json` returns `{"id": "abc123def456", "status": "done", "worktree_removed": true, "branch_deleted": true, "reservations_released": 1}`

---

### Story 9.4: Pause Work — Atomic Teardown with `grava stop`

As an agent,
I want to pause and abandon my worktree without losing the issue state for the next agent,
So that another agent can resume the work from my last Wisp checkpoint.

**Acceptance Criteria:**

**Given** agent `agent-01` has an active worktree for issue `abc123def456`
**When** I run `grava stop abc123def456`
**Then** if the worktree has uncommitted changes, a warning is emitted: `{"warning": "Uncommitted changes in worktree — use --force to proceed"}` and the stop is blocked unless `--force` is passed
**And** with `--force` or clean worktree: issue transitions to `paused`; Wisp state is preserved for next agent resume; worktree directory is deleted; branch `grava/agent-01/abc123def456` is kept (not deleted — available for resume or `grava doctor --fix` cleanup)
**And** `grava stop --json` returns `{"id": "abc123def456", "status": "paused", "worktree_removed": true, "branch_kept": "grava/agent-01/abc123def456", "wisp_preserved": true}`
**And** a subsequent `grava claim abc123def456 --actor agent-02` by a new agent succeeds; `grava history abc123def456` shows the full prior context from Wisp entries
