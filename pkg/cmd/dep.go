package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	depType   string
	batchFile string
)

// depCmd represents the dep command
var depCmd = &cobra.Command{
	Use:   "dep",
	Short: "Manage task dependencies",
	Long: `Create, list, or batch manage directed dependency edges between issues.

The default usage 'grava dep <from> <to>' creates a "blocks" dependency.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		if len(args) == 2 {
			return addDependency(cmd, args[0], args[1])
		}
		return fmt.Errorf("requires exactly 2 arguments for adding a dependency, or use a subcommand")
	},
}

func addDependency(cmd *cobra.Command, fromID, toID string) error {
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

	fmt.Fprintf(cmd.OutOrStdout(), "🔗 Dependency created: %s -[%s]-> %s\n", fromID, depType, toID)
	return nil
}

var depBatchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Batch create dependencies from a JSON file",
	Long: `Provide a JSON file with an array of dependency objects.
Example JSON:
[
  {"from": "grava-123", "to": "grava-456", "type": "blocks"},
  {"from": "grava-789", "to": "grava-abc"}
]`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if batchFile == "" {
			return fmt.Errorf("--file is required")
		}
		f, err := os.Open(batchFile)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer f.Close()

		var deps []struct {
			From string `json:"from"`
			To   string `json:"to"`
			Type string `json:"type"`
		}
		if err := json.NewDecoder(f).Decode(&deps); err != nil {
			return fmt.Errorf("failed to decode JSON: %w", err)
		}

		for _, d := range deps {
			if d.Type == "" {
				d.Type = "blocks"
			}
			_, err := Store.Exec(
				`INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)`,
				d.From, d.To, d.Type, actor, actor, agentModel,
			)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "⚠️ Failed to create %s -> %s: %v\n", d.From, d.To, err)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "🔗 Created: %s -[%s]-> %s\n", d.From, d.Type, d.To)
			}
		}

		return nil
	},
}

var depClearCmd = &cobra.Command{
	Use:   "clear <id>",
	Short: "Remove all dependencies for an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		_, err := Store.Exec(`DELETE FROM dependencies WHERE from_id = ? OR to_id = ?`, id, id)
		if err != nil {
			return fmt.Errorf("failed to clear dependencies: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "🧹 All dependencies for %s cleared.\n", id)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(depCmd)
	depCmd.PersistentFlags().StringVar(&depType, "type", "blocks", "Dependency type (blocks, relates-to, duplicates, parent-child)")

	depCmd.AddCommand(depBatchCmd)
	depBatchCmd.Flags().StringVarP(&batchFile, "file", "f", "", "JSON file containing dependencies")

	depCmd.AddCommand(depClearCmd)
}
