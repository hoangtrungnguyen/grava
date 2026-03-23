package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
)

// jsonErrorEnvelope is the wire format for --json error responses.
type jsonErrorEnvelope struct {
	Error jsonErrorDetail `json:"error"`
}

type jsonErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSONError writes a structured JSON error to cmd.ErrOrStderr() and returns nil
// so that cobra does not double-print the error. Use this in RunE when outputJSON is true.
//
//	if err != nil && outputJSON {
//	    return writeJSONError(cmd, err)
//	}
func writeJSONError(cmd *cobra.Command, err error) error {
	code := "INTERNAL_ERROR"
	message := err.Error()

	var gravaErr *gravaerrors.GravaError
	if errors.As(err, &gravaErr) {
		code = gravaErr.Code
		message = gravaErr.Message
	}

	envelope := jsonErrorEnvelope{
		Error: jsonErrorDetail{Code: code, Message: message},
	}
	b, marshalErr := json.Marshal(envelope)
	if marshalErr != nil {
		// Fallback: plain text to stderr — should never happen
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), `{"error":{"code":"INTERNAL_ERROR","message":"failed to marshal error"}}`+"\n")
		return nil
	}
	_, _ = fmt.Fprintln(cmd.ErrOrStderr(), string(b))
	return nil
}

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
