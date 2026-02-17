# **Architecture Design and MVP Plan for a Dolt-Backed Agentic Issue Tracker**

## **The Paradigm Shift in Autonomous Software Engineering**

The discipline of software engineering is currently undergoing a structural transformation catalyzed by the advent of autonomous Large Language Model (LLM) agents. Historically, the software development lifecycle has been constrained by human cognitive limits, necessitating project management methodologies and tools—such as Agile frameworks, Jira, Linear, and localized Markdown ledgers—that map directly to human patterns of task execution. However, as AI coding assistants evolve from passive autocomplete engines into autonomous agents capable of orchestrating long-running tasks and collaborating across complex codebases, these traditional management tools have emerged as critical operational bottlenecks.1

Autonomous agents, despite their sophisticated reasoning capabilities, are fundamentally constrained by context window limitations and a phenomenon frequently characterized as the "50 First Dates" problem.2 An agent initiates every new computational session devoid of persistent state, historical continuity, or long-term memory. When development teams attempt to bridge this amnesia using standard text-based planning documents, such as TODO.md or PLAN.md, the architecture inevitably collapses under the weight of concurrent operations.3 Markdown files are inherently unqueryable; an agent must consume the entire document within its context window merely to deduce which task is actionable, resulting in severe token inefficiency and rapid context degradation.2 Furthermore, in a multi-agent topology where multiple autonomous entities attempt to mutate a single planning document simultaneously, the system inevitably generates Git merge conflicts that corrupt the task graph and paralyze the development pipeline.3

To resolve these profound architectural deficiencies, the ecosystem requires a transition toward a stateful, highly structured, and deterministic external memory system. The foundational concepts pioneered by Steve Yegge in the "Beads" architecture demonstrate that redefining an issue tracker from a human-readable ledger into a machine-optimized, graph-based database dramatically elevates the operational autonomy of AI agents.3 By migrating this conceptual framework to a dedicated synchronization server backed by Dolt—a version-controlled SQL database—the architecture achieves a mathematically rigorous, multi-agent coordination layer capable of distributed execution.5

The ensuing report provides an exhaustive architectural design and a Minimum Viable Product (MVP) execution plan for building a highly scalable, agentic issue tracker. This system leverages a centralized Dolt synchronization server to coordinate fleets of autonomous agents, utilizes a directed acyclic graph to deterministically compute unblocked "ready" work, and enforces strict access controls to mitigate unauthorized mutations, thereby ensuring a resilient infrastructure for autonomous software development.

## **The Architectural Substrate: Git-for-Data and Dolt**

The foundational prerequisite for an agentic issue tracker is a storage substrate capable of supporting highly concurrent, disconnected, and untrusted write operations at machine speed.5 Traditional relational databases (such as PostgreSQL or MySQL) maintain a single, mutable state; when an UPDATE or DELETE command is executed, the previous state is permanently overwritten unless complex, application-layer audit tables are manually maintained. Conversely, localized file-based databases (such as SQLite) lack the native concurrency controls required for distributed, multi-node synchronization.7

The proposed architecture resolves this dichotomy by employing Dolt as the primary storage backend.9 Dolt operates as the world's only version-controlled SQL database, fusing the relational querying capabilities of standard MySQL with the distributed version control mechanics of Git.6 This "Git-for-Data" paradigm fundamentally alters how agentic memory is managed, distributed, and synchronized across a network.

### **Prolly Trees and Cell-Level Merging**

At its lowest operational layer, Dolt discards traditional B-Tree storage mechanisms in favor of Prolly Trees (Probabilistic B-Trees).6 This cryptographic data structure allows the database to efficiently compute structural differences between two distinct states of a massive table, effectively enabling native branch, diff, and merge operations on structured relational data.10

For a multi-agent issue tracker, this structural capability provides a profound operational advantage. In a highly distributed environment, Agent Alpha might be operating on a feature branch, autonomously updating the status of a specific task to in\_progress. Simultaneously, Agent Beta, operating on the main branch, might update the priority of that exact same task.5 In a traditional SQL architecture, this scenario risks race conditions or necessitates row-level locking that halts asynchronous execution. Dolt, however, computes these discrete alterations deterministically, executing a cell-level three-way merge that integrates both changes without generating a conflict, entirely bypassing the need for human intervention.5

### **Immutable Audit Trails and State Rollbacks**

Because autonomous agents execute decisions at a velocity far exceeding human oversight, the system must guarantee absolute forensic observability. Dolt satisfies this requirement natively. Every mutation to the database—whether an agent creates a new epic, reassigns a bug, or links a dependency—is encapsulated within a discrete commit.13 Each commit contains a cryptographic hash, a timestamp, the authenticated identity of the actor, and a reference to its parent commit.13

This immutable lineage ensures that the entire history of the project's task graph is preserved. If an agent misinterprets a directive and executes a destructive cascade of task deletions, a human operator or a supervisory agent can instantaneously query the commit history and execute a DOLT\_REVERT() procedure, rolling the database back to its exact state prior to the erroneous execution.13 This capability transforms the issue tracker into a highly fault-tolerant memory system where catastrophic data loss is mathematically impossible.4

### **The Server and Replica Topology**

To coordinate a localized development team alongside a fleet of autonomous agents, the architecture utilizes a hybrid topology that supports both offline execution and centralized synchronization.

