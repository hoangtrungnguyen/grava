---
stepsCompleted:
  - step-01-init
  - step-02-context
  - step-03-starter
  - step-04-decisions
  - step-05-patterns
  - step-06-structure
  - step-07-validation
  - step-08-complete
inputDocuments:
  - _bmad-output/planning-artifacts/prd.md
  - docs/architecture/GRAPH_MECHANICS.md
  - docs/architecture/PERFORMANCE_BENCHMARKS.md
  - docs/guides/AGENT_WORKFLOWS.md
  - docs/guides/CLI_REFERENCE.md
workflowType: 'architecture'
lastStep: 8
status: 'complete'
completedAt: '2026-03-21'
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
- **ADR-H5 (`.grava/` git exclusion):** `grava init` writes `.grava/` to `.git/info/exclude` (local-only, never committed) — NOT to `.gitignore`. This protects the Dolt database and all `.grava/` contents from `git clean -fdx`, which deletes files matching `.gitignore` patterns. `.git/info/exclude` is never touched by git clean operations. **Migration:** on `grava init`, if `.gitignore` already contains a `.grava/` entry, remove it from `.gitignore` and write to `.git/info/exclude` instead — print `"migrated .grava/ exclusion from .gitignore to .git/info/exclude"`. Note: `.grava/config.yaml` contains secrets (notifier tokens) and is protected by this same exclusion — it must never be committed.
- **ADR-H6 (Orphaned branch detection):** `grava doctor` adds check #8: scan for `grava/*` branches with no corresponding `.worktrees/{actor}/` directory — report as `⚠️ warning` with: `"warning: orphaned branch {branch} has no active worktree. Run 'git branch -d {branch}' to clean up."` Branch proliferation is cosmetic but should not accumulate silently.

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
2. **Coordinator SPOF (monitoring):** Coordinator crash takes down Dolt for all agents. When any agent detects coordinator-down, it fires the `Notifier` (ADR-N1) before exiting. `grava doctor` detects coordinator-down and surfaces: `"error: grava coordinator is not running. Start with: grava coordinator start"`. **Note:** if all agents have already crashed, no agent remains to auto-restart the coordinator — human intervention is required. The `Notifier` is the mechanism by which the human is informed (console in Phase 1; Telegram/WhatsApp in Phase 2).
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

---

### ADR-N1: Agent Notification Interface

- **Decision:** A `Notifier` interface is defined in Phase 1 with a single method: `Send(title, body string) error`. All system-level alerts route through this interface — no alert is written directly to stderr at call sites.
- **Alert triggers (Phase 1 minimum):**
  1. Any agent detects coordinator-down (`connection refused` + dead PID)
  2. `grava doctor` finds any `❌ fail` check
  3. Stale `in_progress` issue detected (>24h, no active worktree)
- **Phase 1 implementation:** `ConsoleNotifier` — writes to stderr with prefix `[GRAVA ALERT]`. Default when no notifier configured.
- **Phase 2 implementations:** Configured via `.grava/config.yaml`:
  ```yaml
  notifier:
    type: telegram          # console | telegram | whatsapp
    telegram_bot_token: "..."
    telegram_chat_id: "..."
  ```
  `TelegramNotifier` and `WhatsAppNotifier` implement the same interface — zero changes to alert call sites when swapping implementations.
- **Error handling contract:**
  - `Send` errors are **non-fatal** — the primary operation (agent shutdown, doctor check) always completes regardless of notifier state.
  - On any `Send` error: log the error locally to stderr and continue. Never propagate notifier errors up the call stack.
  - If notifier config is missing, malformed, or the configured type is unrecognized: silently fall back to `ConsoleNotifier`. Never panic on misconfiguration.
- **Coordinator crash contract:** When coordinator crashes with `SIGTERM` (graceful), it fires `Notifier.Send` before exit. When agents detect coordinator-down, each fires `Notifier.Send` before exiting. If all agents and coordinator have crashed (full outage), no process remains to auto-restart — human intervention is required. The `Notifier` (Telegram/WhatsApp in Phase 2) is the mechanism by which the human is informed for unattended overnight runs.
- **Rationale:** Console output is invisible during unattended agent swarm runs. The interface decouples notification channel from notification logic — Phase 1 ships console, Phase 2 ships Telegram/WhatsApp, call sites never change. Notification is best-effort — a broken notifier must never take down an otherwise healthy agent.
- **Phase:** Phase 1 (interface + `ConsoleNotifier`); Phase 2 (`TelegramNotifier`, `WhatsAppNotifier`)

---

## Starter Template Evaluation

### Primary Technology Domain

CLI Tool / Local-first AI Agent Orchestration — Go binary using Cobra, Dolt (MySQL-protocol),
single statically-linked binary output (NFR6). No web, mobile, or frontend component.

### Starter Approach: Brownfield Structural Scaffold

This is a brownfield Go CLI project. No external starter template applies.
The "starter" evaluation defines the structural refactoring scaffold that brings
the existing codebase in line with Phase 1 architectural decisions.

### Structural Changes Required (Phase 1 Foundation)

**pkg/cmd reorganization (ADR-003):**
- Split flat `pkg/cmd/` into command groups: `pkg/cmd/issues/`, `pkg/cmd/graph/`,
  `pkg/cmd/maintenance/`, `pkg/cmd/sync/`
- Extract `claim`, `import`, `ready` logic into named functions with
  `func claimIssue(ctx context.Context, store dolt.Store, ...) error` signatures

**Migration ownership (ADR-FM6):**
- Remove `migrate.Run()` from `PersistentPreRunE` in `pkg/cmd/root.go`
- Add schema version check against `.grava/schema_version` file

**Notifier interface (ADR-N1):**
- Define `pkg/notify/notifier.go`: `Notifier` interface + `ConsoleNotifier`

**Git hook subcommands (ADR-001):**
- Add `grava hook <event>` subcommand group to binary

**Worktree resolver (ADR-004):**
- Add `.grava/` resolution priority chain: `GRAVA_DIR` → redirect file → CWD walk

**Note:** The first implementation story should establish this structural scaffold
before any feature work begins.

---

## Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions (Block Implementation):**
- Structured error types (`pkg/errors`) with machine-readable codes — required for NFR5 JSON contract
- `zerolog` structured logging — required for unattended agent diagnostics
- `testify` + hand-written interface mocks — required for named-function unit testability (ADR-003)

**Already Decided (from Steps 2–3 elicitation):**
- Persistence: Dolt (MySQL-protocol), single instance per repo
- CLI: Cobra, `pkg/cmd` reorganized into `issues/`, `graph/`, `maintenance/`, `sync/`
- IDs: 12-char hex base, `SELECT FOR UPDATE` on `child_counters`
- Migrations: `grava init` only, `.grava/schema_version` file
- Notifications: `Notifier` interface + `ConsoleNotifier` (Phase 1)
- Worktree: Redirect file pattern, `.git/info/exclude`

**Deferred Decisions (Post-Phase 1):**
- TTL snapshot (ADR-002): Phase 2
- Daemon/Unix socket: Phase 3
- `TelegramNotifier` / `WhatsAppNotifier`: Phase 2
- `pkg/ops` extraction: when daemon requires it

### Error Handling

**Decision:** Structured `GravaError` type in `pkg/errors/`
- Fields: `Code string` (machine-readable), `Message string` (human-readable), `Cause error` (wrapped)
- Implements `error` and `Unwrap()` — compatible with `errors.Is` / `errors.As`
- `--json` error output: `{"error": {"code": "...", "message": "..."}}`

**Standard error code domains:**

| Domain | Example Codes |
|---|---|
| Init / Setup | `NOT_INITIALIZED`, `SCHEMA_MISMATCH`, `ALREADY_INITIALIZED` |
| Issues | `ISSUE_NOT_FOUND`, `PARENT_NOT_FOUND`, `INVALID_STATUS_TRANSITION` |
| Worktree | `WORKTREE_DIRTY`, `WORKTREE_REMOVE_FAILED`, `REDIRECT_STALE` |
| DB / Coordinator | `DB_UNREACHABLE`, `COORDINATOR_DOWN`, `LOCK_TIMEOUT` |
| Import / Export | `IMPORT_ROLLED_BACK`, `SCHEMA_VALIDATION_FAILED` |
| Claim | `ALREADY_CLAIMED`, `CLAIM_CONFLICT` |

### Logging

**Decision:** `zerolog` (zero-allocation structured logging)
- Global logger in `pkg/log/`, initialized once in root command setup
- Level controlled via `GRAVA_LOG_LEVEL` env var (default: `warn`)
- Console writer in human-facing mode; JSON writer when `--json` flag set — single flag controls both command output and log format
- `GravaError.Code` included as structured field on all error-level log events

### Testing

**Decision:** `testify` (`assert` + `require`) + hand-written interface mocks
- `MockStore` in `pkg/dolt/mock/` — stubs `dolt.Store` for unit tests
- `MockNotifier` in `pkg/notify/mock/` — stubs `Notifier.Send`
- Integration tests in `_integration_test.go` files with `//go:build integration` tag, skipped unless `GRAVA_TEST_DB=1`
- Graph engine tests are pure unit tests (no DB dependency)
- `testify/require` for fatal assertions (stop on first failure); `testify/assert` for non-fatal

### Infrastructure & Deployment

**Single binary, local-first:** No cloud deployment, no container, no CI/CD infrastructure required for the tool itself. Binary distributed via `go install` or pre-built release artifacts. Build via `go build -ldflags="-s -w"` for stripped binary (NFR6).

---

## Implementation Patterns & Consistency Rules

### Critical Conflict Points: 20 identified across 4 categories

### Naming Patterns

**SQL Schema:**
- Tables: `snake_case` plural — `issues`, `dependencies`, `audit_events`
- Columns: `snake_case` — `issue_id`, `created_at`, `agent_model`
- Foreign keys: `{table_singular}_id` — `issue_id`, `parent_id`
- Indexes: `idx_{table}_{column}` — `idx_issues_status`

**GravaError Codes:**
- Format: `SCREAMING_SNAKE_CASE` — `ISSUE_NOT_FOUND`, `COORDINATOR_DOWN`
- Domain prefix required: `WORKTREE_*`, `DB_*`, `CLAIM_*`, `INIT_*`, `IMPORT_*`
- No generic codes: never use `ERROR` or `FAILED` alone

**JSON Output Fields:**
- All `--json` output fields: `snake_case` — `issue_id`, `created_at`, `affected_files`
- Success wrapper: flat object, no `data:` envelope — `{"id": "abc", "status": "created"}`
- Error wrapper: `{"error": {"code": "ISSUE_NOT_FOUND", "message": "..."}}`

**Env Vars:** `GRAVA_` prefix always — `GRAVA_LOG_LEVEL`, `GRAVA_TEST_DB`, `GRAVA_DIR`

**Log Field Keys:** `snake_case` matching JSON output — `issue_id`, `actor`, `schema_version`

### Structure Patterns

**Test placement:** `_test.go` files co-located with the package they test. No separate `tests/` directory.

**Mock placement:** `pkg/{domain}/mock/{domain}_mock.go` — e.g. `pkg/dolt/mock/store_mock.go`, `pkg/notify/mock/notifier_mock.go`. Importable by any test package.

**Command placement:** New commands go in the group matching their primary noun:
- Issue lifecycle → `pkg/cmd/issues/`
- Graph traversal / dependency → `pkg/cmd/graph/`
- DB maintenance, compaction, migration → `pkg/cmd/maintenance/`
- Git sync, hooks, export/import → `pkg/cmd/sync/`

**New packages:** Always under `pkg/`. Never create top-level packages outside `pkg/` or `cmd/`.

### Format Patterns

**Timestamps:**
- DB storage: `DATETIME` column, Go type `time.Time`, stored as UTC
- JSON output: RFC3339 string — `"2026-03-18T10:00:00Z"`
- Never Unix integers in JSON output

**Booleans:** DB storage `INTEGER` `0`/`1` (Dolt compatibility). JSON output `true`/`false`.

**Arrays in DB:** Stored as JSON string columns (e.g. `affected_files TEXT`). Marshaled/unmarshaled at the DB boundary only — never pass raw JSON strings through business logic.

