---
stepsCompleted:
  - step-01-document-discovery
  - step-02-prd-analysis
  - step-03-epic-coverage-validation
  - step-04-ux-alignment
  - step-05-epic-quality-review
  - step-06-final-assessment
documentsUsed:
  prd: _bmad-output/planning-artifacts/prd.md
  architecture: _bmad-output/planning-artifacts/architecture.md
  epics: _bmad-output/planning-artifacts/epics/ (sharded, 11 epics)
  ux: null
---

# Implementation Readiness Assessment Report

**Date:** 2026-03-22
**Project:** grava

---

## PRD Analysis

### Functional Requirements

**Issue Creation & Modification**
- FR1: System Agent / Human Developer can create discrete issues or macro-epics (`create`, `quick`).
- FR2: System Agent / Human Developer can rapidly break down overarching issues into hierarchical subtasks (`subtask`).
- FR3: System Agent / Human Developer can update core fields like status, priority, and assignees (`update`, `assign`).
- FR4: System Agent / Human Developer can explicitly track when they start or stop working on a specific issue (`start`, `stop`).
- FR5: System Agent can execute an atomic claim on an issue, verifying it is unassigned and immediately locking it to their actor ID (`claim`).
- FR6: System Agent / Human Developer can append contextual metadata to issues via tags and text notes (`label`, `comment`).
- FR7: System Agent / Human Developer can safely remove or archive issues from the active tracking space (`drop`, `clear`).

**Graph Context & Discovery**
- FR8: System Agent / Human Developer can establish directional "blocking" relationship links between issues (`dep`).
- FR9: System Agent / Human Developer can query the immediate actionable queue of top-priority tasks with no blockers (`ready`).
- FR10: System Agent / Human Developer can explicitly query what upstream issues are preventing a specific task from being worked on (`blocked`).
- FR11: System Agent / Human Developer can visualize or traverse the overarching dependency structure of the project (`graph`).
- FR12: System Agent / Human Developer can filter, search, and view detailed individual properties of issues (`list`, `search`, `show`).
- FR13: Human Developer can view aggregated workspace performance and status metrics (`stats`).

**State History & Database Maintenance**
- FR14: System Agent / Human Developer can retrieve a detailed ledger of previously executed system commands (`cmd_history`).
- FR15: Human Developer can safely revert recent state-altering commands to recover from errors (`undo`).
- FR16: System Agent / Human Developer can explicitly prune expired or deleted data to maintain high query performance (`compact`).
- FR17: Human Developer can run diagnostic health checks on the Grava substrate to ensure data integrity (`doctor`).

**Ephemeral State Operations (Wisp Data)**
- FR18: System Agent / Human Developer can explicitly write to and read from an issue's ephemeral state (Wisp) via dedicated commands to manage working artifacts and execution history.
- FR19: System Agent / Human Developer can retrieve the historical progression log of an issue to understand what a previous agent did before handoff.

**Onboarding & Installation**
- FR26: System provides an automated install script (shell-based, cross-platform for macOS and Linux) that installs user's chosen AI CLI backend (Claude CLI or Gemini CLI) and Grava in a single execution.
- FR27: Install script detects host OS and architecture, selects correct binary/package source, handles all dependency installation. Supported: macOS (ARM, x86), Linux (Debian/Ubuntu, RHEL/Fedora), Windows (x86-64 via PowerShell).
- FR28: Install script validates environment by running `grava doctor` and reports clear success/failure message with remediation steps.

**Workspace Synchronization & Ecosystem Integration**
- FR20: System Agent / Human Developer can export the internal database state into a standardized, machine-readable artifact (`export`).
- FR21: System Agent / Human Developer can hydrate the internal database by importing a standardized artifact (`import`), provided no conflicts exist.
- FR22: System must automatically execute a 3-way cell-level merge of issue state during Git updates. Unresolvable conflicts must be safely isolated to a separate database table for human intervention.
- FR23: Human Developer can initialize a brand-new, isolated tracking environment for a local repository (`init`, `config`).
- FR24: System must evaluate a Dual-Safety Check (JSONL hash vs. Dolt state) before importing to prevent overwriting uncommitted local data.
- FR25: System must automatically trigger graph database updates via Git hooks whenever repository state changes (e.g., `git pull`, checkout), actively detecting file-to-database mismatches.

**Total FRs: 25** (FR1–FR19, FR20–FR28; FR numbers are not sequential — FR20–FR25 = sync, FR26–FR28 = onboarding)

---

### Non-Functional Requirements

