---
stepsCompleted:
  - step-01-init
  - step-02-discovery
  - step-02b-vision
  - step-02c-executive-summary
  - step-04-journeys
  - step-05-domain
  - step-06-innovation
  - step-07-project-type
  - step-08-scoping
  - step-09-functional
  - step-10-nonfunctional
  - step-11-polish
  - step-e-01-discovery
  - step-e-02-review
  - step-e-03-edit
lastEdited: '2026-03-18'
editHistory:
  - date: '2026-03-18'
    changes: 'Added non-technical user onboarding: Journey 4 (install script setup path for macOS/Linux/Windows), FR26-FR28 (automated install script supporting Claude CLI and Gemini CLI on macOS, Linux, Windows), NFR7-NFR8 (install speed and reliability), Success Criteria onboarding metric, Executive Summary audience segment update.'
  - date: '2026-04-09'
    changes: 'Removed Epic 06 (Onboarding) and associated requirements (FR26-28, NFR7-8). Swapped Epic 05 and Epic 09 to align with the "Worktree First" strategy (moving grava doctor to later phase). Renumbered Epics 07-11 to 06-10 to maintain a continuous project sequence.'
inputDocuments:
  - docs/architecture/GRAPH_MECHANICS.md
  - docs/architecture/PERFORMANCE_BENCHMARKS.md
  - docs/archive/Epic_3_Sync_Server_Archived.md
  - docs/archive/reports/2026-02-24_Multi_Agent_Issue_Tracking_Research.md
  - docs/archive/skills_storage_plan.md
  - docs/design/2026-02-19-add-json-output.md
  - docs/design/2026-02-19-export-import.md
  - docs/design/2026-02-19-history-undo.md
  - docs/design/2026-02-19-list-sorting-design.md
  - docs/design/2026-02-19-list-sorting.md
  - docs/design/2026-02-19-soft-delete.md
  - docs/design/2026-02-20-enhance-init-and-config-design.md
  - docs/design/2026-02-20-enhance-init-and-config-implementation.md
  - docs/design/2026-02-20-implement-start-stop-commands.md
  - docs/design/2026-02-23-persistent-graph-updates-design.md
  - docs/design/2026-02-23-persistent-graph-updates.md
  - docs/design/2026-02-24-local-dolt-installation-design.md
  - docs/design/2026-02-24-local-dolt-installation.md
  - docs/design/2026-03-01-git-merge-driver-design.md
  - docs/design/2026-03-02-test-2-agents-different-repos.md
  - docs/epics/Epic_1.1_additional_commands.md
  - docs/epics/Epic_1_Storage_Substrate.md
  - docs/epics/Epic_2.2_Graph_Lifecycle_and_Propagation.md
  - docs/epics/Epic_2.3_CI_CD_and_Dockerization.md
  - docs/epics/Epic_2.4_Hierarchical_Work_Management.md
  - docs/epics/Epic_2_Graph_Implementation_Plan.md
  - docs/epics/Epic_2_Graph_Mechanics.md
  - docs/epics/Epic_3_Git_Merge_Driver.md
  - docs/epics/Epic_4.1_Ephemeral_Store_Implementation_Plan.md
  - docs/epics/Epic_4_Log_Saver.md
  - docs/epics/Epic_5_Multi_Agent_Repository_Sync_Test.md
  - docs/epics/Epic_5_Security.md
  - docs/epics/Epic_6_MCP_Integration.md
  - docs/epics/Epic_7_Advanced_Analytics.md
  - docs/epics/Epic_8_Advanced_Workflows.md
  - docs/epics/Epic_9_Development_Log.md
  - docs/epics/artifacts/2026-02-24_Docker_and_CI_CD_Research.md
  - docs/epics/artifacts/AgentScheduler_Benchmark_Report.md
  - docs/epics/artifacts/AgentScheduler_Benchmark_Summary.md
  - docs/epics/artifacts/AgentScheduler_Implementation_Summary.md
  - docs/epics/artifacts/EXECUTIVE_SUMMARY.md
  - docs/epics/artifacts/Epic_2_Review_Analysis.md
  - docs/epics/artifacts/INDEX.md
  - docs/epics/artifacts/OPTIMIZATION_SUMMARY.md
  - docs/epics/artifacts/Pearce_Kelly_AgentScheduler_Review.md
  - docs/epics/artifacts/Ready_Query_Optimization_Guide.md
  - docs/guides/AGENT_WORKFLOWS.md
  - docs/guides/CLI_REFERENCE.md
  - docs/guides/DOLT_SETUP.md
  - docs/guides/RELEASE_PROCESS.md
