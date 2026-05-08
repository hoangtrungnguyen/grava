# Package: utils

Path: `github.com/hoangtrungnguyen/grava/pkg/utils`

## Purpose

Cross-cutting helpers used by multiple Grava commands: git worktree
provisioning, redirect-file resolution, schema version checks, dolt binary
location, port allocation, and small environment preflights.

## Key Types & Functions

Worktree lifecycle:
- `IsWorktree`, `FindMainRepo`, `ComputeRedirectPath`,
  `WriteRedirectFile`, `ResolveGravaDirWithRedirect` — ADR-004
  worktree-aware `.grava` resolution.
- `WorktreePath`, `CheckWorktreeConflict`, `ProvisionWorktree`,
  `DeleteWorktree`, `RemoveWorktreeOnly`, `IsWorktreeDirty`,
  `IsInsideClaudeWorktree`.
- `LinkClaudeWorktree`, `SyncClaudeSettings`, `ConfigureGitUser` — wire
  Claude Code's per-worktree state to grava's `.worktree/`.
- `EnsureWorktreeDir`, `EnsureWorktreeGitignore`,
  `SetWorktreeGitConfig`, `EnsureClaudeWorktreeSettings` — `grava init`
  setup helpers.

Preflight & resolution:
- `CheckClaudeInstalled` (skippable via `GRAVA_SKIP_PREFLIGHT=1` or the
  legacy `GRAVA_SKIP_CLAUDE_CHECK=1`).
- `CheckGitVersion`, `ParseAndCheckGitVersion`, `MinGitMajor`,
  `MinGitMinor`.
- `ResolveDoltBinary`, `LocalDoltBinDir`, `LocalDoltBinaryPath`.
- `ResolveGravaDir` (legacy; superseded by
  `ResolveGravaDirWithRedirect`).

Schema versioning:
- `SchemaVersion` constant, `CheckSchemaVersion`, `WriteSchemaVersion`
  (read/write `.grava/schema_version`).

Ports & misc:
- `AllocatePort`, `FindAvailablePort`, `LoadUsedPorts`, `SaveUsedPort`,
  `GetGlobalPortsFile` — per-project port pinning in
  `~/.grava/ports.json`.
- `WriteGitExclude` — manages `.git/info/exclude` and migrates the legacy
  `.gitignore` `.grava/` entry.
- `FindScript` — locates bundled scripts under `scripts/`.

## Dependencies

- Standard library: `os`, `os/exec`, `path/filepath`, `runtime`, `net`,
  `encoding/json`, `strings`, `strconv`, `errors`.
- `github.com/hoangtrungnguyen/grava/pkg/errors` — typed error codes
  (`NOT_INITIALIZED`, `SCHEMA_MISMATCH`, `DB_UNREACHABLE`).

## How It Fits

Almost every grava sub-command (`init`, `claim`, `start`, `stop`,
`merge-driver`, `doctor`) reaches into this package for the OS-side
plumbing it needs. Keeping these helpers here lets commands stay focused on
business logic while sharing audited implementations of file, git, and
process operations.