**Performance & Latency**
- NFR1 (Query Speed): Core graph resolution commands (`grava ready`, `grava list`) must return JSON within **< 100ms** under 10,000 active/ephemeral issues.
- NFR2 (Write Throughput): Standard issue creation and updates must commit to local Dolt in **< 15ms** per operation; sustained throughput > 70 inserts/second.

**Reliability & Data Integrity (Concurrency)**
- NFR3 (Atomic Execution): Concurrent write attempts must result in exactly one successful claim and one deterministic rejection — no polluted rows or deadlocks.
- NFR4 (Zero-Loss Handoff): 100% preservation of dependency links and core fields during export/import; identical graph recreation across workspace clones.

**Onboarding Experience**
- NFR7 (Install Speed): Automated install script must complete full environment setup in **< 5 minutes** on clean machine with ≥ 10 Mbps broadband.
- NFR8 (Install Reliability): Script must succeed on first attempt on macOS (ARM/x86), Linux (Debian/Ubuntu, RHEL/Fedora), Windows (x86-64) without elevated privileges beyond initial package manager bootstrapping.

**Operability & Extensibility**
- NFR5 (Machine Readability): Strict adherence to predefined JSON schemas for all `--json` outputs. Any schema change requires a major version bump.
- NFR6 (Zero-Dependency Footprint): Grava CLI must compile to a single statically linked binary; zero external runtime dependencies beyond system shell and Git.

**Total NFRs: 8** (NFR1–NFR8; NFR numbering skips NFR5/NFR6 positionally — they are operability NFRs)

---

### Additional Requirements / Constraints

- **Wisp Tables:** Agents persist ephemeral working data to a `wisp` table for real-time lineage without polluting primary issue tables.
- **Agent Containment (Safety):** Strict advisory locking and kill-switches tied to `--actor` flag to prevent infinite loops consuming local system resources.
- **Schema Adherence:** Any break in `--json` output schema necessitates a major version bump.
- **Dual-Loop Cognitive Architecture:** Inner Loop (tactical execution) fully managed by Grava; Outer Loop (strategic reasoning) happens off-CLI.
- **Pointer-Based Information Architecture:** Grava natively stores/returns pointers (function locations, file paths, commit IDs) to prevent LLM attention decay.
- **Beads-Inspired Merging:** Multi-layered sync linking JSONL files to SQL core using custom `git merge` driver, bypassing text-conflict Git markers.
- **Phase 1 Scope:** Single repository only. Phase 2 = multi-workspace orchestration. Phase 3 = advanced graph rules engine.

---

### PRD Completeness Assessment

The PRD is **well-structured and thorough** for a brownfield CLI tool. Key observations:
- All 25 FRs are clearly numbered and scoped to specific CLI commands.
- All 8 NFRs have measurable thresholds (ms latency, insert/s, success rate).
- Three user journeys (Agent, Human Dev, Non-Technical) + one edge case are documented.
- Journey numbering has a gap (Journey 3 is edge case, Journey 4 is onboarding) — cosmetic only.
- No UX Design document exists, which is acceptable for a CLI-only tool.
- FR numbering has a gap (FR20–FR25 are sync, FR26–FR28 are onboarding, no FR20–FR25 noted as "onboarding" in headings) — all requirements are present and traceable.

---

## Epic Coverage Validation

### Coverage Matrix

