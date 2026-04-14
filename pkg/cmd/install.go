package cmd

import (
	"fmt"

	"github.com/hoangtrungnguyen/grava/pkg/gitconfig"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Configure Git repository to use the Grava merge driver",
	Long: `Configure the current Git repository to use Grava as a schema-aware
merge driver for JSONL issue files.

Registers the 'grava' merge driver in .git/config so that Git delegates
JSONL file merging to 'grava merge-slot' instead of the default text merge.

Run this once per repository (or once per worktree clone).

Note: 'grava' must be on PATH at merge time. This command will be extended
in future releases to also configure .gitattributes and deploy Git hooks.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := gitconfig.DefaultDriverConfig()
		alreadySet, err := gitconfig.RegisterMergeDriver(cfg, cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}

		if alreadySet {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ Grava merge driver already configured in .git/config")
		} else {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ Grava merge driver registered in .git/config")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().Bool("shared", false, "Install hooks to .grava/hooks and configure core.hooksPath (story 6.4)")
}
