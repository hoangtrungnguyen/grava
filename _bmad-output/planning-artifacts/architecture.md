---
stepsCompleted:
  - step-01-init
  - step-02-context
inputDocuments:
  - _bmad-output/planning-artifacts/prd.md
  - docs/architecture/GRAPH_MECHANICS.md
  - docs/architecture/PERFORMANCE_BENCHMARKS.md
  - docs/guides/AGENT_WORKFLOWS.md
  - docs/guides/CLI_REFERENCE.md
workflowType: 'architecture'
project_name: 'grava'
user_name: 'Htnguyen'
date: '2026-03-11'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

---

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**
25 FRs across 5 domains: Issue Lifecycle (FR1–7), Graph Context & Discovery (FR8–13),
State History & Maintenance (FR14–17), Ephemeral State/Wisp (FR18–19), and Workspace
Sync & Git Integration (FR20–25). The atomic `claim` command and the `ready` queue are
the two most architecturally significant primitives — they define the agent execution
contract and drive all concurrency requirements.

**Non-Functional Requirements:**
- NFR1: <100ms read latency at 10K issues → requires in-memory graph with cache
- NFR2: <15ms write latency, >70 inserts/sec → confirmed met by benchmarks (~5–13ms)
- NFR3: Exactly-once claim under concurrent agents → row-level exclusive locking
- NFR4: Lossless JSONL export/import → strict schema discipline + all-or-nothing transaction
- NFR5: JSON output schema versioned → breaking changes = major version bump
- NFR6: Single statically-linked binary → no Python/shell runtime deps in core paths

**Scale & Complexity:**
The system is a high-complexity, local-first CLI substrate for autonomous agent swarms.
Expected: up to 30 concurrent agents, 10,000+ issues, hundreds of writes/minute during
active sprints.

- Primary domain: CLI Tool / Local-first AI Agent Orchestration
- Complexity level: High
- Estimated architectural components: 8 (CLI layer, graph engine, DB layer, ID generator,
  migration engine, export/import pipeline, Git hook integration, ephemeral store)

### Technical Constraints & Dependencies

- **Dolt (MySQL-protocol)**: Only supported persistence backend. All SQL must be
  MySQL-compatible. Row-level locking via `SELECT ... FOR UPDATE` is required for
  subtask ID atomicity and exclusive issue modification.
- **Single binary (NFR6)**: Git hooks and agent scheduler must eventually be compiled
  into the Grava binary or this NFR is violated. Shell/Python dependencies are a
  transitional violation to be resolved.
- **Git integration**: Merge driver registration is per-repo and requires `.gitattributes`
  configuration — an installation/init UX concern.
- **12-char base ID**: Expanded from 4 chars to 12 hex chars (~4 billion combinations).
  Birthday collision threshold: ~65K issues at 1% probability — sufficient for project scale.

### Cross-Cutting Concerns Identified

1. **Concurrency & Locking**: All write paths must be safe for 30 concurrent agents.
   Issue rows are exclusively locked during any modification — other agents are blocked
   from claiming or modifying the same issue until the lock is released. `SELECT FOR UPDATE`
   on `child_counters` ensures atomic subtask ID generation.
2. **Audit Logging**: Every mutation must be captured in the `events` table with actor + model.
3. **Actor Identity**: `--actor` flag is the agent identity primitive — must be consistent across all commands.
4. **JSON Schema Contract**: Strict versioning discipline across all `--json` outputs.
5. **Graph Coherence**: In-memory AdjacencyDAG and Dolt DB must remain consistent — no divergence.
   Graph is hydrated per-invocation with lazy loading for read-heavy commands (e.g. `ready`
   loads only `open` issues). Optional daemon mode holds the graph in memory across invocations
   for high-frequency swarm workloads.
6. **Git Sync Pipeline**: Hook scripts → export → merge driver → import must preserve full
   state integrity. Import is wrapped in a single all-or-nothing DB transaction. Merge driver
   uses last-write-wins for field-level conflicts, with all resolution decisions logged.

### Pre-mortem Risk Decisions

