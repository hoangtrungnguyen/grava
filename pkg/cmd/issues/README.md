# Package: issues

Path: `github.com/hoangtrungnguyen/grava/pkg/cmd/issues`

## Purpose

Implements the issue management subcommands of the `grava` CLI: create,
show, list, update, drop, assign, label, comment, subtask, quick, claim,
close, start, stop, wisp, history, and undo. Each subcommand is a Cobra
command wired up against a shared `cmddeps.Deps` for runtime services.

## Key Types & Functions

- `AddCommands(root *cobra.Command, d *cmddeps.Deps)` — registers every
  issue command on the supplied root.
- `IssueListItem`, `IssueDetail`, `CommentEntry` — JSON output models for
  list/show outputs.
- `*Params` / `*Result` structs (e.g. `AssignParams`, `LabelParams`,
  `SubtaskParams`, `CommentParams`) — typed inputs/outputs used by the
  pure business-logic helpers behind each command.
- `ParseSortFlag(s string) (string, error)` and `SortColumnMap` — translate
  user-facing `--sort` flags into safe SQL ORDER BY clauses.
- `CreateAffectedFiles`, `UpdateAffectedFiles`, `SubtaskAffectedFiles`,
  `LabelAddFlags`, `LabelRemoveFlags`, `StdinReader` — package-level Cobra
  flag targets and overridable I/O hooks for tests.
- `guardNotArchived(store, id)` — internal precondition check shared by
  mutating commands.

## Dependencies

- `github.com/spf13/cobra` for command wiring.
- `github.com/hoangtrungnguyen/grava/pkg/cmddeps` for shared dependencies
  (Store, Actor, AgentModel, OutputJSON, Notifier).
- `github.com/hoangtrungnguyen/grava/pkg/dolt` for SQL access, audit-log
  events, and `WithAuditedTx`.
- `github.com/hoangtrungnguyen/grava/pkg/errors` for structured
  `GravaError` returns.
- `github.com/hoangtrungnguyen/grava/pkg/graph` for the tree visualisation
  used by `grava show --tree`.

## How It Fits

This is the user-facing surface of Grava's issue tracker. Commands here
write to the Dolt-backed `issues`, `dependencies`, `issue_labels`,
`issue_comments`, and `events` tables and emit audit events that drive
`grava history`, undo, and the agent claim/release lifecycle.

## Usage

Wire into the root command from `pkg/cmd`:

```go
root := &cobra.Command{Use: "grava"}
deps := &cmddeps.Deps{ /* populated in PersistentPreRunE */ }
issues.AddCommands(root, deps)
```
