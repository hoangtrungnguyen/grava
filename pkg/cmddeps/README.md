# Package: cmddeps

Path: `github.com/hoangtrungnguyen/grava/pkg/cmddeps`

## Purpose

Defines the shared dependency struct passed from the root `grava` Cobra
command into every subcommand sub-package, plus the canonical JSON error
envelope writer used when commands run with `--json`.

The package exists in isolation to break a circular import between
`pkg/cmd` and its subcommand packages (`pkg/cmd/issues`,
`pkg/cmd/sync`, etc.), all of which need the same dependency bundle.

## Key Types & Functions

- `Deps` — pointer-field struct holding the runtime dependencies that are
  populated in `PersistentPreRunE` after CLI flag parsing:
  - `Store *dolt.Store` — Dolt SQL handle.
  - `Actor *string` — the human or agent id.
  - `AgentModel *string` — model identifier (e.g. `claude-opus-4-7`).
  - `OutputJSON *bool` — whether commands should emit JSON.
  - `Notifier *notify.Notifier` — operator notification channel.
- `GravaError` — JSON envelope `{code, message}` matching the `--json`
  contract.
- `WriteJSONError(w io.Writer, err error) error` — formats any error as
  `{"error": {"code": ..., "message": ...}}`. Resolves codes from
  `*errors.GravaError` when present, else falls back to substring matches
  on common error texts (`not found`, `ALREADY_CLAIMED`, etc.).

## Dependencies

- `github.com/hoangtrungnguyen/grava/pkg/dolt` for the `Store` interface.
- `github.com/hoangtrungnguyen/grava/pkg/notify` for the notifier handle.
- `github.com/hoangtrungnguyen/grava/pkg/errors` for `GravaError` code
  resolution in `WriteJSONError`.

## How It Fits

`Deps` is the single dependency-injection container threaded through every
subcommand. Pointer fields let subcommand `RunE` closures observe the
current values at execution time, since the root command writes them
during `PersistentPreRunE` after Cobra finishes parsing flags.
`WriteJSONError` keeps machine-readable error output consistent across all
commands.

## Usage

```go
deps := &cmddeps.Deps{
    Store: &store, Actor: &actor, AgentModel: &model,
    OutputJSON: &outputJSON, Notifier: &notifier,
}
issues.AddCommands(root, deps)

// Inside a command:
if err := doThing(); err != nil {
    if *deps.OutputJSON {
        return cmddeps.WriteJSONError(cmd.OutOrStderr(), err)
    }
    return err
}
```
