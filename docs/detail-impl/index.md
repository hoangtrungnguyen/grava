# `docs/detail-impl/` — Module Implementation Reference

This folder contains **detailed implementation documentation** for every Go package in the Grava project.

> _Updated 2026-04-17 (comprehensive doc review)._

---

## Module Index

| Module | File | Purpose |
|:---|:---|:---|
| `pkg/cmd` | [pkg-cmd.md](./pkg-cmd.md) | CLI command registration, lifecycle, and all sub-commands |
| `pkg/cmddeps` | [pkg-cmddeps.md](./pkg-cmddeps.md) | Shared dependency injection, JSON error emitter |
| `pkg/coordinator` | [pkg-coordinator.md](./pkg-coordinator.md) | Background goroutine lifecycle and error propagation |
| `pkg/dolt` | [pkg-dolt.md](./pkg-dolt.md) | Database persistence layer, `WithAuditedTx`, retry logic |
| `pkg/doltinstall` | [pkg-doltinstall.md](./pkg-doltinstall.md) | Automated Dolt binary download + install |
| `pkg/errors` | [pkg-errors.md](./pkg-errors.md) | Structured `GravaError` type and error code catalogue |
| `pkg/gitattributes` | — | Git attributes management (`issues.jsonl merge=grava-merge`) |
| `pkg/gitconfig` | — | Git config management (merge driver registration) |
| `pkg/githooks` | — | Git hook stub deployment (pre-commit, post-merge) |
| `pkg/graph` | [pkg-graph.md](./pkg-graph.md) | DAG engine, traversal, priority inheritance, gate evaluation |
| `pkg/grava` | [pkg-grava.md](./pkg-grava.md) | Domain bootstrap, `.grava/` directory resolution |
| `pkg/idgen` | [pkg-idgen.md](./pkg-idgen.md) | Hierarchical ID generation (`grava-xxxx` and `grava-xxxx.1`) |
| `pkg/log` + `pkg/devlog` | [pkg-log.md](./pkg-log.md) | Zerolog global logger; devlog is deprecated no-op stub |
| `pkg/merge` | — | LWW 3-way merge driver, conflict detection and isolation |
| `pkg/migrate` | [pkg-migrate.md](./pkg-migrate.md) | Goose-based schema migrations (embedded SQL, 11 migrations) |
| `pkg/notify` | [pkg-notify.md](./pkg-notify.md) | Notification abstraction (`ConsoleNotifier`, future integrations) |
| `pkg/orchestrator` | — | Background task orchestration (poller, pool, watchdog) |
| `pkg/utils` | [pkg-utils.md](./pkg-utils.md) | Dolt binary resolution, git worktree orchestration, git version checking |
| `pkg/validation` | [pkg-validation.md](./pkg-validation.md) | Input validators (type, status, priority, date range) |
| `scripts/hooks` | [scripts-hooks.md](./scripts-hooks.md) | Claude Code agent team quality gate hooks (Phase 2) |

---

## How to Update

Regenerated automatically on each commit. To run manually:
```bash
bash scripts/update-docs.sh
```
