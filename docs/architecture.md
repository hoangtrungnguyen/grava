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
Embeds SQL scripts into the binary using `go:embed` and manages schema evolution via Goose.

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
