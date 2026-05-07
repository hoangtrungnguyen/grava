---
stepsCompleted:
  - step-01-document-discovery
  - step-02-prd-analysis
  - step-03-epic-coverage-validation
  - step-04-ux-alignment
  - step-05-epic-quality-review
  - step-06-final-assessment
status: complete
completedAt: '2026-03-21'
project_name: grava
date: '2026-03-21'
documentsUsed:
  prd: _bmad-output/planning-artifacts/prd.md
  architecture: _bmad-output/planning-artifacts/architecture.md
  epics: null
  ux: null
---

# Implementation Readiness Assessment Report

**Date:** 2026-03-21
**Project:** grava

---

## PRD Analysis

### Functional Requirements

| ID | Requirement |
|----|-------------|
| FR1 | Create discrete issues or macro-epics (`create`, `quick`) |
| FR2 | Break down issues into hierarchical subtasks (`subtask`) |
| FR3 | Update core fields: status, priority, assignees (`update`, `assign`) |
| FR4 | Track start/stop working on an issue (`start`, `stop`) |
| FR5 | Atomic claim on an issue, locking to actor ID (`claim`) |
| FR6 | Append metadata via tags and notes (`label`, `comment`) |
| FR7 | Remove or archive issues (`drop`, `clear`) |
| FR8 | Establish directional blocking relationships (`dep`) |
| FR9 | Query top-priority actionable tasks with no blockers (`ready`) |
| FR10 | Query what upstream issues block a specific task (`blocked`) |
| FR11 | Visualize/traverse the dependency structure (`graph`) |
| FR12 | Filter, search, view individual issue properties (`list`, `search`, `show`) |
| FR13 | View aggregated workspace performance metrics (`stats`) |
| FR14 | Retrieve historical command ledger (`cmd_history`) |
| FR15 | Revert recent state-altering commands (`undo`) |
| FR16 | Prune expired/deleted data (`compact`) |
| FR17 | Run diagnostic health checks (`doctor`) |
| FR18 | Write/read ephemeral Wisp state per issue |
| FR19 | Retrieve historical progression log of an issue |
| FR20 | Export database state to machine-readable artifact (`export`) |
| FR21 | Import standardized artifact (`import`) |
| FR22 | Automatic 3-way cell-level merge during Git updates; isolate unresolvable conflicts |
| FR23 | Initialize a new isolated tracking environment (`init`, `config`) |
| FR24 | Dual-Safety Check (JSONL hash vs. Dolt state) before import |
| FR25 | Automatically trigger graph DB updates via Git hooks |
| FR26 | Automated cross-platform install script (macOS, Linux, Windows) |
| FR27 | OS/arch detection, dependency installation without user intervention |
| FR28 | Install script validates environment via `grava doctor` |

**Total FRs: 28**

### Non-Functional Requirements

| ID | Requirement |
|----|-------------|
| NFR1 | Query speed: `ready`, `list` < 100ms at 10,000 issues |
| NFR2 | Write throughput: `create`, `update`, `claim` < 15ms, >70 inserts/sec |
| NFR3 | Atomic execution: concurrent claims → exactly one success, one rejection, no deadlock |
| NFR4 | Zero-loss handoff: 100% preservation of dependency links during export/import |
| NFR5 | Machine readability: strict JSON schema adherence; breaking changes = major version bump |
| NFR6 | Zero-dependency footprint: single statically linked binary |
| NFR7 | Install speed: full setup < 5 minutes on clean machine (≥10 Mbps) |
| NFR8 | Install reliability: succeeds first attempt on macOS (ARM/x86), Linux (Debian/RHEL), Windows (x86-64) |

**Total NFRs: 8**

### Additional Requirements / Constraints

- **Concurrency model:** Dolt advisory locking + `SELECT FOR UPDATE`; wisp tables for ephemeral agent state
- **Agent containment:** Kill-switches tied to `--actor` flag
- **Schema versioning:** `--json` output schema changes require major version bump
- **Beads merging:** Custom Git merge driver for JSONL; schema-aware conflict resolution
- **Pointer-based information:** Grava stores pointers (function locations, file paths, commit IDs), not blobs