### Process Patterns

**Error construction — always use constructor, never struct literal:**
```go
// CORRECT
return errors.New("ISSUE_NOT_FOUND", fmt.Sprintf("issue %s not found", id), err)

// WRONG — never instantiate directly
return &GravaError{Code: "ISSUE_NOT_FOUND", ...}
```

**Error wrapping:** `GravaError` at domain boundaries. `fmt.Errorf("context: %w", err)` only for internal plumbing where no user-facing code is needed.

**Transaction pattern — always:**
```go
tx, err := store.BeginTx(ctx, nil)
if err != nil { return errors.New("DB_UNREACHABLE", "failed to start transaction", err) }
defer tx.Rollback() //nolint:errcheck
// ... work ...
return tx.Commit()
```

**Context propagation:** Always thread `ctx context.Context` as first parameter in named functions. Never use `context.Background()` inside business logic — only at Cobra `RunE` entry points.

**Logger usage:** Pass `zerolog.Logger` as a parameter to named functions, never use the global directly inside `pkg/` business logic. Global `log.Logger` used only in `pkg/cmd` entry points.

**Log-then-return:** Log at the point of handling, not at the point of origination:
```go
// CORRECT — log where you handle the error
if err := claimIssue(ctx, store, id, actor, model); err != nil {
    log.Logger.Error().Str("code", gravaErr.Code).Err(err).Msg(gravaErr.Message)
    return err
}
// WRONG — don't log inside the named function AND at the call site
```

### Enforcement Guidelines

**All AI agents MUST:**
1. Use `GravaError` with a domain-prefixed `SCREAMING_SNAKE_CASE` code for every user-facing error
2. Use `snake_case` for all JSON output fields and SQL columns
3. Thread `context.Context` as first parameter through all named functions
4. Place tests co-located with the package, mocks in `pkg/{domain}/mock/`
5. Use `defer tx.Rollback()` immediately after every `BeginTx` call
6. Never call `context.Background()` inside `pkg/` business logic
7. Use the `GRAVA_` prefix for all environment variables

**Anti-patterns (never do these):**
- Instantiate `GravaError` as struct literal — always use the constructor
- Use `fmt.Println` or `fmt.Fprintf(os.Stderr)` directly — always use `zerolog`
- Return raw `sql` errors to the user — always wrap in `GravaError`
- Use `camelCase` in JSON output fields or SQL columns
- Create a new package outside `pkg/` without explicit architectural approval
- Define `dolt.Event*` constants outside `pkg/dolt/events.go`
- Use raw string literals for audit event types — always use `dolt.Event*` constants
- Call `context.Background()` inside `pkg/` business logic — only at `RunE` entry points
- Use `WithDeadlockRetry` around non-idempotent INSERT operations

**GravaError migration strategy:**
GravaError is introduced in Story 0. Stories touching existing commands migrate those commands' errors to GravaError as part of their scope. New commands use GravaError from day one. `fmt.Errorf` remains valid for internal plumbing (non-user-facing). Migration is one-way per file — any PR touching a file already migrated must not re-introduce `fmt.Errorf` for user-facing errors.

**devlog deprecation:**
Story 0 replaces `pkg/devlog` with `pkg/log` (zerolog). `pkg/devlog` is deleted after Story 0. No new code imports `devlog`.

**pkg/cmd/ reorganization timing:**
Story 0 performs the `pkg/cmd/` reorganization into `issues/`, `graph/`, `maintenance/`, `sync/`, `hook/`, `coordinator/`. All stories after Story 0 place commands in the reorganized group structure. Never add to the flat `pkg/cmd/` after Story 0 completes.

**Stderr output split rule:**
- Inside `cobra.Command.RunE`: use `cmd.ErrOrStderr()` — keeps stderr testable
- Outside Cobra handlers (coordinator goroutines, `pkg/` code, Notifier): use `os.Stderr` directly
- `Notifier.Send` errors always use `fmt.Fprintf(os.Stderr, ...)` — Notifier has no Cobra context

---

### Pattern Risk Tiers

Not all patterns carry equal risk. Agents must prioritize accordingly.

**🔴 CRITICAL — Data Integrity & Concurrency** *(mandatory — never deviate)*
- Transaction pattern: `BeginTx` → `defer Rollback` → mutations → audit log → `Commit`
- Audit log presence: every write command MUST call `LogEventTx` within the transaction
- Lock ordering: multi-row lock acquisitions in lexicographic ID order (ADR-H3)
- Context propagation: `ExecContext`/`QueryContext` always — `context.TODO()` forbidden

**🟡 SIGNIFICANT — Schema & Interop Correctness** *(mandatory — breaking if violated)*
- JSON field tags: `snake_case` (breaks `--json` output contract)
- Event type constants: `dolt.Event*` (breaks audit trail queries if raw strings diverge)
- Priority encoding: `validation.PriorityToString` maps (breaks display/filter consistency)
- Schema version check: must run before DB connect (ADR-FM6)

**🟢 CONVENTION — Readability & Consistency** *(mandatory — inconsistency compounds across agents)*
- Error message format: lowercase, no period, verb-object
- CLI flag naming: kebab-case flags, snake_case viper keys
- Emoji map: `✅` `⚠️` `❌` `👻`
- Test file placement: co-located `*_test.go`

⚠️ Lower tier = lower divergence risk between agents, NOT lower implementation requirement. All patterns are mandatory regardless of tier.

---

### Story 0 Prerequisites Checklist

The following must exist before any feature story begins. Agents implementing feature stories import these directly — do not recreate them.

Before creating any prerequisite: check if it already exists. Import directly if present. Create only if absent.