The architecture supports several operational modes, adapting to the specific concurrency requirements of the deployment environment. The optimal design for a dedicated team relies on a centralized synchronization server communicating with localized replicas.

| Architectural Component | Functional Description | Technical Implementation |
| :---- | :---- | :---- |
| **Dedicated Sync Server** | Acts as the authoritative Single Source of Truth (SSOT). Handles global replication, authentication, and branch management. | dolt sql-server deployed on centralized infrastructure, exposing the remotesapi on port 50051\.15 |
| **Local Workspace Replica** | The agent's immediate querying interface. Enables zero-latency graph traversals and offline commits without network overhead. | A local Dolt database clone residing in the .grava/dolt/ directory of the project workspace.16 |
| **Workspace Daemon** | A localized background process running on the agent's host machine. Batches queries and orchestrates automated syncs. | A Go-based binary utilizing Unix domain sockets (.grava/grava.sock) to communicate with the local replica.17 |
| **JSONL Bridge** | Maintains human-readable, Git-portable backups of the SQL state for legacy fallback recovery and human code reviews. | Automated Git hooks (pre-commit, post-merge) exporting specific SQL views to an issues.jsonl file.18 |

This topology ensures that the issue tracker resides as close to the codebase as possible. Agents operate against their local replica with sub-millisecond latency, entirely insulated from network instability. The synchronization mechanics are abstracted away by the workspace daemon, which continuously aligns the local state with the dedicated sync server.17

## **Structuring Agentic Thought: The Data Schema**

To facilitate machine cognition and bypass the parsing limitations inherent to unstructured text, the database schema must be rigidly defined, explicitly typed, and highly optimized for recursive graph queries. The architecture mandates a departure from traditional database design patterns—specifically the reliance on auto-incrementing integers for primary keys. In a distributed, offline-first environment where multiple agents generate issues concurrently on separate branches, integer keys inevitably collide upon merging.9
Instead, the architecture employs a hash-based alphanumeric identification protocol (e.g., grava-a1b2), extended by a hierarchical suffixing strategy (grava-a1b2.1, grava-a1b2.1.2) to support atomic sub-task generation without collision.9

Instead, the architecture employs a hash-based alphanumeric identification protocol (e.g., grava-a1b2). This cryptographic approach mathematically guarantees zero-conflict identifier generation across multiple discrete agent branches, preserving the integrity of the graph during complex multi-agent merges.9

### **The Core Entity Tables**

The structural foundation of the issue tracker relies on three primary tables, meticulously designed to support agentic reasoning, dependency tracking, and operational forensics.

**The Issues Table**

The issues table acts as the primary ledger for all project objectives, bugs, and feature requests. It accommodates standard project management metadata alongside agent-specific operational fields designed to dictate execution parameters.

| Column Name | Data Type | Structural Constraints | Semantic Description |
| :---- | :---- | :---- | :---- |
| id | VARCHAR(32) | PRIMARY KEY | The hierarchical unique identifier. Hashed prefix + atomic suffix (e.g., grava-x9f2.1.3).17 |
| title | VARCHAR(255) | NOT NULL | A concise, semantic summary of the objective, heavily weighted during vector searches. |
| description | LONGTEXT | NULL | Detailed acceptance criteria, contextual requirements, and necessary code blocks. |
| status | VARCHAR(32) | DEFAULT 'open' | Operational state constraints. Valid values include: open, in\_progress, blocked, closed, tombstone, deferred, and pinned.17 |
| ephemeral | BOOLEAN | DEFAULT FALSE | If true, the issue is a "Wisp" (temporary memory) and is excluded from JSONL exports. |
| priority | INT | DEFAULT 4 | A strictly enforced numerical ranking from 0 (Critical priority) to 4 (Backlog priority).17 |
| issue\_type | VARCHAR(32) | DEFAULT 'task' | Categorization constraints. Valid values include: bug, feature, task, epic, chore, and message.17 |
| assignee | VARCHAR(128) | NULL | The authenticated identity of the specific agent or human user currently claiming the execution of the task.3 |
| metadata | JSON | NULL | An extensible, schema-less JSON payload utilized for custom external integrations or specialized tool tracking.22 |
| await_type | VARCHAR(32) | NULL | For "Gate" issues: The type of external condition being waited on (e.g., gh:pr, timer, human). |
| await_id | VARCHAR(128) | NULL | For "Gate" issues: The specific identifier of the external condition (e.g., PR number, timestamp). |

**The Dependencies Table**

The true power of the architecture resides in the dependencies table. This table defines the directed edges of the project's internal knowledge graph, enabling agents to understand causal, hierarchical, and temporal relationships between discrete tasks.3

| Column Name | Data Type | Structural Constraints | Semantic Description |
| :---- | :---- | :---- | :---- |
| from\_id | VARCHAR(16) | FOREIGN KEY (issues.id) | The origin node of the relationship edge. |
| to\_id | VARCHAR(16) | FOREIGN KEY (issues.id) | The destination node of the relationship edge. |
| type | VARCHAR(32) | NOT NULL | The semantic nature of the edge. Full 19-type spectrum including: blocks, waits-for, conditional-blocks, supersedes, and authored-by.17 |

**The Events (Audit) Table**

While Dolt provides repository-level version control via its commit history, the architecture must also supply a highly accessible, application-level audit trail for immediate agentic debugging. The events table serves as an append-only ledger capturing every atomic mutation applied to any issue.3

