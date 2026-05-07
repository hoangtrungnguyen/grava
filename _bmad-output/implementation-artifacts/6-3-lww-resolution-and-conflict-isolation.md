# Story 6.3: LWW Resolution and Conflict Isolation

Status: ready-for-dev

## Story

As an agent,
I want the merge driver to automatically resolve field-level conflicts using last-write-wins (LWW),
So that the majority of concurrent edits are merged without human intervention.

## Acceptance Criteria

1. **AC#1 -- LWW Policy (Primary)**
   When a field has diverged between `ours` and `theirs`:
   The version with the newer `updated_at` (from Dolt `NOW()`) MUST be chosen.
   LWW MUST be applied per individual field (e.g. if I change title and you change status, both changes are preserved).

2. **AC#2 -- Delete-Wins Policy**
   If one side has deleted an issue and the other has modified it:
   The deletion MUST win in the merged `issues.jsonl`.
   The discarded modification MUST be recorded in the `conflict_records` table as `resolved_status=pending`.

3. **AC#3 -- Unresolvable Conflicts**
   If two sides modified the same field with the EXACT same `updated_at` timestamp:
   The conflict is categorized as "unresolvable".
   The entry is written to `conflict_records`.
   The field in `%A` MUST be preserved as-is (our version wins in the file, but it's flagged for review).

4. **AC#4 -- Conflict Notifications**
   On any write to `conflict_records`, the driver MUST emit a structured alert via the Notifier:
   `{"code": "MERGE_CONFLICT", "issue_id": "...", "field_name": "...", "message": "Manual review required"}`.

5. **AC#5 -- Atomic File Update**
   After resolving all issues, the merged content MUST be written to `%A` and the command MUST exit 0.

## Tasks / Subtasks

- [ ] Task 1: Implement Merge Brain
  - [ ] 1.1 Implement field-by-field comparison of `Issue` structs.
  - [ ] 1.2 Implement LWW logic using `updated_at` timestamps.

- [ ] Task 2: Implement Conflict Storage
  - [ ] 2.1 Create the `conflict_records` table in `pkg/migrate/`.
  - [ ] 2.2 Implement a `Store.RecordConflict(...)` method to insert unresolvable states.

- [ ] Task 3: Implement Notifier Integration
  - [ ] 3.1 Call `notifier.Alert` from within the merge driver process.

- [ ] Task 4: Integration Testing
  - [ ] 4.1 Mock diverged versions of an issue and verify LWW outcomes.
  - [ ] 4.2 Verify `delete-wins` scenario.

## Dev Notes

### Timestamp Precision
Ensure comparing `time.Time` values uses `Equal()` and `After()` taking into account database precision.

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-06-advanced-merge-driver.md#Story-6.3]
- [Architecture: ADR-H1 (Dolt NOW())]