| AC | Package | File | Provides |
|---|---|---|---|
| AC1 | `pkg/dolt` | `events.go` | All `Event*` string constants |
| AC2 | `pkg/validation` | `priority.go` | `PriorityToString`, `StringToPriority`, `Priority*` int constants |
| AC3 | `pkg/utils` | `schema.go` | `CheckSchemaVersion(gravaDir string, binaryVersion int) error` |
| AC4 | `pkg/notify` | `notifier.go` | `Notifier` interface + `ConsoleNotifier` |
| AC5 | `pkg/dolt` | `tx.go` | `WithAuditedTx(ctx, store, []AuditEvent, fn) error` |
| AC6 | `pkg/dolt` | `retry.go` | `WithDeadlockRetry(fn func() error) error` |
| AC7 | `pkg/log` | `log.go` | zerolog global + init; `pkg/devlog` deleted |
| AC8 | `pkg/errors` | `errors.go` | `GravaError` constructor `errors.New(code, msg, cause)` |
| AC9 | `pkg/testutil` | `setup.go` | `setupTestGravaDir(t)`, `zerolog.Nop()` usage documented |
| AC10 | `pkg/cmd/` | *(reorganized)* | `issues/`, `graph/`, `maintenance/`, `sync/`, `hook/`, `coordinator/` |
| AC11 | `pkg/cmd/root.go` | — | `migrate.Run()` removed from `PersistentPreRunE`; schema version check added |

---

### Audit Event Type Constants

All audit event types live in `pkg/dolt/events.go`. Never define event constants outside this file.

```go
// pkg/dolt/events.go
package dolt

const (
    EventCreate           = "create"
    EventUpdate           = "update"
    EventClaim            = "claim"
    EventClose            = "close"
    EventStop             = "stop"
    EventComment          = "comment"
    EventLabelAdd         = "label_add"
    EventLabelRemove      = "label_remove"
    EventAssign           = "assign"
    EventDependencyAdd    = "dependency_add"
    EventDependencyRemove = "dependency_remove"
    EventWispWrite        = "wisp_write"
    EventWispClear        = "wisp_clear"
)
```

Before implementing any command that writes to the events table: check `pkg/dolt/events.go` first. If the required constant doesn't exist, add it there before writing command code.

**Audit command table:**

| Command | Event Constant | oldValue | newValue |
|---|---|---|---|
| `create` | `EventCreate` | `nil` | `{title, type, priority, status}` |
| `update` | `EventUpdate` | `{field, value: old}` | `{field, value: new}` — one call per changed field |
| `claim` | `EventClaim` | `{status: "open"}` | `{status: "in_progress", actor}` |
| `close` | `EventClose` | `{status: "in_progress"}` | `{status: "closed"}` |
| `stop` | `EventStop` | `{status: "in_progress"}` | `{status: "open"}` |
| `comment` | `EventComment` | `nil` | `{comment: "..."}` |
| `label add` | `EventLabelAdd` | `nil` | `{label: "..."}` |
| `label remove` | `EventLabelRemove` | `{label: "..."}` | `nil` |
| `assign` | `EventAssign` | `{actor: old}` | `{actor: new}` |
| `dep --add` | `EventDependencyAdd` | `nil` | `{to_id, type}` |
| `dep --remove` | `EventDependencyRemove` | `{to_id, type}` | `nil` |
| `wisp write` | `EventWispWrite` | `nil` | `{content: "..."}` |
| `wisp clear` | `EventWispClear` | `{count: n}` | `nil` |

Read-only commands (`list`, `show`, `ready`, `stats`, `graph`, `doctor`, `search`, `history`, `version`) — **no audit log**.

wisp and hook commands not listed above: define event types per-feature story, add to `pkg/dolt/events.go` and this table before implementing.

---

### Priority Constants

```go
// pkg/validation/priority.go
package validation

const (
    PriorityCritical = 0
    PriorityHigh     = 1
    PriorityMedium   = 2
    PriorityLow      = 3
)

var PriorityToString = map[int]string{
    PriorityCritical: "critical",
    PriorityHigh:     "high",
    PriorityMedium:   "medium",
    PriorityLow:      "low",
}

var StringToPriority = map[string]int{
    "critical": PriorityCritical,
    "high":     PriorityHigh,
    "medium":   PriorityMedium,
    "low":      PriorityLow,
}
```

All display and parsing code uses these maps. No `switch priority` blocks duplicated across commands.

---

### Transaction Helpers

**`WithAuditedTx` — primary transaction pattern for all write commands:**

```go
// pkg/dolt/tx.go
type AuditEvent struct {
    IssueID   string
    EventType string // always use dolt.Event* constants
    Actor     string
    Model     string
    OldValue  any
    NewValue  any
}

func WithAuditedTx(ctx context.Context, store Store, events []AuditEvent, fn func(tx *sql.Tx) error) error {
    tx, err := store.BeginTx(ctx, nil)
    if err != nil {
        return gravaerrors.New("DB_UNREACHABLE", "failed to start transaction", err)
    }
    defer tx.Rollback() //nolint:errcheck
    if err := fn(tx); err != nil {
        return err
    }
    for _, evt := range events {
        if err := store.LogEventTx(ctx, tx, evt.IssueID, evt.EventType, evt.Actor, evt.Model, evt.OldValue, evt.NewValue); err != nil {
            return gravaerrors.New("DB_UNREACHABLE", "failed to log audit event", err)
        }
    }
    return tx.Commit()
}
```

Use: `return dolt.WithAuditedTx(ctx, Store, []dolt.AuditEvent{{...}}, func(tx *sql.Tx) error { ... })`

For commands with multiple audit events (e.g. `create` with parent → `EventCreate` + `EventDependencyAdd`): pass both in the `[]AuditEvent` slice.

**`WithDeadlockRetry` — for SELECT FOR UPDATE operations only:**

```go
// pkg/dolt/retry.go
func WithDeadlockRetry(fn func() error) error {
    const maxRetries = 3
    for attempt := range maxRetries {
        err := fn()
        if err == nil {
            return nil
        }
        if isMySQLDeadlock(err) && attempt < maxRetries-1 {
            time.Sleep(10 * time.Millisecond)
            continue
        }
        return err
    }
    return nil
}

func isMySQLDeadlock(err error) bool {
    var mysqlErr *mysql.MySQLError
    return errors.As(err, &mysqlErr) && mysqlErr.Number == 1213
}
```

**Restriction:** Use `WithDeadlockRetry` only around `SELECT ... FOR UPDATE` + counter increment operations. Do NOT wrap `WithAuditedTx` in `WithDeadlockRetry` — audit log duplication on retry. All operations inside must be idempotent.

