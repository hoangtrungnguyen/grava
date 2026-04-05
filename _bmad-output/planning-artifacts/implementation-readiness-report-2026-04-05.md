---
stepsCompleted:
  - step-01-document-discovery
  - step-02-prd-analysis
  - step-03-epic-coverage-validation
  - step-04-ux-alignment
  - step-05-epic-quality-review
  - step-06-final-assessment
documentsAssessed:
  prd: _bmad-output/planning-artifacts/prd.md
  architecture: _bmad-output/planning-artifacts/architecture.md
  epics-anchor: _bmad-output/planning-artifacts/epics.md
  epics-detail: _bmad-output/planning-artifacts/epics/
  ux: NOT_FOUND
scope: story-3.1
---

# Implementation Readiness Assessment Report

**Date:** 2026-04-05
**Project:** grava
**Scope:** Story 3.1 — Atomic Issue Claim

## Document Inventory

| Document | Location | Status |
|----------|----------|--------|
| PRD | `_bmad-output/planning-artifacts/prd.md` | Found |
| Architecture | `_bmad-output/planning-artifacts/architecture.md` | Found |
| Epics (anchor) | `_bmad-output/planning-artifacts/epics.md` | Found |
| Epics (detail) | `_bmad-output/planning-artifacts/epics/` (11 epics + requirements) | Found |
| UX Design | — | Not found (specs embedded in PRD/Architecture) |

## PRD Analysis (Scoped to Story 3.1)

### Relevant Functional Requirements

**FR5:** System Agent can execute an atomic claim on an issue, verifying it is unassigned and immediately locking it to their actor ID (`claim`).

**FR3 (partial):** System Agent / Human Developer can update core fields like status, priority, and assignees (`update`, `assign`).

**FR4 (partial):** System Agent / Human Developer can explicitly track when they start or stop working on a specific issue (`start`, `stop`).

### Relevant Non-Functional Requirements

**NFR3 (Atomic Execution):** The system must guarantee that concurrent write attempts by multiple local agents (e.g., two agents executing `grava claim` simultaneously) result in exactly one successful claim and one deterministic rejection, never resulting in a polluted row or deadlock.

**NFR2 (Write Throughput — partial):** Standard issue creation and updates (`create`, `update`, `claim`) must commit to the local Dolt instance in < 15 milliseconds per operation.

**NFR5 (Machine Readability — partial):** Strict adherence to predefined JSON schemas for all `--json` command outputs.

### Relevant User Journeys

**Journey 1 (DevBot Handoff):** Step 3 — DevBot executes `grava claim` to atomically lock the issue to its Actor ID.

**Journey 3 (Contested Issue):** Both DevBot and RefactorBot query `grava ready` simultaneously and see the same issue. DevBot claims first; RefactorBot is rejected deterministically.

### PRD Completeness Assessment

PRD is thorough. FR5, NFR3, and Journey 3 provide clear, unambiguous requirements for the atomic claim feature. The scope is well-defined for Story 3.1.

## Epic Coverage Validation (Scoped to Story 3.1)

### FR Coverage Matrix

| FR | PRD Requirement | Epic Coverage | Status |
|----|----------------|---------------|--------|
| FR5 | Atomic claim — verify unassigned, lock to actor ID | Epic 3, Story 3.1 (grava-e4b2) | Covered |
| NFR3 | Concurrent claim → exactly one success, one rejection | Epic 3, Story 3.1 (NFR Ownership) | Covered |
| NFR2 | Claim commits in <15ms | Epic 1 (WithAuditedTx baseline), validated Epic 3 | Covered |

### Missing Requirements

No missing FRs for Story 3.1 scope. FR5 is fully mapped to Epic 3 Story 3.1 with detailed acceptance criteria.

### Coverage Statistics

- Relevant PRD FRs for Story 3.1: 1 (FR5)
- FRs covered in epics: 1
- Coverage percentage: **100%**

## UX Alignment Assessment

### UX Document Status

Not Found — No standalone UX document exists.

### Assessment

Grava is a **CLI tool** with no web/mobile UI. UX concerns are addressed through:
- CLI command syntax and output formatting (defined in PRD + Architecture)
- JSON schema for `--json` output (NFR5)
- JSON Error Envelope for structured errors
- Console output via `ConsoleNotifier` (ADR-N1)

For Story 3.1 specifically:
- `grava claim` CLI interface is well-defined in the epic acceptance criteria
- Success JSON: `{"id": "...", "status": "in_progress", "assignee": "agent-01"}`
- Error JSON: `{"error": {"code": "CLAIM_CONFLICT", "message": "Issue already claimed by agent-01"}}`

### Warnings

None. No separate UX document is required for this project type.

## Epic Quality Review (Story 3.1)

### Epic Structure Validation

- **User Value Focus:** Epic 3 "Atomic Work Claiming & Ephemeral State" delivers clear user value — agents can atomically claim and track work. PASS.
- **Epic Independence:** Epic 3 depends only on Epic 1 (WithAuditedTx, WithDeadlockRetry). No forward dependency on Epics 4+. PASS.
- **Story Sizing:** Story 3.1 is appropriately scoped — single command (`claim`) with clear boundaries. PASS.

