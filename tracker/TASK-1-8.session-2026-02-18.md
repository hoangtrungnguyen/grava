---
issue: TASK-1-8-SEARCH-AND-MAINTENANCE
status: inProgress
Description: Session implementing grava search, quick, and doctor commands. grava sync planned but not yet implemented — plan archived in docs/archive/grava-sync-plan.md.
---

**Timestamp:** 2026-02-18 15:48:00
**Affected Modules:**
  - pkg/cmd/search.go (new)
  - pkg/cmd/quick.go (new)
  - pkg/cmd/doctor.go (new)
  - pkg/cmd/commands_test.go (10 new tests)
  - docs/CLI_REFERENCE.md (3 new sections: doctor, search, quick)
  - docs/archive/grava-sync-plan.md (new — implementation plan)

---

## Session Details

### What was done

#### 1. `grava search <query>` — `pkg/cmd/search.go`
- LIKE-based full-text search across `title`, `description`, `COALESCE(metadata,'')` columns
- Excludes ephemeral Wisps by default; `--wisp` flag inverts to search only Wisps
- Tabwriter output (same style as `grava list`)
- Prints `🔍 N result(s) for "query"` or `No issues found matching "query"` when empty
- **Tests:** `TestSearchCmd`, `TestSearchCmdNoResults`, `TestSearchCmdWisp`

#### 2. `grava quick` — `pkg/cmd/quick.go`
- Lists open, non-ephemeral issues at or above a priority threshold
- Priority scale: 0=critical, 1=high (default), 2=medium, 3=low, 4=backlog
- Flags: `--priority int` (default 1), `--limit int` (default 20)
- Prints `⚡ N high-priority issue(s) need attention.` or `🎉 You're all caught up!`
- **Tests:** `TestQuickCmd`, `TestQuickCmdAllCaughtUp`, `TestQuickCmdCustomPriority`

#### 3. `grava doctor` — `pkg/cmd/doctor.go`
- Read-only diagnostic command — does NOT modify any data
- 5 checks in order:
  1. DB connectivity (`SELECT VERSION()`)
  2. Required tables: `issues`, `dependencies`, `deletions`, `child_counters` (via `information_schema`)
  3. Orphaned dependency edges (WARN, not FAIL)
  4. Issues with no title (WARN)
  5. Wisp count — WARN if > 100, suggests `grava compact`
- FAIL checks cause non-zero exit; WARN checks do not
- **Tests:** `TestDoctorCmd` with 3 subtests: all-pass, missing-table-fail, warn-path

#### 4. Docs — `docs/CLI_REFERENCE.md`
- Added `### doctor`, `### search`, `### quick` sections (in that order, before Wisps section)
- Each section includes: usage, flags/args, examples, sample output, notes

#### 5. `grava sync` — plan only
- Full plan written to `docs/archive/grava-sync-plan.md`
- Decision: use `os/exec` to shell out to `dolt pull` / `dolt push`
- Bypasses `Store` interface (VCS operation, not SQL)
- `runDolt` function will be injectable for testing without a real `dolt` binary
- Explicitly defers all Epic 3 daemon complexity

### Architecture decisions

- **`doctor` uses `Store.QueryRow` only** — keeps it testable with `sqlmock` without needing a real DB
- **`search` and `quick` reuse the same tabwriter pattern** as `grava list` for visual consistency
- **`sync` bypasses `PersistentPreRunE`** — will be added to the `cmd.Name() == "init"` guard

### Test count
- Before session: 18 tests
- After session: 27 tests (all passing)

### Commit
`fdbe2ef` — feat(cli): implement search, quick, doctor commands (TASK-1-8)

---

## Remaining Work

- [ ] `grava sync` — implement per `docs/archive/grava-sync-plan.md`
  - `pkg/cmd/sync.go`
  - Update `root.go` `PersistentPreRunE` guard
  - 4 unit tests (stub `runDoltFn`)
  - `docs/CLI_REFERENCE.md` sync section
  - Mark TASK-1-8 as `done`
