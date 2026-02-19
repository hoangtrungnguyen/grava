package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var commentLastCommit string

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

		// Use helper to add comment
		if err := addCommentToIssue(id, text); err != nil {
			return err
		}

		// Metadata update (last-commit)
		if cmd.Flags().Changed("last-commit") {
			if err := setLastCommit(id, commentLastCommit); err != nil {
				return err
			}
		}

		if outputJSON {
			resp := map[string]string{
				"id":     id,
				"status": "updated",
				"field":  "comments",
			}
			b, _ := json.MarshalIndent(resp, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		}

		cmd.Printf("💬 Comment added to %s\n", id)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(commentCmd)
	commentCmd.Flags().StringVar(&commentLastCommit, "last-commit", "", "Store the last session's commit hash")
}
