package issues

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/spf13/cobra"
)

// newUndoCmd represents the undo command
func newUndoCmd(d *cmddeps.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "undo <id>",
		Short: "Revert the last change to an issue",
		Long: `Revert the issue to its previous state.
If the issue has uncommitted changes, it reverts to the last committed state (HEAD).
If the issue is clean (matches HEAD), it reverts to the previous commit (HEAD~1).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			store := *d.Store

			// Struct to hold state for comparison
			type IssueState struct {
				Title         string
				Description   string
				Type          string
				Priority      int
				Status        string
				AffectedFiles sql.NullString
			}

			// 1. Get Current State
			var current IssueState
			// We query raw SQL to get even tombstoned issues
			currQuery := `SELECT title, description, issue_type, priority, status, affected_files
			              FROM issues WHERE id = ?`
			err := store.QueryRow(currQuery, id).Scan(
				&current.Title, &current.Description, &current.Type,
				&current.Priority, &current.Status, &current.AffectedFiles,
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
				SELECT title, description, issue_type, priority, status, affected_files
				FROM dolt_history_issues
				WHERE id = ?
				ORDER BY commit_date DESC
				LIMIT 2
			`
			rows, err := store.Query(histQuery, id)
			if err != nil {
				return fmt.Errorf("failed to fetch history: %w", err)
			}
			defer func() { _ = rows.Close() }()

			var history []IssueState
			for rows.Next() {
				var h IssueState
				if err := rows.Scan(
					&h.Title, &h.Description, &h.Type,
					&h.Priority, &h.Status, &h.AffectedFiles,
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

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), actionMsg)

			// 3.5 Check for Session Undo (Git-level Revert)
			if !isDirty {
				var rawMeta sql.NullString
				if err := store.QueryRow("SELECT metadata FROM issues WHERE id = ?", id).Scan(&rawMeta); err == nil && rawMeta.Valid {
					var meta map[string]any
					if err := json.Unmarshal([]byte(rawMeta.String), &meta); err == nil {
						if lastCommit, ok := meta["last_commit"].(string); ok {
							_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Found last session commit: %s\n", lastCommit)
							_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Reverting session commit... ")
							_, err := store.Exec("CALL DOLT_REVERT(?)", lastCommit)
							if err != nil {
								// If revert fails (e.g. conflicts), we might want to fallback to row-level
								// but for now, let's report it as a fail or just continue.
								_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Revert failed: %v. Falling back to row-level undo.\n", err)
							} else {
								_, _ = fmt.Fprintln(cmd.OutOrStdout(), "DONE.")
								_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Session undo successful for %v.\n", id)
								return nil
							}
						}
					}
				}
			}

			// 4. Apply Revert (Row-level fallback)
			// We update the issue with the old values, but set updated_by to us
			updateQ := `
				UPDATE issues
				SET title = ?, description = ?, issue_type = ?, priority = ?, status = ?,
				    affected_files = ?,
				    updated_at = NOW(), updated_by = ?, agent_model = ?
				WHERE id = ?
			`

			res, err := store.Exec(updateQ,
				targetState.Title,
				targetState.Description,
				targetState.Type,
				targetState.Priority,
				targetState.Status,
				targetState.AffectedFiles,
				*d.Actor,      // updated_by (current user)
				*d.AgentModel, // agent_model (current agent)
				id,
			)
			if err != nil {
				return fmt.Errorf("failed to execute undo: %w", err)
			}

			rowsAff, _ := res.RowsAffected()
			if rowsAff == 0 {
				return fmt.Errorf("no rows updated (concurrency issue?)")
			}

			var commentMsg string
			if isDirty {
				commentMsg = "Undo: discarded uncommitted changes (reverted to HEAD)"
			} else {
				commentMsg = "Undo: reverted to PREVIOUS commit"
			}

			if _, err := commentIssue(context.Background(), store, CommentParams{
				ID:      id,
				Message: commentMsg,
				Actor:   *d.Actor,
				Model:   *d.AgentModel,
			}); err != nil {
				return fmt.Errorf("failed to add undo comment: %w", err)
			}

			// Print summary
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Reverted issue %s.\n", id)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   Title: %s\n", targetState.Title)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   Status: %s\n", targetState.Status)

			return nil
		},
	}
}
