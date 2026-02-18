---
issue: TASK-1-6.subtask-1
status: done
Description: Implement ephemeral flag handling for Wisps.
---

## User Story
**As a** system
**I want to** mark issues as ephemeral
**So that** they can be easily cleaned up later

## Acceptance Criteria
- [x] `create` command supports `--ephemeral` flag
- [x] Queries exclude ephemeral items by default

## Session Details — 2026-02-18
### Summary
Implemented ephemeral Wisp flag support across the CLI. The `issues` table already had the `ephemeral tinyint(1) DEFAULT 0` column from a prior schema migration, so this was a pure CLI + test task.

### Changes
- **`pkg/cmd/create.go`**: Added `--ephemeral` bool flag. INSERT now explicitly sets the `ephemeral` column (0 or 1). Output message distinguishes Wisps (`👻 Created ephemeral issue (Wisp): ...`) from normal issues.
- **`pkg/cmd/list.go`**: Default `list` query now includes `WHERE ephemeral = 0` to exclude Wisps. New `--wisp` flag flips this to `WHERE ephemeral = 1`. Also removed dead code from the original WHERE-building attempt.
- **`pkg/cmd/commands_test.go`**: Updated `TestCreateCmd` mock args (+1 arg for `ephemeral=0`). Updated `TestListCmd` query expectation to include `WHERE ephemeral = 0`. Added `TestCreateEphemeralCmd` and `TestListWispCmd`.

### Test Results
All 7 unit tests pass: `TestCreateCmd`, `TestShowCmd`, `TestCreateEphemeralCmd`, `TestListCmd`, `TestListWispCmd`, `TestUpdateCmd`, `TestSubtaskCmd`.
