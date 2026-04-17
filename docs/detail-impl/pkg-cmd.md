# Module: `pkg/cmd`

**Package role:** CLI command registration layer. Wires all Cobra commands, manages lifecycle (PersistentPreRunE / PersistentPostRunE), and builds the shared `cmddeps.Deps` container.

> _Updated 2026-04-17 (comprehensive doc review)._

---

## Sub-commands (pkg/cmd/issues/)

| File | Command |
|:---|:---|
| `assign.go` | `grava assign <id>` |
| `claim.go` | `grava claim <issue-id>` |
| `comment.go` | `grava comment <id> [text]` |
| `create.go` | `grava create` |
| `drop.go` | `grava drop [id]` |
| `history.go` | `grava history <issue-id>` |
| `issues.go` | `grava show <id>` |
| `label.go` | `grava label <id>` |
| `start.go` | `grava start <id>` |
| `stop.go` | `grava stop <id>` |
| `subtask.go` | `grava subtask <parent_id>` |
| `undo.go` | `grava undo <id>` |
| `update.go` | `grava update <id>` |
| `wisp.go` | `grava wisp` |
| `close.go` | `grava close <id>` |

## Files (pkg/cmd/)

| File | Description |
|:---|:---|
| `config.go` | `grava config` — display current configuration |
| `conflicts.go` | `grava conflicts` — view and resolve merge conflicts |
| `db_server.go` | `grava db-start` / `grava db-stop` — Dolt server lifecycle |
| `hook.go` | `grava hook run <name>` — Git hook dispatch (pre-commit reservation enforcement, post-merge sync) |
| `init.go` | `grava init` — repository initialization (Dolt, git config, worktree, Claude settings) |
| `install.go` | `grava install` — register merge driver and git hooks |
| `merge_driver.go` | `grava merge-driver` — LWW 3-way merge for issues.jsonl |
| `merge_slot.go` | `grava merge-slot` — serialize concurrent merge operations |
| `orchestrate.go` | `grava orchestrate` — background task orchestration |
| `resolve.go` | `grava resolve` — dependency resolution commands |
| `root.go` | Root command, PersistentPreRunE/PostRunE lifecycle, audit logging |
| `sync_status.go` | `grava sync-status` — show sync state between DB and JSONL |
| `version.go` | `grava version` |

## Sub-packages

| Package | Description |
|:---|:---|
| `pkg/cmd/issues/` | Issue CRUD commands (create, show, update, claim, label, comment, etc.) |
| `pkg/cmd/graph/` | Dependency graph commands (dep, ready, blocked, visualize, search, stats) |
| `pkg/cmd/maintenance/` | Health commands (doctor, compact, cmd_history, clear-archived) |
| `pkg/cmd/reserve/` | File reservation commands (reserve, enforce) |
| `pkg/cmd/sandbox/` | Sandbox validation scenarios (TS-01 through TS-10) |
| `pkg/cmd/sync/` | Export/import commands (export, import, sync) |