workflowType: 'prd'
documentCounts:
  briefCount: 0
  researchCount: 0
  brainstormingCount: 0
  projectDocsCount: 52
classification:
  projectType: 'CLI Tool'
  domain: 'Developer Tools / AI Agents'
  complexity: 'High'
  projectContext: 'brownfield'
---

# Product Requirements Document - Grava

**Author:** Htnguyen
**Date:** 2026-03-10T21:42:06+07:00

## Executive Summary

Grava is a CLI-based, agent-first issue tracking and task orchestration system engineered to solve the context and synchronization bottlenecks innate to autonomous software development swarms. It replaces brittle API polling and UI-heavy traditional trackers (e.g., Jira, GitHub Issues) with a dedicated, localized, machine-readable graph database. 

Grava is built on three core pillars:
1. **Offline-First Data Storage:** Uses Dolt (a Git-versioned SQL database) combined with custom Git hook mechanics for conflict-free state resolution and JSONL payload serialization.
2. **Graph-Based Context Engine:** Issues are structured as interconnected nodes (Dependencies, Blockers, Epics) queryable natively, providing agents immediate mapping of project state.
3. **Machine-Native Optimization:** Outputs strictly deterministic, predictable JSON. Grava eliminates UI/UX bloat, allowing concurrent agents to maintain state safely without hallucination.

Grava targets two user segments: **autonomous AI agents** (primary consumers) and **technical developers** (operators and orchestrators).

## Project Classification

- **Project Type:** CLI Tool / Agent Orchestration Substrate
- **Domain:** Developer Tools / AI Autonomous Workflow
- **Complexity Level:** High
- **Project Context:** Brownfield

## Success Criteria

### Measurable Outcomes
- **Zero-Loss Handoff:** Achieve 100% preservation of issue context (blockers, relations) when a task is transferred from Agent A to Agent B.
- **Swarm Stability:** Successfully sustain a localized swarm of up to 30 concurrent agents operating autonomously without fatal state conflicts or local resource exhaustion.
- **Human Independence:** Reduce manual developer interventions per completed issue lifecycle to a near-zero threshold, excluding intentional strategic approvals.

## Product Scope

### Phase 1: Core Substrate MVP (Single Repository)
A deterministic, locally executed CLI leveraging the Dolt database that allows individual agents and small localized swarm groups to independently track context, map dependencies via graph relations, and seamlessly execute atomic task handoffs within a single repository.

### Phase 2: Growth (Multi-Workspace Orchestration)
Expanding Grava to actively manage, query, and synchronize issue states and agent workflows across multiple repositories simultaneously on the same local machine.

### Phase 3: Expansion (Advanced Graph Rules Engine / Separate Project Integration)
A conceptually distinct future phase focused on advanced graph analytics (e.g., PageRank for bottlenecks) and complex, automated resolution mechanics when syncing state against remote orchestrator servers or deep remote branch conflicts.

## User Journeys

### Journey 1: DevBot's Flawless Handoff (Agent Success Path)
- **Actor:** DevBot (Autonomous Agent)
- **Goal:** Find ready work, implement, pass to QA seamlessly.
- **Trigger:** DevBot executes `grava ready`.
- **Action Sequence:**
  1. DevBot receives a JSON payload for a ready issue.
  2. DevBot executes `grava dep` to map the local context graph and read history.
  3. DevBot executes `grava claim` to atomically lock the issue to its Actor ID.
  4. DevBot implements the code fix.
  5. DevBot executes `grava update` to assign the `needs-testing` status.