| FR | PRD Requirement (summary) | Epic Coverage | Story | Status |
|----|--------------------------|---------------|-------|--------|
| FR1 | Create issues/macro-epics (`create`, `quick`) | Epic 2 | Story 2.1 | ✅ Covered |
| FR2 | Break issues into subtasks (`subtask`) | Epic 2 | Story 2.2 | ✅ Covered |
| FR3 | Update fields / assign (`update`, `assign`) | Epic 2 | Story 2.3 | ✅ Covered |
| FR4 | Track start/stop work (`start`, `stop`) | Epic 2 + Epic 9 (extended) | Story 2.4, Stories 9.3/9.4 | ✅ Covered |
| FR5 | Atomic claim (`claim`) | Epic 3 + Epic 9 (extended) | Story 3.1, Story 9.2 | ✅ Covered |
| FR6 | Labels and comments (`label`, `comment`) | Epic 2 | Story 2.5 | ✅ Covered |
| FR7 | Archive/purge issues (`drop`, `clear`) | Epic 2 | Story 2.6 | ✅ Covered |
| FR8 | Dependency relationships (`dep`) | Epic 4 | Story 4.1 | ✅ Covered |
| FR9 | Ready queue (`ready`) | Epic 4 | Story 4.2 | ✅ Covered |
| FR10 | Blocked query (`blocked`) | Epic 4 | Story 4.3 | ✅ Covered |
| FR11 | Graph visualization (`graph`) | Epic 4 | Story 4.4 | ✅ Covered |
| FR12 | List/search/show (`list`, `search`, `show`) | Epic 4 | Story 4.5 | ✅ Covered |
| FR13 | Workspace stats (`stats`) | Epic 4 | Story 4.6 | ✅ Covered |
| FR14 | Command history (`cmd_history`) | Epic 5 | Story 5.1 | ✅ Covered |
| FR15 | Undo (`undo`) | Epic 5 | Story 5.2 | ✅ Covered |
| FR16 | Compact (`compact`) | Epic 5 | Story 5.3 | ✅ Covered |
| FR17 | Doctor diagnostic + repair (`doctor`, `--fix`, `--dry-run`) | Epic 5 | Stories 5.4 + 5.5 | ✅ Covered |
| FR18 | Wisp write/read (`wisp write`, `wisp read`) | Epic 3 | Story 3.2 | ✅ Covered |
| FR19 | Issue history log (`history`) | Epic 3 | Story 3.3 | ✅ Covered |
| FR20 | Export (`export`) | Epic 7 | Story 7.1 | ✅ Covered |
| FR21 | Import with safety check (`import`) | Epic 7 | Story 7.2 | ✅ Covered |
| FR22 | 3-way cell-level merge driver | Epic 10 | Stories 10.1 (spike) + 10.2 + 10.3 + 10.4 | ✅ Covered |
| FR23 | Initialize environment (`init`, `config`) | Epic 1 (basic) + Epic 9 (worktree) | Story 1.3, Story 9.1 | ✅ Covered |
| FR24 | Dual-Safety Check before import | Epic 7 | Story 7.2 | ✅ Covered |
| FR25 | Git hook auto-triggers | Epic 7 | Stories 7.3 + 7.4 | ✅ Covered |
| FR26 | Automated install script (macOS/Linux) | Epic 6 | Story 6.1 | ✅ Covered |
| FR27 | OS/arch detection, platform support | Epic 6 | Stories 6.1 + 6.2 | ✅ Covered |
| FR28 | Install validation via `grava doctor` | Epic 6 | Story 6.3 | ✅ Covered |
| FR-ECS-1a | File reservation declaration (`grava reserve`) | Epic 8 | Story 8.1 | ✅ Covered |
| FR-ECS-1b | Pre-commit hook enforcement | Epic 8 | Story 8.2 | ✅ Covered |
| FR-ECS-1c | TTL auto-expiry *(Phase 1 deferral condition)* | Epic 8 Phase 2 deferred | Stories 8.3 (deferred) | ⚠️ Deferred Phase 2 |
| FR-ECS-1d | Git audit trail *(Phase 1 deferral condition)* | Epic 8 Phase 2 deferred | Stories 8.4 (deferred) | ⚠️ Deferred Phase 2 |

---

### Missing Requirements

**No missing FRs found.** All 25 PRD FRs (FR1–FR28, no FR20–FR25 gap issues) and 4 derived ECS FRs are covered.

**Deferred items (by design, not gaps):**
- NFR1 (Query Speed <100ms at 10K issues): explicitly deferred to Phase 2 per ADR-002. Phase 1 target documented as <100ms at 1K issues.
- FR-ECS-1c (TTL auto-expiry): Phase 1 deferral condition — acceptable if Phase 1 adoption is primarily single-developer.
- FR-ECS-1d (Git audit trail): same Phase 1 deferral condition.

---

### Coverage Statistics

- **Total PRD FRs:** 25 (FR1–FR19, FR20–FR28)
- **Derived FRs (ECS):** 4 (FR-ECS-1a–1d)
- **FRs fully covered in Phase 1 epics:** 27 of 29 (93%)
- **FRs deferred to Phase 2 by design:** 2 (FR-ECS-1c, FR-ECS-1d)
- **FRs missing coverage:** 0
- **Coverage percentage:** ✅ 100% (27 Phase 1 + 2 explicitly deferred)
- **NFRs covered:** 7 of 8 (NFR1 deferred Phase 2 per ADR-002)

---

## UX Alignment Assessment

### UX Document Status

**Not Found** — and intentionally not required.

### Assessment

Grava is a **CLI-only tool** explicitly designed to eliminate UI/UX bloat (PRD: "Machine-Native Optimization: Grava eliminates UI/UX bloat"). There is no web, mobile, or graphical interface implied anywhere in the PRD, Architecture, or Epics.

