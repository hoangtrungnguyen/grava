# Story 10.5: TS-03, TS-05, TS-08, TS-09, TS-10 — Remaining Phase 1 Scenarios

Status: ready-for-dev

## Story

As a developer,
I want all remaining Phase 1 sandbox scenarios registered and passing,
So that the full Phase 1 scenario suite gates on all feature epics before Phase 2 begins.

## Acceptance Criteria

1. **AC#1 -- TS-03 Dependency Graph Under Load**
   Validates dependency graph traversal under concurrent load (Epic 4 gate).

2. **AC#2 -- TS-05 Doctor Detection + Fix**
   Validates grava doctor detects and fixes issues (Epic 9 gate).

3. **AC#3 -- TS-08 File Reservation Enforcement**
   Validates pre-commit blocks on reserved paths (Epic 8 gate).

4. **AC#4 -- TS-09 Worktree Agent Crash Recovery**
   Validates worktree cleanup after agent crash (Epic 5 gate).

5. **AC#5 -- TS-10 Large File + Rapid Swarm Claims**
   Validates claim atomicity under high contention with many issues (Epic 5 gate).

## Dev Notes

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-10-sandbox-validation.md#Story-10.5]
