package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// commentCmd represents the comment command
var commentCmd = &cobra.Command{
	Use:   "comment <id> <text>",
	Short: "Append a comment to an issue",
	Long: `Append a comment to an existing issue.

Comments are stored as a JSON array in the issue's metadata column.
Each comment entry records the text, the timestamp, and the actor.

Example:
  grava comment grava-abc "Investigated root cause, see PR #42"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		text := args[1]

		// 1. Fetch current metadata
		row := Store.QueryRow(`SELECT COALESCE(metadata, '{}') FROM issues WHERE id = ?`, id)
		var rawMeta string
		if err := row.Scan(&rawMeta); err != nil {
			return fmt.Errorf("issue %s not found: %w", id, err)
		}

		// 2. Unmarshal metadata
		var meta map[string]any
		if err := json.Unmarshal([]byte(rawMeta), &meta); err != nil {
			return fmt.Errorf("failed to parse metadata for %s: %w", id, err)
		}

		// 3. Append comment entry
		comment := map[string]any{
			"text":      text,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"actor":     "grava-cli",
		}

		var comments []any
		if existing, ok := meta["comments"]; ok {
			if arr, ok := existing.([]any); ok {
				comments = arr
			}
		}
		comments = append(comments, comment)
		meta["comments"] = comments

		// 4. Marshal updated metadata
		updated, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		// 5. Write back
		_, err = Store.Exec(
			`UPDATE issues SET metadata = ?, updated_at = ? WHERE id = ?`,
			string(updated), time.Now(), id,
		)
		if err != nil {
			return fmt.Errorf("failed to save comment on %s: %w", id, err)
		}

		cmd.Printf("💬 Comment added to %s\n", id)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(commentCmd)
}