| Column Name | Data Type | Structural Constraints | Semantic Description |
| :---- | :---- | :---- | :---- |
| id | INTEGER | PRIMARY KEY | A localized sequence identifier for the event log entry.25 |
| issue\_id | VARCHAR(16) | NOT NULL | The target issue undergoing the mutation.25 |
| event\_type | VARCHAR(64) | NOT NULL | The semantic categorization of the mutation (e.g., status\_change, reassigned).25 |
| actor | VARCHAR(128) | NOT NULL | The cryptographic identity of the agent or human executing the mutation.3 |
| old\_value | JSON | NULL | The complete JSON representation of the issue's state prior to the mutation.25 |
| new\_value | JSON | NULL | The complete JSON representation of the issue's state following the mutation.25 |
| timestamp | DATETIME | DEFAULT NOW() | The precise temporal marker of the execution.25 |

### **Semantic Memory Decay and Compaction**

A fundamental challenge in sustaining agentic workflows over prolonged project lifecycles is the continuous accumulation of historical data. As thousands of issues are opened, discussed, and closed, the volume of text threatens to overwhelm the LLM's context window during broad project queries, leading to degraded reasoning performance and excessive inference costs.9

The architecture addresses this inherent limitation through a mechanism termed "Compaction" or "Semantic Memory Decay".9 When an issue has remained in a closed or tombstone state for a configurable duration (managed via a Time-To-Live or wisp\_type threshold), the system triggers an asynchronous summarization process.26 A specialized background agent ingests the verbose history of the issue—including extensive conversational threads, debated code snippets, and iterative acceptance criteria—and distills it into a highly dense, high-entropy summary payload. The original, uncompressed data is excised from the active query tables but remains perpetually preserved and accessible within Dolt's immutable commit history (retrievable via standard dolt log or dolt diff commands).10

This compaction strategy ensures that the active tracking database remains exceptionally lean, maximizing the agent's token efficiency and query velocity without sacrificing the forensic integrity of the project's historical archive.4

### **Advanced Workflows: Rigs, Molecules, and Wisps**

To support enterprise-grade autonomous operations, the architecture adopts three advanced primitives from the Beads paradigm:

**Rigs (Multi-Repository Topology)**
Large-scale software ecosystems rarely reside in a single repository. A "Rig" represents a collection of related repositories (e.g., `frontend`, `backend`, `infra`) that share a unified issue tracker. The system utilizes a `routes.jsonl` configuration file to map issue ID prefixes (e.g., `fe-`, `be-`) to specific file paths or sub-repositories. This allows a single Grava instance to trace dependencies across the entire technology stack.

**Molecules (Workflow Templates)**
Agents frequently execute repeatable, multi-step processes such as "Refactor Module" or "Resolving Security Vulnerability". A "Molecule" is a workflow template that automates the generation of a coordinated tree of issues. When an agent creates a molecule (e.g., `mol_type="swarm_refactor"`), the system programmatically spawns the parent objective and all necessary child tasks, essentially "hydrating" a standardized plan into the active graph.

**Wisps (Ephemeral Memory)**
Not all agent thoughts require permanent archival. "Wisps" are ephemeral issues (`ephemeral: true`) used as scratchpads for intermediate reasoning, self-correction, or temporary state (e.g., "Scanning file for X..."). Wisps exist solely in the local Dolt instance and are strictly excluded from the `issues.jsonl` export to prevent noise from polluting the version control history. They are automatically pruned by a `grava compact` maintenance cycle based on their Time-To-Live (TTL).

**Gates (External Condition Waiting)**
"Gates" are a specialized type of issue that represents a dependency on an external condition or event outside the immediate control of the agent fleet. For example, an agent might create a "Gate" issue to wait for a human code review (`await_type="human"`, `await_id="PR-123"`), a CI/CD pipeline to complete (`await_type="gh:pr"`, `await_id="PR-456"`), or a specific time to elapse (`await_type="timer"`, `await_id="2026-03-01T00:00:00Z"`). The `await_type` and `await_id` fields in the `issues` table allow the system to monitor these external conditions and automatically transition the "Gate" issue's status once the condition is met, unblocking downstream tasks.

## **Graph Mechanics: Tracking Blocked Issues and the Ready Engine**

The most critical operational failure point for any autonomous coding agent is the process of task selection. If an agent operates within a traditional issue tracker and inadvertently selects a frontend implementation task that strictly depends on an unwritten backend database schema, the agent will inevitably attempt to hallucinate the missing API endpoints, generate failing test suites, and enter a destructive, looping failure state.2 Therefore, the issue tracker must natively enforce blocking constraints and mathematically guarantee that agents are only ever presented with strictly actionable directives.

This guarantee is realized through the Directed Acyclic Graph (DAG) formed by the interactions within the dependencies table, which is subsequently processed by the system's "Ready Engine".24

### **Semantic Dependency Edges**

The schema does not merely link issues; it enforces strict semantic boundaries on those relationships, defining precisely how nodes within the graph interact and constrain one another 24:

