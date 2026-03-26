# Epic 11: Sandbox Validation — Integration Gate

**Status:** Planned
**Grava ID:** grava-9507
**Matrix Score:** 3.55 *(C=2 by design — gate epic, not standalone feature)*
**FRs covered:** *(cross-cutting — validates all FRs under concurrent, fault-injected conditions)*

## Goal

After each epic is implemented and merged, a dedicated sandbox validation run confirms the full system behaves correctly under realistic swarm conditions. Scenarios from `sandbox/SANDBOX_VALIDATION_GUIDE.md` are executed progressively — each epic adds new scenarios to the running suite, and the suite must pass before the next epic begins.

## ⚠️ Enforcement Requirement

To prevent this epic from becoming **checkbox theater**, every Epic 11 story must produce an **executable artifact** — a `grava sandbox run --scenario=TS-XX` command that CI can invoke.

- Stories producing only documentation checklists are **rejected**
- **E11-Story-1 must be `grava sandbox run` CLI** (prerequisite) before any scenario stories are written
- Each scenario story gates on its corresponding feature epic being merged and CI passing

## Story Structure

### E11-Story-1: `grava sandbox run` CLI (Prerequisite)

Before any scenario stories, deliver the sandbox runner:
- `grava sandbox run [--scenario=TS-XX] [--all] [--epic=N]`
- Exits 0 on pass, non-zero on fail
- Structured JSON report output: `{"scenario": "TS-01", "status": "pass|fail", "duration_ms": 123, "details": [...]}`
- CI-invocable: single command, no interactive prompts

### E11-Story-2..N: Scenario Stories (Progressive)

Each story adds one or more scenarios to the executable suite. Stories are gated by their corresponding feature epic:

| Story | Scenario(s) | Feature Epic Gate |
|-------|-------------|-------------------|
| E11-S2 | TS-01: Basic issue lifecycle (happy path) | Epic 2 |
| E11-S3 | TS-02: Concurrent atomic claim | Epic 3 |
| E11-S4 | TS-03: Dependency graph traversal under load | Epic 4 |
| E11-S5 | TS-04: Export/import round-trip (NFR4) | Epic 7 |
| E11-S6 | TS-05: Doctor detection + fix | Epic 5 |
| E11-S7 | TS-06: Install script on clean VM | Epic 6 |
| E11-S8 | TS-07: Conflict detection (delete-vs-modify) | Epic 10 |
| E11-S9 | TS-08: File reservation enforcement | Epic 8 |
| E11-S10 | TS-09: Worktree agent crash recovery | Epic 9 |
| E11-S11 | TS-10: Large file + rapid swarm claims | Epic 9 |
| E11-S12 | Phase 2 gate scenarios (8 scenarios from SANDBOX_VALIDATION_GUIDE.md) | All epics |

## References

- `sandbox/SANDBOX_VALIDATION_GUIDE.md` — Phase 2 release-gate scenarios (8 scenarios)
- `_bmad-output/sandbox-testing/test-plan.md` — Phase 1 scenarios TS-01 through TS-10
- `_bmad-output/sandbox-testing/requirements.md` — SBX-ENV, SBX-FR, SBX-NFR, SBX-CHAOS requirements

## NFR Ownership

| NFR | Role |
|-----|------|
| NFR4 (zero-loss handoff) | *Validated* — TS-04 (export/import round-trip) and TS-07 (conflict scenario, Epic 10) are NFR4 acceptance tests |
| NFR7 (install speed) | *Validated* — TS-06 (install script on clean VM, timed) |

## Dependencies

- Epic 10 (Advanced Merge Driver) must ship before TS-07 (conflict detection scenario) can run
- Each story gates on the corresponding feature epic being merged
- E11-Story-1 (`grava sandbox run` CLI) gates all scenario stories

## Note on Matrix Score

C=2 (Standalone Deliverability) is **by design** — this is a gate epic, not a standalone feature. It validates other epics rather than delivering user-facing functionality. The low C score is expected and accepted.

## Stories

### Story 11.1: `grava sandbox run` CLI (Prerequisite — blocks all scenario stories) *(grava-445d)*

As a developer or CI system,
I want a single command that executes named sandbox scenarios and reports pass/fail,
So that every Epic 11 scenario story produces a CI-invocable executable artifact rather than a documentation checklist.

**Acceptance Criteria:**

