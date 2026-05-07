# Package: githooks

Path: `github.com/hoangtrungnguyen/grava/pkg/githooks`

## Purpose

Install grava shim hooks (`pre-commit`, `post-merge`, etc.) into a Git
repository's hooks directory while preserving pre-existing user hooks.

## Key Types & Functions

- `ShimMarker` (`# grava-shim`), `AppendStartMarker` (`# grava-hook-start`),
  `AppendEndMarker` (`# grava-hook-end`) — idempotency markers.
- `InitHookNames` — minimal set used by `AppendStubs` (pre-commit,
  post-merge).
- `HookNames` — broader set used by `DeployAll` (adds post-checkout,
  prepare-commit-msg).
- `AppendResult{Name, Action}` — result per hook for `AppendStubs`. Action
  is `registered`, `appended`, or `skipped`.
- `AppendStubs(hooksDir, hookNames) ([]AppendResult, error)` — append-mode
  install (ADR-H2). Default for `grava init`.
- `DeployResult{Name, Action, Existing}` — result per hook for `DeployAll`.
  Action is `installed`, `updated`, `skipped`, or `preserved-existing`.
- `DeployAll(hooksDir, w) ([]DeployResult, error)` — replace-mode install;
  renames foreign hooks to `<name>.pre-grava`.
- `DefaultHooksDir(repoRoot) string` — `.git/hooks`.
- `SharedHooksDir(repoRoot) string` — `.grava/hooks` (shared mode).

## Dependencies

Standard library only (`os`, `io`, `path/filepath`, `strings`, `fmt`).

## How It Fits

Invoked by `grava install` together with `pkg/gitconfig` (driver registration)
and `pkg/gitattributes` (driver assignment). The deployed shims call back
into the grava CLI with `grava hook run <name> "$@"`, where the full hook
dispatch logic lives.