### PRD Completeness Assessment

✅ PRD is comprehensive and well-structured. All 3 phases defined (Phase 1 = MVP scope). FRs are numbered and traceable. NFRs have measurable thresholds. Three user journeys fully described. One gap: Journey 3 (concurrent claim edge case) is described but FR5 + NFR3 together fully cover it.

---

## Epic Coverage Validation

### Status: ⚠️ No BMAD Epics & Stories Document Exists

No `*epic*.md` was found in `_bmad-output/planning-artifacts/`. The legacy `docs/epics/` files pre-date the BMAD workflow and are not formatted for traceability validation.

### Architecture FR Coverage (Proxy Validation)

Since no formal Epics & Stories doc exists, coverage is validated against the completed Architecture document (`architecture.md` — the closest available specification artifact):

| FR | PRD Requirement (short) | Architecture Coverage | Status |
|----|------------------------|-----------------------|--------|
| FR1 | Create issues/epics | `pkg/cmd/issues/` + `WithAuditedTx` + `EventCreate` | ✅ Covered |
| FR2 | Subtasks | `pkg/cmd/issues/` + parent-child graph relation | ✅ Covered |
| FR3 | Update fields | `pkg/cmd/issues/` + `EventUpdate` + priority maps | ✅ Covered |
| FR4 | Start/stop | `pkg/cmd/issues/` + `EventStart`/`EventStop` | ✅ Covered |
| FR5 | Atomic claim | `SELECT FOR UPDATE` + `WithDeadlockRetry` + `EventClaim` | ✅ Covered |
| FR6 | Tags/comments | `pkg/cmd/issues/` + `EventComment`/`EventLabel` | ✅ Covered |
| FR7 | Drop/archive | `pkg/cmd/issues/` + `EventDrop` | ✅ Covered |
| FR8 | Dep relationships | `pkg/cmd/graph/` + `EventDependencyAdd` | ✅ Covered |
| FR9 | Ready queue | `pkg/cmd/graph/` — graph traversal, no-blocker filter | ✅ Covered |
| FR10 | Blocked query | `pkg/cmd/graph/` + `pkg/graph/` | ✅ Covered |
| FR11 | Graph visualization | `pkg/cmd/graph/` + `pkg/graph/` | ✅ Covered |
| FR12 | List/search/show | `pkg/cmd/issues/` + `--json` flag + `GravaError` | ✅ Covered |
| FR13 | Stats | `pkg/cmd/maintenance/` | ✅ Covered |
| FR14 | Command history | `pkg/cmd/maintenance/` + audit table | ✅ Covered |
| FR15 | Undo | `pkg/cmd/maintenance/` | ✅ Covered |
| FR16 | Compact | `pkg/cmd/maintenance/` | ✅ Covered |
| FR17 | Doctor | `pkg/cmd/maintenance/` | ✅ Covered |
| FR18 | Wisp write/read | `pkg/wisp/` + `pkg/cmd/` wisp commands | ✅ Covered |
| FR19 | Issue history log | `pkg/wisp/` + audit table | ✅ Covered |
| FR20 | Export | `pkg/cmd/sync/` | ✅ Covered |
| FR21 | Import | `pkg/cmd/sync/` + Dual-Safety Check | ✅ Covered |
| FR22 | 3-way Git merge | ADR Git merge driver + `pkg/cmd/sync/` | ✅ Covered |
| FR23 | Init/config | `pkg/cmd/` root + `pkg/migrate/` (Story 0 AC) | ✅ Covered |
| FR24 | Dual-Safety Check | `pkg/cmd/sync/` import validation | ✅ Covered |
| FR25 | Git hook triggers | `pkg/cmd/hook/` | ✅ Covered |
| FR26 | Install script (macOS/Linux/Windows) | ⚠️ Not mentioned in architecture | ⚠️ Gap |
| FR27 | OS/arch detection in install | ⚠️ Not mentioned in architecture | ⚠️ Gap |
| FR28 | Install script validates via `grava doctor` | Links to FR17 (`doctor`) — partial coverage | ⚠️ Gap |

