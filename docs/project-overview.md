# Project Overview: Grava

## Purpose
Grava is an AI-native CLI issue tracker designed for high-performance agentic workflows. It leverages **Dolt**, a version-controlled SQL database, to provide atomic work claiming, ephemeral state management (Wisp), a context-aware dependency graph for issue discovery, and a schema-aware merge driver for `issues.jsonl`.

## Technology Stack
| Category | Technology | Version | Description |
| :--- | :--- | :--- | :--- |
| **Language** | Go | 1.24.0+ (per `go.mod`) | Principal development language |
| **CLI Framework** | Cobra | 1.10.x | Command-line interface orchestration |
| **Configuration** | Viper | 1.21.x | Configuration management |
| **Database** | Dolt | 1.x (MySQL-compatible) | Version-controlled SQL database |
| **Migrations** | Goose | 3.26.x | Schema migration management (11 migrations embedded) |
| **Logging** | Zerolog | 1.34.x | Structured logging |
| **Testing** | Testify / SQLMock | 1.x | Unit and database mocking |

## Repository Classification
- **Type:** Monolith
- **Architecture:** Layered CLI architecture (Cmd -> Pkg -> Internal)
- **Primary Language:** Go

## Core Modules
- **`pkg/cmd`**: CLI command definitions (root, init, install, hook, conflicts, resolve, merge-driver, merge-slot, orchestrate, sync-status, version, …) plus sub-packages: `issues`, `graph`, `maintenance`, `reserve`, `sandbox`, `sync`.
- **`pkg/cmddeps`**: Shared CLI dependency container and JSON-aware error emitter.
- **`pkg/grava`**: Domain bootstrap and `.grava/` directory resolution.
- **`pkg/dolt`**: Database access layer, `WithAuditedTx` transaction wrapper, retry logic.
- **`pkg/doltinstall`**: Auto-install Dolt binary.
- **`pkg/graph`**: Dependency graph engine, traversal, priority inheritance, gate evaluation.
- **`pkg/idgen`**: Hierarchical ID generation (`grava-xxxx` and `grava-xxxx.1`).
- **`pkg/migrate`**: Embedded SQL schema migrations (Goose).
- **`pkg/merge`**: LWW 3-way merge driver for `issues.jsonl`, including conflict detection.
- **`pkg/orchestrator`**: Background task orchestration (poller, worker pool, watchdog).
- **`pkg/coordinator`**: Goroutine lifecycle and error channel propagation.
- **`pkg/notify`**: Notification abstraction (`ConsoleNotifier` plus mocks).
- **`pkg/gitconfig` / `pkg/gitattributes` / `pkg/githooks`**: Git-side wiring for the merge driver and hook stubs.
- **`pkg/log` / `pkg/devlog`**: Zerolog global logger; `devlog` is a deprecated no-op stub.
- **`pkg/errors`**: Structured `GravaError` type and error code catalogue.
- **`pkg/validation`**: Input validators (type, status, priority, date range).
- **`pkg/utils`**: Dolt binary resolution, git worktree orchestration, git version checking.
- **`internal/testutil`**: Shared testing helpers.

## Documentation Index
- [Architecture](./architecture.md)
- [Source Tree Analysis](./source-tree-analysis.md)
- [Data Models](./data-models.md)
- [Development Guide](./development-guide.md)
- [Per-package implementation reference](./detail-impl/index.md)
