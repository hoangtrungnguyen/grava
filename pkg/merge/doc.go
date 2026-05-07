// Package merge implements grava's schema-aware 3-way merge driver for the
// JSONL issue files Git asks it to merge.
//
// The driver treats each JSONL line as an issue object keyed by its "id"
// field and merges the ancestor, current (ours), and other (theirs) versions
// at the field level rather than line by line. Two strategies are exposed:
//
//   - ProcessMerge — pure 3-way merge with field-level conflict markers
//     (objects with "_conflict": true) embedded in the output. Surfaces
//     unresolvable collisions as conflicts that Git reports to the user.
//   - ProcessMergeWithLWW — Last-Write-Wins variant that uses each issue's
//     updated_at timestamp to break ties. Returns a MergeResult with the
//     merged JSONL, an audit trail of ConflictEntry records (including
//     delete-vs-modify "delete-wins" cases), and a HasGitConflict flag for
//     equal-or-missing-timestamp situations that still need user attention.
//
// MarshalSorted produces canonical JSON with keys sorted at every level so
// the same logical merge always yields the same byte sequence and therefore
// the same git object hash. ExtractConflicts re-parses a ProcessMerge output
// to recover ConflictEntry records for downstream tooling. Helper utilities
// (parseJSONL, mergeObjects, mergeObjectsLWW, extractUpdatedAt, conflictID)
// are unexported.
//
// In grava this package is invoked by the 'grava merge-driver' subcommand,
// which is registered in .git/config by pkg/gitconfig and assigned to
// issues.jsonl by pkg/gitattributes. Conflict records produced by the LWW
// path feed grava's conflict-resolution UX.
package merge
