# Architecture: Grava

## Executive Summary
Grava is a command-line application that acts as a client to a **Dolt** database. It provides an agent-friendly interface for managing complex issue graphs, ensuring atomicity via `SELECT FOR UPDATE` claiming and full auditability via append-only event logging.

## Core Architectural Components

### 1. CLI Layer (`pkg/cmd`)
Using **Cobra**, the CLI layer handles command routing, flag parsing, and console output. Sub-packages keep the command hierarchy modular:
- `pkg/cmd/issues/` — issue CRUD (`create`, `show`, `update`, `claim`, `start`, `stop`, `close`, `comment`, `label`, `subtask`, `wisp`, `history`, `assign`, `drop`, `undo`).
- `pkg/cmd/graph/` — dependency graph commands.
- `pkg/cmd/maintenance/` — `doctor`, `compact`, `clear`, `cmd-history`.
- `pkg/cmd/reserve/` — file reservation commands.
- `pkg/cmd/sandbox/` — embedded validation scenarios (`grava sandbox <ts##>`).
- `pkg/cmd/sync/` — JSONL export/import.

The root command (`pkg/cmd/root.go`) wires `PersistentPreRunE` / `PersistentPostRunE` for connection setup, audit logging, and structured error emission via `pkg/cmddeps/json.go`.

### 2. Domain Layer (`pkg/grava`)
Resolves the active `.grava/` directory (env override → upward filesystem walk → repo root), loads project configuration, and provides the bootstrap path that connects every command to the same Dolt instance.

### 3. Persistence Layer (`pkg/dolt`)
A specialized persistence layer for Dolt:
- **`WithAuditedTx`** — wraps a function in a single SQL transaction, persists supplied audit events to the `events` table, and commits atomically. On any error the transaction is rolled back and no audit events are persisted.
- **Retry on serialization failure** — claim and other mutating commands retry on `DB_COMMIT_FAILED` so race losers see a clean error code (e.g. `ALREADY_CLAIMED`) instead of a transient commit failure.
- **Event constants** — `pkg/dolt/events.go` enumerates `EventCreate`, `EventUpdate`, `EventClaim`, `EventRelease`, `EventDrop`, `EventClear`, etc. — never use raw string literals.

### 4. Graph Engine (`pkg/graph`)
DAG traversal for parent-child relationships and blocking dependencies. Powers automated task discovery via `grava ready` and `grava blocked`.

### 5. Migration System (`pkg/migrate`)
Embeds SQL scripts via `go:embed` and applies them through Goose on startup. Currently **11 migrations** covering issues, dependencies, deletions, child counters, audit columns, affected files, story type, work session columns, labels and comments, archived status, wisp entries, file reservations, the command audit log, and conflict records.

### 6. Git Integration Layer (`pkg/gitconfig`, `pkg/gitattributes`, `pkg/githooks`)
Manages the Git-side configuration for merge drivers, attributes, and hook stubs:
- **Merge Driver** — registers `grava-merge` in `.git/config` for LWW 3-way merge of `issues.jsonl`.
- **Git Attributes** — ensures `issues.jsonl merge=grava-merge` in `.gitattributes`.
- **Hook Stubs** — deploys `pre-commit` and `post-merge` stubs that delegate to `grava hook run`.

### 7. Merge Driver (`pkg/merge`)
Schema-aware 3-way merge for `issues.jsonl` with Last-Write-Wins (LWW) semantics:
- Field-level conflict resolution via `updated_at` timestamp comparison.
- Delete-wins policy: when one branch deletes an issue and the other modifies it, deletion is deterministic and produces a `ConflictEntry` audit record but no git-level conflict.
- Equal-timestamp field conflicts produce inline `_conflict` markers, populate `ConflictRecords`, and set `HasGitConflict=true` so `git merge` exits non-zero.
- Conflict records (when `.grava/` is resolvable) are written to `.grava/conflicts.json` for `grava resolve list` consumption.
- End-to-end coverage lives in `pkg/merge/git_driver_integration_test.go` (build tag `integration`); covers clean composes, delete-vs-modify, multi-issue concurrent edits, equal-timestamp conflicts, and `conflicts.json` persistence.

