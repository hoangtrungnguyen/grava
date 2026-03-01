package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Git hooks and merge driver",
	Long:  "Configures the local Git repository's .git/config, .gitattributes, and hooks.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), "install called")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}
