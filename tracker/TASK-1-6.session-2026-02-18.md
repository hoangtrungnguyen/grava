---
issue: TASK-1-6.subtask-2
status: done
Description: Implemented `grava compact` command — purges ephemeral Wisp issues older than N days and records tombstones in the `deletions` DB table. All acceptance criteria met, 9/9 unit tests pass, live E2E verified.
---

**Timestamp:** 2026-02-18 15:24:00
**Affected Modules:**
  - pkg/cmd/compact.go (new)
  - pkg/cmd/commands_test.go (updated)
  - pkg/cmd/create.go (subtask-1 carry-over, already done)
  - pkg/cmd/list.go (subtask-1 carry-over, already done)
  - docs/CLI_REFERENCE.md (compact section added)
  - tracker/TASK-1-6-EPHEMERAL-WISP-SUPPORT.md (closed)
  - tracker/TASK-1-6.subtask-2.md (closed)

---

## Session Summary

### What was built

**`pkg/cmd/compact.go`** — New `grava compact` command:
- `--days int` flag (default 7) — age threshold
- Queries `issues WHERE ephemeral = 1 AND created_at < NOW() - N days`
- For each match: INSERTs tombstone into `deletions` table (`reason='compact'`, `actor='grava-compact'`), then DELETEs the issue
- Graceful "Nothing to compact" message when result set is empty

**`pkg/cmd/commands_test.go`** — `TestCompactCmd` with 2 subtests:
- `purges old wisps`: 2 Wisps found → tombstones inserted → issues deleted → correct output
- `nothing to compact`: empty result → graceful message

**`docs/CLI_REFERENCE.md`** — New `compact` section with flags, examples, output, and tombstone explanation.

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
ok  github.com/hoangtrungnguyen/grava/pkg/cmd  1.239s
```

### Live E2E Test (against running Dolt instance)
| Command | Result |
|---|---|
| `grava create --title "..." --ephemeral` | `👻 Created ephemeral issue (Wisp): grava-a443` |
| `grava list --wisp` | Shows 2 Wisps |
| `grava compact --days 0` | `🧹 Compacted 2 Wisp(s) older than 0 day(s). Tombstones recorded in deletions table.` |
| `grava list --wisp` (after compact) | Empty — Wisps gone |
| `grava list` (after compact) | Normal issues untouched |
| `deletions` table | 4 tombstone rows confirmed |

### Key Design Decision
- Original spec called for `deletions.jsonl` flat file manifest.
- Implemented as `deletions` SQL table instead — already in schema from TASK-1-2, consistent with Dolt-first architecture, and queryable.

### Git Commit
`f1d9557` — `feat(cli): implement grava compact — Wisp compaction with tombstone tracking`
