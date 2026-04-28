// Package gitattributes manages the .gitattributes file entries that wire
// Git up to invoke the Grava merge driver on JSONL issue files.
//
// The package writes and verifies a single line ("issues.jsonl merge=grava-merge")
// in the repository's .gitattributes file. EnsureMergeAttr is idempotent for
// sequential callers and creates the file if it is missing; HasMergeAttr is the
// read-only check used by 'grava doctor'. RepoRoot is a small helper that shells
// out to "git rev-parse --show-toplevel" so callers do not have to.
//
// In grava this package is invoked during 'grava install' (alongside
// pkg/gitconfig and pkg/githooks) so that the schema-aware JSONL merge driver
// registered in .git/config is actually consulted by Git when merging
// issues.jsonl. Without the .gitattributes line Git would fall back to its
// default text driver and corrupt the file.
//
// All file writes go through os.WriteFile and preserve the existing newline
// shape of the file; the line is always written on its own line.
package gitattributes
