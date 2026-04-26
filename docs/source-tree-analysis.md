# Source Tree Analysis: Grava

## Folder Structure

```
grava/
├── cmd/
│   └── grava/                  # Main entry point (main.go)
├── pkg/
│   ├── cmd/                    # Cobra command definitions
│   │   ├── issues/             # Issue management subcommands
│   │   ├── graph/              # Dependency graph commands
│   │   ├── maintenance/        # doctor, compact, clear, cmd-history
│   │   ├── reserve/            # File reservation commands
│   │   ├── sandbox/            # Validation scenarios (TS-01..TS-10)
│   │   └── sync/               # Export/import (JSONL)
│   ├── cmddeps/                # Shared CLI deps + JSON error emitter
│   ├── coordinator/            # Goroutine lifecycle + error channels
│   ├── dolt/                   # Dolt client + transaction wrappers
│   ├── doltinstall/            # Dolt binary install
│   ├── errors/                 # GravaError type and code catalogue
│   ├── gitattributes/          # .gitattributes management
│   ├── gitconfig/              # .git/config merge driver registration
│   ├── githooks/               # Hook stub deployment
│   ├── graph/                  # DAG engine and traversal
│   ├── grava/                  # Domain bootstrap, .grava/ resolution
│   ├── idgen/                  # Hierarchical ID generation
│   ├── log/                    # Zerolog global logger
│   ├── devlog/                 # Deprecated no-op stub
│   ├── merge/                  # LWW 3-way merge driver for issues.jsonl
│   ├── migrate/                # SQL schema migrations (Goose, embedded)
│   ├── notify/                 # Notification abstraction
│   ├── orchestrator/           # Background poller/worker pool/watchdog
│   ├── utils/                  # Shared utilities (worktree, dolt resolve)
│   └── validation/             # Input validators (type, status, …)
├── internal/
│   └── testutil/               # Shared testing helpers
├── scripts/
│   └── hooks/                  # Claude Code agent quality-gate hooks
├── docs/                       # Project documentation
│   ├── detail-impl/            # Per-package implementation reference
│   └── guides/                 # Operational and integration guides
├── _bmad/                      # BMAD framework configuration
└── _bmad-output/               # BMAD planning artifacts
```

> **Note:** Sandbox validation tests previously lived in `sandbox/` at the repo root. They have been extracted to the external `gravav6-sandbox` repository. Embedded scenarios are still available via `grava sandbox <ts##>`.

## Critical Files
- **`go.mod`** — dependency manifest (Go 1.24+).
- **`cmd/grava/main.go`** — application entry point.
- **`pkg/cmd/root.go`** — Cobra root command and `Persistent*RunE` lifecycle.
- **`pkg/dolt/client.go`** — database client implementation.
- **`pkg/dolt/tx.go`** — `WithAuditedTx` transaction + audit wrapper.
- **`pkg/migrate/migrations/`** — SQL schema history (`001_…` … `011_…`).
- **`pkg/cmd/issues/claim.go`** — atomic claim with worktree provisioning and rollback.
- **`pkg/cmd/maintenance/maintenance.go`** — `grava doctor` checks (DB, tables, ghost worktrees, expired leases).
- **`pkg/merge/merge.go`** — LWW 3-way merge engine.
- **`pkg/merge/git_driver_integration_test.go`** — `//go:build integration` tests against real `git merge`.

## Integration Points
- **Dolt / MySQL** — persistence layer (default port `3306`, sandbox uses `3315`).
- **Git** — worktrees, merge driver, hook stubs.
- **GitHub** — via `pkg/notify` and `pkg/cmd/sync` for external issue tracker exchange.
- **Claude Code** — `.claude/settings.json` hooks under `scripts/hooks/` enforce agent-team quality gates.
- **BMAD** — agentic workflow orchestration definitions under `_bmad/` and outputs in `_bmad-output/`.
