package cmd

import (
	"encoding/json"
	"fmt"
	"time"
)

// addCommentToIssue appends a comment to the issue's metadata.
func addCommentToIssue(id string, text string) error {
	comment := map[string]any{
		"text":        text,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"actor":       actor,
		"agent_model": agentModel,
	}

	// 1. Fetch current metadata
	row := Store.QueryRow(`SELECT COALESCE(metadata, '{}') FROM issues WHERE id = ?`, id)
	var rawMeta string
	if err := row.Scan(&rawMeta); err != nil {
		return fmt.Errorf("issue %s not found: %w", id, err)
	}

	var meta map[string]any
	if err := json.Unmarshal([]byte(rawMeta), &meta); err != nil {
		return fmt.Errorf("failed to parse metadata for %s: %w", id, err)
	}

	// 2. Append comment
	var comments []any
	if existing, ok := meta["comments"]; ok {
		if arr, ok := existing.([]any); ok {
			comments = arr
		}
	}
	comments = append(comments, comment)
	meta["comments"] = comments

	// 3. Update metadata
	return updateIssueMetadata(id, meta)
}

// updateIssueMetadata updates the metadata column for an issue.
func updateIssueMetadata(id string, meta map[string]any) error {
	updated, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = Store.Exec(
		`UPDATE issues SET metadata = ?, updated_at = NOW(), updated_by = ?, agent_model = ? WHERE id = ?`,
		string(updated), actor, agentModel, id,
	)
	if err != nil {
		return fmt.Errorf("failed to save metadata for %s: %w", id, err)
	}

	return nil
}

// setLastCommit stores the commit hash in the issue's metadata.
func setLastCommit(id string, hash string) error {
	// 1. Fetch current metadata
	row := Store.QueryRow(`SELECT COALESCE(metadata, '{}') FROM issues WHERE id = ?`, id)
	var rawMeta string
	if err := row.Scan(&rawMeta); err != nil {
		return fmt.Errorf("issue %s not found: %w", id, err)
	}

	var meta map[string]any
	if err := json.Unmarshal([]byte(rawMeta), &meta); err != nil {
		return fmt.Errorf("failed to parse metadata for %s: %w", id, err)
	}

	// 2. Set last_commit
	meta["last_commit"] = hash

	// 3. Update metadata
	return updateIssueMetadata(id, meta)
}
