package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	dropForce bool

	// stdinReader is the reader used for interactive confirmation prompts.
	// Override in tests to inject a fake reader.
	stdinReader io.Reader = os.Stdin
)

// dropCmd represents the drop command
var dropCmd = &cobra.Command{
	Use:   "drop",
	Short: "Delete ALL data from the Grava database (nuclear reset)",
	Long: `Drop deletes ALL data from every table in the Grava database.
This is a destructive, non-reversible operation intended for development
resets or clean-slate scenarios.

Tables are truncated in foreign-key-safe order:
  1. dependencies
  2. events
  3. deletions
  4. child_counters
  5. issues

Example:
  grava drop           # prompts for confirmation
  grava drop --force   # skip confirmation (for CI/scripts)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !dropForce {
			cmd.Print("⚠️  This will DELETE ALL DATA from the Grava database.\nType \"yes\" to confirm: ")

			scanner := bufio.NewScanner(stdinReader)
			scanner.Scan()
			answer := strings.TrimSpace(scanner.Text())

			if answer != "yes" {
				cmd.Println("Aborted. No data was deleted.")
				return fmt.Errorf("user cancelled drop operation")
			}
		}

		// FK-safe deletion order: children before parents
		tables := []string{
			"dependencies",
			"events",
			"deletions",
			"child_counters",
			"issues",
		}

		for _, table := range tables {
			_, err := Store.Exec(fmt.Sprintf("DELETE FROM %s", table))
			if err != nil {
				return fmt.Errorf("failed to delete from %s: %w", table, err)
			}
		}

		cmd.Println("💣 All Grava data has been dropped.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(dropCmd)
	dropCmd.Flags().BoolVar(&dropForce, "force", false, "Skip interactive confirmation prompt")
}
