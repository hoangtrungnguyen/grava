# Story 10.4: TS-04 & TS-07 — Export/Import Round-Trip and Conflict Detection

Status: ready-for-dev

## Story

As a developer,
I want the export/import round-trip and conflict detection scenarios to validate NFR4 and the merge driver,
So that zero-loss handoff and conflict isolation are verified end-to-end.

## Acceptance Criteria

1. **AC#1 -- TS-04 Export/Import Round-Trip**
   When I run `grava sandbox run --scenario=TS-04`,
   Then the scenario: creates issues with dependencies → exports to JSONL → imports into fresh state → verifies 100% field and dependency preservation.

2. **AC#2 -- TS-07 Conflict Detection**
   When I run `grava sandbox run --scenario=TS-07`,
   Then the scenario: creates conflicting JSONL edits → runs ProcessMergeWithLWW → verifies ConflictRecords are populated and HasGitConflict is true for equal-timestamp conflicts.

## Tasks / Subtasks

- [ ] Task 1: Implement TS-04 scenario (export/import round-trip)
- [ ] Task 2: Implement TS-07 scenario (conflict detection via merge driver)
- [ ] Task 3: Register scenarios in sandbox registry

## Dev Notes

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-10-sandbox-validation.md#Story-10.4]