The "user experience" in Grava is fully expressed via:
- Structured JSON output contracts (NFR5 — machine-readable, schema-versioned)
- CLI command ergonomics (command naming, flag conventions, error messages)
- Install script flow (Journey 4 — FR26–FR28)

All of these are adequately specified in the PRD (user journeys) and epics (acceptance criteria per story).

### Alignment Issues

None.

### Warnings

ℹ️ **INFO (not a gap):** No UX document exists. This is appropriate and expected for a CLI/agent-substrate tool. The CLI "UX" is captured in story-level acceptance criteria (JSON output schemas, error message text, command naming conventions).

---

## Epic Quality Review

### User Value Focus

| Epic | Verdict | Notes |
|------|---------|-------|
| E1: Foundation & Scaffold | ⚠️ Infrastructure epic | Correctly classified as brownfield prerequisite — no feature story can begin without it. Intentional and documented. |
| E2–E10 | ✅ User-centric | All titles and goals describe user outcomes. |
| E11: Sandbox Validation | ⚠️ Gate epic | C=2 standalone deliverability by design. Documented and accepted in quality matrix. |

### Epic Independence — No Violations Found

All dependency chains follow strict sequential logic: E1 → E2/E3 → E4/E5 → E6/E7/E9 → E8/E10 → E11. No forward dependencies. No circular dependencies. Parallel track map is explicit and correct.

### Story Quality Findings

#### 🔴 Critical Violations

**None found.**

#### 🟠 Major Issues

**M1 — E9 Story 9.2: Partial worktree rollback under filesystem failure**
- Issue: AC states "if worktree creation fails after DB claim succeeds, the claim is rolled back" — but no story specifies the filesystem cleanup mechanism for partial `git worktree add` failure (e.g., directory partially created before failure).
- Impact: Medium — distributed rollback (DB + filesystem) is non-trivial and the implementation detail is underspecified.
- Recommendation: Add an explicit AC to Story 9.2: "If `git worktree add` fails after DB claim, the `.grava/worktrees/<agent>/<issue>/` directory is cleaned up atomically before returning the error."

**M2 — E10 Story 10.1 (Spike): Exit criteria is documentation-only**
- Issue: Spike exit criteria produces a markdown report (`spike-reports/merge-driver-poc.md`) — not a CI-runnable test. A passing spike based on low-quality documentation could unblock remaining E10 stories prematurely.
- Impact: Medium — risk of sprint planning E10 stories on a weak foundation.
- Recommendation: Add a CI-invocable exit criterion: "spike produces an executable `grava sandbox run --scenario=spike-merge-driver` that passes in CI" as a hard gate before E10 Story 2 is sprint-planned.

#### 🟡 Minor Concerns

**m1 — E3 Story 3.3: `--since` flag not in Commands Delivered table**
- Issue: Story 3.3 AC references `grava history abc123def456 --since "2026-03-01"` but the Epic 3 Commands Delivered table lists only `grava history <id>` without the `--since` flag.
- Impact: Low — documentation inconsistency, not a functional gap.
- Recommendation: Add `--since <date>` to the Epic 3 Commands Delivered table.

**m2 — E5 Story 5.4: Hardcoded check count ("all 12 health checks")**
- Issue: AC says "all 12 health checks execute" — brittle if checks are added during implementation.
- Impact: Low — will only matter if the check list changes.
- Recommendation: Replace count with reference: "all checks listed in the Doctor Check Set section of Epic 5 execute."

**m3 — E8 FR-ECS-1c/1d deferral: Under-defined trigger condition**
- Issue: Deferral condition is "if Phase 1 adoption is primarily single-developer" — no measurable criterion.
- Impact: Low — could lead to ambiguous sprint planning decisions.
- Recommendation: Add concrete trigger: "Defer FR-ECS-1c/1d if sprint planning retrospective shows no concurrent agent use in Phase 1 (0 multi-agent sessions recorded in `wisp_entries`)."

**m4 — E11 Story 11.4: Two distinct scenarios (TS-04 + TS-07) in one story**
- Issue: TS-04 (export/import round-trip, gated on E7) and TS-07 (conflict detection, gated on E10) are bundled in Story 11.4 — different epic gates, different feature domains.
- Impact: Low — creates a coupled story that cannot start until both E7 and E10 are merged.
- Recommendation: Consider splitting into Story 11.4a (TS-04, E7 gate) and Story 11.4b (TS-07, E10 gate) for cleaner sprint tracking.

