package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
		hooksDir := filepath.Join(".git", "hooks")
		if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
			if err := os.MkdirAll(hooksDir, 0755); err != nil {
				return fmt.Errorf("failed to create hooks directory: %w", err)
			}
		}

		preCommitContent := `#!/bin/sh
grava export --file issues.jsonl
git add issues.jsonl
`
		postMergeContent := `#!/bin/sh
grava import --file issues.jsonl --overwrite
`
		postCheckoutContent := `#!/bin/sh
grava import --file issues.jsonl --overwrite
`

		if err := writeExecutableFile(filepath.Join(hooksDir, "pre-commit"), preCommitContent); err != nil {
			return err
		}
		if err := writeExecutableFile(filepath.Join(hooksDir, "post-merge"), postMergeContent); err != nil {
			return err
		}
		if err := writeExecutableFile(filepath.Join(hooksDir, "post-checkout"), postCheckoutContent); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "✅ Grava Git integration installed successfully")
		return nil
	},
}

func writeExecutableFile(path, content string) error {
	err := os.WriteFile(path, []byte(content), 0755)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(installCmd)
}
