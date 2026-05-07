# Package: cmd

Path: `github.com/hoangtrungnguyen/grava/pkg/cmd`

## Purpose

Root cobra command and CLI wiring layer. Owns the `grava` top-level command,
global pre/post-run hooks (logging, `.grava` resolution, schema check, Dolt
connect, audit logging), and registers every command group exposed by sibling
sub-packages.

## Key Types & Functions

- `Execute()` — main entry point invoked from `main.main`; runs cobra and
  formats top-level errors (plain or JSON via `--json`).
- `SetVersion(v string)` — sets the CLI version string.
- `Store dolt.Store` — package-level Dolt connection handle (set in
  `PersistentPreRunE`, closed in `PersistentPostRunE`).
- `Notifier notify.Notifier` — package-level alert sink; tests override.
- `Version string` — current CLI version.
- Direct commands defined here: `init`, `install`, `hook`, `conflicts`,
  `resolve`, `orchestrate`, `db-start`/`db-stop`, `merge-driver`,
  `merge-slot`, `sync-status`, `version`.

## Dependencies

- `github.com/spf13/cobra`, `github.com/spf13/viper`
- `pkg/cmd/graph` (cmdgraph), `pkg/cmd/issues`, `pkg/cmd/maintenance`,
  `pkg/cmd/reserve`, `pkg/cmd/sandbox`, `pkg/cmd/sync` (synccmd)
- `pkg/cmddeps`, `pkg/dolt`, `pkg/errors`, `pkg/grava`, `pkg/log`,
  `pkg/notify`, `pkg/utils`

## How It Fits

`pkg/cmd` is the composition root for the CLI. `main` calls `cmd.Execute()`,
which fires `PersistentPreRunE` to bring up the Dolt-backed store, then
delegates to subcommands. Sub-packages stay decoupled from cobra wiring by
exposing only an `AddCommands(root, deps)` constructor that this package
calls inside `init()`.

## Usage

```sh
grava --help
grava --json issue list
grava init
grava sync-status
```