---

### Command Structure — Named Functions

`claim`, `import`, and `ready` MUST use named functions (reason: extracted to `pkg/ops` for daemon reuse in a later phase — ADR-003):

```go
func claimIssue(ctx context.Context, store dolt.Store, log zerolog.Logger, id, actor, model string) error {
    // business logic
}

var claimCmd = &cobra.Command{
    RunE: func(cmd *cobra.Command, args []string) error {
        return claimIssue(context.Background(), Store, log.Logger, args[0], actor, agentModel)
    },
}
```

All other commands: named functions preferred but not mandatory in Phase 1.

---

### Startup Sequence

`PersistentPreRunE` exact order (after Story 0):

```
1. Initialize pkg/log (zerolog) — flag/viper
2. Skip DB init for: help, init, start, stop, version
3. Resolve .grava/ directory (ADR-004 priority chain: GRAVA_DIR → redirect → CWD walk)
4. Check schema version: CheckSchemaVersion(gravaDir, SchemaVersion)
5. Resolve DB URL (flag → viper → default)
6. Connect: dolt.NewClient(dbURL)
7. Set Store — ready for command use
```

**Sequencing note:** `.grava/config.yaml` is loaded during cobra's `OnInitialize` (before `PersistentPreRunE`). The `.grava/` resolver uses env vars and filesystem detection only — never reads config. No circular dependency.

---

### Human Output Canonical Emoji Map

| Emoji | Usage |
|---|---|
| `✅` | Success operations: create, update, close, assign |
| `⚠️` | Warnings written to `cmd.ErrOrStderr()` (in handlers) or `os.Stderr` (elsewhere) |
| `❌` | Failure/error written to stderr |
| `👻` | Ephemeral/wisp issues specifically |

No other emoji introduced without updating this table. Always use `cmd.Printf` or `cmd.OutOrStdout()` for stdout — never `fmt.Println`.

---

### Notifier Injection

```go
// pkg/cmd/root.go — package-level var, default ConsoleNotifier
var Notifier notify.Notifier = notify.NewConsoleNotifier()
// Commands call: Notifier.Send(...) — never instantiate directly in command code
// Tests inject: MockNotifier from pkg/notify/mock/
```

---

### Test Utilities

**Unit vs integration boundary:**
- Unit test (`*_test.go`): tests a single function/package in isolation; mocks permitted via `pkg/{domain}/mock/`
- Integration test (`*_integration_test.go`, `//go:build integration`): tests command end-to-end against real Dolt; no mocks at DB layer; requires `GRAVA_TEST_DB=1`

**Logger in tests — always suppress output:**
```go
log := zerolog.Nop() // suppress all log output in tests
err := claimIssue(ctx, mockStore, log, ...)
```

**Test environment setup:** `setupTestGravaDir(t *testing.T) string` defined in `pkg/testutil/setup.go`. Creates temp `.grava/` dir with valid `schema_version` file. Required by all integration tests that invoke `PersistentPreRunE`.

**Resolution chain coverage:** Table-driven tests for the `.grava/` resolver in `pkg/utils/dolt_resolver_test.go` — covering `GRAVA_DIR` env, valid redirect, broken redirect, CWD walk.

**Race detection:** `go test -race ./...` required in CI for all packages with concurrent write paths (`claim`, `dep`, `import`).

---

## Project Structure & Boundaries

### Complete Project Directory Structure

