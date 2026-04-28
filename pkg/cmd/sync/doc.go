// Package synccmd implements the sync-related CLI commands: commit, export,
// and import.
//
// The package name is synccmd (not sync) to avoid collision with the
// stdlib sync package. The directory on disk is pkg/cmd/sync but every
// file declares "package synccmd".
//
// commit stages all changed grava tables and writes a Dolt version-control
// commit using CALL DOLT_ADD('-A') followed by CALL DOLT_COMMIT('-m', ?).
// This is the only place in grava that calls dolt commit; every other
// state mutation flows through tables and is folded into the next commit.
//
// export and import speak the canonical flat JSONL format defined here as
// IssueJSONLRecord — one JSON object per line containing an issue with
// its labels, comments, dependencies, and wisp entries embedded. This is
// the same shape the merge driver consumes, so an exported issues.jsonl
// committed to git survives three-way merges. import runs the FR24
// "Dual-Safety Check" (rejecting a stale import when Dolt has uncommitted
// changes) and auto-re-exports issues.jsonl after a successful upsert so
// the on-disk file always matches the database. ValidateJSONL is used by
// pre-commit to reject malformed or legacy-wrapped files. SyncIssuesFile,
// ExportFlatJSONL, and ImportFlatJSONL are exported for git hook handlers
// and sandbox scenarios.
package synccmd
