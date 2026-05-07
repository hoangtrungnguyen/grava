# Package: maintenance

Path: `github.com/hoangtrungnguyen/grava/pkg/cmd/maintenance`

## Purpose

Housekeeping commands for grava: tombstone old Wisps (`compact`), purge or
soft-delete issues (`clear`), inspect/auto-heal system health (`doctor`),
and read the global command audit trail (`cmd-history`).

## Key Types & Functions

- `AddCommands(root *cobra.Command, d *cmddeps.Deps)` — register all
  maintenance commands.
- `RecordCommand(ctx, store, cmdPath, actor, argsJSON, exitCode)` — append
  a row to `cmd_audit_log`; called from the root `PersistentPostRunE`.
- `ClearStdinReader io.Reader` — overridable input source for the
  interactive confirmation in `grava clear`.
- Internal helpers: `queryExpiredLeases`, `releaseExpiredLeases`,
  `queryGhostWorktrees`, `healGhostWorktrees`, `clearArchivedIssues`.

## Dependencies

- `github.com/spf13/cobra`
- `pkg/cmddeps`, `pkg/dolt` (`AuditEvent`, `WithAuditedTx`),
  `pkg/validation` for date-range parsing
- Filesystem (`os`, `filepath`) to detect ghost `.worktree/<id>` dirs

## How It Fits

These commands maintain the long-running health of the Dolt-backed grava
workspace. `doctor --fix` is the canonical recovery tool that pairs with
the claim/worktree lifecycle owned elsewhere — it resolves the two known
drift modes (expired leases, ghost worktrees) without touching git
history. `cmd-history` complements `grava history` (issue-level events)
by surfacing the global command timeline.

## Usage

```sh
grava compact --days 7
grava clear                                # purge archived issues
grava clear --from 2026-01-01 --to 2026-01-31 --force --include-wisps
grava doctor
grava doctor --fix
grava doctor --dry-run --json
grava cmd-history --limit 50
```
