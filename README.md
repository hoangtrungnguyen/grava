# Grava

**The Distributed, Agent-Centric Issue Tracker**

Grava is a next-generation issue tracking system designed specifically for autonomous AI agents. Unlike traditional tools built for human managers, Grava provides a **deterministic, graph-based memory system** that allows fleets of agents to coordinate complex software development tasks without hallucinations or race conditions.

> "Remove the need for managers."

## ⚡ Quick Start

Get from zero to your first issue in under 60 seconds:

```bash
# 1. Install grava
go install github.com/hoangtrungnguyen/grava/cmd/grava@latest

# 2. Initialize in your project (auto-installs Dolt, starts server)
cd your-project
grava init

# 3. Create your first issue
grava create --title "My first task" --type task --priority medium

# 4. See what's tracked
grava list
```

That's it! Dolt (the version-controlled database) is automatically downloaded and managed for you — no separate install needed.

## 📥 Installation

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

## 🚀 Key Features

*   **Dolt-Backed Storage**: Utilizes [Dolt](https://github.com/dolthub/dolt), a version-controlled SQL database, to enable `git`-like semantics (branch, merge, diff) for your issue tracker.
*   **The Ready Engine**: A DAG-based (Directed Acyclic Graph) task selection engine that mathematically guarantees agents only work on unblocked, high-priority tasks.
*   **Agent-Native Interface**: Exposes a structured MCP (Model Context Protocol) server instead of a web UI, allowing agents to interact via strictly typed tools.
*   **Distributed Synchronization**: Supports offline-first development with a background daemon that syncs state between local replicas and a central server.
*   **Flight Recorder**: Comprehensive logging and artifact storage to debug agent decision-making processes ("vibe coding").

## 🖥️ CLI Commands

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

See **[CLI Reference](docs/CLI_REFERENCE.md)** for full documentation.

## ❓ Troubleshooting

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

## 📚 Documentation

The project governance and architecture are strictly documented:

*   **[Architecture Overview](docs/AI%20Agent%20Issue%20Tracker%20Architecture%20&%20MVP.md)**: Deep dive into the system design, Prolly Trees, and the "Ready Engine".
*   **[MVP Epics & Roadmap](docs/Agent_Issue_Tracker_MVP_Epics.md)**: The step-by-step implementation plan.

### Core Modules (Epics)

1.  **[Storage Substrate](docs/epics/Epic_1_Storage_Substrate.md)**: Dolt initialization and schema.
2.  **[Graph Mechanics](docs/epics/Epic_2_Graph_Mechanics.md)**: Dependency logic and topological sorting.
3.  **[Git Merge Driver](docs/epics/Epic_3_Git_Merge_Driver.md)**: Schema-aware merging for `issues.jsonl`.
4.  **[Flight Recorder](docs/epics/Epic_4_Log_Saver.md)**: Structured logging and session context.
5.  **[Security](docs/epics/Epic_5_Security.md)**: mTLS and RBAC for agent safety.
6.  **[MCP Integration](docs/epics/Epic_6_MCP_Integration.md)**: The interface for AI agents.
7.  **[Advanced Analytics](docs/epics/Epic_7_Advanced_Analytics.md)**: PageRank and critical path analysis (Optional).

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
