// Package gitconfig reads and writes the Git configuration entries that
// register the Grava schema-aware merge driver in a repository's .git/config.
//
// The driver is identified by DriverName ("grava-merge") and is configured
// with two keys: merge.grava-merge.name (human label) and
// merge.grava-merge.driver (the command Git invokes during a 3-way merge,
// "grava merge-driver %O %A %B"). RegisterMergeDriver is idempotent and only
// rewrites when the local config drifts from the desired DriverConfig;
// IsRegistered, GetLocal, and Get answer the same question at different
// scopes (local-only versus the effective config chain).
//
// In grava this package is paired with pkg/gitattributes (which tells Git
// when to use the driver) and pkg/githooks (which installs the shim hooks).
// Together they form the install-time wiring that makes JSONL issue merges
// schema-aware. All operations shell out to the 'git' binary through
// os/exec, so a working git installation is required at runtime.
package gitconfig
