# Package: gitattributes

Path: `github.com/hoangtrungnguyen/grava/pkg/gitattributes`

## Purpose

Manage the `.gitattributes` entry that tells Git to use the Grava merge driver
for `issues.jsonl`. Without this entry the registered merge driver in
`.git/config` is never invoked.

## Key Types & Functions

- `MergeAttrLine` — exact line written: `issues.jsonl merge=grava-merge`.
- `AttrFileName` — `.gitattributes`.
- `EnsureMergeAttr(repoRoot string) (added bool, err error)` — idempotent
  writer; creates the file if missing, appends the line on its own line.
- `HasMergeAttr(repoRoot string) (bool, error)` — read-only check used by
  doctor / install verification.
- `RepoRoot() (string, error)` — shells `git rev-parse --show-toplevel`.

## Dependencies

Standard library only (`bufio`, `bytes`, `os`, `os/exec`, `path/filepath`,
`strings`).

## How It Fits

Called from `grava install` alongside `pkg/gitconfig` (driver registration)
and `pkg/githooks` (hook deployment). All three together establish the
git-side integration that makes JSONL issue merges schema-aware.

## Usage

```go
root, err := gitattributes.RepoRoot()
if err != nil {
    return err
}
added, err := gitattributes.EnsureMergeAttr(root)
if err != nil {
    return err
}
if added {
    fmt.Println("registered grava-merge in .gitattributes")
}
```
