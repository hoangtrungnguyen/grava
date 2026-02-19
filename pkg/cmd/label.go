package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// labelCmd represents the label command
var labelCmd = &cobra.Command{
	Use:   "label <id> <label>",
	Short: "Add a label to an issue",
	Long: `Add a label to an existing issue.

Labels are stored as a JSON array in the issue's metadata column.
Adding a label that already exists is a no-op (idempotent).

Example:
  grava label grava-abc "needs-review"
  grava label grava-abc "priority:high"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		label := args[1]

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

		// 3. Merge label (idempotent)
		var labels []string
		if existing, ok := meta["labels"]; ok {
			if arr, ok := existing.([]any); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						labels = append(labels, s)
					}
				}
			}
		}

		for _, l := range labels {
			if l == label {
				if outputJSON {
					resp := map[string]string{
						"id":     id,
						"status": "unchanged",
						"field":  "labels",
						"note":   fmt.Sprintf("Label %q already present", label),
					}
					b, _ := json.MarshalIndent(resp, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(b))
					return nil
				}
				cmd.Printf("🏷️  Label %q already present on %s\n", label, id)
				return nil
			}
		}
		labels = append(labels, label)
		meta["labels"] = labels

		// 4. Marshal updated metadata
		updated, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		// 5. Write back
		_, err = Store.Exec(
			`UPDATE issues SET metadata = ?, updated_at = ?, updated_by = ?, agent_model = ? WHERE id = ?`,
			string(updated), time.Now(), actor, agentModel, id,
		)
		if err != nil {
			return fmt.Errorf("failed to save label on %s: %w", id, err)
		}

		if outputJSON {
			resp := map[string]string{
				"id":     id,
				"status": "updated",
				"field":  "labels",
			}
			b, _ := json.MarshalIndent(resp, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		}

		cmd.Printf("🏷️  Label %q added to %s\n", label, id)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(labelCmd)
}
