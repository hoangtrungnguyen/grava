# Source Tree Analysis: Grava

## Folder Structure

```
grava/
├── cmd/
│   └── grava/                  # Main entry point (main.go)
├── pkg/
│   ├── cmd/                    # Cobra command definitions
│   │   ├── bootstrap.go        # `grava bootstrap` — agent-team plugin install helper
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
├── .claude/
│   ├── agents/                 # Agent team prompts (coder, reviewer, bug-hunter, planner, pr-creator)
│   ├── skills/
│   │   ├── ship/               # `/ship` orchestrator skill
│   │   ├── plan/               # `/plan` orchestrator skill
│   │   ├── hunt/               # `/hunt` orchestrator skill
│   │   └── grava-*/            # Pipeline-active skills (grava-cli, grava-dev-task, grava-code-review, grava-bug-hunt, grava-gen-issues, …)
│   └── settings.json           # Hook registrations (PostToolUse, Stop, TaskCompleted, …)
├── .claude-plugin/
│   └── marketplace.json        # Plugin marketplace catalog (story 2B.17)
├── plugins/
│   └── grava/                  # Plugin distribution: skills + agents + hooks bundled
├── scripts/
│   ├── hooks/                  # Stop + TaskCompleted/Created + TeammateIdle hooks (warn-in-progress, validate-task-complete, …)
│   ├── git-hooks/              # commit-msg hook (bug-hunt token enqueue)
│   ├── pr-merge-watcher.sh     # cron — async PR merge tracking (Phase 4)
│   ├── pre-merge-check.sh      # pre-PR cross-branch regression probe
│   ├── run-pending-hunts.sh    # cron — hourly bug-hunt drain
│   ├── install-hooks.sh        # one-time per-clone git-hook installer
│   └── preflight-gh.sh         # `gh` CLI auth precheck
├── .github/workflows/
│   └── pre-merge-check.yml     # CI mirror of the local pre-merge probe
├── docs/                       # Project documentation
│   ├── detail-impl/            # Per-package implementation reference
│   └── guides/                 # Operational and integration guides (incl. AGENT_TEAM.md)
├── plan/
│   └── phase2B/                # Active phase plan (agent-team pipeline)
├── _bmad/                      # BMAD framework configuration
└── _bmad-output/               # BMAD planning artifacts
```

> **Note:** Sandbox validation tests previously lived in `sandbox/` at the repo root. They have been extracted to the external `gravav6-sandbox` repository. Embedded scenarios are still available via `grava sandbox <ts##>`.

## Critical Files
- **`go.mod`** — dependency manifest (Go 1.24+).
- **`cmd/grava/main.go`** — application entry point.
- **`pkg/cmd/root.go`** — Cobra root command and `Persistent*RunE` lifecycle.
- **`pkg/cmd/bootstrap.go`** — `grava bootstrap` agent-team plugin install helper (binary check, git hook install, cron print).
- **`pkg/dolt/client.go`** — database client implementation.
- **`pkg/dolt/tx.go`** — `WithAuditedTx` transaction + audit wrapper.
- **`pkg/migrate/migrations/`** — SQL schema history (`001_…` … `011_…`).
- **`pkg/cmd/issues/claim.go`** — atomic claim with worktree provisioning and rollback.
- **`pkg/cmd/issues/wisp.go`** — Wisp ephemeral state read/write/delete (read by `/ship` re-entry, watcher, `grava doctor`).
- **`pkg/cmd/maintenance/maintenance.go`** — `grava doctor` checks (DB, tables, ghost worktrees, expired leases, stale heartbeat).
- **`pkg/merge/merge.go`** — LWW 3-way merge engine.
- **`pkg/merge/git_driver_integration_test.go`** — `//go:build integration` tests against real `git merge`.
- **`.claude/skills/ship/SKILL.md`** — `/ship` orchestrator (Phase 0 discover/gate, Phase 1–4 agent dispatch).
- **`.claude/agents/coder.md`** — coder agent: implements via grava-dev-task TDD workflow.
- **`scripts/pr-merge-watcher.sh`** — async PR merge tracker; closes grava issues on merge, distils rejection notes on close-without-merge.

## Integration Points
- **Dolt / MySQL** — persistence layer (default port `3306`, sandbox uses `3315`).
- **Git** — worktrees, merge driver, hook stubs (incl. `commit-msg` for bug-hunt token enqueue).
- **GitHub** — via `pkg/notify` + `pkg/cmd/sync` for issue tracker exchange; `gh` CLI for PR create/poll (used by pr-creator agent + watcher).
- **Claude Code** — `.claude/settings.json` registers `Stop` (warn-in-progress), `TaskCompleted` (validate-task-complete), `TaskCreated` (review-loop-guard), `TeammateIdle` (check-teammate-idle) hooks. Plugin distribution via `.claude-plugin/marketplace.json` + `plugins/grava/`. (Pipeline-phase writes go through `grava signal` directly — no PostToolUse hook needed.)
- **Cron** — `scripts/pr-merge-watcher.sh` (every 5 min), `scripts/run-pending-hunts.sh` (hourly), nightly `claude -p "/hunt since-last-tag"`.
- **BMAD** — agentic workflow orchestration definitions under `_bmad/` and outputs in `_bmad-output/`.