```
grava/
├── go.mod
├── go.sum
├── Makefile                        ← build, test, lint, integration-test targets
├── .gitignore
├── .gitattributes                  ← merge driver: *.jsonl merge=grava-merge
├── .github/
│   └── workflows/
│       └── ci.yml                  ← go vet, staticcheck, go test -race, go build
├── cmd/
│   └── grava/
│       └── main.go                 ← entry point: sets version, calls pkg/cmd.Execute()
├── pkg/
│   ├── cmd/
│   │   ├── root.go                 ← rootCmd, PersistentPreRunE/PostRunE, global flags, Notifier var
│   │   ├── version.go              ← grava version
│   │   ├── init.go                 ← grava init [--enable-worktrees]
│   │   ├── start.go                ← grava start (dolt server — non-coordinator mode)
│   │   ├── stop.go                 ← grava stop (dolt server — non-coordinator mode)
│   │   ├── issues/
│   │   │   ├── create.go           ← grava create (FR1)
│   │   │   ├── show.go             ← grava show (FR2)
│   │   │   ├── list.go             ← grava list (FR3)
│   │   │   ├── update.go           ← grava update (FR4)
│   │   │   ├── drop.go             ← grava drop (FR5)
│   │   │   ├── label.go            ← grava label add/remove (FR6)
│   │   │   ├── assign.go           ← grava assign (FR6)
│   │   │   ├── comment.go          ← grava comment (FR7)
│   │   │   ├── subtask.go          ← grava subtask (FR1 child issues)
│   │   │   ├── quick.go            ← grava quick (FR1 fast create)
│   │   │   └── wisp.go             ← grava wisp write/clear/show (FR18–19)
│   │   ├── graph/
│   │   │   ├── dep.go              ← grava dep --add/--remove (FR8–9)
│   │   │   ├── ready.go            ← grava ready (FR10–11) — named function required
│   │   │   ├── blocked.go          ← grava blocked (FR12)
│   │   │   └── graph.go            ← grava graph [--mermaid] (FR13)
│   │   ├── maintenance/
│   │   │   ├── compact.go          ← grava compact (FR14)
│   │   │   ├── doctor.go           ← grava doctor (FR15, ADR-FM7)
│   │   │   ├── stats.go            ← grava stats (FR16)
│   │   │   ├── search.go           ← grava search (FR17)
│   │   │   ├── cmd_history.go      ← grava history (FR14 audit trail)
│   │   │   └── undo.go             ← grava undo (FR14)
│   │   ├── sync/
│   │   │   ├── export.go           ← grava export → issues.jsonl (FR20–21)
│   │   │   ├── import.go           ← grava import — named function required (FR22–23)
│   │   │   └── commit.go           ← grava commit (FR24)
│   │   ├── hook/
│   │   │   └── hook.go             ← grava hook post-merge/pre-commit (ADR-001, FR25)
│   │   └── coordinator/
│   │       └── coordinator.go      ← grava coordinator start/stop/status (ADR-FM3)
│   ├── dolt/
│   │   ├── client.go               ← Store interface + Client implementation
│   │   ├── events.go               ← Event* string constants [Story 0 AC1]
│   │   ├── tx.go                   ← WithAuditedTx([]AuditEvent) [Story 0 AC5]
│   │   ├── retry.go                ← WithDeadlockRetry() [Story 0 AC6]
│   │   ├── client_integration_test.go
│   │   └── mock/
│   │       └── store_mock.go       ← MockStore for unit tests [Story 0 AC1]
│   ├── graph/
│   │   ├── dag.go                  ← AdjacencyDAG core
│   │   ├── graph.go                ← public graph API
│   │   ├── types.go                ← Node, Edge, shared types
│   │   ├── loader.go               ← DB hydration (lazy: open issues only)
│   │   ├── ready_engine.go         ← ready queue computation (FR10–11)
│   │   ├── traversal.go            ← BFS/DFS traversal
│   │   ├── topology.go             ← topological sort
│   │   ├── cycle.go                ← cycle detection
│   │   ├── gates.go                ← blocking gate evaluation
│   │   ├── cache.go                ← Phase 2+ TTL snapshot stub
│   │   ├── priority_queue.go       ← priority-ordered ready queue
│   │   ├── mermaid.go              ← Mermaid diagram generation (FR13)
│   │   ├── errors.go               ← graph-specific sentinel errors
│   │   └── *_test.go               ← co-located unit + benchmark tests
│   ├── idgen/
│   │   ├── generator.go            ← 12-char hex base IDs + child ID (SELECT FOR UPDATE)
│   │   └── generator_test.go
│   ├── migrate/
│   │   └── migrate.go              ← schema migrations — run only from grava init
│   ├── errors/                     ← [Story 0 AC8]
│   │   └── errors.go               ← GravaError{Code, Message, Cause} + constructor
│   ├── log/                        ← [Story 0 AC7] replaces pkg/devlog
│   │   └── log.go                  ← zerolog global init, GRAVA_LOG_LEVEL
│   ├── notify/
│   │   ├── notifier.go             ← Notifier interface + ConsoleNotifier [Story 0 AC4]
│   │   └── mock/
│   │       └── notifier_mock.go    ← MockNotifier for tests
│   ├── validation/
│   │   ├── validation.go           ← input validation (issue type, status transitions)
│   │   └── priority.go             ← Priority* constants + PriorityToString [Story 0 AC2]
│   ├── utils/
│   │   ├── path.go                 ← filesystem helpers
│   │   ├── dolt_resolver.go        ← .grava/ resolution chain (ADR-004)
│   │   ├── dolt_resolver_test.go   ← table-driven resolution chain tests
│   │   ├── schema.go               ← CheckSchemaVersion() [Story 0 AC3]
│   │   └── net.go                  ← TCP ping / port availability
│   └── testutil/                   ← [Story 0 AC9]
│       └── setup.go                ← setupTestGravaDir(t), test environment helpers
│
├── docs/
│   ├── architecture/
│   │   ├── GRAPH_MECHANICS.md
│   │   └── PERFORMANCE_BENCHMARKS.md
│   ├── guides/
│   │   ├── AGENT_WORKFLOWS.md
│   │   └── CLI_REFERENCE.md
│   └── epics/
│
└── issues.jsonl                    ← main repo root — shared JSONL export/import source of truth
```

**Runtime directory (not committed — `.git/info/exclude`):**
```
.grava/
├── dolt/                           ← Dolt data directory
├── schema_version                  ← plain text integer (e.g. "7")
├── config.yaml                     ← notifier tokens, db_url override (secrets)
├── coordinator.pid                 ← present only when --enable-worktrees active
├── graph.snapshot.json             ← Phase 2+ only
└── grava.sock                      ← Phase 3+ only

.worktrees/                         ← present only when --enable-worktrees
├── agent-a/
│   └── .grava/redirect             ← relative path: "../../.grava"
└── agent-b/
    └── .grava/redirect
```

---

### Architectural Boundaries

**CLI → DB Layer**
- Boundary: `pkg/dolt.Store` interface — all DB access flows through it
- Write path: `dolt.WithAuditedTx(ctx, store, []AuditEvent, fn)` — transaction + audit atomically
- Read path: `store.QueryContext` / `store.QueryRowContext`
- Concurrency: row-level locking via `SELECT ... FOR UPDATE` — never application-level locks
- Connection pool: `db.SetMaxOpenConns(20)` in `NewClient`

**CLI → Graph Engine**
- Boundary: `pkg/graph` public API — `graph.Load`, `graph.Ready`, `graph.Blocked`
- Phase 1: hydrated from DB on each command invocation (no cache)
- Read commands pass `dolt.Store` to `graph.Load`; graph engine owns all traversal logic
- Write commands update DB first; graph reflects on next hydration

**CLI → Notifications**
- Boundary: `pkg/notify.Notifier` interface (ADR-N1)
- Injected via `var Notifier notify.Notifier` in `pkg/cmd/root.go`
- Phase 1: `ConsoleNotifier` (stderr `[GRAVA ALERT]` prefix)
- Alert triggers: coordinator-down, `doctor` fail, stale `in_progress` issue

**CLI → ID Generation**
- Boundary: `pkg/idgen.Generator` interface
- Base IDs: 12-char hex (`GenerateBaseID`) — ~4B combinations (birthday threshold ~65K at 1%)
- Child IDs: atomic `SELECT FOR UPDATE` on `child_counters` (`GenerateChildID`)

**Git ↔ Grava**
- Hook stubs in `.git/hooks/` call `grava hook <event>` (ADR-001)
- `grava hook post-merge` → triggers import pipeline
- Merge driver registered in `.gitattributes`: `*.jsonl merge=grava-merge`
- `.grava/` excluded via `.git/info/exclude` — never `.gitignore` (ADR-H5)