| Risk | Decision |
|---|---|
| ID collision (4-char = 65K combinations) | Expand base ID to **12 hex chars** (~4B combinations) |
| Graph hydration stampede at 30-agent scale | **Lazy load** open issues only + **configurable connection pool** + **optional daemon mode** |
| Subtask ID race condition | `SELECT FOR UPDATE` on `child_counters` row; issue row exclusively locked during any modification |
| Partial import state corruption | Entire import wrapped in a **single DB transaction** (all-or-nothing) |
| Merge driver silent data loss | **Last-write-wins** for field-level conflicts; all resolution decisions logged |

### Architectural Evolution Paths

**Phase Priority:**
Phase 1 focus is exclusively **Functionality, Reliability, and Usability**. All performance optimizations are deferred.

> **Known Phase 1 Trade-off:** NFR1 (<100ms read latency at 10K issues) is **intentionally not addressed in Phase 1**. Direct DB hydration on every command invocation may exceed this threshold at scale. This is an accepted trade-off — correctness and simplicity take precedence. NFR1 compliance is a Phase 2 goal via TTL snapshot (ADR-002).

**Daemon Mode (Graph Hydration):**
- **Phase 1:** Direct DB hydration on every command invocation. No snapshot, no caching. Simple, correct, debuggable.
- **Phase 2 (Performance):** TTL-based on-disk snapshot (`.grava/graph.snapshot.json`, 5s TTL). Read commands serve from snapshot if fresh; fall back to DB on expiry. Write commands bypass snapshot + invalidate. Atomic write via tmp+rename.
- **Phase 3 (Scale):** Unix socket daemon (`grava serve`) — persistent process holding full in-memory AdjacencyDAG, serving all commands via local RPC. Eliminates per-invocation hydration cost entirely for 30-agent swarm workloads.

**pkg/cmd Structure:**
- **Now (Phase 1):** Reorganize `pkg/cmd` into command groups (`issues/`, `graph/`, `maintenance/`, `sync/`) — reduces cognitive load, no logic changes, low risk.
- **Later Phase:** Surgically extract `claim`, `import`, and `ready` into `pkg/ops` — the three concurrency-sensitive, high-value operations that need unit testability and daemon reuse without full Cobra invocation. Full service layer deferred until daemon mode is built.

### Architecture Decision Records

**ADR-001: Git Hook Binary Strategy**
- **Decision:** `grava hook <event>` subcommands compiled into the binary. `grava init` writes one-liner shell stubs (`#!/bin/sh\ngrava hook post-merge`) to `.git/hooks/` during repository initialization.
- **Rationale:** Satisfies NFR6 (zero external runtime deps), enables proper Go error propagation and structured logging, and makes hooks unit-testable. Shell scripts in `scripts/hooks/` become deprecated reference implementations.
- **Phase:** Phase 1

**ADR-002: TTL Snapshot Invalidation Protocol**
- **Decision:** Deferred to Phase 2 (performance phase). Phase 1 hydrates directly from DB on every command.
- **Future design (Phase 2):** Two-tier read/write protocol — read commands serve from snapshot if age < TTL (default 5s, configurable); write commands bypass snapshot and invalidate it. Atomic write via tmp file + `os.Rename()`.
- **Phase:** Phase 2

**ADR-003: pkg/ops Interface Preparation**
- **Decision:** `claim`, `import`, and `ready` logic lives in named functions (not anonymous `RunE` closures) with signature `func claimIssue(ctx context.Context, store dolt.Store, id, actor, model string) error`. No `pkg/ops` package created yet — functions remain in `pkg/cmd` command groups but are trivially extractable.
- **Rationale:** Respects YAGNI (no over-engineering before daemon mode exists) while ensuring `context.Context` is threaded through from day one and future extraction to `pkg/ops` is a file move, not a refactor.
- **Phase:** Phase 1 (prep); extraction in later phase when daemon mode requires it

**ADR-004: Worktree-Aware Redirect Architecture (inspired by Beads)**
- **Decision:** Grava adopts a redirect file pattern for Git worktree support. The main repository holds the single real `.grava/` directory (Dolt DB, snapshot, daemon socket). Each agent worktree contains only `.grava/redirect` — a relative path back to the main repo's `.grava/` (e.g. `../../.grava`). All Grava commands resolve the active `.grava/` via this priority chain:
  1. `GRAVA_DIR` environment variable (if set)
  2. Per-worktree `.grava/redirect` file (if present)
  3. Main repository's `.grava/` directory
  4. Walk up filesystem from CWD
