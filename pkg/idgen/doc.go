// Package idgen generates the hierarchical issue identifiers used throughout
// grava (e.g. "grava-a1b2c3d4" for a top-level issue and "grava-a1b2c3d4.1"
// for a child).
//
// IDGenerator is the interface; StandardGenerator is the default
// implementation. Top-level IDs are produced by GenerateBaseID, which
// combines the current nanosecond timestamp with a crypto/rand value, hashes
// the pair with SHA-256, and emits "<prefix>-<first 8 hex chars>". Eight hex
// chars give ~4.29B combinations, ample headroom for cross-system mirror
// flows (Plane sync, imports). Pre-2026-05 grava used 4 hex chars (~65k
// combos); those legacy IDs remain valid throughout the codebase and the
// validation helpers in pkg/validation accept both widths.
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
