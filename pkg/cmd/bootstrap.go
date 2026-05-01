package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Set up grava in a project (binary, git hooks, cron lines)",
	Long: `bootstrap verifies the grava binary is on PATH, checks the Claude Code plugin
is installed, installs git hooks, and prints the cron lines needed for the
merge watcher and hunt scheduler.

Steps:
  [1/4] Check grava binary on PATH
  [2/4] Check Claude Code plugin install
  [3/4] Install git hooks (.git/hooks/commit-msg)
  [4/4] Print cron lines for crontab -e`,
	RunE: func(cmd *cobra.Command, args []string) error {
		printCron, _ := cmd.Flags().GetBool("print-cron")
		skipGitHooks, _ := cmd.Flags().GetBool("skip-git-hooks")
		skipBinaryCheck, _ := cmd.Flags().GetBool("skip-binary-check")

		if printCron {
			return printCronLines(cmd)
		}

		repoRoot, err := gitRepoRoot()
		if err != nil {
			return fmt.Errorf("not inside a git repository: %w", err)
		}

		allOK := true

		// Step 1: binary check
		if !skipBinaryCheck {
			fmt.Fprintf(cmd.OutOrStdout(), "[1/4] Checking grava binary on $PATH...                     ")
			path, err := exec.LookPath("grava")
			if err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "MISSING")
				fmt.Fprintln(cmd.ErrOrStderr(), "  Fix: go install github.com/hoangtrungnguyen/grava@latest")
				allOK = false
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "OK (%s)\n", path)
			}
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "[1/4] Skipped (--skip-binary-check)")
		}

		// Step 2: plugin install check
		fmt.Fprintf(cmd.OutOrStdout(), "[2/4] Checking Claude Code plugin install...                ")
		pluginCacheBase := filepath.Join(os.Getenv("HOME"), ".claude", "plugins", "cache")
		pluginPath := filepath.Join(pluginCacheBase, "grava", "grava")
		if _, err := os.Stat(pluginPath); err == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "OK (%s)\n", pluginPath)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "NOT FOUND")
			fmt.Fprintln(cmd.ErrOrStderr(), "  Fix: in Claude Code, run:")
			fmt.Fprintln(cmd.ErrOrStderr(), "    /plugin marketplace add ./")
			fmt.Fprintln(cmd.ErrOrStderr(), "    /plugin install grava@grava")
			allOK = false
		}

		// Step 3: git hooks
		if !skipGitHooks {
			fmt.Fprintf(cmd.OutOrStdout(), "[3/4] Installing git hooks (.git/hooks/commit-msg)...       ")
			installScript := filepath.Join(repoRoot, "scripts", "install-hooks.sh")
			if _, err := os.Stat(installScript); os.IsNotExist(err) {
				// Try plugin path
				installScript = filepath.Join(pluginPath, "scripts", "install-git-hooks.sh")
			}
			if _, err := os.Stat(installScript); err == nil {
				out, err := exec.Command("bash", installScript).CombinedOutput()
				if err != nil {
					fmt.Fprintln(cmd.OutOrStdout(), "FAILED")
					fmt.Fprintf(cmd.ErrOrStderr(), "  Error: %s\n  Output: %s\n", err, out)
					allOK = false
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "OK")
				}
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "SKIPPED (install script not found)")
			}
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "[3/4] Skipped (--skip-git-hooks)")
		}

		// Step 4: cron lines
		fmt.Fprintln(cmd.OutOrStdout(), "[4/4] Cron lines (paste into `crontab -e`):")
		if err := printCronLines(cmd); err != nil {
			return err
		}

		if !allOK {
			fmt.Fprintln(cmd.ErrOrStderr(), "\nBootstrap incomplete — fix the errors above and re-run.")
			return fmt.Errorf("bootstrap incomplete")
		}
		fmt.Fprintln(cmd.OutOrStdout(), "\nBootstrap complete. Try: /ship <issue-id>")
		return nil
	},
}

func printCronLines(cmd *cobra.Command) error {
	repoRoot, _ := gitRepoRoot()
	if repoRoot == "" {
		repoRoot = "$(pwd)"
	}

	pluginRoot := filepath.Join(os.Getenv("HOME"), ".claude", "plugins", "cache", "grava", "grava")

	watcherScript := filepath.Join(repoRoot, "scripts", "pr-merge-watcher.sh")
	if _, err := os.Stat(watcherScript); os.IsNotExist(err) {
		watcherScript = filepath.Join(pluginRoot, "scripts", "pr-merge-watcher.sh")
	}

	huntScript := filepath.Join(repoRoot, "scripts", "run-pending-hunts.sh")
	if _, err := os.Stat(huntScript); os.IsNotExist(err) {
		huntScript = filepath.Join(pluginRoot, "scripts", "run-pending-hunts.sh")
	}

	lines := []string{
		fmt.Sprintf("*/5 * * * * cd %s && %s >> %s/.grava/watcher.log 2>&1",
			repoRoot, watcherScript, repoRoot),
		fmt.Sprintf("0 * * * * cd %s && %s",
			repoRoot, huntScript),
		fmt.Sprintf("0 2 * * * cd %s && claude -p \"/hunt since-last-tag\" >> %s/.grava/hunt.log 2>&1",
			repoRoot, repoRoot),
	}

	for _, l := range lines {
		fmt.Fprintln(cmd.OutOrStdout(), "      "+l)
	}
	return nil
}

func gitRepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func init() {
	bootstrapCmd.Flags().Bool("print-cron", false, "Emit only the cron lines (no banner; suitable for pipe)")
	bootstrapCmd.Flags().Bool("skip-git-hooks", false, "Skip step 3 (git hook installation)")
	bootstrapCmd.Flags().Bool("skip-binary-check", false, "Skip step 1 (grava binary on PATH check)")
	rootCmd.AddCommand(bootstrapCmd)
}