- **Worktree Structure:**
  ```
  repo/
  ├── .grava/                        ← Real DB, config (Phase 1 only)
  │   ├── dolt/                      ← Shared Dolt instance
  │   ├── coordinator.pid            ← Present only when --enable-worktrees
  │   ├── graph.snapshot.json        ← Phase 2+ only (not present in Phase 1)
  │   └── grava.sock                 ← Phase 3+ only (not present in Phase 1)
  └── .worktrees/                    ← Present only when --enable-worktrees
      ├── agent-a/                   ← Agent A worktree (branch: grava/agent-a/{issue-id})
      │   └── .grava/redirect        ← "../../.grava"
      └── agent-b/                   ← Agent B worktree
          └── .grava/redirect        ← "../../.grava"
  ```
- **Worktree detection (two-step):**
  1. **Primary:** Check if `.git` is a file (worktree) or directory (main repo) — zero subprocess cost, used for init routing (ADR-FM1).
  2. **Secondary:** `git rev-parse --git-dir --git-common-dir` used only to compute the relative redirect path from worktree to main repo root. Result cached per process lifetime.
- **`grava init` in worktree:** Detects worktree context → creates minimal `.grava/` with `redirect` file only. Never creates a second Dolt instance.
- **`grava claim` lifecycle:** Creates worktree at `.worktrees/{actor}/` → writes redirect → checks out branch `grava/{actor}/{issue-id}` → agent works in isolation.
- **Worktree lifecycle commands (two distinct semantics):**
  - `grava close <id>` — **complete the work**: two-phase teardown, issue status → `closed`, wisps deleted.
  - `grava stop <id>` — **abandon/pause the work**: two-phase teardown, issue status → `open` (returns to ready queue), wisps **preserved** so the next agent has full context on prior work.
- **Worktree teardown (shared by both `close` and `stop`):** Two-phase atomic operation:
  1. Check for uncommitted changes in `.worktrees/{actor}/` — if present, abort before any DB write: `"error: worktree {actor} has uncommitted changes. Commit or stash before closing."` Never silently delete work.
  2. Check if `.worktrees/{actor}/` exists — if already absent (crash recovery path), skip to step 4.
  3. Run `git worktree remove .worktrees/{actor}/` and delete branch `grava/{actor}/{issue-id}`. If this fails, surface: `"error: could not remove worktree directory .worktrees/{actor}/: {reason}. Fix the git issue and retry."` and abort without touching the DB.
  4. Begin DB transaction — update issue status (`closed` or `open`), handle wisps per command semantics, commit.
  - **Idempotency:** If worktree is already absent (process crashed after `git worktree remove` but before DB commit), `grava close <id>` skips step 3 and proceeds to DB commit safely. Re-running after a crash always converges to correct state.
- **Branch naming:** `grava/{agent-id}/{issue-id}` e.g. `grava/agent-a/grava-a1b2c3d4ef12`.
- **`issues.jsonl` location:** Main repo root only — shared single source of truth. Worktrees do not have their own copies.
- **Redirect file:** Never committed (`.gitignore`). Supports relative and absolute paths. No chained redirects (one level only).
- **Rationale:** Eliminates DB fragmentation across worktrees. All agents share one Dolt instance, one daemon, one TTL snapshot — concurrency is managed at the DB layer (row locks, `SELECT FOR UPDATE`) not at the filesystem layer. Directly inspired by Beads' proven worktree isolation pattern.
- **Phase:** Phase 1 (redirect + worktree lifecycle); Phase 2 (daemon socket shared across worktrees)

