# grava — Requirements Inventory

*Source documents: PRD, Architecture, Implementation Readiness Report, Edge Case Resolution Strategy, Failure Recovery Strategy*

## Functional Requirements

FR1: System Agent / Human Developer can create discrete issues or macro-epics (`create`, `quick`).
FR2: System Agent / Human Developer can rapidly break down overarching issues into hierarchical subtasks (`subtask`).
FR3: System Agent / Human Developer can update core fields like status, priority, and assignees (`update`, `assign`).
FR4: System Agent / Human Developer can explicitly track when they start or stop working on a specific issue (`start`, `stop`).
FR5: System Agent can execute an atomic claim on an issue, verifying it is unassigned and immediately locking it to their actor ID (`claim`).
FR6: System Agent / Human Developer can append contextual metadata to issues via tags and text notes (`label`, `comment`).
FR7: System Agent / Human Developer can safely remove or archive issues from the active tracking space (`drop`, `clear`).
FR8: System Agent / Human Developer can establish directional "blocking" relationship links between issues (`dep`).
FR9: System Agent / Human Developer can query the immediate actionable queue of top-priority tasks with no blockers (`ready`).
FR10: System Agent / Human Developer can explicitly query what upstream issues are preventing a specific task from being worked on (`blocked`).
FR11: System Agent / Human Developer can visualize or traverse the overarching dependency structure of the project (`graph`).
FR12: System Agent / Human Developer can filter, search, and view detailed individual properties of issues (`list`, `search`, `show`).
FR13: Human Developer can view aggregated workspace performance and status metrics (`stats`).
FR14: System Agent / Human Developer can retrieve a detailed ledger of previously executed system commands (`cmd_history`).
FR15: Human Developer can safely revert recent state-altering commands to recover from errors (`undo`).
FR16: System Agent / Human Developer can explicitly prune expired or deleted data to maintain high query performance (`compact`).
FR17: Human Developer can run diagnostic health checks on the Grava substrate to ensure data integrity (`doctor`).
FR18: System Agent / Human Developer can explicitly write to and read from an issue's ephemeral state (Wisp) via dedicated commands to manage working artifacts and execution history.
FR19: System Agent / Human Developer can retrieve the historical progression log of an issue to understand what a previous agent did before handoff.
FR20: System Agent / Human Developer can export the internal database state into a standardized, machine-readable artifact (`export`).
FR21: System Agent / Human Developer can hydrate the internal database by importing a standardized artifact (`import`), provided no conflicts exist.
FR22: The System must automatically execute a 3-way cell-level merge of issue state during Git updates. If cell-level changes cannot be merged, the System must safely isolate and save the unresolvable conflict data to a separate database table for human intervention.
FR23: Human Developer can initialize a brand-new, isolated tracking environment for a local repository (`init`, `config`).
FR24: The System must evaluate a Dual-Safety Check (JSONL hash vs. Dolt state) before importing to prevent overwriting uncommitted local data.
FR25: The System must automatically trigger graph database updates via Git hooks whenever the repository state changes (e.g., `git pull`, checkout), actively detecting any file-to-database mismatches.
FR26: System provides an automated install script (shell-based, cross-platform for macOS and Linux) that installs the user's chosen AI CLI backend (Claude CLI or Gemini CLI) and Grava in a single execution, requiring no prior CLI experience.
FR27: The install script detects the host OS and architecture, selects the correct binary or package source, and handles all dependency installation without user intervention beyond selecting the preferred AI backend. Supported platforms: macOS (ARM, x86), Linux (Debian/Ubuntu, RHEL/Fedora), and Windows (x86-64 via PowerShell installer).
FR28: The install script validates the completed environment by running `grava doctor` and reports a clear success or failure message with remediation steps if setup is incomplete.

### Derived Requirements (from Edge Case Resolution Strategy)

FR-ECS-1a: File reservation declaration — agents declare advisory leases before modifying files (`file_reservations` table).
FR-ECS-1b: Pre-commit hook enforcement — blocks unauthorized commits to reserved paths.
FR-ECS-1c: TTL auto-expiry — stale leases released automatically on agent crash. *(Phase 1 deferral condition — see Epic 8)*
FR-ECS-1d: Git artifact audit trail — lease state written to `file_reservations/<sha1>.json`. *(Phase 1 deferral condition — see Epic 8)*

## Non-Functional Requirements

