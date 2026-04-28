# Package: reserve

Path: `github.com/hoangtrungnguyen/grava/pkg/cmd/reserve`

## Purpose

Implements advisory file-path leases (`file_reservations` table) used by
multiple agents to coordinate exclusive write access to overlapping paths.
Provides the `grava reserve` command tree and the pre-commit overlap check.

## Key Types & Functions

- `AddCommands(root *cobra.Command, d *cmddeps.Deps)` — register the
  reserve subcommands.
- `Reservation` — JSON view of a `file_reservations` row.
- `DeclareParams`, `DeclareResult` — input/output for declaring a lease.
- `DeclareReservation(ctx, store, p) (DeclareResult, error)` — atomic
  insert with overlap check; returns `FILE_RESERVATION_CONFLICT` on
  contention.
- `Conflict`, `CheckStagedConflicts(ctx, store, paths, actor)` — pre-commit
  hook helper that flags staged paths overlapping with leases held by
  other agents.

## Dependencies

- `github.com/spf13/cobra`
- `pkg/cmddeps`, `pkg/dolt`, `pkg/errors`, `pkg/grava` (path resolution)
- Standard library: `crypto/rand`, `crypto/sha1`, `crypto/sha256`,
  `path/filepath`

## How It Fits

File reservations are advisory but enforced at commit time. `reserve.go`
implements the user-facing CLI (declare/list/release) and the core
DeclareReservation logic; `enforce.go` is consumed by the pre-commit hook
in `pkg/cmd/hook.go` to block or warn on conflicting staged changes. The
maintenance package's `doctor --fix` cleans up expired leases that this
package created.

## Usage

```sh
grava reserve declare "src/auth/**" --reason "auth refactor" --ttl 60
grava reserve list
grava reserve release res-a1b2c3
# Pre-commit hook (automatic): checks staged paths against active leases.
```