The blocks relationship represents a strict temporal and operational prerequisite.24 If Issue A is defined as blocking Issue B, the system algorithmically prevents Issue B from transitioning into a ready or in\_progress state until Issue A achieves an unequivocally closed status.
The conditional-blocks relationship offers nuanced control: Issue A blocks Issue B *only* if Issue A fails or is closed with a specific resolution. This supports "Plan B" workflows depending on the outcome of a primary task.
The waits-for relationship establishes a soft dependency, indicating that Issue B is technically executable but optimally should wait for Issue A. Unlike strict blocking, this does not mathematically remove the task from the ready pool but drastically lowers its sorting priority.

The parent-child relationship establishes a structural composition link.24 An Epic (the parent node) functions as an organizational container for discrete Tasks (the child nodes). Crucially, an Epic does not block its children from execution; rather, the completion state of the Epic is fundamentally a derived function of its children's collective completion. This allows agents to decompose massive requirements into highly granular, executable units.27

The related relationship constitutes a soft, informational linkage.24 It serves as a contextual beacon, alerting an agent that the historical discussions or architectural decisions embedded within one issue might heavily inform the execution of another, yet it imposes zero operational or temporal constraints on execution.21

The discovered-from relationship provides a critical provenance mechanism for the audit trail.24 In autonomous workflows, an agent executing a routine feature implementation (Task X) will frequently identify an unrelated architectural vulnerability or memory leak, subsequently generating a new bug report (Bug Y).24 The discovered-from link connects Y to X, permanently recording the forensic context of how and why the new work was identified, effectively preventing the loss of discovered work.3

### **The Ready Engine Algorithm**

When an agent requires its next objective, it does not rely on probabilistic guessing or semantic searches of a markdown file. Instead, it invokes the Ready Engine (typically via a CLI command like grava ready or an equivalent MCP tool invocation).19 The Ready Engine executes a highly optimized recursive SQL query against the local Dolt database to compute the precise set of actionable tasks.

The engine isolates "ready" work by filtering the entirety of the issues table through strict Boolean logic gates: Firstly, the issue's status must evaluate exactly to open. The engine actively filters out any issue currently marked as in\_progress, closed, deferred, or tombstone.17 Secondly, and most importantly, the engine performs a topological analysis of the dependency graph.30 It calculates the indegree of every node specifically along blocks edges. Any issue that possesses an incoming blocks relationship from a node that is *not* closed is mathematically eliminated from the pool of ready work.21

Because Dolt utilizes localized SQL execution, this complex transitive blocking computation resolves in less than 10 milliseconds, providing the agent with instantaneous, deterministic orientation.17

Once the pool of mathematically unblocked issues is isolated, the engine applies a secondary sorting algorithm based on the priority integer (where 0 represents a critical mandate and 4 represents backlog exploration).24 The agent is deterministically fed the highest-priority, unblocked task. This mechanical precision entirely eliminates the need for constant human triage and drastically reduces the cognitive load on the LLM.24

### **Advanced Graph Analytics for Topology Optimization**

Beyond binary unblocking calculations, the architecture utilizes asynchronous, background graph algorithms to analyze and optimize the overarching project topology.28 As the task graph expands to encompass thousands of nodes, understanding structural vulnerabilities becomes paramount.

The system periodically executes algorithms such as PageRank to identify "foundational" tasks—issues that, while perhaps possessing a low explicit priority, act as the root blockers for a massive volume of downstream work.30 Betweenness Centrality calculations detect bottleneck issues that serve as the sole execution bridge between two distinct, large-scale clusters of development.30 Furthermore, Critical Path Depth analysis calculates the longest unbroken chain of dependencies, identifying the keystone tasks that possess absolutely zero temporal slack.30 Finally, Cycle Detection mechanisms utilizing Tarjan’s Strongly Connected Components algorithm continuously monitor the graph to ensure no circular dependencies (e.g., Task A blocks Task B, which inadvertently blocks Task A) are accidentally introduced by hallucinating agents, immediately flagging any logical paradoxes for human review.30

## **Multi-Agent Concurrency and Synchronization Server Design**

Deploying a fleet of autonomous agents operating at machine speed necessitates a synchronization architecture vastly superior to traditional polling mechanisms. If multiple agents, operating on discrete compute nodes, attempt to read and write to a shared database without a robust conflict-resolution and debouncing strategy, the system will rapidly succumb to transactional locking and state fragmentation.27

To facilitate flawless multi-agent coordination, the architecture implements a highly resilient synchronization flow utilizing localized Workspace Daemons communicating with a central Dedicated Sync Server.

### **The Workspace Daemon and the LSP Model**

The system utilizes a localized background daemon process for every active agent workspace, adopting an architecture heavily inspired by the Language Server Protocol (LSP) model.17 When an agent initializes the tracker within its project directory, the daemon boots and establishes communication via a Unix domain socket (.grava/grava.sock, or a named pipe on Windows environments).17

This daemon functions as an intelligent synchronization broker. It maintains a persistent, open connection to the local Dolt database replica, which drastically minimizes query latency by eliminating the overhead of repeatedly establishing database connections.17 Crucially, the daemon implements sophisticated batching and debouncing logic. When an agent executes a rapid, sequential series of commands—such as updating an issue's description, appending a new dependency link, and finally claiming the issue by altering its assignee status—the daemon absorbs these atomic mutations locally.17

Rather than overwhelming the network with micro-transactions, the daemon initiates a debounce window (typically configured to 5 seconds). Once this window expires without further local write activity, the daemon autonomously orchestrates a comprehensive synchronization cycle with the central server.17

### **The Synchronization Protocol (dolt-native mode)**

