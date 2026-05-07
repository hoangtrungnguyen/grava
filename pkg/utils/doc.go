// Package utils collects cross-cutting helpers shared by Grava commands.
//
// The package is intentionally a flat grab-bag of small, well-tested
// primitives rather than a single coherent abstraction. Major groupings:
//
//   - Worktree management — IsWorktree, FindMainRepo, ProvisionWorktree,
//     DeleteWorktree, RemoveWorktreeOnly, WorktreePath, IsWorktreeDirty,
//     LinkClaudeWorktree, SyncClaudeSettings, ConfigureGitUser, plus the
//     ADR-004 redirect chain (WriteRedirectFile, ResolveGravaDirWithRedirect)
//     and the init-time helpers EnsureWorktreeDir,
//     EnsureWorktreeGitignore, SetWorktreeGitConfig,
//     EnsureClaudeWorktreeSettings.
//   - Git/Claude tooling preflight — CheckClaudeInstalled, CheckGitVersion,
//     ParseAndCheckGitVersion (minimum git 2.17 for worktree remove).
//   - Dolt binary resolution — ResolveDoltBinary, LocalDoltBinDir,
//     LocalDoltBinaryPath (prefers .grava/bin, falls back to PATH).
//   - Schema versioning — SchemaVersion constant plus
//     CheckSchemaVersion / WriteSchemaVersion against
//     .grava/schema_version.
//   - Port allocation — AllocatePort, FindAvailablePort, plus per-project
//     persistence in ~/.grava/ports.json.
//   - Misc — WriteGitExclude (manages .git/info/exclude and migrates the
//     legacy .gitignore entry), FindScript (locates bundled shell scripts),
//     ResolveGravaDir (legacy GRAVA_DIR + walk-up resolver, kept for
//     compatibility with ResolveGravaDirWithRedirect).
//
// All functions here are intentionally side-effect-narrow (read or write
// specific files / run specific subprocesses) so they can be composed by
// commands such as `grava init`, `grava claim`, and the merge driver
// without each command re-implementing the same OS plumbing.
package utils
