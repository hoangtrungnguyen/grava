package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/hoangtrungnguyen/grava/pkg/merge"
)

const ts07ID = "TS-07"

func init() {
	Register(Scenario{
		ID:       ts07ID,
		Name:     "Conflict Detection (Delete-vs-Modify)",
		EpicGate: 6,
		Run:      runTS07,
	})
}

// runTS07 validates the LWW merge driver under two conflict scenarios:
//  1. Field conflict with equal timestamps → HasGitConflict=true
//  2. Delete-vs-modify → delete wins, conflict recorded but not a git conflict
func runTS07(ctx context.Context, _ dolt.Store) Result {
	details := []string{}

	// --- Scenario A: Equal-timestamp field conflict ---
	// Both branches modify the same issue's title at the same timestamp.
	ts := "2026-04-15T10:00:00Z"
	ancestor := mustJSONL(map[string]interface{}{
		"id": "conflict-test-1", "title": "Original Title",
		"status": "open", "priority": 1, "type": "task",
		"created_at": ts, "updated_at": ts,
		"created_by": "sandbox", "updated_by": "sandbox",
	})
	current := mustJSONL(map[string]interface{}{
		"id": "conflict-test-1", "title": "Local Change",
		"status": "open", "priority": 1, "type": "task",
		"created_at": ts, "updated_at": ts,
		"created_by": "sandbox", "updated_by": "sandbox",
	})
	other := mustJSONL(map[string]interface{}{
		"id": "conflict-test-1", "title": "Remote Change",
		"status": "open", "priority": 1, "type": "task",
		"created_at": ts, "updated_at": ts,
		"created_by": "sandbox", "updated_by": "sandbox",
	})

	result, err := merge.ProcessMergeWithLWW(ancestor, current, other)
	if err != nil {
		return fail(ts07ID, fmt.Sprintf("scenario A: merge failed: %v", err), details...)
	}

	if !result.HasGitConflict {
		return fail(ts07ID, "scenario A: expected HasGitConflict=true for equal-timestamp conflict", details...)
	}
	details = append(details, "scenario A: equal-timestamp conflict detected (HasGitConflict=true)")

	if len(result.ConflictRecords) == 0 {
		return fail(ts07ID, "scenario A: expected ConflictRecords to be populated", details...)
	}
	details = append(details, fmt.Sprintf("scenario A: %d conflict record(s) generated", len(result.ConflictRecords)))

	// --- Scenario B: Delete-vs-modify (delete wins) ---
	// Ancestor has the issue, current modifies it, other deletes it.
	tsOld := "2026-04-14T10:00:00Z"
	tsNew := "2026-04-15T12:00:00Z"
	ancestorB := mustJSONL(map[string]interface{}{
		"id": "conflict-test-2", "title": "Will Be Deleted",
		"status": "open", "priority": 1, "type": "task",
		"created_at": tsOld, "updated_at": tsOld,
		"created_by": "sandbox", "updated_by": "sandbox",
	})
	currentB := mustJSONL(map[string]interface{}{
		"id": "conflict-test-2", "title": "Modified After Delete",
		"status": "open", "priority": 2, "type": "task",
		"created_at": tsOld, "updated_at": tsNew,
		"created_by": "sandbox", "updated_by": "sandbox",
	})
	otherB := "" // Empty = issue deleted on other branch

	resultB, err := merge.ProcessMergeWithLWW(ancestorB, currentB, otherB)
	if err != nil {
		return fail(ts07ID, fmt.Sprintf("scenario B: merge failed: %v", err), details...)
	}

	// Delete-wins should NOT set HasGitConflict (deletion is deterministic)
	if resultB.HasGitConflict {
		return fail(ts07ID, "scenario B: delete-wins should NOT set HasGitConflict", details...)
	}
	details = append(details, "scenario B: delete-wins resolved without git conflict")

	// But it should produce a conflict record for audit
	if len(resultB.ConflictRecords) == 0 {
		return fail(ts07ID, "scenario B: expected audit ConflictRecord for delete-wins", details...)
	}
	details = append(details, fmt.Sprintf("scenario B: %d audit conflict record(s) for delete-wins", len(resultB.ConflictRecords)))

	// Verify the merged output does NOT contain the deleted issue
	if resultB.Merged != "" && containsIssueID(resultB.Merged, "conflict-test-2") {
		return fail(ts07ID, "scenario B: deleted issue should not appear in merged output", details...)
	}
	details = append(details, "scenario B: deleted issue absent from merged output")

	// --- Scenario C: LWW resolution (newer wins) ---
	tsOlder := "2026-04-14T08:00:00Z"
	tsNewer := "2026-04-15T14:00:00Z"
	ancestorC := mustJSONL(map[string]interface{}{
		"id": "conflict-test-3", "title": "Original",
		"status": "open", "priority": 1, "type": "task",
		"created_at": tsOld, "updated_at": tsOld,
		"created_by": "sandbox", "updated_by": "sandbox",
	})
	currentC := mustJSONL(map[string]interface{}{
		"id": "conflict-test-3", "title": "Local Update",
		"status": "open", "priority": 1, "type": "task",
		"created_at": tsOld, "updated_at": tsOlder,
		"created_by": "sandbox", "updated_by": "local-agent",
	})
	otherC := mustJSONL(map[string]interface{}{
		"id": "conflict-test-3", "title": "Remote Update (Newer)",
		"status": "in_progress", "priority": 2, "type": "task",
		"created_at": tsOld, "updated_at": tsNewer,
		"created_by": "sandbox", "updated_by": "remote-agent",
	})

	resultC, err := merge.ProcessMergeWithLWW(ancestorC, currentC, otherC)
	if err != nil {
		return fail(ts07ID, fmt.Sprintf("scenario C: merge failed: %v", err), details...)
	}

	if resultC.HasGitConflict {
		return fail(ts07ID, "scenario C: LWW with clear winner should NOT produce git conflict", details...)
	}
	details = append(details, "scenario C: LWW resolved to newer version (no conflict)")

	return pass(ts07ID, details...)
}

// mustJSONL marshals a map into a single JSONL line.
func mustJSONL(m map[string]interface{}) string {
	b, err := json.Marshal(m)
	if err != nil {
		panic(fmt.Sprintf("mustJSONL: %v", err))
	}
	return string(b) + "\n"
}

// containsIssueID checks if a JSONL string contains a line with the given issue ID.
func containsIssueID(jsonl, issueID string) bool {
	for _, line := range splitLines(jsonl) {
		var rec map[string]interface{}
		if json.Unmarshal([]byte(line), &rec) == nil {
			if rec["id"] == issueID {
				return true
			}
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range splitByNewline(s) {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitByNewline(s string) []string {
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

// Ensure time import is used (for future timestamp assertions)
var _ = time.Now
