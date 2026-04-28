# Package: synccmd

Path: `github.com/hoangtrungnguyen/grava/pkg/cmd/sync` (package name: `synccmd`)

## Purpose

Implements `grava commit`, `grava export`, and `grava import`. Defines the
canonical flat JSONL on-disk format (`issues.jsonl`) shared with the merge
driver and git hooks, and is the single call site for `dolt commit`.

## Key Types & Functions

- `AddCommands(root *cobra.Command, d *cmddeps.Deps)` — register commit /
  export / import.
- `IssueJSONLRecord`, `CommentRecord`, `DepRecord`, `WispEntryRecord` —
  flat JSONL line schemas.
- `ImportResult` — `{Imported, Updated, Skipped}`.
- `ExportFlatJSONL(ctx, store, w, includeWisps) (int, error)` — exported
  version of the writer.
- `ImportFlatJSONL(ctx, store, r, overwrite) (ImportResult, error)` —
  transactional upserter (rolls back on any error; FK checks disabled
  inside the tx so deps can reference yet-to-be-imported issues).
- `SyncIssuesFile(ctx, store, path)` — reload `issues.jsonl` after a
  merge or checkout (used by hook handlers).
- `ValidateJSONL(r io.Reader) error` — pre-commit validator that detects
  parse errors, missing IDs, and the legacy wrapped format.

## Dependencies

- `github.com/spf13/cobra`
- `pkg/cmddeps`, `pkg/dolt`, `pkg/errors`
- Standard library only for the JSONL pipeline (`bufio`, `encoding/json`,
  `os/exec`)

## How It Fits

This package is grava's persistence boundary between Dolt and git.
`grava commit` stages and commits the Dolt history; `grava export` writes
the same data as `issues.jsonl` so it can flow through git, where the
custom merge driver merges it record-by-record; `grava import` brings
external `issues.jsonl` data back into Dolt with the FR24 dual-safety
check.

## Usage

```sh
grava commit -m "close ISSUE-42"
grava export                       # writes <git-root>/issues.jsonl
grava export -o /tmp/snapshot.jsonl --include-wisps
grava import issues.jsonl
```
