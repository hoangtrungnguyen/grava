package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var depType string

// depCmd represents the dep command
var depCmd = &cobra.Command{
	Use:   "dep <from_id> <to_id>",
	Short: "Create a dependency between two issues",
	Long: `Create a directed dependency edge from one issue to another.

The relationship is stored in the dependencies table. The default
dependency type is "blocks" (from_id blocks to_id).

Example:
  grava dep grava-abc grava-def           # grava-abc blocks grava-def
  grava dep grava-abc grava-def --type relates-to`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fromID := args[0]
		toID := args[1]

		if fromID == toID {
			return fmt.Errorf("from_id and to_id must be different issues")
		}

		_, err := Store.Exec(
			`INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)`,
			fromID, toID, depType, actor, actor, agentModel,
		)
		if err != nil {
			return fmt.Errorf("failed to create dependency %s -> %s: %w", fromID, toID, err)
		}

		if outputJSON {
			resp := map[string]string{
				"from_id": fromID,
				"to_id":   toID,
				"type":    depType,
				"status":  "created",
			}
			b, _ := json.MarshalIndent(resp, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		}

		cmd.Printf("🔗 Dependency created: %s -[%s]-> %s\n", fromID, depType, toID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(depCmd)
	depCmd.Flags().StringVar(&depType, "type", "blocks", "Dependency type (blocks, relates-to, duplicates, parent-child)")
}