**Red/Blue Hardening Decisions**
- **ADR-H1 (Merge driver timestamp):** Merge driver must use Dolt server `NOW()` for all `updated_at` resolution timestamps — never client wall clock. Prevents clock-skewed agents from winning LWW incorrectly.
- **ADR-H2 (Hook registration):** `grava init` three-step hook protocol: (1) no existing hook → write stub; (2) existing hook already contains `grava hook` → skip (idempotent); (3) existing hook with other content → append `grava hook <event>` as last line + print warning. Never silently overwrite.
- **ADR-H3 (dep deadlock prevention):** All multi-row lock acquisitions (e.g. `grava dep --add`) acquire row locks in lexicographic order of issue ID. CLI sorts `[]string{fromID, toID}` before locking. Retry on `ERROR 1213: Deadlock found` (max 3 attempts, 10ms backoff).
- **ADR-H4 (Phase 2 cache concurrency):** In-memory graph cache propagation under concurrent daemon RPC calls is a deferred Phase 2 design constraint. The cache `RWMutex` strategy must be audited when daemon mode is implemented — per-invocation CLI model is safe (no shared graph memory between processes).

### Failure Mode Decisions

**ADR-FM1: `.git` Directory Enforcement**
- **Decision:** `grava init` checks that `.git` exists as a **directory** in the repo root. If `.git` is absent or is a file, abort with: `error: .git must be a directory. Grava requires a standard Git repository. Bare repos and non-standard setups are not supported.`
- **Worktree detection:** A `.git` **file** (not directory) signals a Git worktree — `grava init` creates redirect only, no full init. A `.git` **directory** signals the main repo — full init proceeds.
- **Rationale:** Reliable, zero-ambiguity worktree vs main repo detection without parsing `git rev-parse` output edge cases.
- **Phase:** Phase 1

**ADR-FM2: Snapshot Size Optimization**
- **Decision:** Deferred to a later phase. When implemented, snapshot will store only `open` status issues and their blocking edges — excluding `closed`, `tombstone`, and `deferred` nodes to bound snapshot size to the active workload.
- **Phase:** Later phase optimization

**ADR-FM3: Master Coordinator Agent (opt-in)**
- **Decision:** The coordinator is **opt-in**, enabled via `grava init --enable-worktrees`. Default `grava init` (single-repo, no worktrees) does not start a coordinator — it initializes `.grava/`, runs migrations once inline, and exits.
- **When enabled** (`--enable-worktrees`), `grava init` starts a **long-running coordinator process** (`grava coordinator`) responsible for:
  1. Starting and managing the Dolt server process lifecycle (start, stop, health checks, restart on crash)
  2. Running schema migrations exclusively — regular agents never run migrations when coordinator is active
  3. Owning the `.grava/coordinator.pid` lock file
  4. Exposing a health endpoint that regular agents poll before connecting to Dolt
  5. Managing worktree lifecycle (create/remove `.worktrees/{actor}/` directories and redirect files)
- **Regular agent contract:** Agents only interact with issues via standard Grava commands. They never call `migrate.Run()` or manage the Dolt server directly.
- **`grava init` mandatory:** Must be the first command run in any repository. All other Grava commands check for `.grava/` existence and abort with `error: grava is not initialized. Run 'grava init' first.`
- **Coordinator startup flow (--enable-worktrees only):**
  ```
  grava init --enable-worktrees
    └── starts grava coordinator (background process)
          ├── initializes .grava/dolt/ (Dolt data directory)
          ├── starts dolt sql-server
          ├── runs schema migrations (once, exclusively)
          ├── writes .grava/coordinator.pid
          └── enters health-check loop (monitors Dolt, restarts if down)
  ```
- **Without --enable-worktrees:** `grava init` runs migrations inline (single process, no race), configures `.grava/`, and exits. Dolt server managed externally or via `grava start`/`grava stop`.
- **Coordinator subcommands:**
  - `grava coordinator start` — requires `.grava/` to exist (i.e. `grava init` must have been run first in this directory). If `.grava/` is absent: `"error: grava is not initialized. Run 'grava init' in a new folder first."` Coordinator never performs init itself — `grava init` is the sole entry point for a new repository.
  - `grava coordinator stop` — gracefully stops coordinator and Dolt server
  - `grava coordinator status` — reports coordinator PID, Dolt health, worktree count
- **Rationale:** Opt-in keeps the default setup simple and reliable for single-agent use. Multi-branch swarm users explicitly choose the coordinator complexity. Single coordinator = single migration path, eliminating the 30-agent concurrent migration race.
- **Phase:** Phase 1

