---
stepsCompleted:
  - step-01-init
  - step-02-context
inputDocuments:
  - _bmad-output/planning-artifacts/prd.md
  - docs/architecture/GRAPH_MECHANICS.md
  - docs/architecture/PERFORMANCE_BENCHMARKS.md
  - docs/guides/AGENT_WORKFLOWS.md
  - docs/guides/CLI_REFERENCE.md
workflowType: 'architecture'
project_name: 'grava'
user_name: 'Htnguyen'
date: '2026-03-11'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

---

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**
25 FRs across 5 domains: Issue Lifecycle (FR1–7), Graph Context & Discovery (FR8–13),
State History & Maintenance (FR14–17), Ephemeral State/Wisp (FR18–19), and Workspace
Sync & Git Integration (FR20–25). The atomic `claim` command and the `ready` queue are
the two most architecturally significant primitives — they define the agent execution
contract and drive all concurrency requirements.

**Non-Functional Requirements:**
- NFR1: <100ms read latency at 10K issues → requires in-memory graph with cache
- NFR2: <15ms write latency, >70 inserts/sec → confirmed met by benchmarks (~5–13ms)
- NFR3: Exactly-once claim under concurrent agents → row-level exclusive locking
- NFR4: Lossless JSONL export/import → strict schema discipline + all-or-nothing transaction
- NFR5: JSON output schema versioned → breaking changes = major version bump
- NFR6: Single statically-linked binary → no Python/shell runtime deps in core paths

**Scale & Complexity:**
The system is a high-complexity, local-first CLI substrate for autonomous agent swarms.
Expected: up to 30 concurrent agents, 10,000+ issues, hundreds of writes/minute during
active sprints.

- Primary domain: CLI Tool / Local-first AI Agent Orchestration
- Complexity level: High
- Estimated architectural components: 8 (CLI layer, graph engine, DB layer, ID generator,
  migration engine, export/import pipeline, Git hook integration, ephemeral store)

### Technical Constraints & Dependencies

- **Dolt (MySQL-protocol)**: Only supported persistence backend. All SQL must be
  MySQL-compatible. Row-level locking via `SELECT ... FOR UPDATE` is required for
  subtask ID atomicity and exclusive issue modification.
- **Single binary (NFR6)**: Git hooks and agent scheduler must eventually be compiled
  into the Grava binary or this NFR is violated. Shell/Python dependencies are a
  transitional violation to be resolved.
- **Git integration**: Merge driver registration is per-repo and requires `.gitattributes`
  configuration — an installation/init UX concern.
- **12-char base ID**: Expanded from 4 chars to 12 hex chars (~4 billion combinations).
  Birthday collision threshold: ~65K issues at 1% probability — sufficient for project scale.

### Cross-Cutting Concerns Identified

1. **Concurrency & Locking**: All write paths must be safe for 30 concurrent agents.
   Issue rows are exclusively locked during any modification — other agents are blocked
   from claiming or modifying the same issue until the lock is released. `SELECT FOR UPDATE`
   on `child_counters` ensures atomic subtask ID generation.
2. **Audit Logging**: Every mutation must be captured in the `events` table with actor + model.
3. **Actor Identity**: `--actor` flag is the agent identity primitive — must be consistent across all commands.
4. **JSON Schema Contract**: Strict versioning discipline across all `--json` outputs.
5. **Graph Coherence**: In-memory AdjacencyDAG and Dolt DB must remain consistent — no divergence.
   Graph is hydrated per-invocation with lazy loading for read-heavy commands (e.g. `ready`
   loads only `open` issues). Optional daemon mode holds the graph in memory across invocations
   for high-frequency swarm workloads.
6. **Git Sync Pipeline**: Hook scripts → export → merge driver → import must preserve full
   state integrity. Import is wrapped in a single all-or-nothing DB transaction. Merge driver
   uses last-write-wins for field-level conflicts, with all resolution decisions logged.

### Pre-mortem Risk Decisions

| Risk | Decision |
|---|---|
| ID collision (4-char = 65K combinations) | Expand base ID to **12 hex chars** (~4B combinations) |
| Graph hydration stampede at 30-agent scale | **Lazy load** open issues only + **configurable connection pool** + **optional daemon mode** |
| Subtask ID race condition | `SELECT FOR UPDATE` on `child_counters` row; issue row exclusively locked during any modification |
| Partial import state corruption | Entire import wrapped in a **single DB transaction** (all-or-nothing) |
| Merge driver silent data loss | **Last-write-wins** for field-level conflicts; all resolution decisions logged |

### Architectural Evolution Paths

**Daemon Mode (Graph Hydration):**
- **MVP (Phase 1):** TTL-based on-disk snapshot (`.grava/graph.snapshot.json`, 5s TTL). Read-heavy commands (`ready`, `list`, `graph`) serve from snapshot if fresh; fall back to DB hydration on expiry. Write commands (`claim`, `update`, `create`) always hit DB directly and invalidate snapshot.
- **Phase 2:** Unix socket daemon (`grava serve`) — persistent process holding full in-memory AdjacencyDAG, serving all commands via local RPC. Eliminates per-invocation hydration cost entirely for swarm workloads.

**pkg/cmd Structure:**
- **Now (Phase 1):** Reorganize `pkg/cmd` into command groups (`issues/`, `graph/`, `maintenance/`, `sync/`) — reduces cognitive load, no logic changes, low risk.
- **Later Phase:** Surgically extract `claim`, `import`, and `ready` into `pkg/ops` — the three concurrency-sensitive, high-value operations that need unit testability and daemon reuse without full Cobra invocation. Full service layer deferred until daemon mode is built.

### Architecture Decision Records

**ADR-001: Git Hook Binary Strategy**
- **Decision:** `grava hook <event>` subcommands compiled into the binary. `grava init` writes one-liner shell stubs (`#!/bin/sh\ngrava hook post-merge`) to `.git/hooks/` during repository initialization.
- **Rationale:** Satisfies NFR6 (zero external runtime deps), enables proper Go error propagation and structured logging, and makes hooks unit-testable. Shell scripts in `scripts/hooks/` become deprecated reference implementations.
- **Phase:** Phase 1

**ADR-002: TTL Snapshot Invalidation Protocol**
- **Decision:** Two-tier read/write protocol:
  1. **Read commands** (`ready`, `list`, `graph`, `show`, `blocked`, `dep`) → serve from `.grava/graph.snapshot.json` if age < TTL (default 5s, configurable); hydrate from DB on miss/expiry and atomically refresh snapshot via tmp file + `os.Rename()`.
  2. **Write commands** (`claim`, `create`, `update`, `drop`, `assign`, `label`, `comment`) → always bypass snapshot, write to DB, then delete snapshot to force re-hydration on next read.
- **Rationale:** Write commands must never read stale state for correctness. Atomic tmp+rename prevents partial JSON corruption if process is killed mid-write. Read commands tolerate eventual consistency up to TTL.
- **Phase:** Phase 1

**ADR-003: pkg/ops Interface Preparation**
- **Decision:** `claim`, `import`, and `ready` logic lives in named functions (not anonymous `RunE` closures) with signature `func claimIssue(ctx context.Context, store dolt.Store, id, actor, model string) error`. No `pkg/ops` package created yet — functions remain in `pkg/cmd` command groups but are trivially extractable.
- **Rationale:** Respects YAGNI (no over-engineering before daemon mode exists) while ensuring `context.Context` is threaded through from day one and future extraction to `pkg/ops` is a file move, not a refactor.
- **Phase:** Phase 1 (prep); extraction in later phase when daemon mode requires it