### Coverage Statistics

- Total PRD FRs: 28
- FRs covered in architecture: 25
- FRs with gaps: 3 (FR26, FR27, FR28 — install script)
- Architecture coverage: **89%**

### Missing Coverage Notes

**FR26–FR28 (Install Script)** — The architecture document covers the core CLI substrate comprehensively but does not address the automated install script. This is an operational/DevOps concern that likely belongs in a separate story or epic (e.g., "Epic: Developer Experience & Onboarding"). It does not block Phase 1 core implementation but must be planned before Phase 1 ships.

---

## UX Alignment Assessment

### UX Document Status

**Not Found** — No UX design document exists in `_bmad-output/planning-artifacts/`.

### Is UX Implied?

Grava is a **CLI tool** — its "UX" is the command interface itself. The PRD explicitly states "Machine-Native Optimization: Eliminates UI/UX bloat." There are no web, mobile, or graphical interface components implied.

**Assessment: UX document is intentionally absent and appropriate for this project type.**

The CLI interface is covered by:
- `docs/guides/CLI_REFERENCE.md` (existing reference)
- PRD FR1–FR28 command specifications
- Architecture `--json` flag + JSON Error Envelope contract (C1 critical gap now documented)

### Warnings

- ⚠️ **Install Script UX (FR26–FR28):** The install script interaction (menu prompt: "Which AI backend?") is the only user-facing UI flow not explicitly designed. Recommend adding a short install script UX spec (2–3 lines per prompt) when the install epic is created.
- No other UX gaps identified for a CLI-primary tool.

---

## Epic Quality Review

### Status: ⚠️ No BMAD-Format Epics Document — Legacy Epics Assessed

No formal BMAD epics & stories document exists. The `docs/epics/` directory contains 15+ legacy epic files from pre-BMAD work. These are assessed for quality signals to inform the upcoming `/bmad-bmm-create-epics-and-stories` run.

### Legacy Epic Signal Analysis

| Legacy Epic | User Value? | Independence? | FR Coverage | Quality Signal |
|-------------|-------------|--------------|-------------|----------------|
| Epic_1_Storage_Substrate | ⚠️ Technical milestone | Standalone | FR23, FR17 | 🟠 Rename needed |
| Epic_1.1_additional_commands | ✅ User commands | Depends on Epic 1 | FR1–FR7 | ✅ Good intent |
| Epic_2_Graph_Implementation_Plan | ⚠️ Technical | Depends on Epic 1 | FR8–FR11 | 🟠 Rename needed |
| Epic_2.2_Graph_Lifecycle_and_Propagation | ✅ User value | Depends on Epic 2 | FR8–FR13 | ✅ Good intent |
| Epic_3_Git_Merge_Driver | ✅ User value (conflict-free) | Depends on Epic 1+2 | FR22, FR24, FR25 | ✅ Good |
| Epic_4_Log_Saver / Ephemeral Store | ✅ Wisp/agent value | Depends on Epic 1 | FR18–FR19 | ✅ Good |
| Epic_5_Multi_Agent_Sync | ✅ Swarm value | Phase 2 scope | FR20–FR21 | ⚠️ Phase 2 only |
| Epic_6_MCP_Integration | ✅ Ecosystem value | Phase 3 scope | — | ⚠️ Phase 3 |

### Critical Violations (for BMAD Epics to Avoid)

🔴 **Technical Milestone Naming** — "Storage Substrate" and "Graph Implementation Plan" are infrastructure milestones, not user value statements. When creating BMAD epics, rename to user-outcome framing:
- ❌ "Epic: Storage Substrate Setup" → ✅ "Epic: Issue Tracking Foundation (create, update, list, claim)"
- ❌ "Epic: Graph Implementation Plan" → ✅ "Epic: Dependency & Context Graph (dep, ready, blocked, graph)"

🔴 **Story 0 Must Be First** — Architecture mandates an 11-AC Story 0 (infrastructure scaffold) before any feature story. This is not present in legacy epics. Must be Epic 1 Story 1 in the BMAD plan.

