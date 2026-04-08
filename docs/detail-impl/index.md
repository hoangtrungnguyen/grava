# `docs/detail-impl/` — Module Implementation Reference

This folder contains **detailed implementation documentation** for every Go package in the Grava project.

> _Auto-generated on 2026-04-08 at commit `baefe84`. Updated on every `git commit` via `.git/hooks/post-commit`._

---

## Module Index

| Module | File | Purpose |
|:---|:---|:---|
| `pkg/cmd` | [pkg-cmd.md](./pkg-cmd.md) | CLI command registration, lifecycle, and all sub-commands |
| `pkg/cmddeps` | [pkg-cmddeps.md](./pkg-cmddeps.md) | Shared dependency injection, JSON error emitter |
| `pkg/dolt` | [pkg-dolt.md](./pkg-dolt.md) | Database persistence layer, `WithAuditedTx`, retry logic |
| `pkg/graph` | [pkg-graph.md](./pkg-graph.md) | DAG engine, traversal, priority inheritance, gate evaluation |
| `pkg/grava` | [pkg-grava.md](./pkg-grava.md) | Domain bootstrap, `.grava/` directory resolution |
| `pkg/errors` | [pkg-errors.md](./pkg-errors.md) | Structured `GravaError` type and error code catalogue |
| `pkg/idgen` | [pkg-idgen.md](./pkg-idgen.md) | Hierarchical ID generation (`grava-xxxx` and `grava-xxxx.1`) |
| `pkg/migrate` | [pkg-migrate.md](./pkg-migrate.md) | Goose-based schema migrations (embedded SQL) |
| `pkg/notify` | [pkg-notify.md](./pkg-notify.md) | Notification abstraction (`ConsoleNotifier`, future integrations) |
| `pkg/coordinator` | [pkg-coordinator.md](./pkg-coordinator.md) | Background goroutine lifecycle and error propagation |
| `pkg/validation` | [pkg-validation.md](./pkg-validation.md) | Input validators (type, status, priority, date range) |
| `pkg/utils` | [pkg-utils.md](./pkg-utils.md) | Dolt binary resolution, git exclude, network helpers |
| `pkg/log` + `pkg/devlog` | [pkg-log.md](./pkg-log.md) | Zerolog global logger; devlog is deprecated no-op stub |
| `pkg/doltinstall` | [pkg-doltinstall.md](./pkg-doltinstall.md) | Automated Dolt binary download + install |

---

## How to Update

Regenerated automatically on each commit. To run manually:
```bash
bash scripts/update-docs.sh
```
