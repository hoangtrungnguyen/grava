# Grava

**The Distributed, Agent-Centric Issue Tracker**

Grava is a next-generation issue tracking system designed specifically for autonomous AI agents. Unlike traditional tools built for human managers, Grava provides a **deterministic, graph-based memory system** that allows fleets of agents to coordinate complex software development tasks without hallucinations or race conditions.

> "Remove the need for managers."

## 🚀 Key Features

*   **Dolt-Backed Storage**: Utilizes [Dolt](https://github.com/dolthub/dolt), a version-controlled SQL database, to enable `git`-like semantics (branch, merge, diff) for your issue tracker.
*   **The Ready Engine**: A DAG-based (Directed Acyclic Graph) task selection engine that mathematically guarantees agents only work on unblocked, high-priority tasks.
*   **Agent-Native Interface**: Exposes a structured MCP (Model Context Protocol) server instead of a web UI, allowing agents to interact via strictly typed tools.
*   **Distributed Synchronization**: Supports offline-first development with a background daemon that syncs state between local replicas and a central server.
*   **Flight Recorder**: Comprehensive logging and artifact storage to debug agent decision-making processes ("vibe coding").

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

## � Installation

To install Grava to your `/usr/local/bin`, run:

```bash
curl -sL https://raw.githubusercontent.com/hoangtrungnguyen/grava/main/scripts/install.sh | bash
```

## 🛠️ Development Setup

**Prerequisites:** Go 1.25+, [Dolt](https://github.com/dolthub/dolt), mysql-client

```bash
# 1. Start the Dolt SQL server
./scripts/start_dolt_server.sh

# 2. Initialize the schema
./scripts/apply_schema.sh

# 3. Build the CLI
go build -o bin/grava ./cmd/grava/

# 4. Create your first issue
./bin/grava create --title "Fix login bug" --type bug --priority high

# 5. List issues
./bin/grava list
```

## 🖥️ CLI Commands

| Command | Description |
|---|---|
| `grava init` | Initialize Grava environment |
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

## 🧪 Testing

```bash
# Unit tests (no DB required)
go test ./...

# Full E2E smoke tests (requires Dolt running)
./scripts/test/e2e_test_all_commands.sh
```

## License

[MIT](LICENSE)
