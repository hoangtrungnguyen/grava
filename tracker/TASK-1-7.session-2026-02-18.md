---
issue: TASK-1-7-ADVANCED-ISSUE-MANAGEMENT
status: done
Description: Implemented four advanced issue management commands — comment, dep, label, assign. All 17 unit tests pass.
---

**Timestamp:** 2026-02-18 15:28:00
**Affected Modules:**
  - pkg/cmd/comment.go (new)
  - pkg/cmd/dep.go (new)
  - pkg/cmd/label.go (new)
  - pkg/cmd/assign.go (new)
  - pkg/cmd/commands_test.go (updated — 8 new tests)
  - docs/CLI_REFERENCE.md (4 new command sections)
  - tracker/TASK-1-7-ADVANCED-ISSUE-MANAGEMENT.md (closed)

---

## Session Summary

### What was built

**`pkg/cmd/comment.go`** — `grava comment <id> <text>`:
- Fetches current `metadata` JSON from the issue row
- Appends a comment object `{text, timestamp, actor}` to `metadata.comments[]`
- Writes back via `UPDATE issues SET metadata = ?, updated_at = ?`
- Returns error with issue ID if issue not found

**`pkg/cmd/dep.go`** — `grava dep <from_id> <to_id> [--type <type>]`:
- Validates `from_id != to_id` (guard against self-loops)
- Inserts directly into the existing `dependencies` table: `(from_id, to_id, type)`
- Default type: `blocks`; supports any string via `--type` flag
- DB composite PK `(from_id, to_id, type)` naturally prevents duplicate edges

**`pkg/cmd/label.go`** — `grava label <id> <label>`:
- Fetches current `metadata` JSON
- Idempotent: if label already present, prints "already present" and returns without writing
- Otherwise appends label string to `metadata.labels[]` and writes back

**`pkg/cmd/assign.go`** — `grava assign <id> <user>`:
- Single `UPDATE issues SET assignee = ?, updated_at = ? WHERE id = ?`
- Checks `RowsAffected() == 0` → returns "not found" error
- Empty string `""` clears the assignee

### Test Results
```
PASS: TestCreateCmd
PASS: TestShowCmd
PASS: TestCreateEphemeralCmd
PASS: TestListCmd
PASS: TestListWispCmd
PASS: TestCompactCmd/purges_old_wisps
PASS: TestCompactCmd/nothing_to_compact
PASS: TestUpdateCmd
PASS: TestSubtaskCmd
PASS: TestCommentCmd
PASS: TestCommentCmdIssueNotFound
PASS: TestDepCmd
PASS: TestDepCmdCustomType
PASS: TestDepCmdSameID
PASS: TestLabelCmd
PASS: TestLabelCmdIdempotent
PASS: TestAssignCmd
PASS: TestAssignCmdNotFound
ok  github.com/hoangtrungnguyen/grava/pkg/cmd  0.363s
```

### Key Design Decisions
- **`comment` and `label` use `metadata` JSON column** — already in schema, no migration needed. Read-modify-write pattern (fetch → unmarshal → mutate → marshal → update).
- **`dep` uses the `dependencies` table directly** — already in schema with the right columns (`from_id`, `to_id`, `type`). Composite PK prevents duplicates at the DB level.
- **`assign` uses the `assignee` column directly** — already in schema, simplest possible implementation.
- **Error-path tests omit `ExpectClose()`** — Cobra skips `PersistentPostRunE` when `RunE` returns an error, so `Store.Close()` is never called in error scenarios.
