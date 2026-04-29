# Story 2B.0c: `grava update --description-append` Flag

CLI prerequisite for the Phase 2B pipeline. The watcher (story 2B.12) needs to record a "PR Rejection Notes" section onto a grava issue when its PR closes without merge. The note must persist on the issue itself (not just a wisp) so that a human investigating the issue or a coder spawned via `/ship <id> --retry` reads the rejection context as part of the issue's canonical body.

Today there is no way to mutate an issue's description from the CLI. `grava comment` adds chronological comments — useful but separate from description.

## File

`pkg/cmd/update.go` (new or existing — adds one flag)

## Required surface

```
grava update <issue-id> --description-append <text>
grava update <issue-id> --description-append-from-stdin   # alternative for multi-line text
```

Semantics:

- Pure append. No diff, no dedup. Caller is responsible for not appending duplicates (gate with a wisp before calling — see story 2B.12).
- `\n\n` is automatically inserted between existing description and the new text — caller does not need to prefix with newlines.
- Empty existing description → appended text becomes the description (no leading whitespace).
- Stdin variant for multi-line markdown blocks — shell heredocs are awkward to pass as `--description-append "<long string>"` due to quoting.
- `--json` output: `{"updated": true, "description_length": <int>}`

## Why append, not edit

Editing requires either an interactive editor (breaks cron + automation) or a structured patch (overkill for the watcher's append-only use case). The pipeline's only writer is the watcher; it always appends a timestamped section. Append is the minimum viable surface; richer edit can come later as a separate story.

## Why not just `grava comment`

Comments are chronological side-channel notes. Description is the canonical "what this issue is about" text — the surface coders, reviewers, and planners read first. PR rejection context belongs in the canonical text so the next agent that picks up the issue (via `/ship --retry` or a fresh claim) sees the rejection without scrolling comment history.

In practice the watcher will write to **both** — description-append for canonical context, `grava comment` for the chronology — but description-append is the load-bearing call.

## Acceptance Criteria

- `grava update grava-x --description-append "Hello"` on an empty-description issue → description becomes `Hello`
- `grava update grava-x --description-append "World"` on an issue with description `Hello` → description becomes `Hello\n\nWorld`
- `--description-append-from-stdin` reads stdin until EOF and appends as a single block
- Idempotency is the caller's responsibility (no internal dedup) — second identical call appends a duplicate (this is a feature, not a bug, for the watcher's "first CLOSED detection" gating model)
- Empty `--description-append ""` → no-op, exit 0 (don't append a blank section)
- `--json` returns `{"updated": true, "description_length": <int>}` after success
- Help text documents append-only semantics and the `\n\n` separator behavior

## Test Plan

- Empty issue → append "A" → description is `A`
- Append "B" → description is `A\n\nB`
- Append "" → description unchanged
- Stdin variant: `echo -e "line1\nline2" | grava update <id> --description-append-from-stdin` → description ends with those lines as a block

## Pipeline impact

- Story 2B.12 (watcher CLOSED branch): records rejection notes to issue description before flipping `pipeline_phase=failed`
- Story 2B.5 (`/ship --retry`): coder reads issue via `grava show` and gets the rejection context as part of the description, no special wisp lookup needed

## Dependencies

- None (CLI-internal change)

## Out of scope

- Description prepend, replace, structured patch — file separately if needed
- Description history / audit log — out of scope for the pipeline
