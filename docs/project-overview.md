# Project Overview: Grava

## Purpose
Grava is an AI-native CLI issue tracker designed for high-performance agentic workflows. It leverages **Dolt**, a version-controlled SQL database, to provide atomic work claiming, ephemeral state management (Wisp), and a context-aware dependency graph for issue discovery.

## Technology Stack
| Category | Technology | Version | Description |
| :--- | :--- | :--- | :--- |
| **Language** | Go | 1.24.0 | Principal development language |
| **CLI Framework** | Cobra | 1.10.x | Command-line interface orchestration |
| **Configuration** | Viper | 1.21.x | Configuration management |
| **Database** | Dolt / MySQL | 1.x | Version-controlled SQL database |
| **Migrations** | Goose | 3.26.x | Schema migration management |
| **Logging** | Zerolog | 1.34.x | Structured logging |
| **Testing** | Testify / SQLMock | 1.x | Unit and database mocking |

## Repository Classification
- **Type:** Monolith
- **Architecture:** Layered CLI architecture (Cmd -> Pkg -> Internal)
- **Primary Language:** Go

## Core Modules
- **pkg/cmd**: CLI command definitions (assign, claim, create, update, etc.).
- **pkg/grava**: Domain entities and business logic.
- **pkg/dolt**: Database access layer and transaction management.
- **pkg/graph**: Dependency graph and traversal logic.
- **pkg/migrate**: SQL schema migrations.
- **sandbox/**: Python-based integration test suite.

## Documentation Index
- [Architecture](./architecture.md)
- [Source Tree Analysis](./source-tree-analysis.md)
- [Data Models](./data-models.md)
- [Development Guide](./development-guide.md)
- [API Contracts](./api-contracts.md) _(To be generated)_
