# Story 2B.0a: `grava list --label` Flag

CLI prerequisite for the Phase 2B pipeline. The watcher (story 2B.12) and `/ship` (story 2B.5) discover awaiting-merge issues via the `pr-created` label — without a label filter on `grava list`, the watcher would have to fetch all issues and filter client-side every cycle (wasteful) or fall back on a wisp-based discovery (more code, more state). A first-class label filter is the smallest viable surface.

## File

`pkg/cmd/list.go` (existing — adds one flag)

## Current state

```
$ grava list --help
Flags:
  -a, --assignee string    Filter by assignee
  -p, --priority int       Filter by priority
      --sort string        Sort by fields
  -s, --status string      Filter by status
  -t, --type string        Filter by type
      --wisp               Show only ephemeral Wisp issues
```

No `--label` flag.

## Required change

Add `-L, --label strings` (repeatable, `cobra.StringSliceVar`). Semantics:

- Single label: `grava list --label pr-created` → issues with the `pr-created` label
- Multiple labels: `grava list --label pr-created --label needs-human` → AND semantics (issues with **both** labels). Match `grava label`'s `--add` repeatable convention.
- Combinable with existing filters: `grava list --status in_progress --label pr-created --json`

## SQL surface

`pkg/store/issue_store.go` (or wherever `ListIssues` lives) gains a `Labels []string` field on its filter struct. Query joins `issue_labels` on each label and requires all to match (HAVING COUNT(DISTINCT label) = N pattern, or repeated INNER JOINs).

## Acceptance Criteria

- `grava list --label foo` returns only issues with label `foo`
- `grava list --label foo --label bar` returns only issues that have **both** labels (AND, not OR)
- `grava list --label nonexistent` returns empty array (no error, JSON `[]`)
- `--label` composes with `--status`, `--type`, `--priority`, `--assignee` (all filters AND together)
- `--json` output is unchanged in shape — only the filter changes
- Idempotent: running the same command twice returns the same result
- Help text documents the flag and the AND semantics

## Test Plan

- Add label `foo` to issue A, `foo` + `bar` to issue B, `bar` to issue C
- `grava list --label foo --json | jq length` → 2 (A + B)
- `grava list --label foo --label bar --json | jq length` → 1 (B only)
- `grava list --status open --label foo` → respects both filters
- Empty result: `grava list --label zzz_nonexistent --json` → `[]`, exit 0

## Why AND (not OR)

The watcher's only consumer-pattern is "issues currently in `pr-created` state" — a single label. Multi-label callers (e.g. "issues that are both `bug` and `needs-human`") want intersection, not union. OR semantics on labels is rare and easily emulated by two calls + jq merge; AND is the harder case to emulate and is what the watcher needs.

## Dependencies

- None (CLI-internal change)

## Out of scope

- `--label-not foo` (negation) — file separately if needed
- OR semantics — opt-out via separate calls

## Pipeline impact

- Story 2B.12 (`pr-merge-watcher.sh`): line 50 `grava list --label pr-created --json` becomes valid
- Story 2B.5 (`/ship`): future re-entry filters by `--label needs-human` if/when needed
- Implementation Order: this story lands BEFORE story 2B.5 / 2B.12 implementations
