// Package grava resolves the location of the active .grava/ data directory
// for the running CLI process.
//
// ResolveGravaDir implements the ADR-004 priority chain used by every grava
// subcommand to find its working directory:
//
//  1. The GRAVA_DIR environment variable, if set and pointing at a directory.
//  2. A .grava/redirect file discovered by walking up from the current
//     working directory; its content is treated as a relative or absolute
//     path to the real .grava/ directory (used to share state across
//     worktrees).
//  3. A .grava/ directory discovered by walking up from the current working
//     directory.
//
// Failures surface as typed errors via pkg/errors with codes NOT_INITIALIZED
// (no GRAVA_DIR, no redirect, no .grava/ found) or REDIRECT_STALE (a
// redirect file points to a missing target). This package is intentionally
// tiny and dependency-free apart from the shared error helpers; it is the
// first call most subcommands make so they can locate the Dolt database
// under .grava/dolt/ and other state files.
package grava
