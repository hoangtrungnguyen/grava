package cmd

import (
	"fmt"

	"github.com/hoangtrungnguyen/grava/pkg/gitattributes"
	"github.com/hoangtrungnguyen/grava/pkg/gitconfig"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Configure Git repository to use the Grava merge driver",
	Long: `Configure the current Git repository to use Grava as a schema-aware
merge driver for JSONL issue files.

Steps performed:
  1. Register 'grava' merge driver in .git/config
  2. Ensure 'issues.jsonl merge=grava' is present in .gitattributes

Run this once per repository (or once per worktree clone).
Note: 'grava' must be on PATH at merge time.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Step 1: register merge driver in .git/config
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

		// Step 2: ensure .gitattributes routes issues.jsonl through the driver
		root, err := gitattributes.RepoRoot()
		if err != nil {
			return err
		}
		added, err := gitattributes.EnsureMergeAttr(root)
		if err != nil {
			return err
		}
		if added {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ Added 'issues.jsonl merge=grava' to .gitattributes")
		} else {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ .gitattributes already contains 'issues.jsonl merge=grava'")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().Bool("shared", false, "Install hooks to .grava/hooks and configure core.hooksPath (story 6.4)")
}