The automated synchronization sequence strictly adheres to distributed version control paradigms, executed programmatically by the daemon to ensure global state alignment without human intervention:

First, the daemon initiates a dolt pull operation, retrieving the latest commit graph from the central Dedicated Sync Server to ascertain any mutations executed by other agents across the network.10

Second, if remote changes exist, Dolt performs a cell-level merge into the local branch.9 This is the critical juncture where Dolt's superiority is evident. If Agent A (locally) modified the priority of grava-100 and Agent B (remotely) modified the assignee of grava-100, Dolt mathematically merges both discrete cell alterations without generating a merge conflict.

Third, in the exceedingly rare event of a direct cell collision (e.g., two agents simultaneously updating the status column of the exact same row), the daemon consults predefined semantic conflict resolution strategies (such as prioritizing the newest timestamp, or enforcing a specific hierarchy where a tombstone status universally overrides a closed status).21

Fourth, following a successful merge, the daemon executes a dolt commit, permanently sealing the local changes into an immutable cryptographic snapshot.10

Fifth, an automated Git hook (pre-commit) triggers an export of the current database state into the human-readable issues.jsonl file. This ensures that the issue tracker remains perfectly synchronized with the source code repository's standard Git history, enabling legacy fallback recovery and simplified human code reviews.18

Finally, the daemon executes a dolt push, propelling the newly finalized local commits to the remotesapi endpoint of the central server, thereby aligning the global project state.15

### **Server Infrastructure: Direct-to-Standby Cluster Replication**

To ensure the high availability and absolute reliability of the central Dedicated Sync Server, the architecture supports advanced cluster replication topologies. In an enterprise deployment subject to heavy, continuous multi-agent traffic, the server utilizes Direct-to-Standby replication protocols.33

Under this configuration, the primary dolt sql-server instance operates as the sole endpoint for accepting write operations from the distributed agent daemons. Upon the successful execution of every SQL transaction COMMIT, the primary server immediately streams the replicated writes directly to a configured array of hot-standby servers.33

In the event of primary node degradation or catastrophic hardware failure, an automated failover sequence triggers the dolt\_assume\_cluster\_role stored procedure.34 This procedure gracefully terminates existing connections on the primary, places it into read-only mode, and seamlessly promotes a synchronized standby node to the primary role. This ensures that the fleet of autonomous agents experiences no disruption in task coordination or loss of execution state during infrastructure anomalies.33

## **The Integration Layer: Model Context Protocol (MCP)**

For the underlying database architecture to be actionable, it must be presented to the autonomous agents through a highly structured, machine-readable interface. The architecture achieves this interoperability by leveraging the Model Context Protocol (MCP), a standardized framework for exposing external tools and context to Large Language Models.19

The integration layer operates as an MCP server wrapper surrounding the core issue tracking CLI binary. Instead of forcing the LLM to construct raw SQL queries—a process highly susceptible to syntax hallucination and schema mismatches—the MCP server exposes predefined, rigorously typed RPC tools.19

The tool suite includes:

* **init**: Bootstraps the local Dolt database and establishes the workspace daemon.19  
* **create**: Ingests JSON payloads containing a title, priority, and type, returning the newly generated hash-based ID.19  
* **update**: Facilitates atomic mutations to specific fields (e.g., claiming an issue or transitioning a status) without overwriting entire rows.19  
* **ready**: The primary orientation tool. It executes the complex graph traversal logic and returns an optimized JSON array of unblocked, high-priority tasks.19  
* **dep**: The mechanism for building the project graph, requiring the agent to specify the parent ID, child ID, and strict semantic relationship type.19

To ensure seamless onboarding, human supervisors inject contextual instructions into the agent's system prompt via an AGENTS.md or CLAUDE.md file.24 This file establishes the operational mandate: the agent is instructed to absolutely rely on the ready tool for task discovery, to systematically document its progress using the update tool, and to meticulously link discovered bugs using the dep tool, thereby closing the loop of autonomous execution.24

## **Minimum Viable Product (MVP) Execution Plan**

Deploying this sophisticated, multi-agent architecture requires a phased, highly disciplined iterative approach. The MVP execution plan deliberately eschews theoretical AI safety research, concentrating entirely on the concrete engineering implementation of the storage substrate, the mathematical graph traversal logic, the centralized synchronization server, and robust operational security protocols.

### **Phase 1: Storage Substrate and Schema Implementation**

The primary objective of the initial phase is to establish the version-controlled Dolt database, deploy the core schema, and ensure that the foundational data structures operate flawlessly in a localized environment before introducing the complexities of network synchronization or agentic interaction.

The engineering team will initiate the core repository and instantiate the local Dolt databases using standard initialization commands.10 Following initialization, precise Data Definition Language (DDL) scripts will be executed to construct the issues, dependencies, and events tables, strictly enforcing the column types and foreign key constraints detailed in the architectural design.7 A critical focus during this phase is the implementation of the cryptographic, hash-based ID generator, ensuring that all generated issues possess universally unique identifiers to neutralize future merge collisions.9 Finally, the foundational Go-based CLI tools (e.g., create, update, show) will be developed, allowing human engineers to manually populate the database and rigorously verify the schema's integrity.19

### **Phase 2: Graph Mechanics and Blocked Issue Tracking**