### Acceptance Criteria Review (Story 3.1)

| AC | Testable | Complete | Specific | Status |
|----|----------|----------|----------|--------|
| Happy path: claim unassigned issue | Yes | Yes | Yes (exact SQL, JSON output) | PASS |
| Concurrent claim: exactly one succeeds | Yes | Yes | Yes (CLAIM_CONFLICT error code) | PASS |
| Non-concurrent re-claim: same error | Yes | Yes | Yes | PASS |
| <15ms performance (NFR2) | Yes | Yes | Yes | PASS |
| JSON output schema | Yes | Yes | Yes (exact structure given) | PASS |

### Dependency Verification

| Dependency | Required By | Exists in Codebase | Status |
|------------|-------------|-------------------|--------|
| `WithAuditedTx` (pkg/dolt/tx.go) | Claim atomicity | YES — fully implemented | PASS |
| `WithDeadlockRetry` (pkg/dolt/retry.go) | Concurrent claim retry | YES — fully implemented | PASS |
| `GravaError` type (pkg/errors/) | Structured errors | YES — Code, Message, Cause fields | PASS |
| `Store` interface with `BeginTx`, `LogEventTx` | Transaction + audit | YES — fully implemented | PASS |
| `issues` table with `assignee`, `status` columns | Data model | YES — migration 001 | PASS |
| `SELECT FOR UPDATE` pattern | Row locking | Documented in Architecture | PASS |

### Codebase State Discovery

**CRITICAL FINDING:** The `claim` command implementation **already exists** in `pkg/cmd/issues/claim.go` with:
- `claimIssue()` function using `WithAuditedTx` correctly
- `SELECT status FROM issues WHERE id = ? FOR UPDATE` pattern
- `ALREADY_CLAIMED` and `INVALID_STATUS_TRANSITION` error codes
- `ClaimResult` JSON struct with `id`, `status`, `actor` fields
- Tests in `claim_test.go` covering: happy path, not found, already claimed, invalid transition

### Discrepancies Between AC and Implementation

| Epic AC | Implementation | Discrepancy |
|---------|---------------|-------------|
| Error code `CLAIM_CONFLICT` | Uses `ALREADY_CLAIMED` | Minor naming difference |
| `assignee` field in JSON output | Uses `actor` field name (`"actor"`) | JSON key mismatch — AC says `"assignee"`, code says `"actor"` |
| `--actor` flag usage | Uses `*d.Actor` from Deps | Implicit via Deps — matches architecture |
| `SELECT FOR UPDATE` on issues row | SELECT status only (not full row) | Acceptable — locks row regardless of columns selected |

### Best Practices Compliance Checklist

- [x] Epic delivers user value
- [x] Epic independent (only Epic 1 dependency)
- [x] Story appropriately sized
- [x] No forward dependencies
- [x] Database tables already exist (created in Epic 1)
- [x] Clear acceptance criteria (Given/When/Then format)
- [x] Traceability to FR5 maintained

### Quality Issues

**🟡 Minor Concerns:**
1. **Error code naming:** AC specifies `CLAIM_CONFLICT`, implementation uses `ALREADY_CLAIMED`. Either works semantically, but alignment with the AC would reduce ambiguity.
2. **JSON field name:** AC specifies `"assignee": "agent-01"`, implementation returns `"actor": "agent-01"`. This should be aligned for NFR5 schema compliance.

**No critical or major issues found.**

## Summary and Recommendations

### Overall Readiness Status

## **READY** — with minor fixes recommended

Story 3.1 is **already implemented** in the codebase. The core `grava claim` command is complete with proper atomic transaction handling, row-level locking, and test coverage. No new implementation is needed — only two minor alignment fixes.

### Critical Issues Requiring Immediate Action

**None.** No critical or major issues found.

### Recommended Next Steps

1. **Align JSON field name:** Change `ClaimResult.Actor` json tag from `"actor"` to `"assignee"` in `pkg/cmd/issues/claim.go:20` to match the epic AC specification (`{"assignee": "agent-01"}`) and ensure NFR5 schema compliance.

2. **Align error code:** Decide whether to rename `ALREADY_CLAIMED` to `CLAIM_CONFLICT` (per AC) or update the AC to match the implementation. The implementation's `ALREADY_CLAIMED` is semantically clear; updating the AC may be lower risk.

3. **Add concurrent claim integration test:** The unit tests cover single-agent scenarios well. Consider adding a concurrent integration test (two goroutines claiming the same issue simultaneously) to validate NFR3 against a real Dolt instance.

4. **Mark Story 3.1 as done** in `sprint-status.yaml` after fixes are applied.

### Final Note

This assessment identified **2 minor issues** (error code naming, JSON field name) across 6 validation categories. The core implementation is sound, well-tested, and follows architecture patterns correctly. The claim command was likely implemented as part of Epic 2's development work. Address the minor alignment items and proceed to Story 3.2 (Wisp Ephemeral State).
