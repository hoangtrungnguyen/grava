# Package: sandbox

Path: `github.com/hoangtrungnguyen/grava/pkg/cmd/sandbox`

## Purpose

Integration validation scenarios runnable from the CLI. Each scenario
asserts a specific concurrency, correctness, or recovery property of grava
against a real Dolt store and worktree layout, complementing unit tests
with end-to-end coverage.

## Key Types & Functions

- `AddCommands(root *cobra.Command, d *cmddeps.Deps)` — register the
  `sandbox` and `sandbox run` commands.
- `Scenario` — `{ID, Name, EpicGate, Run}` describing one runnable check.
- `Result` — JSON-serialisable outcome (`pass`/`fail`, durationMs,
  details, error).
- `Register(s Scenario)` — add a scenario to the global registry.
- `All() []Scenario`, `Find(id string) (*Scenario, bool)`,
  `Run(ctx, store, s) Result` — registry helpers.

## Dependencies

- `github.com/spf13/cobra`
- `pkg/cmddeps`, `pkg/dolt`
- Sibling CLI packages (`pkg/cmd/graph`, `pkg/cmd/sync`, `pkg/cmd/reserve`)
  for exported helpers used by the scenarios

## How It Fits

Sandbox is grava's living acceptance suite. Scenarios are gated by
`EpicGate` so a partially-complete epic only runs the checks meaningful
at that stage. The TS-01..TS-10 scenarios live in this package as
individual files and self-register via `init`. The package depends on
the same exported APIs that real users invoke, so passing scenarios
reflect production behavior.

## Usage

```sh
grava sandbox run --scenario=TS-01
grava sandbox run --epic=3
grava sandbox run --all --json
```