🟠 **Phase Boundary Violation Risk** — Epic_5 (Multi-Agent Sync) and Epic_6 (MCP) belong to Phase 2/3 respectively. The BMAD epics must explicitly gate these behind Phase 1 completion. Do not let Phase 2 stories bleed into Phase 1 sprint planning.

🟠 **Install Script Epic Missing** — FR26–FR28 have no corresponding legacy epic. A dedicated "Developer Experience & Onboarding" epic must be created.

### Best Practices Checklist for Upcoming Epics Creation

When running `/bmad-bmm-create-epics-and-stories`, enforce:

- [ ] Epic 1 Story 1 = Story 0 (infrastructure scaffold, 11 ACs from architecture)
- [ ] All epic titles framed as user outcomes, not technical milestones
- [ ] FR26–FR28 covered by a dedicated Onboarding epic
- [ ] Phase 2 epics (FR20–FR21, multi-repo sync) clearly marked as out of Phase 1 scope
- [ ] Each story independently completable (no forward dependencies)
- [ ] Every story has testable ACs in Given/When/Then format
- [ ] DB tables created per-story, not bulk-created in Story 0

---

## Summary and Recommendations

### Overall Readiness Status

**⚠️ NEEDS WORK — Ready for Epics Creation, Not Yet for Sprint Planning**

The PRD and Architecture are both excellent and complete. The project is well-positioned to begin epics and stories creation. However, **sprint planning cannot begin** until the formal BMAD Epics & Stories document is produced and the 2 critical architecture gaps are resolved.

---

### Critical Issues Requiring Immediate Action

| # | Issue | Severity | Blocks |
|---|-------|----------|--------|
| C1 | No BMAD Epics & Stories document | 🔴 Critical | Sprint Planning |
| C2 | JSON Error Envelope contract not implemented | 🔴 Critical | Story 0 completion |
| C3 | Coordinator Error Channel pattern not implemented | 🔴 Critical | Story 0 completion |

---

### Recommended Next Steps

1. **Run `/bmad-bmm-create-epics-and-stories`** (fresh context window) — Create the formal BMAD epics & stories document using the PRD + architecture as input. Ensure Epic 1 Story 1 = Story 0 with the 11 ACs from the architecture document. Use user-outcome epic naming (not technical milestones).

2. **Resolve Architecture Critical Gaps (C2 + C3)** — Before Story 0 is implemented, add two ACs to Story 0:
   - JSON Error Envelope: `{"error": {"code": "...", "message": "..."}}` for all `--json` error paths
   - Coordinator Error Channel: `Start(ctx) <-chan error` pattern; goroutines must not `log.Fatal`/`os.Exit`

3. **Add Onboarding Epic (FR26–FR28)** — Create a "Developer Experience & Onboarding" epic covering the cross-platform install script. Scope it as Phase 1 completion requirement (must ship before Phase 1 is "done").

4. **Gate Phase 2 epics** — When creating epics, explicitly mark FR20–FR21 (import/export advanced sync) and all multi-repo features as Phase 2. Do not include them in Phase 1 sprint planning.

5. **Run `/bmad-bmm-check-implementation-readiness` again** after epics are created — This will enable full Epic Quality Review (step 5) with a real BMAD epics document.

---

### Final Note

This assessment identified **6 issues** across **4 categories**:
- 1 missing artifact (Epics & Stories document)
- 3 FR coverage gaps (FR26–FR28, install script)
- 2 critical architecture gaps (JSON error envelope, coordinator error channel)
- 4 epic quality signals from legacy epics (naming, Story 0, phase boundaries, onboarding epic)

**The PRD is comprehensive (28 FRs, 8 NFRs, fully specified).** The Architecture is complete (status: `complete`, 8 steps, 6 advanced elicitation rounds). The project is well-structured for Phase 1 implementation once the Epics & Stories document is created.

**Report saved:** `_bmad-output/planning-artifacts/implementation-readiness-report-2026-03-21.md`
