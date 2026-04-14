package cmd

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/spf13/cobra"
)

// mergeDriverName is the git merge driver identifier used in .git/config and .gitattributes.
const mergeDriverName = "grava"

// mergeDriverCmd is the driver command string stored in .git/config.
// %O = ancestor, %A = current (result written here), %B = other.
// Requires 'grava' on PATH at merge time; run 'which grava' to verify.
const mergeDriverCmd = "grava merge-slot --ancestor %O --current %A --other %B"

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
		if err := registerMergeDriver(cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
			return err
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ Grava merge driver registered in .git/config")
		return nil
	},
}

// registerMergeDriver writes the grava merge driver configuration to .git/config.
func registerMergeDriver(stdout, stderr io.Writer) error {
	configs := [][]string{
		{"merge." + mergeDriverName + ".name", "Grava JSONL Merge Driver"},
		{"merge." + mergeDriverName + ".driver", mergeDriverCmd},
	}

	for _, kv := range configs {
		c := exec.Command("git", "config", kv[0], kv[1]) //nolint:gosec
		c.Stdout = stdout
		c.Stderr = stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to set git config %s: %w", kv[0], err)
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().Bool("shared", false, "Install hooks to .grava/hooks and configure core.hooksPath (used in story 6.4)")
}

// isInGitRepo returns true if the current directory is inside a Git repository.
func isInGitRepo() bool {
	c := exec.Command("git", "rev-parse", "--git-dir")
	return c.Run() == nil
}

// gitConfigGet reads a single git config value. Returns ("", false) if not set.
func gitConfigGet(key string) (string, bool) {
	c := exec.Command("git", "config", "--get", key) //nolint:gosec
	out, err := c.Output()
	if err != nil {
		return "", false
	}
	val := string(out)
	if len(val) > 0 && val[len(val)-1] == '\n' {
		val = val[:len(val)-1]
	}
	return val, true
}

// gitConfigSet writes a git config value, routing output to the provided writers.
func gitConfigSet(key, value string, stdout, stderr io.Writer) error {
	c := exec.Command("git", "config", key, value) //nolint:gosec
	c.Stdout = stdout
	c.Stderr = stderr
	return c.Run()
}
