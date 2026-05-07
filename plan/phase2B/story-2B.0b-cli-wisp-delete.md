# Story 2B.0b: `grava wisp delete` Subcommand

CLI prerequisite for the Phase 2B pipeline. Two callers need to clear wisp keys:

1. `/ship` Phase 4 resume (story 2B.5) — clears `pr_new_comments` after the coder resolves PR feedback. Without delete, the watcher would re-flag the same comments next cycle (since `pr_last_seen_comment_id` advances but the wisp key still holds the stale JSON, and the re-entry guard reads "non-empty `pr_new_comments`" as a signal to resume).
2. `scripts/run-pending-hunts.sh` (story 2B.15) — historically planned to drain a `_global pending_hunt` wisp; that approach has been retired in favour of the file-based queue `.grava/pending-hunts.txt` (the grava CLI rejects non-issue wisp namespaces). The hunt drain therefore no longer calls `wisp delete`. This caller is preserved here only as historical context — the load-bearing caller is now caller (1).

Today `grava wisp` only supports `read` and `write`. There is no way to remove a key — `wisp write k ""` writes an empty string, which is *not* the same as absent (the `[ -n "$X" ]` guards in `/ship` would still treat it as set).

## File

`pkg/cmd/wisp.go` (existing — adds one subcommand)

## Current state

```
$ grava wisp --help
Available Commands:
  read        Read wisp entries for an issue
  write       Write a key-value pair to an issue's wisp store
```

## Required change

Add `delete` subcommand. Surface:

```
grava wisp delete <issue-id> <key>
```

- Removes the row from the wisp store
- Idempotent: deleting a non-existent key is a graceful no-op (exit 0, no error message)
- `--json` global flag prints `{"deleted": true}` or `{"deleted": false, "reason": "key not found"}` — match the existing wisp command convention

Aliases: accept `rm` and `del` as aliases (dev ergonomics; matches `git branch -D` muscle memory).

## SQL surface

`pkg/store/wisp_store.go` (or wherever `WriteWisp` lives) gains a `DeleteWisp(issueID, key string) (bool, error)` method. Returns `(true, nil)` on row deleted, `(false, nil)` on not-found.

## Acceptance Criteria

- `grava wisp write grava-x foo bar` then `grava wisp delete grava-x foo` then `grava wisp read grava-x foo` → empty output, exit 0
- `grava wisp delete grava-x nonexistent` → exit 0, no error (idempotent)
- `grava wisp delete grava-x foo --json` → `{"deleted": true}` after a successful delete; `{"deleted": false, "reason": "key not found"}` after the key is already gone
- `grava wisp delete` (no args) → usage error, exit 2
- `grava wisp delete grava-nonexistent foo` → exit 0, no error (treats missing issue as "key not found")
- Aliases `grava wisp rm` and `grava wisp del` work identically
- Help text shows the alias list

## Test Plan

- Round-trip: write → delete → read → assert empty
- Double-delete: delete twice → second is graceful no-op
- Non-issue: delete on nonexistent issue id → graceful no-op (no panic, no creation of phantom issue)
- JSON format: `--json` matches the documented shape

## Why "graceful no-op" on missing key

Both pipeline callers (`/ship` Phase 4 clear, hunt drain) call delete unconditionally without first checking existence — that's the entire point of using delete instead of `[ -n "$X" ] && wisp_write empty`. Erroring on missing keys would force every caller to add an existence check, defeating the simplification.

## Dependencies

- None (CLI-internal change)

## Out of scope

- Bulk delete (`--all`, prefix match) — file separately if needed
- Audit log of deletes — wisps are by definition ephemeral; deletes don't need history

## Pipeline impact

- Story 2B.5 (`/ship` Phase 4 resume): `grava wisp delete "$ISSUE_ID" pr_new_comments` becomes valid
- Implementation Order: this story lands BEFORE story 2B.5

## Wisp namespace policy

Wisps are scoped to a real grava issue id. The CLI MUST reject sentinel / synthetic ids like `_global` or `grava-global`. Pipeline-wide queues (e.g. pending hunts) use file-based storage under `.grava/` instead — see story 2B.15. This story therefore does not need a "graceful no-op on sentinel id" carve-out: callers that pass a sentinel are buggy and should fail loudly.
