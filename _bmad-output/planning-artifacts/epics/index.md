---
stepsCompleted:
  - step-01-validate-prerequisites
  - step-02-design-epics
inputDocuments:
  - _bmad-output/planning-artifacts/prd.md
  - _bmad-output/planning-artifacts/architecture.md
  - _bmad-output/planning-artifacts/implementation-readiness-report-2026-03-21.md
---

# grava - Epic Breakdown Index

This folder contains the complete epic and story breakdown for grava, decomposing requirements from the PRD and Architecture into implementable epics.

## Epic Files

| Epic | File | Score | Status |
|------|------|-------|--------|
| E1: Foundation & Scaffold | [epic-01-foundation.md](epic-01-foundation.md) | 3.80 | Planned |
| E2: Issue Lifecycle | [epic-02-issue-lifecycle.md](epic-02-issue-lifecycle.md) | 4.40 | Planned |
| E3: Atomic Claim & Wisp | [epic-03-atomic-claim.md](epic-03-atomic-claim.md) | 4.60 | Planned |
| E4: Dependency Graph | [epic-04-dependency-graph.md](epic-04-dependency-graph.md) | 4.40 | Planned |
| E5: Worktree Orchestration | [epic-05-worktree.md](epic-05-worktree.md) | 4.05 | Planned |
| E6: Advanced Merge Driver | [epic-06-advanced-merge-driver.md](epic-06-advanced-merge-driver.md) | 4.25 | Planned |
| E7: Git Sync — Export, Import & Hook Pipeline | [epic-07-git-sync.md](epic-07-git-sync.md) | 4.20 | Planned |
| E8: File Reservation | [epic-08-file-reservation.md](epic-08-file-reservation.md) | 4.05 | Planned |
| E9: Health & Maintenance | [epic-09-health-maintenance.md](epic-09-health-maintenance.md) | 4.15 | Planned |
| E10: Sandbox Validation | [epic-10-sandbox-validation.md](epic-10-sandbox-validation.md) | 3.55 | Planned |

## Requirements Inventory

The full requirements inventory (FRs, NFRs, additional requirements, FR Coverage Map) is in [requirements.md](requirements.md).

## Epic Quality Matrix Score

*Generated via Advanced Elicitation — Comparative Analysis Matrix. Criteria weighted by architectural and delivery risk.*

**Criteria:** A=User Value Clarity (20%) · B=FR Coverage Completeness (20%) · C=Standalone Deliverability (20%) · D=Implementation Risk (15%) · E=Story Decomposability (15%) · F=Testability/Sandbox Gate (10%)

| Epic | A | B | C | D | E | F | **Score** |
|------|---|---|---|---|---|---|-----------|
| E1: Foundation & Scaffold | 3 | 4 | 5 | 4 | 4 | 3 | **3.80** |
| E2: Issue Lifecycle | 5 | 5 | 4 | 3 | 5 | 4 | **4.40** |
| E3: Atomic Claim & Wisp | 5 | 5 | 4 | 5 | 4 | 5 | **4.60** |
| E4: Dependency Graph | 5 | 5 | 4 | 3 | 5 | 4 | **4.40** |
| E5: Worktree Orchestration | 5 | 4 | 3 | 5 | 3 | 5 | **4.05** |
| E6: Advanced Merge Driver | 4 | 5 | 4 | 5 | 3 | 5 | **4.25** |
| E7: Git Sync (Export/Import/Hooks) | 4 | 5 | 4 | 3 | 4 | 5 | **4.20** |
| E8: File Reservation | 4 | 5 | 3 | 4 | 4 | 4 | **4.05** |
| E9: Health & Maintenance | 4 | 5 | 4 | 3 | 4 | 5 | **4.15** |
| E10: Sandbox Validation | 4 | 5 | 2 | 2 | 3 | 5 | **3.55** |

**Key actions applied:** FR-ECS-1 split into FR-ECS-1a/b/c/d. Story 0 decomposed into 0a/0b/0c. NFR ownership anchored per epic. E10 merge driver spike mandatory (extracted from E7). E8 Phase 1 deferral condition documented. E11 CI enforcement required. FR22 → Epic 10.

## Parallel Track Map

```
E1-Story-0a ──► E2 ────────────────────────────────────────┐
             └► E3 ────────────────────────────────────────►│
E1-complete ──► E4 ────────────────────────────────────────►│
             └► E5 (Worktree) ──► E6 (Merge) ──► E7 ────────►│
             └► E7 (Sync) ──► E8 (Reservation) ────────────►│
             └► E7 ────────► E9 (Health) ────────────►│
                                                       └► E10 (gates each epic)
```
