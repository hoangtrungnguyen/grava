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
	mergeOutput   string
)

var mergeSlotCmd = &cobra.Command{
	Use:   "merge-slot",
	Short: "Three-way merge driver for JSONL issues",
	Long:  "Executes a schema-aware three-way merge for JSONL files tracked by git.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if mergeAncestor == "" || mergeCurrent == "" || mergeOther == "" || mergeOutput == "" {
			return fmt.Errorf("missing required merge arguments")
		}

		ancestorBytes, err := os.ReadFile(mergeAncestor)
		if err != nil {
			return fmt.Errorf("failed to read ancestor: %w", err)
		}
		currentBytes, err := os.ReadFile(mergeCurrent)
		if err != nil {
			return fmt.Errorf("failed to read current: %w", err)
		}
		otherBytes, err := os.ReadFile(mergeOther)
		if err != nil {
			return fmt.Errorf("failed to read other: %w", err)
		}

		merged, hasConflict, err := merge.ProcessMerge(string(ancestorBytes), string(currentBytes), string(otherBytes))
		if err != nil {
			return fmt.Errorf("failed to process merge: %w", err)
		}

		if writeErr := os.WriteFile(mergeOutput, []byte(merged), 0644); writeErr != nil {
			return fmt.Errorf("failed to write output: %w", writeErr)
		}

		if hasConflict {
			// Git expects exit code non-zero for conflict
			os.Exit(1)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(mergeSlotCmd)

	mergeSlotCmd.Flags().StringVar(&mergeAncestor, "ancestor", "", "Ancestor version (%O)")
	mergeSlotCmd.Flags().StringVar(&mergeCurrent, "current", "", "Current version (%A)")
	mergeSlotCmd.Flags().StringVar(&mergeOther, "other", "", "Other version (%B)")
	mergeSlotCmd.Flags().StringVar(&mergeOutput, "output", "", "Output file (usually %A)")
}