**ADR-FM4: Claim Lock Lifecycle**
- **Decision:** Row lock on the issue is held **only for the duration of the claim transaction**. Once `grava claim` commits (status → `in_progress`, actor assigned), the DB lock is released. Subsequent operations (`update`, `comment`, `label`) each acquire a fresh short-lived row lock per transaction. No long-held lock spans the agent's entire work session.
- **Rationale:** Long-held locks across a full work session (minutes to hours) would block all other agents from reading or modifying the issue. Short per-transaction locks preserve concurrency while still guaranteeing atomic writes.
- **Phase:** Phase 1

**ADR-FM5: Wisps Lifecycle**
- **Decision:** The `wisps` table serves as an agent's ephemeral activity log for an issue:
  - Agents write activities continuously during work (`grava wisp write`)
  - On agent crash while `in_progress`: wisp rows are **preserved** — the next agent to pick up the issue reads wisps to understand prior state and avoid duplicating work
  - On issue `closed`: all wisp rows for that issue are **deleted** (cleanup) — `grava compact` or the close transaction handles this
  - `grava show <id>` includes recent wisp summary to give any agent full context on the current issue state
- **Rationale:** Wisps provide crash-safe agent handoff without polluting the permanent `issues` or `events` tables. Cleanup on close keeps the table bounded in size.
- **Phase:** Phase 1

**ADR-FM6: Migration Ownership**
- **Decision:** Migrations run **only during `grava init`**, never on every command invocation. The current `migrate.Run()` call in `PersistentPreRunE` must be removed.
  - **Default mode (no coordinator):** `grava init` runs `migrate.Run()` inline once. After successful migration, writes the current schema version integer to `.grava/schema_version` (plain text file, single integer — e.g. `"7"`). All subsequent commands read this file and compare against the binary's embedded `const SchemaVersion int`. On mismatch: `"error: schema is out of date (file: {n}, binary: {m}). Run 'grava init' to apply migrations."` The file is readable without a DB connection.
  - **Coordinator mode (`--enable-worktrees`):** Coordinator runs `migrate.Run()` exclusively on startup and writes `.grava/schema_version`. Regular agents never touch migrations.
- **Rationale:** Running migrations on every command invocation in `PersistentPreRunE` causes a 30-agent concurrent migration race in the default model and violates the coordinator ownership contract in the multi-worktree model. A plain-text version file is readable without a DB connection — agents can detect staleness and surface a clear error before attempting any DB calls.
- **Phase:** Phase 1

**ADR-FM7: grava doctor — Phase 1 Minimum Check Set**
- **Decision:** `grava doctor` runs the following checks in Phase 1:
  1. `.grava/` directory exists and is writable by current process user
  2. `.git` exists as a directory (main repo) or valid redirect file (worktree)
  3. If worktree: redirect file points to an existing `.grava/` directory
  4. Dolt server is reachable (TCP ping to configured DB URL)
  5. Schema version matches binary's expected version
  6. No issues in `in_progress` state older than configurable stale threshold (default: 24h) — reports as warning, not error. If stale `in_progress` issue has no corresponding `.worktrees/{actor}/` directory (ghost state from crash), surface: `"warning: issue {id} has been in_progress for {n}h with no active worktree. Run 'grava close {id}' to clean up."`
  7. If `--enable-worktrees`: coordinator PID file exists and process is alive
- **Output:** Structured report with `✅ pass` / `⚠️ warning` / `❌ fail` per check. Exits non-zero if any `❌ fail` found.
- **Phase:** Phase 1

### Architecture Scorecard

| Area | Weighted Score | Status |
|---|---|---|
| Core data integrity | 4.49 / 5 | ✅ Strong |
| Worktree & init | 4.80 / 5 | ✅ Strong |
| Code structure | 4.68 / 5 | ✅ Strong |
| **Overall** | **4.82 / 5** | ✅ Coherent |

_Scorecard updated after Comparative Analysis Matrix re-score: ADR-004 (3.65→4.90) and ADR-FM3 (3.65→4.70) gaps closed via two-phase atomic teardown, coordinator prerequisite guard, and `.grava/schema_version` file._

