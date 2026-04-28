// Package cmd is the root cobra command for the grava CLI and the wiring
// layer that registers every subcommand provided by sibling sub-packages.
//
// This package owns the top-level "grava" command (rootCmd), the global
// PersistentPreRunE/PostRunE hooks (logger init, .grava resolution, schema
// version check, Dolt connection, and command audit logging), and the
// shared package-level state used by command implementations: Store
// (dolt.Store handle), Notifier (notify.Notifier sink), and the cmddeps.Deps
// pointer struct passed to every sub-package's AddCommands.
//
// Several CLI commands live directly in this package — root, init, install,
// hook, conflicts, resolve, orchestrate, db-start/stop, merge-driver,
// merge-slot, sync-status, and version — because they either run before the
// DB is up, mediate hook dispatch, or operate purely on .grava on-disk
// state. Higher-level command groups (issues, graph/dep/ready/blocked,
// maintenance, sync/commit/export/import, reserve, sandbox) are owned by
// sub-packages and registered into rootCmd via Execute -> init() ->
// <subpkg>.AddCommands(rootCmd, deps).
//
// Entry point: main calls cmd.SetVersion(...) then cmd.Execute(). Errors
// are emitted as plain text on stderr by default, or a structured JSON
// envelope when --json (outputJSON) is set.
package cmd
