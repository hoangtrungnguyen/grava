package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var hookCmd = &cobra.Command{
	Use:    "hook",
	Short:  "Internal commands for git hook execution",
	Hidden: true, // Hide from main help as it's intended for internal shim use
}

var hookRunCmd = &cobra.Command{
	Use:   "run <hookname>",
	Short: "Execute a specific git hook's logic",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		hookName := args[0]

		switch hookName {
		case "pre-commit":
			return runPreCommit(cmd)
		case "post-merge", "post-checkout":
			return runPostMergeCheckout(cmd)
		default:
			// Unrecognized hook, just exit cleanly or log
			return nil
		}
	},
}

func runPreCommit(cmd *cobra.Command) error {
	// Equivalent to:
	// grava export --file issues.jsonl
	// git add issues.jsonl

	exportCmd := exec.Command("grava", "export", "--file", "issues.jsonl")
	exportCmd.Stdout = cmd.OutOrStdout()
	exportCmd.Stderr = cmd.ErrOrStderr()
	if err := exportCmd.Run(); err != nil {
		return fmt.Errorf("pre-commit export failed: %w", err)
	}

	gitAddCmd := exec.Command("git", "add", "issues.jsonl")
	gitAddCmd.Stdout = cmd.OutOrStdout()
	gitAddCmd.Stderr = cmd.ErrOrStderr()
	if err := gitAddCmd.Run(); err != nil {
		return fmt.Errorf("pre-commit git add failed: %w", err)
	}

	return nil
}

func runPostMergeCheckout(cmd *cobra.Command) error {
	// Equivalent to:
	// grava import --file issues.jsonl --overwrite

	importCmd := exec.Command("grava", "import", "--file", "issues.jsonl", "--overwrite")
	importCmd.Stdout = cmd.OutOrStdout()
	importCmd.Stderr = cmd.ErrOrStderr()

	// We might fail if issues.jsonl doesn't exist yet, which is fine on fresh clones
	output, err := importCmd.CombinedOutput()
	if err != nil {
		// Only complain if it's a real error, not just finding missing file
		if !strings.Contains(string(output), "no such file") {
			return fmt.Errorf("hook import failed: %w\n%s", err, string(output))
		}
	} else {
		fmt.Fprint(cmd.OutOrStdout(), string(output))
	}

	return nil
}

func init() {
	hookCmd.AddCommand(hookRunCmd)
	rootCmd.AddCommand(hookCmd)
}
