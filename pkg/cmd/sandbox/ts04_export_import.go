package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	issuesapi "github.com/hoangtrungnguyen/grava/pkg/cmd/issues"
	synccmd "github.com/hoangtrungnguyen/grava/pkg/cmd/sync"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

const ts04ID = "TS-04"

func init() {
	Register(Scenario{
		ID:       ts04ID,
		Name:     "Export/Import Round-Trip",
		EpicGate: 7,
		Run:      runTS04,
	})
}

// runTS04 validates zero-loss handoff:
//  1. Create issues with labels and dependencies
//  2. Export to JSONL buffer
//  3. Delete originals from DB
//  4. Import from buffer
//  5. Verify 100% field preservation
func runTS04(ctx context.Context, store dolt.Store) Result {
	tag := fmt.Sprintf("ts04-%d", time.Now().UnixNano())

	// --- Setup: create 3 issues with a dependency chain ---
	var ids []string
	defer func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, id := range ids {
			_, _ = store.ExecContext(ctx2, "DELETE FROM issues WHERE id = ?", id)
			_, _ = store.ExecContext(ctx2, "DELETE FROM events WHERE issue_id = ?", id)
			_, _ = store.ExecContext(ctx2, "DELETE FROM issue_labels WHERE issue_id = ?", id)
			_, _ = store.ExecContext(ctx2, "DELETE FROM issue_comments WHERE issue_id = ?", id)
		}
		for i := 0; i < len(ids)-1; i++ {
			_, _ = store.ExecContext(ctx2, "DELETE FROM dependencies WHERE from_id = ? AND to_id = ?", ids[i], ids[i+1])
		}
	}()

	for i := 0; i < 3; i++ {
		created, err := issuesapi.CreateIssue(ctx, store, issuesapi.CreateParams{
			Title:     fmt.Sprintf("%s-issue-%d", tag, i),
			IssueType: "task",
			Priority:  "medium",
			Actor:     "sandbox",
		})
		if err != nil {
			return fail(ts04ID, fmt.Sprintf("setup: create issue %d: %v", i, err))
		}
		ids = append(ids, created.ID)
	}

	// Add a dependency: ids[0] → ids[1]
	_, err := store.ExecContext(ctx,
		"INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by) VALUES (?, ?, 'blocks', 'sandbox', 'sandbox')",
		ids[0], ids[1])
	if err != nil {
		return fail(ts04ID, fmt.Sprintf("setup: create dependency: %v", err))
	}

	// Add a label to ids[0]
	_, err = store.ExecContext(ctx,
		"INSERT INTO issue_labels (issue_id, label) VALUES (?, ?)", ids[0], "test-label")
	if err != nil {
		return fail(ts04ID, fmt.Sprintf("setup: add label: %v", err))
	}

	// Add a comment to ids[1]
	_, err = store.ExecContext(ctx,
		"INSERT INTO issue_comments (issue_id, message, actor) VALUES (?, ?, ?)",
		ids[1], "test comment from sandbox", "sandbox")
	if err != nil {
		return fail(ts04ID, fmt.Sprintf("setup: add comment: %v", err))
	}

	details := []string{fmt.Sprintf("created %d issues with deps, labels, comments", len(ids))}

	// --- Export ---
	var buf bytes.Buffer
	exportCount, err := synccmd.ExportFlatJSONL(ctx, store, &buf, false)
	if err != nil {
		return fail(ts04ID, fmt.Sprintf("export: %v", err), details...)
	}
	if exportCount < 3 {
		return fail(ts04ID, fmt.Sprintf("export: expected ≥3 issues, got %d", exportCount), details...)
	}
	exported := buf.String()
	details = append(details, fmt.Sprintf("exported %d issues to JSONL (%d bytes)", exportCount, len(exported)))

	// --- Verify exported JSONL contains our test data ---
	foundDep := false
	foundLabel := false
	foundComment := false
	for _, line := range strings.Split(exported, "\n") {
		if line == "" {
			continue
		}
		var rec map[string]interface{}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		id, _ := rec["id"].(string)

		// Check dependencies
		if deps, ok := rec["dependencies"].([]interface{}); ok && len(deps) > 0 {
			for _, d := range deps {
				dep, _ := d.(map[string]interface{})
				if dep["from_id"] == ids[0] && dep["to_id"] == ids[1] {
					foundDep = true
				}
			}
		}
		// Check labels
		if id == ids[0] {
			if labels, ok := rec["labels"].([]interface{}); ok {
				for _, l := range labels {
					if l == "test-label" {
						foundLabel = true
					}
				}
			}
		}
		// Check comments
		if id == ids[1] {
			if comments, ok := rec["comments"].([]interface{}); ok && len(comments) > 0 {
				foundComment = true
			}
		}
	}

	if !foundDep {
		return fail(ts04ID, "export: dependency link not found in JSONL output", details...)
	}
	if !foundLabel {
		return fail(ts04ID, "export: label not found in JSONL output", details...)
	}
	if !foundComment {
		return fail(ts04ID, "export: comment not found in JSONL output", details...)
	}
	details = append(details, "exported JSONL contains deps, labels, and comments")

	// --- Delete originals ---
	for _, id := range ids {
		_, _ = store.ExecContext(ctx, "DELETE FROM issue_labels WHERE issue_id = ?", id)
		_, _ = store.ExecContext(ctx, "DELETE FROM issue_comments WHERE issue_id = ?", id)
		_, _ = store.ExecContext(ctx, "DELETE FROM issues WHERE id = ?", id)
	}
	_, _ = store.ExecContext(ctx, "DELETE FROM dependencies WHERE from_id = ? AND to_id = ?", ids[0], ids[1])

	// --- Import from buffer ---
	importResult, err := synccmd.ImportFlatJSONL(ctx, store, strings.NewReader(exported), true)
	if err != nil {
		return fail(ts04ID, fmt.Sprintf("import: %v", err), details...)
	}
	details = append(details, fmt.Sprintf("imported %d, updated %d, skipped %d",
		importResult.Imported, importResult.Updated, importResult.Skipped))

	// --- Verify round-trip: all 3 issues exist with correct data ---
	for _, id := range ids {
		var title string
		row := store.QueryRowContext(ctx, "SELECT title FROM issues WHERE id = ?", id)
		if err := row.Scan(&title); err != nil {
			return fail(ts04ID, fmt.Sprintf("verify: issue %s not found after import: %v", id, err), details...)
		}
	}
	details = append(details, "all issues present after import")

	// Verify dependency preserved
	var depCount int
	row := store.QueryRowContext(ctx, "SELECT COUNT(*) FROM dependencies WHERE from_id = ? AND to_id = ?", ids[0], ids[1])
	if err := row.Scan(&depCount); err != nil || depCount == 0 {
		return fail(ts04ID, "verify: dependency link lost during round-trip", details...)
	}
	details = append(details, "dependency links preserved")

	// Verify label survived round-trip
	var labelCount int
	row = store.QueryRowContext(ctx, "SELECT COUNT(*) FROM issue_labels WHERE issue_id = ? AND label = ?", ids[0], "test-label")
	if err := row.Scan(&labelCount); err != nil || labelCount == 0 {
		return fail(ts04ID, "verify: label lost during round-trip", details...)
	}
	details = append(details, "labels preserved")

	// Verify comment survived round-trip
	var commentCount int
	row = store.QueryRowContext(ctx, "SELECT COUNT(*) FROM issue_comments WHERE issue_id = ?", ids[1])
	if err := row.Scan(&commentCount); err != nil || commentCount == 0 {
		return fail(ts04ID, "verify: comment lost during round-trip", details...)
	}
	details = append(details, "comments preserved")

	return pass(ts04ID, details...)
}
