---
project: grava
phase: Phase 1
created: '2026-03-21'
scope: sandbox-testing
sourceDocuments:
  - _bmad-output/planning-artifacts/prd.md
  - _bmad-output/planning-artifacts/architecture.md
  - _bmad-output/planning-artifacts/implementation-readiness-report-2026-03-21.md
status: draft
---

# Sandbox Testing Requirements

**Project:** grava — Phase 1 Core Substrate Validation
**Date:** 2026-03-21

This document defines requirements specific to the sandbox testing environment. These requirements govern how the sandbox is set up, what scenarios it must exercise, and what constitutes a valid pass/fail signal.

---

## 1. Sandbox Environment Requirements

### SBX-ENV-1: Isolation
The sandbox must run on a single local machine in a dedicated temporary repository. It must not interfere with the developer's active Grava repository or any active worktrees.

### SBX-ENV-2: Clean State
Each sandbox test run begins with a freshly initialized Grava environment (`grava init`). No data must carry over from previous runs unless explicitly testing persistence across restarts.

### SBX-ENV-3: Reproducibility
Sandbox test scenarios must be scriptable and repeatable. A single shell script or Makefile target must be able to reproduce any scenario from a clean state.

### SBX-ENV-4: Agent Simulation
Agents are simulated via parallel shell processes or Go test goroutines. Each simulated agent must use a unique `--actor` flag value (e.g., `agent-001` through `agent-030`).

### SBX-ENV-5: Dolt Server
The sandbox runs a local Dolt SQL server instance. The Dolt server must be started before any test scenario and stopped cleanly after. The server port must not conflict with the developer's primary Dolt instance.

---

## 2. Functional Requirements (Sandbox-Scoped)

These FRs from the PRD are prioritized for sandbox validation based on concurrency risk and Phase 1 criticality.

### Priority 1 — Concurrency & Atomicity (Must Pass)

| Req ID | Description | Source |
|--------|-------------|--------|
| SBX-FR-1 | Two agents executing `grava claim` on the same issue simultaneously must result in exactly one claim success and one deterministic rejection | NFR3, FR5 |
| SBX-FR-2 | `grava ready` queried by 30 agents concurrently must return consistent, non-conflicting issue queues | FR9, NFR3 |
| SBX-FR-3 | `grava dep --add` executed concurrently with overlapping issue pairs must not deadlock (max 3 retry attempts, 10ms backoff per ADR-H3) | FR8, ADR-H3 |
| SBX-FR-4 | `grava subtask` executed concurrently for the same parent must produce unique, sequential child IDs without collision | FR2, ADR: child_counters |

### Priority 2 — Data Integrity (Must Pass)

| Req ID | Description | Source |
|--------|-------------|--------|
| SBX-FR-5 | `grava export` followed by `grava import` on a fresh repo must reproduce an identical issue graph (100% fidelity) | NFR4, FR20, FR21 |
| SBX-FR-6 | A 3-way Git merge (simulated via two branches with divergent issue state) must resolve correctly using the merge driver; unresolvable conflicts must be isolated to the conflicts table | FR22 |
| SBX-FR-7 | `grava compact` called while agents are `in_progress` must not delete any wisp rows for active issues | ADR-FM5, FR16 |
| SBX-FR-8 | A full Dolt crash mid-import must result in zero partial state (full rollback, no orphaned rows) | CM-1, FR21 |

### Priority 3 — Lifecycle & Teardown (Should Pass)

| Req ID | Description | Source |
|--------|-------------|--------|
| SBX-FR-9 | `grava close <id>` with uncommitted worktree changes must abort before any DB write | ADR-004 teardown |
| SBX-FR-10 | `grava stop <id>` must return issue to `open` status with wisp rows preserved for the next agent | ADR-004, ADR-FM5 |
| SBX-FR-11 | `grava doctor` must detect and report all 7 Phase 1 checks (ADR-FM7), exiting non-zero on any `❌ fail` | FR17, ADR-FM7 |
| SBX-FR-12 | `grava init` run twice on the same repo must be fully idempotent — no duplicate hooks, no second coordinator, no config overwrite | CM-4 |

### Priority 4 — Git Hook Integration (Should Pass)

| Req ID | Description | Source |
|--------|-------------|--------|
| SBX-FR-13 | `git pull` in a repo with active Git hooks must trigger `grava hook post-merge` and reconcile any JSONL-to-DB mismatches | FR25, ADR-001 |
| SBX-FR-14 | Hook stubs written by `grava init` must be valid shell scripts executable on macOS and Linux | ADR-H2, ADR-001 |

---

## 3. Non-Functional Requirements (Sandbox-Scoped)

| Req ID | Threshold | Measurement Method | Source |
|--------|-----------|-------------------|--------|
| SBX-NFR-1 | `grava ready` and `grava list` return in < 100ms at 10,000 issues | `time` wrapper on CLI invocation | NFR1 |
| SBX-NFR-2 | `grava create` and `grava claim` commit in < 15ms per op; sustained >70 ops/sec across 30 agents | Aggregate timing across agent simulation script | NFR2 |
| SBX-NFR-3 | Zero deadlocks observed across 1,000 concurrent write operations | Dead lock error count = 0 in Dolt error log | NFR3 |
| SBX-NFR-4 | Binary executable runs without any external runtime dependency (no Python, Node, etc.) | `ldd` on Linux / `otool -L` on macOS | NFR6 |

---

## 4. Chaos / Fault Injection Requirements

These requirements validate system resilience under failure conditions. Each scenario is derived from the Architecture Chaos Monkey decisions (CM-1 through CM-6).

| Req ID | Scenario | Expected Behavior |
|--------|----------|-------------------|
| SBX-CHAOS-1 | Kill Dolt process mid-import (CM-1) | CLI outputs rollback message; DB state unchanged; retry succeeds |
| SBX-CHAOS-2 | Kill coordinator process while 5 agents are connected (CM-2) | Agents detect coordinator-down, fire ConsoleNotifier, exit cleanly |
| SBX-CHAOS-3 | Move repository to a new directory while agents are idle (CM-3) | `grava doctor` passes; relative redirect paths remain valid |
| SBX-CHAOS-4 | Run `grava init` twice in rapid succession (CM-4) | Second run is a no-op; no duplicated hooks, config unchanged |
| SBX-CHAOS-5 | Agent re-claims an issue it already has a worktree for (CM-5) | Existing worktree is reused with log message; no second `git worktree add` |
| SBX-CHAOS-6 | `grava compact` called during 10 active in_progress claims (CM-6) | Compact runs safely; zero impact on active wisps or in_progress issues |

---

## 5. Constraints

- All sandbox tests must run without network access (offline-first validation)
- No Telegram/WhatsApp notifier required in sandbox — `ConsoleNotifier` only
- Sandbox may use `--enable-worktrees` flag for multi-agent worktree scenarios
- Sandbox results must be captured as structured output (JSON or markdown report) for post-run review
