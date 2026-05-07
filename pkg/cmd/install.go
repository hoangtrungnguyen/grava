package cmd

import (
	"fmt"

	"github.com/hoangtrungnguyen/grava/pkg/gitattributes"
	"github.com/hoangtrungnguyen/grava/pkg/gitconfig"
	"github.com/hoangtrungnguyen/grava/pkg/githooks"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Configure Git repository to use the Grava merge driver",
	Long: `Configure the current Git repository to use Grava as a schema-aware
merge driver for JSONL issue files.

Steps performed:
  1. Register 'grava-merge' merge driver in .git/config
  2. Ensure 'issues.jsonl merge=grava-merge' is present in .gitattributes
  3. Deploy pre-commit, post-merge, post-checkout, and prepare-commit-msg hooks

Use --shared to install hooks to .grava/hooks and configure core.hooksPath,
which allows the hook configuration to be committed and shared with the team.

Note: 'grava' must be on PATH at merge time.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := gitattributes.RepoRoot()
		if err != nil {
			return err
		}

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
		added, err := gitattributes.EnsureMergeAttr(root)
		if err != nil {
			return err
		}
		if added {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ Added 'issues.jsonl merge=grava-merge' to .gitattributes")
		} else {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ .gitattributes already contains 'issues.jsonl merge=grava-merge'")
		}

		// Step 3: deploy Git hooks
		shared, _ := cmd.Flags().GetBool("shared")
		var hooksDir string
		if shared {
			hooksDir = githooks.SharedHooksDir(root)
			if err := gitconfig.Set("core.hooksPath", hooksDir, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
				return fmt.Errorf("failed to set core.hooksPath: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ core.hooksPath configured to %s\n", hooksDir)
		} else {
			hooksDir = githooks.DefaultHooksDir(root)
		}

		results, err := githooks.DeployAll(hooksDir, cmd.OutOrStdout())
		if err != nil {
			return err
		}
		for _, r := range results {
			switch r.Action {
			case "installed":
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Hook installed: %s\n", r.Name)
			case "updated":
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Hook updated:   %s\n", r.Name)
			case "skipped":
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Hook up-to-date: %s\n", r.Name)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().Bool("shared", false,
		"Install hooks to .grava/hooks and set core.hooksPath for team sharing")
}
