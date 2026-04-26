# Project Documentation: Grava

This repository contains the documentation for **Grava**, an AI-native CLI issue tracker.

### Project Overview
- **Type:** CLI / Backend
- **Primary Language:** Go
- **Architecture:** Layered CLI (Cobra + Dolt)

### Core Documentation
- [Project Overview](./project-overview.md)
- [Architecture](./architecture.md)
- [Source Tree Analysis](./source-tree-analysis.md)
- [Data Models](./data-models.md)
- [Development Guide](./development-guide.md)

### Detailed Implementation Reference
Per-package documentation in [`detail-impl/`](./detail-impl/index.md):
- CLI commands, dependency injection, persistence, graph engine, merge driver,
  ID generation, migrations, validation, hooks.

### Guides
Operational and integration guides in [`guides/`](./guides):
- [`AGENT_WORKFLOWS.md`](./guides/AGENT_WORKFLOWS.md) — agent collaboration patterns
- [`CLI_REFERENCE.md`](./guides/CLI_REFERENCE.md) — full command reference
- [`DOLT_SETUP.md`](./guides/DOLT_SETUP.md) — Dolt server configuration
- [`RELEASE_PROCESS.md`](./guides/RELEASE_PROCESS.md) — release procedure
- [`agent_instructions.md`](./guides/agent_instructions.md) — instructions consumed by AI agents
- [`claude-worktree-integration.md`](./guides/claude-worktree-integration.md) — Claude Code worktree handoff

### External References
- [README.md](../README.md)
- [Setup Local Environment](../SETUP-LOCAL-ENVIRONMENT.md)
- [Conflict Resolution Reference (Beads)](./beads_conflict_resolution.md) — design reference for a sibling system on Dolt
- Sandbox validation lives in the external `gravav6-sandbox` repository.

---
*Master Index for AI-Assisted Development*