**Known gaps to address in future phases:**
1. **Last-write-wins (Phase 2 revisit):** LWW is lossy for string fields where both edits carry real intent. Phase 2 should implement field-type-aware resolution: string fields → three-way text merge; numeric/enum fields → LWW.
2. **Coordinator SPOF (monitoring):** Coordinator crash takes down Dolt for all agents. `grava doctor` must detect coordinator-down state and surface: `error: grava coordinator is not running. Start with: grava coordinator start`.
3. **Cold-start hydration monitoring:** Log full DB hydration time to devlog on every cold start. If consistently >80ms, prioritize ADR-FM2 snapshot optimization ahead of Phase 3 daemon work.

---

### Chaos Monkey Hardening Decisions

Six break scenarios were simulated to test system resilience. Hardening decisions per scenario:

**CM-1: Import transaction killed mid-flight**
- **Break:** Dolt crashes during bulk `grava import`, leaving partial rows.
- **Hardening:** Surface a clear user-facing message on connection loss mid-import: `"Import rolled back — database connection lost. Your data is unchanged. Safe to retry."`. No partial state is possible because the entire import is a single DB transaction (ADR pre-mortem decision). CLI must catch `driver: bad connection` and `ErrBadConn` and map these to the friendly rollback message.
- **Phase:** Phase 1

**CM-2: Coordinator process dies while agents are connected**
- **Break:** Coordinator crashes; agents get `connection refused` on next DB call.
- **Hardening:** On DB connection failure in `--enable-worktrees` mode: (1) check `coordinator.pid` — if PID file missing or process not alive, surface targeted error: `"error: grava coordinator is not running. Start with: grava coordinator start"`. (2) `grava coordinator start` must be idempotent — check PID alive before starting; remove stale PID file before writing new one. Error must distinguish "coordinator never started" vs "coordinator crashed unexpectedly".
- **Phase:** Phase 1

**CM-3: Moving the repository to a different folder**
- **Break:** Original concern was redirect file pointing to a stale absolute path after repo move.
- **Hardening (revised):** This is a **non-issue by design** for standard usage. `.worktrees/` lives _inside_ the main repo directory — moving the repo directory moves both `.grava/` and `.worktrees/` together. Redirect files use **relative paths** (e.g. `../../.grava`) so the relationship is preserved regardless of the absolute repo location. The only edge case (non-standard: worktrees placed outside the repo root) is unsupported and should be documented. `grava doctor` check #3 (ADR-FM7) catches any stale redirect that does occur.
- **Phase:** Phase 1 (by design — relative paths + co-located `.worktrees/`)

**CM-4: `grava init` run twice (idempotency)**
- **Break:** Running `grava init` a second time could start a second coordinator, double-register hooks, or overwrite config.
- **Hardening:** `grava init` is fully idempotent: (1) If `.grava/` exists — skip DB/config creation, log `"grava already initialized, skipping"`. (2) Hook registration uses ADR-H2 three-step protocol (already idempotent). (3) Coordinator start: check PID alive before starting second instance — if alive, skip and log `"coordinator already running (PID {n})"`. (4) Config file overwrite is a no-op if content is unchanged — use content-hash comparison before writing.
- **Phase:** Phase 1

**CM-5: `grava claim` called while agent already has a branch for that issue**
- **Break:** Agent re-claims an issue it already has a branch for — `git worktree add` fails with `"branch already checked out"`.
- **Hardening:** Normal close/complete automatically tears down the worktree (ADR-004 teardown). Ghost worktrees only arise from crash or incomplete teardown. Before `git worktree add`: (1) Check if `.worktrees/{actor}/` exists. (2) If exists and branch matches `grava/{actor}/{issue-id}` → reuse existing worktree, log `"resuming existing worktree for {issue-id}"`. (3) If exists with a _different_ branch → error: `"error: worktree {actor} is checked out to a different branch. Complete or stop the current work before claiming a new issue."`. `grava doctor` check #3 also catches orphaned worktrees that survived a crash.
- **Phase:** Phase 1

**CM-6: `grava compact` called during an active `in_progress` claim**
- **Break:** Compaction deletes ephemeral/closed data while an agent is mid-work.
- **Hardening:** Already safe by design ✅ — `grava compact` only deletes `wisps` for `closed` issues and `tombstone` nodes. It does not touch `in_progress` issues or their wisps. No additional hardening required. Document this scope boundary explicitly in `grava compact --help`.
- **Phase:** Phase 1 (safe by design)