With the static storage schema established, the second phase focuses entirely on the dynamic dependency graph. This phase builds the mathematical "Ready Engine" that differentiates actionable work from blocked tasks, fulfilling a core requirement of the architectural mandate.

Engineers will implement the logic supporting the four distinct semantic dependency types (blocks, related, parent-child, discovered-from) within the linking tools.19 The centerpiece of this phase is the development of the complex SQL queries and local graph traversals required to execute topological sorting, specifically filtering the issues table to identify nodes possessing an indegree of zero on open blocks edges.24 Concurrently, the priority sorting logic will be implemented to rank the unblocked issues algorithmically. This phase concludes with the exposure and rigorous testing of the ready and blocked commands, verifying that the system can accurately compute the actionable task list in under 10 milliseconds.19

### **Phase 3: Dedicated Sync Server and Daemon Orchestration**

This phase marks the transition of the system from a localized, single-user tool into a distributed, multi-agent platform by introducing the central server and the automated synchronization daemons.

The infrastructure team will provision and deploy the central dolt sql-server instance on dedicated cloud infrastructure, explicitly enabling the remotesapi endpoint to accept incoming replication traffic.15 Simultaneously, the development team will engineer the localized background workspace daemon, implementing the Unix domain socket architecture and the critical 5-second debounce logic required for batching rapid local mutations.17 The core automated synchronization loop—comprising the pull, 3-way cell merge, commit, and push operations—must be perfected to ensure seamless state alignment.9 Finally, Git hook integrations will be deployed to continuously export the database state into the issues.jsonl format, bridging the gap between the SQL tracker and the source code repository.18

### **Phase 4: Security Implementation and Access Control**

Before the architecture is exposed to fully autonomous LLM agents or deployed in a production environment, the operational boundaries and security protocols must be strictly enforced. This phase establishes zero-trust network access and granular database privileges.

Network security will be fortified by configuring Mutual TLS (mTLS) on the central Dolt server. By enforcing the listener.require\_client\_cert: true parameter and deploying signed TLS certificates to all authorized clients, the system will cryptographically reject any unauthorized connection attempts.37 Within the database, the privileges.db grant tables will be initialized to define discrete, Role-Based Access Control (RBAC) profiles. Human administrators will retain broad schema control, while autonomous agents will be assigned highly restrictive roles, explicitly granted SELECT, INSERT, and UPDATE permissions while being completely denied DELETE or DROP capabilities.34 To ensure absolute accountability, the immutable events append-only log will be activated, tying every row mutation to the specific cryptographic identity of the executing actor, enabling immediate forensic rollbacks if anomalous behavior is detected.25

### **Phase 5: MCP Integration and Agent Onboarding**

The final phase of the MVP bridges the secure, synchronized, graph-aware database directly to the cognitive layer of the autonomous agents, bringing the system to full operational status.

Developers will construct the Model Context Protocol (MCP) server wrapper, translating the underlying database capabilities into standardized, strictly typed JSON RPC tools (create, ready, dep, update) that agents like Claude or Amp can invoke natively without generating raw SQL.19 System prompts and context injection documents (AGENTS.md) will be drafted, instructing the agents on the precise operational workflow: mandating that they query for unblocked work upon initialization, update task statuses sequentially, and meticulously link discovered bugs.24 The MVP concludes with extensive end-to-end integration testing, deploying multiple autonomous agents concurrently to verify that the Ready Engine successfully prevents out-of-order execution and that the Sync Server flawlessly merges their simultaneous updates.

| Implementation Phase | Core Objectives | Projected Deliverable |
| :---- | :---- | :---- |
| **Phase 1: Storage Substrate** | Dolt initialization, schema DDL execution, hash-based ID generation, basic CRUD CLI tools. | A locally functional, version-controlled SQL issue tracker operating without a central server. |
| **Phase 2: Graph Mechanics** | Semantic edge definitions, topological sort algorithms, prioritization logic, ready command exposure. | A functional graph traversal engine that accurately computes unblocked work in milliseconds. |
| **Phase 3: Synchronization Server** | Central dolt sql-server deployment, workspace daemon engineering, debounce logic, push/pull automation. | A distributed network where multiple clients sync state to the central server without human intervention. |
| **Phase 4: Security Implementation** | mTLS enforcement, SQL grant table configuration, RBAC assignment, immutable event logging activation. | A secure, zero-trust server environment resilient to unauthorized access and destructive commands. |
| **Phase 5: Agent Integration** | MCP server wrapper development, JSON RPC tool exposure, context injection (AGENTS.md), end-to-end testing. | A production-ready MVP where AI agents independently query, execute, and sync work autonomously. |

## **Conclusion**

The architecture detailed herein transcends traditional software project management paradigms. By coupling the mathematically rigid graph logic of the Beads concept with the distributed, cell-level versioning capabilities of a Dolt SQL server, the system provides autonomous AI agents with a flawless, highly queryable external memory system.

The meticulously designed schema neutralizes the context-window limitations that typically constrain Large Language Models, while the deterministic topological sorting of the "Ready Engine" eliminates the hallucination risks inherently associated with autonomous task selection. Supported by a highly resilient workspace daemon for continuous state synchronization and fortified by strict Mutual TLS network encryption and granular SQL privilege controls, this architecture ensures that a fleet of agents can collaborate safely and continuously. Ultimately, this MVP plan provides the precise engineering roadmap required to realize highly scalable, fully autonomous software development lifecycles.

