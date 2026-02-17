issue: TASK-1-3-HIERARCHICAL-ID-GENERATOR
status: done
Description: Generate atomic, hierarchical IDs (e.g., grava-a1b2.1) to break down tasks recursively without ID collisions.
---

**Timestamp:** 2026-02-17 18:50:00
**Affected Modules:**
  - pkg/idgen/
  - pkg/dolt/
  - .grava/dolt/

---

## User Story
**As a** developer
**I want to** generate atomic, hierarchical IDs (e.g., `grava-a1b2.1`)
**So that** I can break down tasks recursively without ID collisions

## Acceptance Criteria
- [x] Generator produces `grava-XXXX` (hash-based) for top-level issues
- [x] Generator supports atomic increment for child issues (`.1`, `.2`) via `child_counters` table
- [x] IDs are guaranteed unique across distributed environments
- [x] Generator integrated into issue creation flow
- [x] Performance: <1ms generation time
- [x] Unit tests cover collision scenarios and hierarchy depth (parent.child.grandchild)

## Session Details
- Implemented `pkg/idgen` with `GenerateBaseID` (SHA-256 hash) and `GenerateChildID` (DB counters).
- Implemented `pkg/dolt` client using Advisory Locks (`GET_LOCK`) on dedicated connections to ensure atomicity in distributed Dolt environment.
- Verified with unit tests and high-concurrency integration tests.
