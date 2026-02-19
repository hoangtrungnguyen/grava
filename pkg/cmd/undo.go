package cmd

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// undoCmd represents the undo command
var undoCmd = &cobra.Command{
	Use:   "undo <id>",
	Short: "Revert the last change to an issue",
	Long: `Revert the issue to its previous state.
If the issue has uncommitted changes, it reverts to the last committed state (HEAD).
If the issue is clean (matches HEAD), it reverts to the previous commit (HEAD~1).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		// Struct to hold state for comparison
		type IssueState struct {
			Title         string
			Description   string
			Type          string
			Priority      int
			Status        string
			AffectedFiles sql.NullString
			UpdatedAt     time.Time // Added UpdatedAt field
		}

		// 1. Get Current State
		var current IssueState
		// We query raw SQL to get even tombstoned issues
		currQuery := `SELECT title, description, issue_type, priority, status, affected_files, updated_at
		              FROM issues WHERE id = ?`
		err := Store.QueryRow(currQuery, id).Scan(
			&current.Title, &current.Description, &current.Type,
			&current.Priority, &current.Status, &current.AffectedFiles, &current.UpdatedAt, // Added &current.UpdatedAt
		)
		if err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("issue %s not found", id)
			}
			return fmt.Errorf("failed to fetch current state: %w", err)
		}

		// 2. Get History (Last 2 commits)
		// We need full details to restore
		histQuery := `
			SELECT title, description, issue_type, priority, status, affected_files, updated_at
			FROM dolt_history_issues
			WHERE id = ?
			ORDER BY commit_date DESC
			LIMIT 2
		`
		rows, err := Store.Query(histQuery, id)
		if err != nil {
			return fmt.Errorf("failed to fetch history: %w", err)
		}
		defer rows.Close()

		var history []IssueState
		for rows.Next() {
			var h IssueState
			if err := rows.Scan(
				&h.Title, &h.Description, &h.Type,
				&h.Priority, &h.Status, &h.AffectedFiles, &h.UpdatedAt, // Added &h.UpdatedAt
			); err != nil {
				return fmt.Errorf("failed to scan history: %w", err)
			}
			history = append(history, h)
		}

		if len(history) == 0 {
			return fmt.Errorf("no history found for issue %s (is it committed?)", id)
		}

		// 3. Determine Target State
		var targetState IssueState
		var actionMsg string

		// Compare Current vs Head (history[0])
		isDirty := current != history[0]

		if isDirty {
			actionMsg = "Discarding uncommitted changes (reverting to HEAD)..."
			targetState = history[0]
		} else {
			if len(history) < 2 {
				return fmt.Errorf("cannot undo: issue is in its initial state (no previous commit)")
			}
			actionMsg = "Issue is clean. Reverting to PREVIOUS commit..."
			targetState = history[1]
		}

		fmt.Println(actionMsg)

		// 4. Apply Revert
		// We update the issue with the old values, but set updated_by to us
		updateQ := `
			UPDATE issues
			SET title = ?, description = ?, issue_type = ?, priority = ?, status = ?,
			    affected_files = ?,
			    updated_at = NOW(), updated_by = ?, agent_model = ?
			WHERE id = ?
		`

		res, err := Store.Exec(updateQ,
			targetState.Title,
			targetState.Description,
			targetState.Type,
			targetState.Priority,
			targetState.Status,
			targetState.AffectedFiles,
			actor,      // updated_by (current user)
			agentModel, // agent_model (current agent)
			id,
		)
		if err != nil {
			return fmt.Errorf("failed to execute undo: %w", err)
		}

		rowsAff, _ := res.RowsAffected()
		if rowsAff == 0 {
			return fmt.Errorf("no rows updated (concurrency issue?)")
		}

		// Print summary
		fmt.Printf("✅ Reverted issue %s.\n", id)
		fmt.Printf("   Title: %s\n", targetState.Title)
		fmt.Printf("   Status: %s\n", targetState.Status)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(undoCmd)
}
