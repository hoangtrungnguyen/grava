# Grava

**The Distributed, Agent-Centric Issue Tracker**

Grava is an issue tracking system built for autonomous AI agents. It provides a deterministic, graph-based memory layer that lets agent fleets coordinate complex software tasks without hallucinations or race conditions.

> "Remove the need for managers."

## Key Features

| Feature | Summary |
|---|---|
| **Dolt-Backed Storage** | Version-controlled SQL database with `git`-like branch/merge/diff semantics |
| **Ready Engine** | DAG-based task selector — agents only pick up unblocked, highest-priority work |
| **Agent Team Pipeline** | Full Claude Code agent team (`coder`, `reviewer`, `bug-hunter`, `planner`, `pr-creator`) wired through `/ship`, `/plan`, `/hunt` — spec → code → review → PR → merge |
| **MCP Interface** | Structured Model Context Protocol server; no web UI, strictly typed agent tools |
| **Distributed Sync** | Offline-first; background daemon syncs local replicas to a central server |
| **Flight Recorder** | Structured logs and artifact storage for debugging agent decision trails |
| **Wisp (Ephemeral Issues)** | Lightweight scratch issues for transient agent state; auto-purged via `grava compact` |
| **History & Undo** | Full revision history per issue; single-command rollback via `grava undo` |

## Install

**Go install (recommended):**
```bash
go install github.com/hoangtrungnguyen/grava/cmd/grava@latest
```

**Shell script** (installs to `~/.local/bin`):
```bash
curl -sL https://raw.githubusercontent.com/hoangtrungnguyen/grava/main/scripts/install.sh | bash
```

**Docker sandbox:**
```bash
docker run -it ghcr.io/hoangtrungnguyen/grava:latest
# or, from the repo:
docker compose run --rm sandbox
```

**Verify:**
```bash
grava version
```

## Quickstart

```bash
grava init                                              # initialize + start Dolt server
grava create --title "My first task" --type task --priority medium
grava list
```

## Command Reference

| Command | Description |
|---|---|
| `grava init` | Initialize Grava environment |
| `grava version` | Print version |
| `grava create` | Create an issue (`--ephemeral` for Wisps) |
| `grava subtask <id>` | Create a hierarchical subtask |
| `grava show <id>` | Show issue details |
| `grava list` | List issues (`--wisp` for ephemeral) |
| `grava update <id>` | Update issue fields |
| `grava comment <id> <text>` | Append a comment |
| `grava dep <from> <to>` | Create a dependency edge |
| `grava label <id> <label>` | Add a label |
| `grava assign <id> <user>` | Assign to a user or agent |
| `grava history <id>` | View revision history |
| `grava undo <id>` | Revert last change to an issue |
| `grava compact` | Purge old ephemeral Wisps |
| `grava doctor` | Run built-in diagnostics |

See **[CLI Reference](docs/guides/CLI_REFERENCE.md)** for full documentation.

## Agent Team (Claude Code Pipeline)

Grava ships a multi-agent pipeline as a Claude Code plugin:

```bash
/plugin marketplace add hoangtrungnguyen/grava
/plugin install grava@grava
grava bootstrap          # installs git hook, prints cron lines
```

| Command | Purpose |
|---|---|
| `/ship [id]` | Ship one issue end-to-end (code → review → PR → handoff) |
| `/ship` (no id) | Auto-pick next ready `task`/`bug` from backlog |
| `/plan <doc>` | Break a spec document into a grava issue hierarchy |
| `/hunt [scope]` | Audit codebase for bugs, file as grava issues |

Full reference: **[Agent Team Guide](docs/guides/AGENT_TEAM.md)**

## Troubleshooting

| Error | Fix |
|---|---|
| `grava: command not found` | Add `~/go/bin` or `~/.local/bin` to `$PATH` |
| `failed to connect to database` | Run `grava start` or `grava init` |
| `port 3306 is already in use` | Check `.grava.yaml` for the configured port; `grava init` auto-picks a free port |

## Architecture

Grava is built on **Dolt** (version-controlled SQL) with layered components: CLI (Cobra), domain bootstrap, persistence with audited transactions, DAG graph engine, embedded migrations (Goose), schema-aware merge driver, file-reservation system, and a Claude Code agent team plugin.

Full breakdown: **[Architecture](docs/architecture.md)** — 12 components, data flow, invariants, design decisions.

## Docs

- [Documentation Index](docs/index.md)
- [Project Overview](docs/project-overview.md)
- [Architecture](docs/architecture.md)
- [Development Guide](docs/development-guide.md)

## Contributing

**Prerequisites:** Go 1.24+, [Dolt](https://github.com/dolthub/dolt), mysql-client

```bash
git clone https://github.com/hoangtrungnguyen/grava.git && cd grava
go build -o bin/grava ./cmd/grava/
./bin/grava init
```

**Tests:**
```bash
go test ./...                                 # unit tests (no DB required)
./scripts/test/e2e_test_all_commands.sh       # E2E smoke tests (requires Dolt)
```

## License

[MIT](LICENSE)
