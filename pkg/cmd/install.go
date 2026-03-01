package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Git hooks and merge driver",
	Long:  "Configures the local Git repository's .git/config, .gitattributes, and hooks.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Configure .git/config
		execCmd := exec.Command("git", "config", "merge.grava.name", "Grava JSONL Merge Driver")
		if err := execCmd.Run(); err != nil {
			return fmt.Errorf("failed to configure git merge.grava.name: %w", err)
		}

		execCmd = exec.Command("git", "config", "merge.grava.driver", "grava merge-slot --ancestor %O --current %A --other %B --output %A")
		if err := execCmd.Run(); err != nil {
			return fmt.Errorf("failed to configure git merge.grava.driver: %w", err)
		}

		// 2. Configure .gitattributes
		attrLine := "issues.jsonl merge=grava\n"
		var attrFile *os.File
		var err error

		if _, err = os.Stat(".gitattributes"); os.IsNotExist(err) {
			attrFile, err = os.Create(".gitattributes")
		} else {
			attrFile, err = os.OpenFile(".gitattributes", os.O_APPEND|os.O_WRONLY, 0644)
		}
		if err != nil {
			return fmt.Errorf("failed to open .gitattributes: %w", err)
		}
		defer attrFile.Close()

		// Read to see if already exists (simplified: just append if creating open failed, but we should actually read it in real robust implementation)
		// For now, we continually append.
		if _, err := attrFile.WriteString(attrLine); err != nil {
			return fmt.Errorf("failed to write to .gitattributes: %w", err)
		}

		// 3. Create hooks
		shared, _ := cmd.Flags().GetBool("shared")
		var hooksDir string
		if shared {
			hooksDir = filepath.Join(".grava", "hooks")
			execCmd = exec.Command("git", "config", "core.hooksPath", hooksDir)
			if err := execCmd.Run(); err != nil {
				return fmt.Errorf("failed to configure git core.hooksPath: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Configured core.hooksPath to", hooksDir)
		} else {
			hooksDir = filepath.Join(".git", "hooks")
		}

		if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
			if err := os.MkdirAll(hooksDir, 0755); err != nil {
				return fmt.Errorf("failed to create hooks directory: %w", err)
			}
		}

		preCommitContent := `#!/bin/sh
# grava-shim
grava hook run pre-commit
`
		postMergeContent := `#!/bin/sh
# grava-shim
grava hook run post-merge
`
		postCheckoutContent := `#!/bin/sh
# grava-shim
grava hook run post-checkout
`

		if err := installHookSafely(filepath.Join(hooksDir, "pre-commit"), preCommitContent, cmd); err != nil {
			return err
		}
		if err := installHookSafely(filepath.Join(hooksDir, "post-merge"), postMergeContent, cmd); err != nil {
			return err
		}
		if err := installHookSafely(filepath.Join(hooksDir, "post-checkout"), postCheckoutContent, cmd); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "✅ Grava Git integration installed successfully")
		return nil
	},
}

func installHookSafely(path, content string, cmd *cobra.Command) error {
	// 1. Check if the hook exists
	if _, err := os.Stat(path); err == nil {
		// Hook exists. Check if it's already a grava shim.
		existingContent, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read existing hook %s: %w", path, err)
		}

		// If it's a grava shim, we can just overwrite it (it's safe).
		if !strings.Contains(string(existingContent), "# grava-shim") {
			// It's a user's custom hook or another tool's hook.
			// Rename it to .old so we can chain it.
			oldPath := path + ".old"

			// Don't overwrite an existing .old
			if _, err := os.Stat(oldPath); os.IsNotExist(err) {
				fmt.Fprintf(cmd.OutOrStdout(), "⚠️  Existing hook found at %s. Renaming to %s to preserve it.\n", path, oldPath)
				if err := os.Rename(path, oldPath); err != nil {
					return fmt.Errorf("failed to rename existing hook to %s: %w", oldPath, err)
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "⚠️  Existing non-Grava hook and .old hook both found at %s. Overwriting the primary hook, but keeping .old.\n", path)
			}
		}
	}

	// 2. Write the new grava shim
	err := os.WriteFile(path, []byte(content), 0755)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

func init() {
	installCmd.Flags().Bool("shared", false, "Install hooks to .grava/hooks and configure core.hooksPath for team sharing")
	rootCmd.AddCommand(installCmd)
}