- **Resolution:** QABot executes `grava ready`, immediately sees the new issue, and begins testing with zero human intervention.

### Journey 2: The Human Architect Orchestrates (Human Success Path)
- **Actor:** Htnguyen (Human Developer)
- **Goal:** Safely provide initial epics and requirements to the system.
- **Trigger:** A new sprint plan is drafted.
- **Action Sequence:**
  1. Creates a single structured markdown file representing the Epic.
  2. Executes `grava create` and `grava subtask` via a quick bash script to seed the database.
  3. Establishes dependencies using `grava dep`.
  4. Executes `grava list --json` to monitor the board.
- **Resolution:** Agents immediately begin claiming the ready tasks and updating the board autonomously. The developer manages the macro-strategy, Grava manages the micro-tasks.

### Journey 3: The Contested Issue (Edge Case)
- **Actor:** DevBot and RefactorBot
- **Goal:** Prevent duplicated effort and state conflicts.
- **Trigger:** Both bots query `grava ready` simultaneously and see the same urgent bug.
- **Action Sequence:**
  1. DevBot runs `grava claim` to change the status and acquire the lock.
  2. RefactorBot, running milliseconds behind, attempts the exact same claim command.
  3. Grava rejects RefactorBot's command because the cell-level state has already been altered.
- **Resolution:** RefactorBot reads the deterministic JSON error, immediately queries `grava ready` again, and moves to a different task.

## Domain-Specific Requirements

### Technical Constraints & Concurrency
- **Native SQL Querying:** Grava fully embraces Dolt's SQL engine. Agents use native SQL queries to read state, map dependencies, and understand context.
- **Wisp Tables:** Agents persist ephemeral working data (logs, thoughts, intermediate artifacts) to a `wisp` table to provide real-time lineage without polluting the primary permanent issue tables.

### Security, Safety, & Standards
- **Agent Containment:** Grava utilizes strict advisory locking and kill-switches tied to the `--actor` flag to prevent infinite loops from consuming local system resources.
- **Schema Adherence:** Outbound output formats must be rigidly adhered to via `--json`. Any break in schema necessitates a major version bump.

## Innovation & Novel Patterns

1. **Dual-Loop Cognitive Architecture:** Grava implements a bifurcated reasoning system. The "Outer Loop" (strategic reasoning) happens off-CLI, while Grava entirely facilitates the "Inner Loop" (tactical execution), separating strategy from execution noise over the Progress Ledger.
2. **Pointer-Based Information Architecture:** To prevent LLM attention decay, Grava natively stores and returns pointers (function locations, file paths, commit IDs), forcing agents to fetch only the explicit data they need rather than full artifact blobs.
3. **Beads-Inspired Merging:** Implements a multi-layered synchronization approach linking standard text-based JSONL files to a resilient relational SQL core using a custom `git merge` driver, bypassing standard text-conflict Git markers entirely in favor of schema-aware resolution.

## Functional Requirements

### Issue Creation & Modification
- **FR1:** System Agent / Human Developer can create discrete issues or macro-epics (`create`, `quick`).
- **FR2:** System Agent / Human Developer can rapidly break down overarching issues into hierarchical subtasks (`subtask`).
- **FR3:** System Agent / Human Developer can update core fields like status, priority, and assignees (`update`, `assign`).
- **FR4:** System Agent / Human Developer can explicitly track when they start or stop working on a specific issue (`start`, `stop`).
- **FR5:** System Agent can execute an atomic claim on an issue, verifying it is unassigned and immediately locking it to their actor ID (`claim`).
- **FR6:** System Agent / Human Developer can append contextual metadata to issues via tags and text notes (`label`, `comment`).
- **FR7:** System Agent / Human Developer can safely remove or archive issues from the active tracking space (`drop`, `clear`).

