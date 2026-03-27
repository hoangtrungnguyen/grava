package cmd

import (
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Grava",
	Long:  `All software has versions. This is Grava's.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Printf("Grava CLI version %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
