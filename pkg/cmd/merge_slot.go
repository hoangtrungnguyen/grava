package cmd

import (
	"fmt"
	"os"

	"github.com/hoangtrungnguyen/grava/pkg/merge"
	"github.com/spf13/cobra"
)

var (
	mergeAncestor string
	mergeCurrent  string
	mergeOther    string
)

var mergeSlotCmd = &cobra.Command{
	Use:   "merge-slot",
	Short: "Three-way merge driver for JSONL issues files",
	Long: `Executes a schema-aware three-way merge for JSONL files tracked by git.

Git invokes this as a merge driver: grava merge-slot --ancestor %O --current %A --other %B
The merged result is written back to the current (%A) file path.
Exit code 1 signals git that conflicts remain.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if mergeAncestor == "" || mergeCurrent == "" || mergeOther == "" {
			return fmt.Errorf("--ancestor, --current, and --other are required")
		}

		ancestorBytes, err := os.ReadFile(mergeAncestor)
		if err != nil {
			return fmt.Errorf("failed to read ancestor file: %w", err)
		}
		currentBytes, err := os.ReadFile(mergeCurrent)
		if err != nil {
			return fmt.Errorf("failed to read current file: %w", err)
		}
		otherBytes, err := os.ReadFile(mergeOther)
		if err != nil {
			return fmt.Errorf("failed to read other file: %w", err)
		}

		merged, hasConflict, err := merge.ProcessMerge(
			string(ancestorBytes),
			string(currentBytes),
			string(otherBytes),
		)
		if err != nil {
			return fmt.Errorf("merge failed: %w", err)
		}

		// Write merged result back to the current (%A) file
		if err := os.WriteFile(mergeCurrent, []byte(merged), 0644); err != nil { //nolint:gosec
			return fmt.Errorf("failed to write merge result to %s: %w", mergeCurrent, err)
		}

		if hasConflict {
			// Non-zero exit tells git that conflicts remain
			os.Exit(1)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(mergeSlotCmd)

	mergeSlotCmd.Flags().StringVar(&mergeAncestor, "ancestor", "", "Ancestor version file path (%O)")
	mergeSlotCmd.Flags().StringVar(&mergeCurrent, "current", "", "Current version file path (%A) — result is written here")
	mergeSlotCmd.Flags().StringVar(&mergeOther, "other", "", "Other version file path (%B)")
}