### Graph Context & Discovery
- **FR8:** System Agent / Human Developer can establish directional "blocking" relationship links between issues (`dep`).
- **FR9:** System Agent / Human Developer can query the immediate actionable queue of top-priority tasks with no blockers (`ready`).
- **FR10:** System Agent / Human Developer can explicitly query what upstream issues are preventing a specific task from being worked on (`blocked`).
- **FR11:** System Agent / Human Developer can visualize or traverse the overarching dependency structure of the project (`graph`).
- **FR12:** System Agent / Human Developer can filter, search, and view detailed individual properties of issues (`list`, `search`, `show`).
- **FR13:** Human Developer can view aggregated workspace performance and status metrics (`stats`).

### State History & Database Maintenance
- **FR14:** System Agent / Human Developer can retrieve a detailed ledger of previously executed system commands (`cmd_history`).
- **FR15:** Human Developer can safely revert recent state-altering commands to recover from errors (`undo`).
- **FR16:** System Agent / Human Developer can explicitly prune expired or deleted data to maintain high query performance (`compact`).
- **FR17:** Human Developer can run diagnostic health checks on the Grava substrate to ensure data integrity (`doctor`).

### Ephemeral State Operations (Wisp Data)
- **FR18:** System Agent / Human Developer can explicitly write to and read from an issue's ephemeral state (Wisp) via dedicated commands to manage working artifacts and execution history.
- **FR19:** System Agent / Human Developer can retrieve the historical progression log of an issue to understand what a previous agent did before handoff.

### Workspace Synchronization & Ecosystem Integration
- **FR20:** System Agent / Human Developer can export the internal database state into a standardized, machine-readable artifact (`export`).
- **FR21:** System Agent / Human Developer can hydrate the internal database by importing a standardized artifact (`import`), provided no conflicts exist.
- **FR22:** The System must automatically execute a 3-way cell-level merge of issue state during Git updates. If cell-level changes cannot be merged, the System must safely isolate and save the unresolvable conflict data to a separate database table for human intervention.
- **FR23:** Human Developer can initialize a brand-new, isolated tracking environment for a local repository (`init`, `config`).
- **FR24:** The System must evaluate a Dual-Safety Check (JSONL hash vs. Dolt state) before importing to prevent overwriting uncommitted local data.
- **FR25:** The System must automatically trigger graph database updates via Git hooks whenever the repository state changes (e.g., `git pull`, checkout), actively detecting any file-to-database mismatches.

## Non-Functional Requirements

### Performance & Latency
- **NFR1 (Query Speed):** Core graph resolution commands (e.g., `grava ready`, `grava list`) must return structured JSON to the standard output within **< 100 milliseconds** under an average load of 10,000 active and ephemeral issues.
- **NFR2 (Write Throughput):** Standard issue creation and updates (`create`, `update`, `claim`) must commit to the local Dolt instance in **< 15 milliseconds** per operation (accounting for advisory lock acquisition during subtask creation) to maintain a sustained throughput of >70 inserts per second.

### Reliability & Data Integrity (Concurrency)
- **NFR3 (Atomic Execution):** The system must guarantee that concurrent write attempts by multiple local agents (e.g., two agents executing `grava claim` simultaneously) result in exactly one successful claim and one deterministic rejection, never resulting in a polluted row or deadlock.
- **NFR4 (Zero-Loss Handoff):** The system must guarantee 100% preservation of an issue's dependency links and core fields during state export (`issues.jsonl`) and hydration, ensuring identical graph recreation across workspace clones.

### Operability & Extensibility
- **NFR5 (Machine Readability):** The system must maintain strict adherence to predefined JSON schemas for all `--json` command outputs. Any change to the output schema must trigger a major version bump to prevent breaking autonomous agent prompts.
- **NFR6 (Zero-Dependency Footprint):** The Grava CLI must compile to a single, statically linked binary for target OS architectures, requiring zero external runtime dependencies (e.g., Python, Node.js) beyond the system shell and Git.