**Coordinator ↔ Dolt Server** *(--enable-worktrees only)*
- Coordinator exclusively owns Dolt server lifecycle (ADR-FM3)
- Regular agents never start/stop Dolt; poll coordinator health before connecting
- PID lock: `.grava/coordinator.pid`

**Agent ↔ Grava Binary**
- Contract: CLI flags + `--json` stdout + exit codes (`0`=success, `1`=error)
- `--actor`: agent identity primitive — required across all state-changing commands
- `--agent-model`: AI model identifier written to audit trail
- JSON schema: versioned — breaking changes = major version bump (NFR5)

---

### FR → Directory Mapping

| FR Domain | FRs | Primary Location | Supporting |
|---|---|---|---|
| Issue Lifecycle | FR1–7 | `pkg/cmd/issues/` | `pkg/dolt/`, `pkg/idgen/` |
| Graph Context & Discovery | FR8–13 | `pkg/graph/`, `pkg/cmd/graph/` | `pkg/dolt/` |
| State History & Maintenance | FR14–17 | `pkg/cmd/maintenance/` | `pkg/migrate/`, `pkg/dolt/` |
| Ephemeral State / Wisp | FR18–19 | `pkg/cmd/issues/wisp.go` | `pkg/dolt/` (wisps table) |
| Workspace Sync & Git | FR20–25 | `pkg/cmd/sync/`, `pkg/cmd/hook/` | `pkg/graph/` (import hydration) |

**Cross-cutting concerns → locations:**

| Concern | Location |
|---|---|
| Actor identity (`--actor`) | `pkg/cmd/root.go` global → propagated via named function args |
| Audit logging | `pkg/dolt/events.go` + `dolt.WithAuditedTx` → all write commands |
| Error contract (NFR5) | `pkg/errors/` → all command packages → `--json` error output |
| Schema migrations | `pkg/migrate/` → `grava init` only |
| `.grava/` resolution | `pkg/utils/dolt_resolver.go` → `pkg/cmd/root.go` `PersistentPreRunE` |
| Schema version check | `pkg/utils/schema.go` → `PersistentPreRunE` step 4 |
| Notifications | `pkg/notify/` → coordinator, doctor, stale-issue detection |
| Deadlock retry | `pkg/dolt/retry.go` → `dep.go`, `idgen/generator.go` |

---

### Data Flow

**Write path — `grava claim abc123`:**
```
Cobra RunE
  → PersistentPreRunE:
      pkg/utils.CheckSchemaVersion(.grava/schema_version)
      pkg/dolt.NewClient(dbURL)
  → claimIssue(ctx, store, log, id, actor, model):
      dolt.WithDeadlockRetry(func() {
          SELECT id FROM issues WHERE id=? FOR UPDATE
      })
      dolt.WithAuditedTx(ctx, store, []AuditEvent{
          {id, EventClaim, actor, model, old, new}
      }, func(tx) {
          UPDATE issues SET status='in_progress', actor=?, updated_at=NOW()
      })
  → stdout: ✅ Claimed: abc123  (or --json {"id":"abc123","status":"claimed"})
```

**Read path — `grava ready`:**
```
Cobra RunE
  → PersistentPreRunE: CheckSchemaVersion, NewClient
  → readyIssues(ctx, store, log):
      graph.Load(ctx, store, filter=open)   ← lazy: open issues only
      graph.Ready(dag)                       ← topological sort + gate evaluation
  → stdout: issue list (or --json array)
```

**Git sync path — post-merge hook:**
```
.git/hooks/post-merge  →  grava hook post-merge
  → pkg/cmd/hook: invokes grava import pipeline
  → importIssues(ctx, store, log, path):
      dolt.WithAuditedTx(ctx, store, auditEvents, func(tx) {
          parse issues.jsonl
          upsert all rows in single transaction (all-or-nothing)
      })
  → merge driver (issues.jsonl conflicts):
      last-write-wins per field
      updated_at resolved via Dolt server NOW() (ADR-H1)
```

---

### CI Pipeline

```yaml
# .github/workflows/ci.yml
jobs:
  build:
    steps:
      - go vet ./...
      - staticcheck ./...
      - go build -ldflags="-s -w -X main.version=$(git describe --tags)" ./cmd/grava/

  test-unit:
    steps:
      - go test -race ./...

  test-integration:
    services:
      dolt: (dolt sql-server on port 3306)
    env:
      GRAVA_TEST_DB: "1"
    steps:
      - go test -race -tags integration ./...
```

**Binary distribution:** `go install github.com/hoangtrungnguyen/grava/cmd/grava@latest` or pre-built release artifacts via GitHub Releases. No container, no cloud deployment required (NFR6).

---

## Architecture Validation Results

_Step 7 validation — coherence, requirements coverage, implementation readiness, and gap analysis. Enriched with Hindsight Reflection (Advanced Elicitation)._

### Coherence Check ✅

| Area | Status | Notes |
|------|--------|-------|
| ADRs ↔ Patterns | ✅ Pass | WithAuditedTx aligns with ADR-003 (Dolt substrate); GravaError aligns with ADR-001 |
| Patterns ↔ Structure | ✅ Pass | `pkg/dolt/tx.go`, `pkg/dolt/retry.go`, `pkg/validation/priority.go` all have designated homes |
| Structure ↔ FRs | ✅ Pass | All 8 FRs map to at least one directory; FR5/FR6 map to `pkg/liveness/` and `pkg/cmd/coordinator/` |
| Startup Sequence ↔ ADRs | ✅ Pass | ADR-FM6 (migrations only in `grava init`) reflected in 7-step PersistentPreRunE sequence |
| Test Strategy ↔ Structure | ✅ Pass | `pkg/testutil/` designated; mock policy (unit=ok, integration=forbidden) documented |

### Requirements Coverage ✅