NFR1 (Query Speed): Core graph resolution commands must return structured JSON within <100ms under 10,000 active/ephemeral issues. *(Deferred to Phase 2 — ADR-002)*
NFR2 (Write Throughput): `create`, `update`, `claim` must commit in <15ms per operation; >70 inserts/second sustained.
NFR3 (Atomic Execution): Concurrent `grava claim` by multiple agents results in exactly one success and one deterministic rejection — no polluted rows, no deadlock.
NFR4 (Zero-Loss Handoff): 100% preservation of dependency links and core fields during export/import across workspace clones.
NFR5 (Machine Readability): Strict JSON schema adherence for all `--json` outputs; schema changes trigger major version bump.
NFR6 (Zero-Dependency Footprint): Single statically-linked binary; zero external runtime dependencies beyond system shell and Git.
NFR7 (Install Speed): Full environment setup in <5 minutes on clean machine with ≥10 Mbps connection.
NFR8 (Install Reliability): First-attempt install success on macOS (ARM/x86), Linux (Debian/Ubuntu, RHEL/Fedora), Windows (x86-64 PowerShell) without elevated privileges beyond initial package manager bootstrapping.

### NFR Ownership Map

| NFR | Owned By | Validated By |
|-----|----------|--------------|
| NFR1 | Deferred Phase 2 (ADR-002) | — |
| NFR2 | Epic 1 (WithAuditedTx baseline) | Epic 7 (Git hook writes), Epic 5 (compact) |
| NFR3 | Epic 3 (claim concurrency) | Epic 9 (worktree multi-agent) |
| NFR4 | Epic 7 (export/import pipeline) | Epic 11 (TS-04, TS-07) |
| NFR5 | Epic 1 (JSON Error Envelope, Story 0a) | All epics exposing `--json` |
| NFR6 | Epic 1 | All epics (no separate runtime) |
| NFR7 | Epic 6 (install script) | Epic 11 (TS-install scenario) |
| NFR8 | Epic 6 (install script) | Epic 6 (CI matrix: macOS/Linux/Windows) |

## Additional Requirements

**From Architecture (Technical — Phase 1 Implementation Impact):**

- **Brownfield Structural Scaffold (Story 0 prerequisite):** Reorganize `pkg/cmd/` into command groups: `pkg/cmd/issues/`, `pkg/cmd/graph/`, `pkg/cmd/maintenance/`, `pkg/cmd/sync/`. Extract `claim`, `import`, `ready` into named functions (ADR-003).
- **Migration ownership fix:** Remove `migrate.Run()` from `PersistentPreRunE`; migrations run only during `grava init` (ADR-FM6).
- **Notifier interface:** `pkg/notify/notifier.go` — `Notifier` interface + `ConsoleNotifier` (ADR-N1).
- **Git hook subcommands:** `grava hook <event>` subcommand group; `grava init` writes one-liner shell stubs (ADR-001).
- **Worktree resolver:** `.grava/` resolution: `GRAVA_DIR` env → redirect file → CWD walk (ADR-004).
- **Structured error types:** `GravaError` in `pkg/errors/` with `Code`, `Message`, `Cause` — required for NFR5.
- **JSON Error Envelope:** All `--json` error paths return `{"error": {"code": "...", "message": "..."}}` (C2 gap).
- **Coordinator Error Channel:** `Start(ctx) <-chan error`; goroutines must not call `log.Fatal`/`os.Exit` (C3 gap).
- **Structured logging:** `zerolog` for unattended agent diagnostics.
- **Testing library:** `testify` + hand-written interface mocks.
- **Row-level locking:** `SELECT FOR UPDATE` on `child_counters`; `WithDeadlockRetry` (max 3, 10ms backoff) (ADR-H3).
- **Coordinator (opt-in):** `grava init --enable-worktrees` starts `grava coordinator` process (ADR-FM3).
- **Worktree lifecycle:** `grava close <id>` (complete) vs `grava stop <id>` (pause/abandon) — atomic teardown (ADR-004).
- **Git Merge Driver (Beads-Inspired):** `grava-merge` registered via `.gitattributes`; LWW per field; `updated_at` via Dolt `NOW()`; conflicts → `conflict_records` table (ADR-H1).
- **Import Pipeline:** All-or-nothing transaction; full rollback on crash/connection loss (CM-1, FR21, FR24).
- **Dual-Safety Check:** JSONL hash vs Dolt state before import (FR24).
- **Git Hook Pipeline:** `grava hook post-merge` triggers import pipeline; idempotent registration (ADR-H2).
- **Chaos hardening:** All 6 CM scenarios addressed in Phase 1 (CM-1 through CM-6).
- **`grava doctor` Phase 1 checks:** 7 mandatory checks including stale `in_progress` and orphaned branch (ADR-FM7, ADR-H6).
- **`.grava/` git exclusion:** `grava init` writes to `.git/info/exclude`, not `.gitignore` (ADR-H5).
- **12-char hex base ID:** Birthday collision threshold ~65K issues at 1%.
- **Phase 1 known trade-off:** NFR1 deferred; direct DB hydration per invocation in Phase 1 (ADR-002).

