# Architecture: Grava

## Executive Summary
Grava is built as a command-line application that acts as a client to a **Dolt** database. It provides an agent-friendly interface for managing complex issue graphs, ensuring atomicity via SELECT FOR UPDATE claiming and full auditability via event logging.

## Core Architectural Components

### 1. CLI Layer (`pkg/cmd`)
Using **Cobra**, the CLI layer handles command routing, flag parsing, and console output. It leverages sub-packages (e.g., `issues/`, `sync/`) to maintain a clean command hierarchy.
- **Root Command**: `pkg/cmd/root.go`
- **Error Handling**: Centralized JSON-aware error emitter in `pkg/cmddeps/json.go`.

### 2. Domain Layer (`pkg/grava`)
Defines the core entities (`Issue`, `Dependency`, `Event`) and the **Resolver** which orchestrates logic that spans multiple persistence calls.

### 3. Persistence Layer (`pkg/dolt`)
A specialized persistence layer for Dolt. It implements Git-like versioning concepts (commits, branches) within a SQL context.
- **Transactional Safety**: `WithAuditedTx` ensures that every state change is atomically recorded in the `events` table.
- **Conflict Management**: Uses Dolt system tables to resolve state discrepancies.

### 4. Graph Engine (`pkg/graph`)
Handles DAG (Directed Acyclic Graph) traversal for parent-child relationships and blocking dependencies. Essential for automated task discovery.

### 5. Migration System (`pkg/migrate`)
Embeds SQL scripts into the binary using `go:embed` and manages schema evolution via Goose. Currently 11 migrations covering all tables.

### 6. Git Integration Layer (`pkg/gitconfig`, `pkg/gitattributes`, `pkg/githooks`)
Manages the Git-side configuration for merge drivers, attributes, and hook stubs:
- **Merge Driver**: Registers `grava-merge` in `.git/config` for LWW 3-way merge of `issues.jsonl`.
- **Git Attributes**: Ensures `issues.jsonl merge=grava-merge` in `.gitattributes`.
- **Hook Stubs**: Deploys `pre-commit` and `post-merge` hook stubs that delegate to `grava hook run`.

### 7. Merge Driver (`pkg/merge`)
Schema-aware 3-way merge for `issues.jsonl` using Last-Write-Wins (LWW) semantics:
- Field-level conflict resolution by `updated_at` timestamp comparison.
- Delete-wins policy: when one branch deletes an issue, deletion is deterministic.
- Equal-timestamp conflicts produce `ConflictEntry` records and set `HasGitConflict=true`.

### 8. Sync Pipeline (`pkg/cmd/sync`)
Export/import pipeline for Git-tracked JSONL files:
- **Export**: Flat JSONL with embedded labels, comments, dependencies, and wisp entries.
- **Import**: Upsert with overwrite mode that cleans stale related data.
- **Dual-Safety Check**: Content hash (skip unchanged) + Dolt uncommitted changes guard.

### 9. File Reservation System (`pkg/cmd/reserve`)
Advisory file-path leases for concurrent edit safety:
- Declare exclusive/shared leases with TTL auto-expiry.
- Pre-commit hook enforcement blocks commits to paths held by other agents.
- Glob pattern matching including `**` recursive wildcards.

### 10. Orchestrator (`pkg/orchestrator`)
Background task orchestration with poller, worker pool, and watchdog for long-running operations.

### 11. Sandbox Validation (`pkg/cmd/sandbox`)
Integration validation framework with 10 executable scenarios (TS-01 through TS-10) covering concurrent claims, dependency graphs, export/import round-trips, merge conflicts, file reservations, and swarm load testing.

## Data Flow
1. **User/Agent Command**: Received via CLI.
2. **Flag Parsing**: Viper/Cobra resolves configuration and arguments.
3. **Command Execution**: Logic calls `pkg/grava` resolver.
4. **Database Transaction**: `pkg/dolt` executes SQL within `WithAuditedTx`.
5. **Audit Logging**: `events` table updated automatically.
6. **Output**: CLI emits human-readable text or machine-readable JSON.

## Design Decisions
- **Dolt-Native**: Explicitly designed to use Dolt Features (commits, sql-server).
- **Agent-First**: High priority on JSON output and atomic operations (Claiming).
- **Hierarchical IDs**: Custom ID generation to support `grava-xxxx.1` subtask nesting.
