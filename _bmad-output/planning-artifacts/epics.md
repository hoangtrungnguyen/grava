---
stepsCompleted:
  - step-01-validate-prerequisites
  - step-02-design-epics
  - step-03-create-stories
inputDocuments:
  - _bmad-output/planning-artifacts/prd.md
  - _bmad-output/planning-artifacts/architecture.md
  - _bmad-output/planning-artifacts/implementation-readiness-report-2026-03-21.md
redirectTo: _bmad-output/planning-artifacts/epics/
---

# grava — Epic Breakdown (Workflow Anchor)

> All epic content lives in the [epics/](epics/) folder.
> This file is the canonical BMAD workflow state anchor and index.

---

## Epic Index

| Epic | File | Stories | Status |
|------|------|---------|--------|
| E1: Foundation & Scaffold | [epics/epic-01-foundation.md](epics/epic-01-foundation.md) | 3 (1.1–1.3) | ✅ Complete |
| E2: Issue Lifecycle | [epics/epic-02-issue-lifecycle.md](epics/epic-02-issue-lifecycle.md) | 6 (2.1–2.6) | ✅ Complete |
| E3: Atomic Claim & Wisp | [epics/epic-03-atomic-claim.md](epics/epic-03-atomic-claim.md) | 3 (3.1–3.3) | ✅ Complete |
| E4: Dependency Graph | [epics/epic-04-dependency-graph.md](epics/epic-04-dependency-graph.md) | 6 (4.1–4.6) | ✅ Complete |
| E5: Worktree Orchestration | [epics/epic-05-worktree.md](epics/epic-05-worktree.md) | 4 (5.1–5.4) | ✅ Complete |
| E6: Advanced Merge Driver | [epics/epic-06-advanced-merge-driver.md](epics/epic-06-advanced-merge-driver.md) | 4 (6.1 spike + 6.2–6.4) | ✅ Complete |
| E7: Git Sync — Export, Import & Hook Pipeline | [epics/epic-07-git-sync.md](epics/epic-07-git-sync.md) | 4 (7.1–7.4) | ✅ Complete |
| E8: File Reservation | [epics/epic-07-file-reservation.md](epics/epic-07-file-reservation.md) | 2 Phase 1 (8.1–8.2) | ✅ Complete |
| E9: Health & Maintenance | [epics/epic-08-health-maintenance.md](epics/epic-08-health-maintenance.md) | 5 (9.1–9.5) | ✅ Complete |
| E10: Sandbox Validation | [epics/epic-10-sandbox-validation.md](epics/epic-10-sandbox-validation.md) | 5 (10.1 CLI + 10.2–10.5) | ✅ Complete |

**Total Phase 1 stories: 45** (32 feature + 2 spikes + 5 Phase 1 E8 + 5 sandbox + 1 E8 CLI prereq)

## Phase 2 Deferred Stories

| File | Contents |
|------|----------|
| [epics/phase-2-deferred/epic-08-stories-phase2.md](epics/phase-2-deferred/epic-08-stories-phase2.md) | E8 Stories 8.3 (TTL Auto-Expiry) + 8.4 (Git Audit Trail) |

## Supporting Documents

| Document | Purpose |
|----------|---------|
| [epics/index.md](epics/index.md) | Epic list, quality matrix scores, parallel track map |
| [epics/requirements.md](epics/requirements.md) | Full FR/NFR inventory and FR coverage map |