**From Edge Case Resolution Strategy:**

- **Schema-Aware 3-Way Merge Driver:** Full 3-way parse (Ancestor `%O`, Ours `%A`, Theirs `%B`) keyed by `Issue_ID`. Policies: `delete-wins` and `conflict`. `conflict_records` table: `{id, base_version, our_version, their_version, resolved_status}`.
- **File Reservation System:** `file_reservations` table `{id, project_id, agent_id, path_pattern, exclusive, reason, created_ts, expires_ts, released_ts}`. TTL auto-expiry. Pre-commit enforcement. `FILE_RESERVATION_CONFLICT` error code. Audit artifacts: `file_reservations/<sha1(path)>.json`.

**From Failure Recovery Strategy:**

- **`grava doctor` extended flags:** `--fix` with backup-before-purge; `--dry-run`; default = diagnostic only.
- **Extended doctor check set:** Stale `in_progress` + expired Wisp heartbeat; orphaned worktree directory; orphaned branch (ADR-H6); stale lock files; expired file reservations.
- **TTL-based lease auto-release:** Auto-release past `expires_ts`. Issue stays `in_progress` — preserved for next agent via Wisp resume (ADR-FM5).
- **Orphaned branch recovery:** Enumerate `grava/{agent-id}/{issue-id}` branches; cross-reference `issues` table; clean or flag dirty branches; emit structured report.

**From Implementation Readiness Report:**

- Story 0 must be first story in Epic 1; no feature work before scaffold complete.
- All epic titles framed as user outcomes.
- FR26–FR28 require dedicated Onboarding epic.
- Phase 2 features explicitly out of Phase 1 scope.

## FR Coverage Map

| FR | Epic | Description |
|----|------|-------------|
| FR1 | Epic 2 | Create issues/epics |
| FR2 | Epic 2 | Subtasks |
| FR3 | Epic 2 | Update fields |
| FR4 | Epic 2 | Start/stop work tracking |
| FR5 | Epic 3 | Atomic claim |
| FR6 | Epic 2 | Labels/comments |
| FR7 | Epic 2 | Drop/archive |
| FR8 | Epic 4 | Dep relationships |
| FR9 | Epic 4 | Ready queue |
| FR10 | Epic 4 | Blocked query |
| FR11 | Epic 4 | Graph visualization |
| FR12 | Epic 4 | List/search/show |
| FR13 | Epic 4 | Stats |
| FR14 | Epic 5 | Command history |
| FR15 | Epic 5 | Undo |
| FR16 | Epic 5 | Compact |
| FR17 | Epic 5 | Doctor (diagnostic + repair) |
| FR18 | Epic 3 | Wisp write/read |
| FR19 | Epic 3 | Issue history log |
| FR20 | Epic 7 | Export |
| FR21 | Epic 7 | Import (with Dual-Safety Check) |
| FR22 | Epic 10 | 3-way schema-aware merge driver |
| FR23 | Epic 1 (basic init) + Epic 9 (worktree init) | Initialize environment |
| FR24 | Epic 7 | Dual-Safety Check before import |
| FR25 | Epic 7 | Git hook triggers |
| FR26 | Epic 6 | Automated install script |
| FR27 | Epic 6 | OS/arch detection, dependency installation |
| FR28 | Epic 6 | Install validation via grava doctor |
| FR-ECS-1a | Epic 8 | File reservation declaration |
| FR-ECS-1b | Epic 8 | Pre-commit hook enforcement |
| FR-ECS-1c | Epic 8 | TTL auto-expiry *(Phase 1 deferral condition)* |
| FR-ECS-1d | Epic 8 | Git artifact audit trail *(Phase 1 deferral condition)* |