**Given** Grava is installed in a sandbox environment
**When** I run `grava sandbox run --scenario=TS-01`
**Then** the specified scenario executes end-to-end and exits 0 on pass, non-zero on fail
**And** `grava sandbox run --all` runs all registered scenarios in sequence; exits non-zero if any scenario fails
**And** `grava sandbox run --epic=2` runs only scenarios gated by Epic 2
**And** structured JSON report is printed: `{"scenario": "TS-01", "status": "pass|fail", "duration_ms": 123, "details": [{"step": "...", "result": "pass|fail", "message": "..."}]}`
**And** `grava sandbox run` requires no interactive prompts — fully CI-invocable with a single command
**And** attempting to run a scenario whose feature epic is not yet merged returns `{"error": {"code": "SCENARIO_GATE_NOT_MET", "message": "TS-07 requires Epic 10 (Advanced Merge Driver) to be merged first"}}`

---

### Story 11.2: TS-01 — Basic Issue Lifecycle (Epic 2 Gate) *(grava-fc24)*

As a developer,
I want the basic issue lifecycle scenario to execute automatically in the sandbox,
So that Epic 2 delivery is validated end-to-end under realistic conditions.

**Acceptance Criteria:**

**Given** Epic 2 is merged and `grava sandbox run` CLI is available (Story 11.1)
**When** I run `grava sandbox run --scenario=TS-01`
**Then** the scenario executes: create issue → update status → add label → add comment → drop issue — all via CLI
**And** each step produces the expected structured JSON output
**And** the scenario exits 0 (pass); `grava list` confirms the issue is archived
**And** scenario completes in <30 seconds in a clean sandbox environment

---

### Story 11.3: TS-02 — Concurrent Atomic Claim (Epic 3 Gate) *(grava-a66d)*

As a developer,
I want the concurrent claim scenario to prove NFR3 atomicity under realistic conditions,
So that Epic 3's guarantee is validated beyond unit tests.

**Acceptance Criteria:**

**Given** Epic 3 is merged
**When** I run `grava sandbox run --scenario=TS-02`
**Then** the scenario launches two concurrent `grava claim` processes targeting the same issue
**And** exactly one claim succeeds and one receives `CLAIM_CONFLICT` — no polluted row, no deadlock
**And** the scenario is repeated 10 times; NFR3 holds on every iteration
**And** scenario exits 0 (pass) with result: `{"iterations": 10, "conflicts_correctly_rejected": 10, "deadlocks": 0}`

---

### Story 11.4: TS-04 & TS-07 — Export/Import Round-Trip and Conflict Detection (Epic 7 + Epic 10 Gate) *(grava-0a16)*

As a developer,
I want the export/import round-trip and conflict detection scenarios to validate NFR4 and the merge driver,
So that zero-loss handoff and conflict isolation are verified end-to-end.

**Acceptance Criteria:**

**Given** Epics 7 and 10 are merged
**When** I run `grava sandbox run --scenario=TS-04`
**Then** the scenario: export issues → clone workspace → import → verify 100% field and dependency preservation
**And** NFR4 is satisfied: `{"dependency_links_preserved": 100, "field_loss": 0}`
**And** `grava sandbox run --scenario=TS-07` executes: create conflicting edits on two branches → merge → verify `conflict_records` is populated and Notifier alert is emitted
**And** both scenarios exit 0 (pass)

---

### Story 11.5: TS-03, TS-05, TS-06, TS-08, TS-09, TS-10 — Remaining Phase 1 Scenarios *(grava-c395)*

As a developer,
I want all remaining Phase 1 sandbox scenarios registered and passing,
So that the full Phase 1 scenario suite gates on all feature epics before Phase 2 begins.

**Acceptance Criteria:**

**Given** all feature epics (E2–E10) are merged
**When** I run `grava sandbox run --all`
**Then** all 10 Phase 1 scenarios execute and pass:
- TS-03: Dependency graph traversal under load (Epic 4 gate)
- TS-05: Doctor detection + fix (Epic 5 gate)
- TS-06: Install script on clean VM, timed — completes in <5 minutes (NFR7 gate, Epic 6 gate)
- TS-08: File reservation enforcement — exclusive lease blocks concurrent commit (Epic 8 gate)
- TS-09: Worktree agent crash recovery — stopped agent's Wisp resumed by new agent (Epic 9 gate)
- TS-10: Large file + rapid swarm claims — 10 agents claiming simultaneously (Epic 9 gate, NFR3 extended)
**And** `grava sandbox run --all` exits 0 with `{"scenarios_passed": 10, "scenarios_failed": 0}`
**And** this story constitutes the **Phase 1 completion gate** — all scenarios passing is the prerequisite for Phase 2 sprint planning
