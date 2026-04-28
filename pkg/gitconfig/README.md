# Package: gitconfig

Path: `github.com/hoangtrungnguyen/grava/pkg/gitconfig`

## Purpose

Register and verify the Grava merge driver in a repository's local
`.git/config`. Idempotent across `grava install` and `grava doctor` runs.

## Key Types & Functions

- `DriverName` (`grava-merge`), `DriverCmd` (`grava merge-driver %O %A %B`),
  `DriverHumanName` — driver constants.
- `DriverConfig{Name, Driver}` — snapshot of the two config values.
- `DefaultDriverConfig() DriverConfig` — the canonical config grava expects.
- `RegisterMergeDriver(cfg, stdout, stderr) (alreadySet bool, err error)` —
  idempotent writer; returns `alreadySet=true` if local config already
  matches.
- `IsRegistered() bool` — checks the local config (not global / system).
- `GetLocal()`, `Get()` — read DriverConfig from local-only or effective chain.
- `Set`, `GetLocalValue`, `GetValue` — low-level git-config wrappers.
- `IsInGitRepo() bool` — `git rev-parse --git-dir` probe.

## Dependencies

Standard library only (`os/exec`, `io`, `strings`, `fmt`). Requires the
`git` binary on PATH.

## How It Fits

Called by `grava install` to register the merge driver and by `grava doctor`
to detect drift. Counterpart to `pkg/gitattributes` (which assigns the driver
to `issues.jsonl`) and `pkg/githooks` (which installs hook shims).

## Usage

```go
if !gitconfig.IsInGitRepo() {
    return errors.New("not a git repo")
}
already, err := gitconfig.RegisterMergeDriver(
    gitconfig.DefaultDriverConfig(), os.Stdout, os.Stderr)
```
