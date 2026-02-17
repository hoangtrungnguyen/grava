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

## 🛠️ Getting Started

*Prerequisites: Go 1.21+, Dolt*

*(Coming Soon: Installation instructions during Epic 1 implementation)*

## License

[MIT](LICENSE)
