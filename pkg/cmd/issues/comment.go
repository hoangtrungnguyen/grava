package issues

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/spf13/cobra"
)

// CommentParams holds all inputs for the commentIssue named function.
type CommentParams struct {
	ID      string
	Message string
	Actor   string
	Model   string
}

// CommentResult is the JSON output model for the comment command (NFR5).
type CommentResult struct {
	ID        string `json:"id"`
	CommentID int64  `json:"comment_id"`
	Message   string `json:"message"`
	Actor     string `json:"actor"`
	CreatedAt string `json:"created_at"`
}

// commentIssue appends a timestamped comment to the issue_comments table.
// The operation is wrapped in WithAuditedTx for audit traceability.
func commentIssue(ctx context.Context, store dolt.Store, params CommentParams) (CommentResult, error) {
	if params.Message == "" {
		return CommentResult{}, gravaerrors.New("MISSING_REQUIRED_FIELD",
			"--message is required", nil)
	}

	if err := guardNotArchived(store, params.ID); err != nil {
		return CommentResult{}, err
	}

	now := time.Now()
	var commentID int64

	err := dolt.WithAuditedTx(ctx, store, []dolt.AuditEvent{
		{
			IssueID:   params.ID,
			EventType: dolt.EventComment,
			Actor:     params.Actor,
			Model:     params.Model,
			NewValue:  map[string]any{"message": params.Message},
		},
	}, func(tx *sql.Tx) error {
		// Pre-read: validate issue existence
		var exists string
		row := tx.QueryRowContext(ctx, "SELECT id FROM issues WHERE id = ?", params.ID)
		if err := row.Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return gravaerrors.New("ISSUE_NOT_FOUND",
					fmt.Sprintf("Issue %s not found", params.ID), nil)
			}
			return gravaerrors.New("DB_UNREACHABLE", "failed to read issue", err)
		}

		// Insert comment
		result, err := tx.ExecContext(ctx,
			"INSERT INTO issue_comments (issue_id, message, actor, agent_model, created_at) VALUES (?, ?, ?, ?, ?)",
			params.ID, params.Message, params.Actor, params.Model, now,
		)
		if err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to insert comment", err)
		}

		commentID, _ = result.LastInsertId()
		return nil
	})
	if err != nil {
		return CommentResult{}, err
	}

	return CommentResult{
		ID:        params.ID,
		CommentID: commentID,
		Message:   params.Message,
		Actor:     params.Actor,
		CreatedAt: now.Format(time.RFC3339),
	}, nil
}

// newCommentCmd builds the `grava comment` cobra command.
// Interface: grava comment <id> --message "text" OR grava comment <id> "text" (backward-compat)
func newCommentCmd(d *cmddeps.Deps) *cobra.Command {
	var message string

	cmd := &cobra.Command{
		Use:   "comment <id> [text]",
		Short: "Append a comment to an issue",
		Long: `Append a comment to an existing issue.

Comments are stored in the issue_comments table with timestamps and actor info.

Examples:
  grava comment grava-abc --message "Investigated root cause, see PR #42"
  grava comment grava-abc -m "Fix confirmed"
  grava comment grava-abc "Backward-compatible positional text"`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			// Resolve message: prefer --message flag, fall back to positional arg
			msg := message
			if msg == "" && len(args) == 2 {
				msg = args[1]
			}

			result, err := commentIssue(cmd.Context(), *d.Store, CommentParams{
				ID:      id,
				Message: msg,
				Actor:   *d.Actor,
				Model:   *d.AgentModel,
			})
			if err != nil {
				if *d.OutputJSON {
					return writeJSONError(cmd, err)
				}
				return err
			}

			// Handle --last-commit (orthogonal to comments table, writes to metadata)
			if cmd.Flags().Changed("last-commit") {
				if err := setLastCommit(d, id, commentLastCommit); err != nil {
					return err
				}
			}

			if *d.OutputJSON {
				b, _ := json.MarshalIndent(result, "", "  ") //nolint:errcheck
				fmt.Fprintln(cmd.OutOrStdout(), string(b))   //nolint:errcheck
				return nil
			}

			cmd.Printf("💬 Comment added to %s\n", id)
			return nil
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Comment text")
	cmd.Flags().StringVar(&commentLastCommit, "last-commit", "", "Store the last session's commit hash")
	return cmd
}