#### **Works cited**

1. Gas Town, Beads, and the Rise of Agentic Development with Steve Yegge, accessed February 16, 2026, [https://softwareengineeringdaily.com/2026/02/12/gas-town-beads-and-the-rise-of-agentic-development-with-steve-yegge/](https://softwareengineeringdaily.com/2026/02/12/gas-town-beads-and-the-rise-of-agentic-development-with-steve-yegge/)  
2. Beads \- Memory for your Agent and The Best Damn Issue Tracker Your're Not Using, accessed February 16, 2026, [https://ianbull.com/posts/beads/](https://ianbull.com/posts/beads/)  
3. Introducing Beads: A coding agent memory system | by Steve Yegge | Medium, accessed February 16, 2026, [https://steve-yegge.medium.com/introducing-beads-a-coding-agent-memory-system-637d7d92514a](https://steve-yegge.medium.com/introducing-beads-a-coding-agent-memory-system-637d7d92514a)  
4. The Beads Revolution: How I Built The TODO System That AI Agents Actually Want to Use, accessed February 16, 2026, [https://steve-yegge.medium.com/the-beads-revolution-how-i-built-the-todo-system-that-ai-agents-actually-want-to-use-228a5f9be2a9](https://steve-yegge.medium.com/the-beads-revolution-how-i-built-the-todo-system-that-ai-agents-actually-want-to-use-228a5f9be2a9)  
5. A Day in Gas Town | DoltHub Blog, accessed February 16, 2026, [https://www.dolthub.com/blog/2026-01-15-a-day-in-gas-town/](https://www.dolthub.com/blog/2026-01-15-a-day-in-gas-town/)  
6. Agentic Memory | DoltHub Blog, accessed February 16, 2026, [https://www.dolthub.com/blog/2026-01-22-agentic-memory/](https://www.dolthub.com/blog/2026-01-22-agentic-memory/)  
7. beads/docs/EXTENDING.md at main · steveyegge/beads \- GitHub, accessed February 16, 2026, [https://github.com/steveyegge/beads/blob/main/docs/EXTENDING.md](https://github.com/steveyegge/beads/blob/main/docs/EXTENDING.md)  
8. AGENTS.md \- Dicklesworthstone/beads\_rust \- GitHub, accessed February 16, 2026, [https://github.com/Dicklesworthstone/beads\_rust/blob/main/AGENTS.md](https://github.com/Dicklesworthstone/beads_rust/blob/main/AGENTS.md)  
9. beads/README.md at main · steveyegge/beads · GitHub, accessed February 16, 2026, [https://github.com/steveyegge/beads/blob/main/README.md](https://github.com/steveyegge/beads/blob/main/README.md)  
10. dolt/README.md at main · dolthub/dolt \- GitHub, accessed February 16, 2026, [https://github.com/dolthub/dolt/blob/main/README.md](https://github.com/dolthub/dolt/blob/main/README.md)  
11. DVC vs. Git-LFS vs. Dolt vs. lakeFS: Data Versioning Compared, accessed February 16, 2026, [https://lakefs.io/blog/dvc-vs-git-vs-dolt-vs-lakefs/](https://lakefs.io/blog/dvc-vs-git-vs-dolt-vs-lakefs/)  
12. Dolt is Git for data \- Hacker News, accessed February 16, 2026, [https://news.ycombinator.com/item?id=22731928](https://news.ycombinator.com/item?id=22731928)  
13. Dolt for MySQL Database Versioning | DoltHub Blog, accessed February 16, 2026, [https://www.dolthub.com/blog/2023-01-30-dolt-for-mysql-backups/](https://www.dolthub.com/blog/2023-01-30-dolt-for-mysql-backups/)  
14. Procedures \- Dolt Documentation \- DoltHub, accessed February 16, 2026, [https://docs.dolthub.com/sql-reference/version-control/dolt-sql-procedures](https://docs.dolthub.com/sql-reference/version-control/dolt-sql-procedures)  
15. Dolt SQL Server Push Support | DoltHub Blog, accessed February 16, 2026, [https://www.dolthub.com/blog/2023-12-29-sql-server-push-support/](https://www.dolthub.com/blog/2023-12-29-sql-server-push-support/)  
16. beads/docs/QUICKSTART.md at main · steveyegge/beads \- GitHub, accessed February 16, 2026, [https://github.com/steveyegge/beads/blob/main/docs/QUICKSTART.md](https://github.com/steveyegge/beads/blob/main/docs/QUICKSTART.md)  
17. beads/docs/ARCHITECTURE.md at main · steveyegge/beads · GitHub, accessed February 16, 2026, [https://github.com/steveyegge/beads/blob/main/docs/ARCHITECTURE.md](https://github.com/steveyegge/beads/blob/main/docs/ARCHITECTURE.md)  
18. beads/AGENT\_INSTRUCTIONS.md at main · steveyegge/beads \- GitHub, accessed February 16, 2026, [https://github.com/steveyegge/beads/blob/main/AGENT\_INSTRUCTIONS.md](https://github.com/steveyegge/beads/blob/main/AGENT_INSTRUCTIONS.md)  
19. beads/docs/PLUGIN.md at main · steveyegge/beads · GitHub, accessed February 16, 2026, [https://github.com/steveyegge/beads/blob/main/docs/PLUGIN.md](https://github.com/steveyegge/beads/blob/main/docs/PLUGIN.md)  
20. Beads Blows Up \- Steve Yegge \- Medium, accessed February 16, 2026, [https://steve-yegge.medium.com/beads-blows-up-a0a61bb889b4](https://steve-yegge.medium.com/beads-blows-up-a0a61bb889b4)  
21. beads/CHANGELOG.md at main · steveyegge/beads \- GitHub, accessed February 16, 2026, [https://github.com/steveyegge/beads/blob/main/CHANGELOG.md](https://github.com/steveyegge/beads/blob/main/CHANGELOG.md)  
22. bd doctor should report runtime health and sync freshness · Issue \#1639 · steveyegge/beads, accessed February 16, 2026, [https://github.com/steveyegge/beads/issues/1639](https://github.com/steveyegge/beads/issues/1639)  
23. feat: Dolt schema upgrade for existing DBs (auto-add issues.metadata column) \#1414, accessed February 16, 2026, [https://github.com/steveyegge/beads/issues/1414](https://github.com/steveyegge/beads/issues/1414)  
24. The Beads Memory System: Technical Architecture and Integration with Gemini CLI for Agentic Workflows | by Maddula Sampath Kumar | Google Cloud \- Community | Feb, 2026 | Medium, accessed February 16, 2026, [https://medium.com/google-cloud/the-beads-memory-system-technical-architecture-and-integration-with-gemini-cli-for-agentic-c2aa36430802](https://medium.com/google-cloud/the-beads-memory-system-technical-architecture-and-integration-with-gemini-cli-for-agentic-c2aa36430802)  
25. Feature request: expose events table via CLI and warn on text field overwrite (SQLite) · Issue \#1385 · steveyegge/beads \- GitHub, accessed February 16, 2026, [https://github.com/steveyegge/beads/issues/1385](https://github.com/steveyegge/beads/issues/1385)  
26. steveyegge/beads v0.49.4 on GitHub \- NewReleases.io, accessed February 16, 2026, [https://newreleases.io/project/github/steveyegge/beads/release/v0.49.4](https://newreleases.io/project/github/steveyegge/beads/release/v0.49.4)  
27. automazeio/ccpm: Project management system for Claude Code using GitHub Issues and Git worktrees for parallel agent execution., accessed February 16, 2026, [https://github.com/automazeio/ccpm](https://github.com/automazeio/ccpm)  
28. Projects | Jeffrey Emanuel, accessed February 16, 2026, [https://jeffreyemanuel.com/projects](https://jeffreyemanuel.com/projects)  
29. Explore beads library and usage with AMP \- Amp Code, accessed February 16, 2026, [https://ampcode.com/threads/T-adc03ba9-db60-49e6-bae9-e5f9749f4312](https://ampcode.com/threads/T-adc03ba9-db60-49e6-bae9-e5f9749f4312)  
30. Dicklesworthstone/beads\_viewer: Graph-aware TUI for the Beads issue tracker: PageRank, critical path, kanban, dependency DAG visualization, and robot-mode JSON API \- GitHub, accessed February 16, 2026, [https://github.com/Dicklesworthstone/beads\_viewer](https://github.com/Dicklesworthstone/beads_viewer)  
31. Best design pattern to synchronize local and cloud databases?, accessed February 16, 2026, [https://softwareengineering.stackexchange.com/questions/455988/best-design-pattern-to-synchronize-local-and-cloud-databases](https://softwareengineering.stackexchange.com/questions/455988/best-design-pattern-to-synchronize-local-and-cloud-databases)  
32. beads/docs/CONFIG.md at main · steveyegge/beads \- GitHub, accessed February 16, 2026, [https://github.com/steveyegge/beads/blob/main/docs/CONFIG.md](https://github.com/steveyegge/beads/blob/main/docs/CONFIG.md)  
33. Replication \- Dolt Documentation \- DoltHub, accessed February 16, 2026, [https://docs.dolthub.com/sql-reference/server/replication](https://docs.dolthub.com/sql-reference/server/replication)  
34. Configuration | Dolt Documentation, accessed February 16, 2026, [https://docs.dolthub.com/sql-reference/server/configuration](https://docs.dolthub.com/sql-reference/server/configuration)  
35. jleechanorg/mcp\_mail: Fork of mcp\_agent\_mail \- A mail-like coordination layer for coding agents \- GitHub, accessed February 16, 2026, [https://github.com/jleechanorg/mcp\_mail](https://github.com/jleechanorg/mcp_mail)  
36. beads/docs/SYNC.md at main · steveyegge/beads · GitHub, accessed February 16, 2026, [https://github.com/steveyegge/beads/blob/main/docs/SYNC.md](https://github.com/steveyegge/beads/blob/main/docs/SYNC.md)  
37. Requiring Client Certificates | DoltHub Blog, accessed February 16, 2026, [https://www.dolthub.com/blog/2025-12-01-require-client-cert/](https://www.dolthub.com/blog/2025-12-01-require-client-cert/)  
38. Access Management | Dolt Documentation, accessed February 16, 2026, [https://docs.dolthub.com/sql-reference/server/access-management](https://docs.dolthub.com/sql-reference/server/access-management)  
39. modelcontextprotocol/servers: Model Context Protocol Servers \- GitHub, accessed February 16, 2026, [https://github.com/modelcontextprotocol/servers](https://github.com/modelcontextprotocol/servers)