// Package githooks deploys grava shim hooks into a repository's hook
// directory, preserving any pre-existing non-grava hooks.
//
// Two installation modes are supported. AppendStubs (ADR-H2) is the default
// for 'grava init': it either writes a minimal new stub or appends a
// grava-hook-start / grava-hook-end bracketed invocation to a pre-existing
// hook so the user's content keeps running. DeployAll is the older replace
// mode: it writes a full shim that delegates entirely to 'grava hook run',
// renaming any non-grava file to <name>.pre-grava. Both modes are idempotent
// and use embedded markers (ShimMarker, AppendStartMarker, AppendEndMarker)
// to detect previous installations.
//
// Hooks installed by AppendStubs are limited to InitHookNames (pre-commit,
// post-merge) — the minimal set required by the Git Sync pipeline (FR25).
// DeployAll installs the broader HookNames list (pre-commit, post-merge,
// post-checkout, prepare-commit-msg). DefaultHooksDir and SharedHooksDir
// resolve the target directory; the latter is used when --shared is passed
// to 'grava install' so that hooks live under .grava/hooks for sharing
// across worktrees.
package githooks
