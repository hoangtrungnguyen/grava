# Grava

**The Distributed, Agent-Centric Issue Tracker**

## 📝 Project Brief

Grava is a next-generation issue tracking system designed specifically for autonomous AI agents. Unlike traditional tools built for human managers, Grava provides a **deterministic, graph-based memory system** that allows fleets of agents to coordinate complex software development tasks without hallucinations or race conditions.

> "Remove the need for managers."

### 🚀 Key Features

*   **Dolt-Backed Storage**: Utilizes [Dolt](https://github.com/dolthub/dolt), a version-controlled SQL database, to enable `git`-like semantics (branch, merge, diff) for your issue tracker.
*   **The Ready Engine**: A DAG-based (Directed Acyclic Graph) task selection engine that mathematically guarantees agents only work on unblocked, high-priority tasks.
*   **Agent Team Pipeline**: Full Claude Code agent team (`coder`, `reviewer`, `bug-hunter`, `planner`, `pr-creator`) wired through `/ship`, `/plan`, `/hunt` skills — takes issues from spec → code → review → PR → merge. See [Agent Team Guide](docs/guides/AGENT_TEAM.md).
*   **Agent-Native Interface**: Exposes a structured MCP (Model Context Protocol) server instead of a web UI, allowing agents to interact via strictly typed tools.
*   **Distributed Synchronization**: Supports offline-first development with a background daemon that syncs state between local replicas and a central server.
*   **Flight Recorder**: Comprehensive logging and artifact storage to debug agent decision-making processes ("vibe coding").

## 📥 How to Install

### Option 1: Go Install (Recommended)

If you have Go installed, this is the fastest way — no `sudo`, no `curl`:

```bash
go install github.com/hoangtrungnguyen/grava/cmd/grava@latest
```

### Option 2: Shell Script

Downloads the latest binary to `~/.local/bin` (no `sudo` required):

```bash
curl -sL https://raw.githubusercontent.com/hoangtrungnguyen/grava/main/scripts/install.sh | bash
```

> **Tip:** To install to a different directory, set `INSTALL_DIR`:
> ```bash
> curl -sL https://raw.githubusercontent.com/hoangtrungnguyen/grava/main/scripts/install.sh | INSTALL_DIR=/usr/local/bin bash
> ```

### Option 3: Try with Docker (Zero-Commitment Sandbox)

Want to try Grava without installing anything on your host machine? Use our pre-configured Docker sandbox:

#### Using Docker Run:
```bash
docker run -it ghcr.io/hoangtrungnguyen/grava:latest
```

This will automatically initialize a Grava and Dolt environment and drop you into a read-to-use bash shell.

#### Using Docker Compose:
If you have the repository cloned, you can simply run:
```bash
docker compose run --rm sandbox
```

### Verify Installation

```bash
grava version
```

## ❓ Troubleshoot Installing

### `grava: command not found`

Your install directory is not in your `PATH`. If you used `go install`, ensure `$GOPATH/bin` (usually `~/go/bin`) is in your PATH:

```bash
export PATH="$HOME/go/bin:$PATH"
```

If you used the shell script, add `~/.local/bin`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

### `failed to connect to database`

The Dolt server is not running. Start it:

```bash
grava start
```

Or re-initialize if this is a fresh project:

```bash
grava init
```

### `port 3306 is already in use`

Another Dolt server or MySQL instance is using the default port. Grava will automatically pick an available port during `grava init`. If you're running `grava start` manually, check `.grava.yaml` for the configured port.

### Health check

Run the built-in diagnostics:

```bash
grava doctor
```

## 🖥️ Common Grava Commands

Get from zero to your first issue in under 60 seconds:

```bash
# 1. Initialize in your project (auto-installs Dolt, starts server)
grava init

# 2. Create your first issue
grava create --title "My first task" --type task --priority medium

# 3. See what's tracked
grava list
```

### Command Reference

| Command | Description |
|---|---|
| `grava init` | Initialize Grava environment |
| `grava version` | Print the version number |
| `grava create` | Create a new issue (`--ephemeral` for Wisps) |
| `grava subtask <id>` | Create a hierarchical subtask |
| `grava show <id>` | Show issue details |
| `grava list` | List issues (`--wisp` for ephemeral) |
| `grava update <id>` | Update issue fields |
| `grava comment <id> <text>` | Append a comment |
| `grava dep <from> <to>` | Create a dependency edge |
| `grava label <id> <label>` | Add a label |
| `grava assign <id> <user>` | Assign to a user or agent |
| `grava history <id>` | View revision history for an issue |
| `grava undo <id>` | Revert the last change to an issue |
| `grava compact` | Purge old ephemeral Wisps |

See **[CLI Reference](docs/guides/CLI_REFERENCE.md)** for full documentation.

## 🤖 Agent Team (Claude Code Pipeline)

Grava ships a multi-agent pipeline as a Claude Code plugin. After installing the binary:

```bash
# In Claude Code session, inside any project:
/plugin marketplace add hoangtrungnguyen/grava
/plugin install grava@grava
grava bootstrap                 # installs git hook, prints cron lines
```

You get 3 slash-commands + 5 agents + 13 skills + auto-registered hooks:

| Command | Purpose |
|---------|---------|
| `/ship [id]` | Ship one issue end-to-end (code → review → PR → handoff) |
| `/ship` (no id) | Auto-pick next ready `task`/`bug` from backlog |
| `/plan <doc>` | Break a spec document into a grava issue hierarchy |
| `/hunt [scope]` | Audit codebase for bugs, file as grava issues |

Async PR-merge tracking via `scripts/pr-merge-watcher.sh` (cron). Hourly bug-hunt drain via `scripts/run-pending-hunts.sh`.

Full reference: **[Agent Team Guide](docs/guides/AGENT_TEAM.md)** — phases, signals, wisp keys, recovery patterns.

## 🧩 Core Modules

The system is built upon the following core epics/modules:

1.  **[Storage Substrate](docs/epics/Epic_1_Storage_Substrate.md)**: Dolt initialization and schema.
2.  **[Graph Mechanics](docs/epics/Epic_2_Graph_Mechanics.md)**: Dependency logic and topological sorting.
3.  **[Git Merge Driver](docs/epics/Epic_3_Git_Merge_Driver.md)**: Schema-aware merging for `issues.jsonl`.
4.  **[Flight Recorder](docs/epics/Epic_4_Log_Saver.md)**: Structured logging and session context.
5.  **[Security](docs/epics/Epic_5_Security.md)**: mTLS and RBAC for agent safety.
6.  **[MCP Integration](docs/epics/Epic_6_MCP_Integration.md)**: The interface for AI agents.
7.  **[Advanced Analytics](docs/epics/Epic_7_Advanced_Analytics.md)**: PageRank and critical path analysis (Optional).

## 📚 Link to Docs

The project governance and architecture are strictly documented. Here are the most important resources:

*   **[Documentation Index](docs/index.md)**: The root of all Grava documentation.
*   **[Project Overview](docs/project-overview.md)**: Detailed feature breakdown and design philosophy.
*   **[Architecture](docs/architecture.md)**: Deep dive into the system design, Prolly Trees, and the "Ready Engine".
*   **[Development Guide](docs/development-guide.md)**: Information on setting up the local environment, building, and testing.

## 🛠️ Contributing

### Development Setup

**Prerequisites:** Go 1.24+, [Dolt](https://github.com/dolthub/dolt), mysql-client

```bash
# 1. Clone the repository
git clone https://github.com/hoangtrungnguyen/grava.git
cd grava

# 2. Build the CLI
go build -o bin/grava ./cmd/grava/

# 3. Initialize (auto-downloads Dolt, starts server)
./bin/grava init

# 4. Create a test issue
./bin/grava create --title "Fix login bug" --type bug --priority high

# 5. List issues
./bin/grava list
```

### Testing

```bash
# Unit tests (no DB required)
go test ./...

# Full E2E smoke tests (requires Dolt running)
./scripts/test/e2e_test_all_commands.sh
```

## License

[MIT](LICENSE)