// Package idgen generates the hierarchical issue identifiers used throughout
// grava (e.g. "grava-a1b2" for a top-level issue and "grava-a1b2.1" for a
// child).
//
// IDGenerator is the interface; StandardGenerator is the default
// implementation. Top-level IDs are produced by GenerateBaseID, which
// combines the current nanosecond timestamp with a crypto/rand value, hashes
// the pair with SHA-256, and emits "<prefix>-<first 4 hex chars>". Four hex
// chars give 65,536 combinations, which is acceptable collision risk for a
// project-scale tracker; callers that need stronger guarantees should
// retry on collision at the database layer.
//
// Child IDs are produced by GenerateChildID(parentID), which delegates to
// the backing dolt.Store via GetNextChildSequence to obtain the next
// monotonic sequence number for that parent and returns "<parentID>.<n>".
// This places the uniqueness invariant in the database, where it is
// enforced transactionally, and keeps the generator deterministic per
// parent.
//
// Used by the issue-creation paths in grava (CLI commands and the Linear /
// import bridges) so every node added to the graph in pkg/graph has a stable
// identifier.
package idgen
