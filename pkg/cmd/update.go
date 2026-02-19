package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/validation"
	"github.com/spf13/cobra"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an existing issue",
	Long: `Update specific fields of an existing issue.
Only the flags provided will be updated.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		// Build dynamic query
		query := "UPDATE issues SET updated_at = ?, updated_by = ?, agent_model = ?"
		queryParams := []any{time.Now(), actor, agentModel}

		if cmd.Flags().Changed("title") {
			val, _ := cmd.Flags().GetString("title")
			query += ", title = ?"
			queryParams = append(queryParams, val)
		}
		if cmd.Flags().Changed("desc") {
			val, _ := cmd.Flags().GetString("desc")
			query += ", description = ?"
			queryParams = append(queryParams, val)
		}
		if cmd.Flags().Changed("type") {
			val, _ := cmd.Flags().GetString("type")
			if err := validation.ValidateIssueType(val); err != nil {
				return err
			}
			query += ", issue_type = ?"
			queryParams = append(queryParams, val)
		}
		if cmd.Flags().Changed("priority") {
			query += ", priority = ?"
			val, _ := cmd.Flags().GetString("priority")
			pInt, err := validation.ValidatePriority(val)
			if err != nil {
				return err
			}
			queryParams = append(queryParams, pInt)
		}
		if cmd.Flags().Changed("status") {
			val, _ := cmd.Flags().GetString("status")
			if err := validation.ValidateStatus(val); err != nil {
				return err
			}
			query += ", status = ?"
			queryParams = append(queryParams, val)
		}
		if cmd.Flags().Changed("files") {
			query += ", affected_files = ?"
			// Use global slice var
			val := updateAffectedFiles
			b, _ := json.Marshal(val)
			queryParams = append(queryParams, string(b))
		}

		query += " WHERE id = ?"
		queryParams = append(queryParams, id)

		// 1. Regular database update
		result, err := Store.Exec(query, queryParams...)
		if err != nil {
			return fmt.Errorf("failed to update issue %s: %w", id, err)
		}

		// 2. Metadata update (last-commit)
		if cmd.Flags().Changed("last-commit") {
			val, _ := cmd.Flags().GetString("last-commit")
			if err := setLastCommit(id, val); err != nil {
				return err
			}
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}

		if rowsAffected == 0 && !cmd.Flags().Changed("last-commit") {
			return fmt.Errorf("issue %s not found or no changes made", id)
		}

		if outputJSON {
			resp := map[string]string{
				"id":     id,
				"status": "updated",
			}
			b, _ := json.MarshalIndent(resp, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "✅ Updated issue %s\n", id)
		return nil
	},
}

var updateAffectedFiles []string

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().StringP("title", "t", "", "Update title")
	updateCmd.Flags().StringP("desc", "d", "", "Update description")
	updateCmd.Flags().String("type", "", "Update type")
	updateCmd.Flags().StringP("priority", "p", "", "Update priority")
	updateCmd.Flags().StringP("status", "s", "", "Update status")
	updateCmd.Flags().StringSliceVar(&updateAffectedFiles, "files", []string{}, "Update affected files")
	updateCmd.Flags().String("last-commit", "", "Store the last session's commit hash")
}
