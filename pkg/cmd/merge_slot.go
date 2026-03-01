package cmd

import (
	"fmt"

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
		fmt.Fprintln(cmd.OutOrStdout(), "merge-slot called")
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
