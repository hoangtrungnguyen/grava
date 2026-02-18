---
issue: TASK-1-6.subtask-2
status: done
Description: Create compaction logic for clearing old Wisps.
---

## User Story
**As a** system
**I want to** delete old ephemeral issues
**So that** the database stays clean

## Acceptance Criteria
- [x] `grava compact` command implemented
- [x] deletes ephemeral issues older than X days
- [x] Records deletions in `deletions` table

## Session Details — 2026-02-18
### Summary
Implemented the `grava compact` command that purges ephemeral Wisp issues older than a configurable number of days and records each deletion as a tombstone in the `deletions` table (already present in the schema from TASK-1-2).

### Changes
- **`pkg/cmd/compact.go`**: New command. Accepts `--days int` flag (default 7). Queries `issues WHERE ephemeral = 1 AND created_at < cutoff`, then for each match: INSERTs a row into `deletions` (`reason='compact'`, `actor='grava-compact'`), then DELETEs the issue. Reports count of purged Wisps.
- **`pkg/cmd/commands_test.go`**: Added `TestCompactCmd` with two subtests — `purges old wisps` (2 Wisps deleted, tombstones recorded) and `nothing to compact` (empty result set, graceful message).
- **`docs/CLI_REFERENCE.md`**: Added `compact` command section with flags, examples, output, and tombstone explanation.

### Test Results
All 9 unit tests pass: `TestCreateCmd`, `TestShowCmd`, `TestCreateEphemeralCmd`, `TestListCmd`, `TestListWispCmd`, `TestCompactCmd/purges_old_wisps`, `TestCompactCmd/nothing_to_compact`, `TestUpdateCmd`, `TestSubtaskCmd`.