| Requirement | Covered By | Status |
|-------------|-----------|--------|
| FR1 Issue CRUD | `pkg/cmd/issues/` + `pkg/dolt/` | ✅ |
| FR2 Graph mechanics | `pkg/cmd/graph/` + `pkg/graph/` | ✅ |
| FR3 Agent orchestration | `pkg/cmd/coordinator/` + `pkg/agent/` | ✅ |
| FR4 Git sync | `pkg/cmd/sync/` + `pkg/git/` | ✅ |
| FR5 Liveness / circuit breaking | `pkg/liveness/` + `pkg/wisp/` | ✅ |
| FR6 Worktree awareness | ADR-004 redirect pattern + `pkg/cmd/coordinator/` | ✅ |
| NFR1 Local-first | No server deployment; `dolt sql-server` embedded | ✅ |
| NFR2 Audit trail | `WithAuditedTx` + `dolt.Event*` constants | ✅ |
| NFR3 Extensibility | Notifier interface + ConsoleNotifier/TelegramNotifier | ✅ |
| NFR4 Observability | zerolog structured logging; `pkg/devlog` deprecated Story 0 | ✅ |
| NFR5 Testability | `setupTestGravaDir` + testify + hand-written mocks | ✅ |
| NFR6 No cloud | Binary distribution via `go install` / GitHub Releases | ✅ |

### Implementation Readiness ✅

All Story 0 prerequisites are specified:
1. `pkg/errors/` — GravaError type with Code/Message/Cause/Unwrap
2. `pkg/dolt/events.go` — all `Event*` constants
3. `pkg/dolt/tx.go` — `WithAuditedTx(ctx, store, []AuditEvent, fn)`
4. `pkg/dolt/retry.go` — `WithDeadlockRetry` (SELECT FOR UPDATE only)
5. `pkg/validation/priority.go` — constants + bidirectional maps
6. `pkg/testutil/setup.go` — `setupTestGravaDir(t)` mandatory helper
7. `pkg/migrate/` — migration runner extracted from PersistentPreRunE
8. zerolog replacing `pkg/devlog`
9. `gravaerrors` canonical import alias
10. Go 1.22+ minimum (range N syntax)
11. `pkg/cmd/` reorganization into sub-packages

### Gap Analysis

#### 🔴 CRITICAL Gaps (must fix before Story 1)

**C1 — JSON Error Envelope Contract** _(Hindsight Reflection H2)_

When `--json` flag is active, ALL output must be JSON — including errors. Currently unspecified, leading to mixed stdout/stderr behavior.

```
Success:  {"data": {...}}
Error:    {"error": {"code": "SCREAMING_SNAKE_CASE", "message": "human readable"}}
```

Rules:
- `"data": null` when error is present
- Non-zero exit code still applies regardless of JSON mode
- `cmd.ErrOrStderr()` used for JSON error output in RunE context
- Enforcement: integration test asserting `--json` flag always produces valid JSON on both success and failure paths

**C2 — Coordinator Error Channel Pattern** _(Hindsight Reflection H4)_

`pkg/cmd/coordinator/` goroutines have no agreed error propagation path. Without a contract, implementors will use `log.Fatal`, `os.Exit`, or silent swallowing.

```go
// pkg/cmd/coordinator/coordinator.go
type Coordinator struct{ ... }

// Start returns an error channel; caller must select on it
func (c *Coordinator) Start(ctx context.Context) <-chan error

// RunE selects:
// select { case err := <-coord.Start(ctx): ...; case <-ctx.Done(): ... }
```

Rules:
- Goroutines MUST NOT call `log.Fatal` or `os.Exit`
- All fatal goroutine errors propagate via the returned `chan error`
- Coordinator respects `ctx` cancellation for graceful shutdown

#### 🟡 IMPORTANT Gaps (fix in Story 0 or Story 1)

**I1 — Go 1.22+ Minimum Version** _(Validation + First Principles)_

`range N` integer syntax used in `WithDeadlockRetry` requires Go 1.22+. Must be declared in `go.mod` and CI matrix.

```
go 1.22
```

**I2 — `gravaerrors` Import Alias** _(Validation gap)_

`pkg/errors` collides with stdlib `errors` package. Canonical alias required in all files importing it:

```go
import gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
```

**I3 — Wisp State Machine** _(Hindsight Reflection H1)_

`pkg/wisp/` and `pkg/liveness/` are defined structurally but canonical state transitions are unspecified.

```
PENDING → RUNNING → HEALTHY
                  → STALE    (no heartbeat for N seconds; pkg/liveness/ owns threshold)
                  → DEAD     (max restart attempts exceeded)
STALE   → RUNNING (resurrection; pkg/liveness/ exclusively owns this transition)
DEAD    → (terminal; requires manual intervention)
```

**I4 — Migration Rollback Contract** _(Hindsight Reflection H5)_

`pkg/migrate/` must expose:
- `Run(ctx, store) error` — idempotent; no-op if schema version matches
- `Rollback(ctx, store, targetVersion int) error` — backs out to specified version
- `grava init` exits non-zero with `MIGRATION_FAILED` on failure; leaves DB at last-good version

**I5 — `setupTestGravaDir` Mandatory Enforcement** _(Hindsight Reflection H3)_

`setupTestGravaDir(t)` is **required** (not optional) in every test that touches the `.grava/` directory.
Enforcement: CI check:
```bash
# Fails if any _test.go touching .grava/ omits setupTestGravaDir
grep -r "\.grava/" --include="*_test.go" -l | xargs grep -L "setupTestGravaDir" && exit 1 || exit 0
```

#### 🟢 NICE-TO-HAVE Gaps (Phase 2)

**N1 — TelegramNotifier implementation** — documented in ADR-N1 as Phase 2; no action needed now.

**N2 — `grava graph` visual output format** — ASCII vs Mermaid vs JSON unspecified; deferred to UX story.

### Validation Summary

| Category | Count | Status |
|----------|-------|--------|
| 🔴 Critical gaps | 2 | Must resolve before Story 1 — add C1 + C2 to Story 0 ACs |
| 🟡 Important gaps | 5 | Resolve in Story 0 or Story 1 |
| 🟢 Nice-to-have | 2 | Deferred to Phase 2 |
| ✅ Coherence checks | 5/5 | Pass |
| ✅ Requirements covered | 12/12 | Pass |
| ✅ Story 0 ACs specified | 11/11 | Pass |

**Architecture is implementation-ready** after resolving the 2 critical gaps (C1, C2). Add JSON Error Envelope contract and Coordinator Error Channel pattern to Story 0 acceptance criteria before closing the architecture phase.
