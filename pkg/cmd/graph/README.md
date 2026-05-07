# Package: cmdgraph

Path: `github.com/hoangtrungnguyen/grava/pkg/cmd/graph`

## Purpose

CLI commands for managing and querying the issue dependency graph. Provides
edge edit (`dep`), graph analysis (`graph stats|cycle|health|visualize`),
and high-level queries (`ready`, `blocked`, `search`, `stats`).

## Key Types & Functions

- `AddCommands(root *cobra.Command, d *cmddeps.Deps)` — register every
  command in this package onto the root cobra tree.
- `ReadyQueue(ctx, store, limit) ([]*graph.ReadyTask, error)` — exported
  wrapper around the internal ready-queue computation, used by sandbox.
- `IssueListItem`, `BlockerItem`, `StatsResult` — JSON shapes for
  search/blocked/stats output.
- `SearchWisp`, `SearchLabels`, `StatsDays` — exported flag pointers for
  test reset hooks.

## Dependencies

- `github.com/spf13/cobra`
- `pkg/cmddeps` (shared command deps)
- `pkg/dolt` (`Store`, `WithDeadlockRetry`, `WithAuditedTx`, audit events)
- `pkg/graph` (`AdjacencyDAG`, `LoadGraphFromDB`, `ReadyEngine`,
  `GateEvaluator`, `RenderOptions`)
- `pkg/errors` for typed CLI errors

## How It Fits

This package is the cobra-facing surface for grava's graph subsystem. It
loads the live graph from Dolt on each invocation, validates edits with
cycle detection on blocking edges, and writes through audited transactions
(deadlock-retried). All read paths reuse `graph.AdjacencyDAG` so behavior
matches the library used by the resolver, ready engine, and tests.

## Usage

```sh
grava dep ISSUE-1 ISSUE-2 --type blocks
grava graph cycle
grava graph visualize --format dot
grava ready --limit 10
grava blocked ISSUE-3
grava search "auth bug" --label P0
grava stats --days 14 --json
```