### 8. Sync Pipeline (`pkg/cmd/sync`)
Export/import pipeline for Git-tracked JSONL files:
- **Export** — flat JSONL with embedded labels, comments, dependencies, and wisp entries.
- **Import** — upsert with overwrite mode that cleans stale related rows.
- **Dual-Safety Check** — content hash (skip unchanged) plus a Dolt uncommitted-changes guard.

### 9. File Reservation System (`pkg/cmd/reserve`)
Advisory file-path leases for concurrent edit safety:
- Exclusive / shared leases with TTL auto-expiry.
- Pre-commit hook enforcement blocks commits to paths held by other agents.
- Glob pattern matching including `**` recursive wildcards.
- `grava doctor --fix` releases expired leases (FR-ECS-1c).

### 10. Orchestrator (`pkg/orchestrator`) and Coordinator (`pkg/coordinator`)
`pkg/orchestrator` runs the poller / worker pool / watchdog for background work; `pkg/coordinator` manages goroutine lifecycle and propagates errors via channels.

### 11. Doctor / Health Checks (`pkg/cmd/maintenance`)
`grava doctor` runs diagnostic checks against the Grava database and reports component health:
- DB connectivity, required tables, orphan dependencies, untitled issues, Wisp count.
- **Ghost worktrees** — issues with `status='in_progress'` whose `.worktree/<id>` directory has been removed (typically after a reaped container). `--dry-run` lists IDs; `--fix` resets `status='open'`, clears `assignee`, and writes a `release` audit event with `reason=ghost_worktree`. The Git branch `grava/<id>` is preserved so partial work can be revisited.
- **Expired file leases** — releases reservations whose `expires_ts` is past.
- JSON output: aggregated under `fix_results []` (a legacy `fix_result` key is preserved for the expired-lease entry).

### 12. Sandbox Validation (`pkg/cmd/sandbox`)
Validation framework with executable scenarios `TS-01`, `TS-02`, `TS-03`, `TS-04`, `TS-05`, `TS-07`, `TS-08`, `TS-09`, `TS-10` (9 scenarios; `TS-06` reserved). Covers concurrent claims, ready queue, dependency graphs, export/import round-trips, doctor, conflict detection, file reservations, worktree-crash recovery, and swarm load testing.

## Data Flow
1. **User/Agent Command** received via CLI.
2. **Flag Parsing** by Cobra/Viper.
3. **Command Execution** dispatches into `pkg/cmd/...` handlers.
4. **Database Transaction** opens through `pkg/dolt` inside `WithAuditedTx`.
5. **Audit Logging** writes one or more rows to the `events` table atomically.
6. **Output** as human-readable text or machine-readable JSON when `--json` is set.

## Notable Invariants
- **Claim ordering** (`pkg/cmd/issues/claim.go`) — DB claim runs first; the worktree-conflict check follows and rolls back the DB claim if stale local artifacts are detected. Concurrent claimants therefore see `ALREADY_CLAIMED` rather than `WORKTREE_CONFLICT`.
- **Atomic audit** — every state-changing command emits its mutations and audit events inside one `WithAuditedTx` call. Half-written audits cannot exist.
- **Hierarchical IDs** — `pkg/idgen` generates IDs like `grava-xxxx` and subtask IDs like `grava-xxxx.1`, using `child_counters` for atomicity.

## Design Decisions
- **Dolt-Native** — written specifically for Dolt features (commits, sql-server, merge primitives).
- **Agent-First** — JSON output mode and atomic operations are first-class.
- **Single binary** — no separate daemon; all background work is opt-in through `grava orchestrate`.
