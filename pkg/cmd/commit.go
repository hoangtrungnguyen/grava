package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var commitMessage string

// commitCmd represents the commit command
var commitCmd = &cobra.Command{
	Use:   "commit -m <message>",
	Short: "Commit changes to the Dolt database",
	Long:  `Commit all staged changes (all modified issues and dependencies) to the Dolt version history.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if commitMessage == "" {
			return fmt.Errorf("commit message is required (use -m)")
		}

		// Run dolt_commit via SQL
		// 1. Stage all changes (including new tables/rows)
		var err error
		_, err = Store.Exec("CALL DOLT_ADD('-A')")
		if err != nil {
			return fmt.Errorf("failed to stage changes: %w", err)
		}

		// 2. Commit
		query := "CALL DOLT_COMMIT('-m', ?)"
		var hash string
		err = Store.QueryRow(query, commitMessage).Scan(&hash)
		if err != nil {
			return fmt.Errorf("failed to commit: %w", err)
		}

		cmd.Printf("✅ Committed changes. Hash: %s\n", hash)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(commitCmd)
	commitCmd.Flags().StringVarP(&commitMessage, "message", "m", "", "Commit message")
	_ = commitCmd.MarkFlagRequired("message")
}
