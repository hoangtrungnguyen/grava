# Agent Issue Tracker - MVP Epics

Project name: Grava

## Overview

This document defines the epics for building the Minimum Viable Product (MVP) of a Dolt-backed AI Agent Issue Tracker. The system enables autonomous AI agents to coordinate work through a version-controlled, graph-aware database with deterministic task selection and multi-agent synchronization.

---

## Epics

The detailed requirements and user stories for each epic are maintained in separate documents:

### [Epic 1: Storage Substrate and Schema Implementation](epics/Epic_1_Storage_Substrate.md)
**Goal:** Establish the version-controlled Dolt database foundation with core schema and basic CRUD operations.

### [Epic 2: Graph Mechanics and Blocked Issue Tracking](epics/Epic_2_Graph_Mechanics.md)
**Goal:** Build the dependency graph engine and "Ready Engine" that computes actionable work.

### [Epic 3: Git Merge Driver](epics/Epic_3_Git_Merge_Driver.md)
**Goal:** Transform Git from a text-based version control system into a schema-aware database manager for Grava.

### [Epic 4: Grava Flight Recorder (Log Saver)](epics/Epic_4_Log_Saver.md)
**Goal:** Implement a persistent, structured logging system that captures command execution, internal errors, stack traces, and agent interactions.

### [Epic 5: Security Implementation and Access Control](epics/Epic_5_Security.md)
**Goal:** Implement zero-trust security and granular permissions before exposing to autonomous agents.

### [Epic 6: MCP Integration and Agent Onboarding](epics/Epic_6_MCP_Integration.md)
**Goal:** Bridge the database to AI agents via Model Context Protocol and complete end-to-end testing.

### [Epic 7 (Optional): Advanced Graph Analytics](epics/Epic_7_Advanced_Analytics.md)
**Goal:** Implement sophisticated graph analysis for project optimization (post-MVP enhancement).

### [Epic 8: Advanced Workflows (Rigs, Molecules)](epics/Epic_8_Advanced_Workflows.md)
**Goal:** Implement enterprise-scale workflow automation, multi-repository support, and database maintenance/compaction.

---

## Implementation Timeline

### Phase 1: Foundation (Weeks 1-2)
- Epic 1: Storage Substrate and Schema Implementation

### Phase 2: Intelligence (Weeks 3-4)
- Epic 2: Graph Mechanics and Blocked Issue Tracking

### Phase 3: Distribution (Weeks 5-6)
- Epic 3: Git Merge Driver

### Phase 4: Observability (Week 7)
- Epic 4: Grava Flight Recorder (Log Saver)

### Phase 5: Security (Week 8)
- Epic 5: Security Implementation and Access Control

### Phase 6: Integration (Week 9)
- Epic 6: MCP Integration and Agent Onboarding

### Phase 7: Enhancement (Post-MVP, Optional)
- Epic 7: Advanced Graph Analytics

---

## Definition of Done

An epic is considered complete when:
1. All user stories within the epic are implemented
2. Acceptance criteria for each story are met
3. Unit and integration tests achieve >85% coverage
4. Code review approved by at least 2 team members
5. Documentation updated (README, API docs, AGENTS.md)
6. Performance benchmarks meet stated requirements
7. Security review passed (for Epics 5-6)
8. Demo completed to stakeholders

---

## Risk Mitigation

| Risk | Impact | Mitigation Strategy |
|------|--------|-------------------|
| Dolt performance degradation at scale | High | Implement caching layer, optimize queries, conduct load testing early |
| Cell-level merge conflicts in practice | Medium | Define clear conflict resolution strategies, extensive testing with concurrent agents |
| MCP integration breaking changes | Medium | Pin MCP server version, maintain compatibility layer |
| Certificate management complexity | Low | Automate cert generation, clear rotation procedures |
| Agent misuse of tracker | High | Strict RBAC, comprehensive audit logging, automated anomaly detection |

---

## Success Metrics

### Technical Metrics
- Ready Engine query time: <10ms (p95)
- Sync cycle completion: <2 seconds (p95)
- System uptime: >99.9%
- Concurrent agent capacity: 50+ agents
- Zero data loss incidents

### Product Metrics
- Agent adoption rate: 80% of agents use tracker within 2 weeks
- Task completion rate improvement: 30% increase vs. baseline
- Blocking issue resolution time: 40% reduction
- Agent confusion/error rate: <5% of operations

---

## Appendix: Epic Dependencies

```
Epic 1 (Foundation)
    ↓
Epic 2 (Graph Mechanics) ← requires Epic 1 schema
    ↓
Epic 3 (Git Merge Driver) ← requires Epic 1 & 2 core functionality
    ↓
Epic 4 (Log Saver) ← requires Epic 1 & 2 core functionality
    ↓
Epic 5 (Security) ← requires Epic 3 & 4
    ↓
Epic 6 (MCP Integration) ← requires Epic 1-5 complete
    ↓
Epic 7 (Analytics) ← optional, requires Epic 2 graph engine
```
