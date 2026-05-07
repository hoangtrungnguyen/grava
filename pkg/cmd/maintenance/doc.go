// Package maintenance implements the housekeeping CLI commands: compact,
// doctor, clear, and cmd-history.
//
// Compact soft-deletes (tombstones) ephemeral Wisp issues older than a
// cutoff and records each in the deletions table. Clear permanently purges
// archived issues, or — when --from/--to are given — soft-deletes any
// issues created in that date window with an interactive yes/no prompt
// (overridable via ClearStdinReader for tests). cmd-history reads the
// cmd_audit_log table populated by the root command's PersistentPostRunE.
//
// Doctor is the system-health dashboard. It runs read-only checks against
// the Dolt schema and data (table existence, orphaned dependencies, untitled
// issues, Wisp count, ghost worktrees, expired file leases). With --fix it
// repairs the two auto-healable findings: it releases expired
// file_reservations rows and resets ghost in_progress claims back to open
// (preserving any grava/<id> branch). --dry-run reports what --fix would
// change without writing.
//
// AddCommands(root, deps) registers all commands into the root cobra tree.
// The package also exposes RecordCommand (called from the root post-run
// hook) so any cobra command path can be appended to cmd_audit_log.
package maintenance
