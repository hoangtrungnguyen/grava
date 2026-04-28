# Package: merge

Path: `github.com/hoangtrungnguyen/grava/pkg/merge`

## Purpose

Schema-aware 3-way merge driver for grava's JSONL issue files. Merges
records by `id` and resolves at the field level instead of line by line,
with an optional Last-Write-Wins (LWW) variant driven by `updated_at`.

## Key Types & Functions

- `ProcessMerge(ancestor, current, other) (string, bool, error)` — pure
  3-way merge; returns merged JSONL, a `hasConflict` flag, and parse error.
  Conflicts are encoded inline as `{"_conflict": true, "local": …, "remote":
  …}` markers (field-level) or as a top-level `_conflict` object
  (delete-vs-modify).
- `ProcessMergeWithLWW(ancestor, current, other) (MergeResult, error)` — LWW
  variant. `MergeResult.HasGitConflict` is true only for equal-or-missing
  timestamps; `delete-wins` collisions are recorded but do not block.
- `MergeResult{Merged, ConflictRecords, HasGitConflict}` — LWW output.
- `ConflictEntry{ID, IssueID, Field, Local, Remote, DetectedAt, Resolved}`
  — audit record (`Field == ""` denotes a whole-issue / delete conflict).
- `MarshalSorted(v) ([]byte, error)` — JSON encoding with map keys sorted
  at every level so merges are byte-stable.
- `ExtractConflicts(mergedJSONL, detectedAt) ([]ConflictEntry, error)` —
  recover ConflictEntry records from a `ProcessMerge` output.

## Dependencies

Standard library only (`bytes`, `crypto/sha1`, `encoding/json`, `fmt`,
`reflect`, `sort`, `strings`, `time`).

## How It Fits

Called by the `grava merge-driver` subcommand, which Git invokes via the
configuration written by `pkg/gitconfig` and the assignment in
`pkg/gitattributes`. LWW conflict records feed grava's conflict-resolution
UX. Determinism via `MarshalSorted` ensures merges produce stable git
object hashes across hosts.

## Usage

```go
res, err := merge.ProcessMergeWithLWW(ancestor, current, other)
if err != nil {
    return err
}
os.WriteFile(pathA, []byte(res.Merged), 0644)
if res.HasGitConflict {
    os.Exit(1)
}
```