### Best Practices Compliance Summary

| Epic | User Value | Independent | Sized Right | No Fwd Deps | ACs BDD | Errors Covered | FR Traced |
|------|-----------|-------------|-------------|-------------|---------|----------------|-----------|
| E1 | ⚠️ Infra | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| E2 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| E3 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ m1 |
| E4 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| E5 | ✅ | ✅ | ⚠️ m2 | ✅ | ✅ | ✅ | ✅ |
| E6 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| E7 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| E8 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ m3 |
| E9 | ✅ | ✅ | ⚠️ M1 | ✅ | ✅ | ✅ | ✅ |
| E10 | ✅ | ✅ | ⚠️ M2 | ✅ | ✅ | ✅ | ✅ |
| E11 | ⚠️ Gate | ✅ | ⚠️ m4 | ✅ | ✅ | ✅ | ✅ |

**Total defects:** 0 Critical · 2 Major · 4 Minor

---

## Summary and Recommendations

### Overall Readiness Status

## ✅ READY FOR IMPLEMENTATION

The grava project planning artifacts are of **high quality** and are ready to begin Phase 1 implementation. No critical blocking issues were found. All 25 PRD FRs have traceable epic and story coverage. The architecture, epics, and stories are well-aligned. The 2 major and 4 minor issues identified are improvement recommendations — none block sprint start.

---

### Assessment Summary

| Area | Status | Key Finding |
|------|--------|-------------|
| PRD Completeness | ✅ Complete | 25 FRs + 8 NFRs, all measurable |
| FR Coverage | ✅ 100% | All FRs covered in E1–E11; 2 deferred by design |
| UX Alignment | ✅ N/A (intended) | CLI-only tool — no UX doc needed |
| Epic Structure | ✅ Sound | 0 critical violations; dependency chain correct |
| Story Quality | ⚠️ 2 Major, 4 Minor | No blockers; improvements recommended |
| NFR Coverage | ✅ 7/8 Phase 1 | NFR1 deferred Phase 2 per ADR-002 (documented) |

---

### Critical Issues Requiring Immediate Action

**None.** No blocking issues found. Implementation may proceed.

---

### Recommended Next Steps (Priority Order)

1. **[M1 — Before E9 sprint planning]** Add explicit AC to Story 9.2: specify the filesystem cleanup protocol when `git worktree add` fails after a successful DB claim. This prevents partial state (claimed issue + orphaned directory) from entering production.

2. **[M2 — Before E10 Story 2 sprint planning]** Upgrade E10 Story 10.1 (Spike) exit criteria: require a CI-runnable executable artifact (e.g., `grava sandbox run --scenario=spike-merge-driver`) in addition to the markdown report. This prevents proceeding on a poorly-validated spike.

3. **[m1 — Quick fix, before E3 implementation]** Add `--since <date>` flag to the Epic 3 Commands Delivered table to match Story 3.3 AC.

4. **[m2 — Quick fix, before E5 implementation]** Replace "all 12 health checks execute" in Story 5.4 AC with a reference to the Epic 5 Doctor Check Set section to avoid brittle count.

5. **[m3 — Before E8 deferral decision]** Add a concrete, measurable deferral trigger for FR-ECS-1c/1d: "Defer if sprint planning retrospective shows zero multi-agent sessions (no concurrent agent `wisp_entries`)." This prevents ambiguous decision-making at sprint planning.

6. **[m4 — Optional refactor]** Consider splitting E11 Story 11.4 into 11.4a (TS-04, E7 gate) and 11.4b (TS-07, E10 gate) for independent tracking.

---

### Final Note

This assessment examined 5 planning artifacts (PRD, Architecture, 11 Epic files, requirements index, and epics index) covering 25 FRs, 8 NFRs, 11 epics, and 45 stories.

**Findings:** 0 Critical · 2 Major · 4 Minor across 6 assessment categories.

The planning artifacts demonstrate a thorough, brownfield-aware decomposition with well-structured acceptance criteria, correct dependency sequencing, and explicit deferral decisions. The team can proceed to Phase 1 sprint planning with confidence.

**Address the 2 Major issues before their respective epics are sprint-planned. All 4 Minor items can be resolved in a quick doc-cleanup pass before the corresponding epic begins.**

---

*Assessment completed: 2026-03-22*
*Assessor: BMAD Product Manager / Scrum Master Agent*
*Report: [_bmad-output/planning-artifacts/implementation-readiness-report-2026-03-22.md](_bmad-output/planning-artifacts/implementation-readiness-report-2026-03-22.md)*

