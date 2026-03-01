package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var resolveCmd = &cobra.Command{
	Use:   "resolve",
	Short: "Interactive conflict resolution for issues.jsonl",
	Long:  "Provides an interactive prompt to resolve field-level JSONL merge conflicts.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), "resolve called")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(resolveCmd)
}
