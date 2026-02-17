---
issue: TASK-1-3-HIERARCHICAL-ID-GENERATOR
status: todo
Description: Generate atomic, hierarchical IDs (e.g., grava-a1b2.1) to break down tasks recursively without ID collisions.
---

**Timestamp:** 2026-02-17 17:55:00
**Affected Modules:**
  - lib/core/
  - .grava/dolt/

---

## User Story
**As a** developer
**I want to** generate atomic, hierarchical IDs (e.g., `grava-a1b2.1`)
**So that** I can break down tasks recursively without ID collisions

## Acceptance Criteria
- [ ] Generator produces `grava-XXXX` (hash-based) for top-level issues
- [ ] Generator supports atomic increment for child issues (`.1`, `.2`) via `child_counters` table
- [ ] IDs are guaranteed unique across distributed environments
- [ ] Generator integrated into issue creation flow
- [ ] Performance: <1ms generation time
- [ ] Unit tests cover collision scenarios and hierarchy depth (parent.child.grandchild)
