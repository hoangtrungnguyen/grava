package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/merge"
	"github.com/spf13/cobra"
)

// conflictsCmd is the parent for conflict management sub-commands.
// Conflicts are written to .grava/conflicts.json by the merge-driver (file-based,
// no DB required). resolve/dismiss also persist to conflict_records DB table
// when a Store connection is available.
var conflictsCmd = &cobra.Command{
	Use:   "conflicts",
	Short: "View and resolve merge conflicts",
	Long: `List, resolve, or dismiss merge conflicts recorded by the grava-merge driver.

Conflict records are stored in .grava/conflicts.json (written by grava merge-driver)
and, when a DB connection is available, persisted to the conflict_records table.`,
}

// conflictsListCmd shows pending conflict records.
var conflictsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending merge conflicts",
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, _, err := readConflicts()
		if err != nil {
			return err
		}

		// Sync pending entries to DB for audit (non-fatal, best-effort).
		if Store != nil {
			importPendingConflicts(cmd.Context(), entries)
		}

		var pending []merge.ConflictEntry
		for _, e := range entries {
			if !e.Resolved {
				pending = append(pending, e)
			}
		}

		if outputJSON {
			if pending == nil {
				pending = []merge.ConflictEntry{}
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(pending)
		}

		if len(pending) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No pending conflicts.")
			return nil
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d pending conflict(s):\n\n", len(pending))
		for _, e := range pending {
			field := e.Field
			if field == "" {
				field = "(whole issue)"
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"  ID:       %s\n  Issue:    %s\n  Field:    %s\n  Local:    %s\n  Remote:   %s\n  Detected: %s\n\n",
				e.ID, e.IssueID, field,
				string(e.Local), string(e.Remote),
				e.DetectedAt.Format(time.RFC3339))
		}
		return nil
	},
}

var conflictsResolveAccept string

// conflictsResolveCmd applies a resolution to a pending conflict.
var conflictsResolveCmd = &cobra.Command{
	Use:   "resolve <conflict-id>",
	Short: "Resolve a conflict by accepting ours or theirs",
	Long: `Apply a resolution to a pending conflict.

  --accept=ours   — keep the value from the current branch (%A)
  --accept=theirs — take the value from the other branch (%B)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if conflictsResolveAccept != "ours" && conflictsResolveAccept != "theirs" {
			return fmt.Errorf("--accept must be 'ours' or 'theirs'")
		}
		conflictID := args[0]

		entries, conflictsPath, err := readConflicts()
		if err != nil {
			return err
		}

		target, err := findConflictEntry(entries, conflictID)
		if err != nil {
			return err
		}
		if target.Resolved {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Conflict %s is already resolved.\n", conflictID)
			return nil
		}

		var winner json.RawMessage
		if conflictsResolveAccept == "ours" {
			winner = target.Local
		} else {
			winner = target.Remote
		}

		if err := applyConflictResolution(target, winner); err != nil {
			return fmt.Errorf("failed to apply resolution: %w", err)
		}

		target.Resolved = true
		if err := writeConflicts(conflictsPath, entries); err != nil {
			return fmt.Errorf("failed to update conflicts file: %w", err)
		}

		// Persist resolution to DB (non-fatal).
		if Store != nil {
			persistConflictResolution(cmd.Context(), target, conflictsResolveAccept)
		}

		if outputJSON {
			resp := map[string]interface{}{
				"id":         target.ID,
				"issue_id":   target.IssueID,
				"field":      target.Field,
				"resolution": conflictsResolveAccept,
				"status":     "resolved",
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(resp)
		}

		field := target.Field
		if field == "" {
			field = "(whole issue)"
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(),
			"✅ Resolved conflict %s: issue %s field %q → %s\n",
			conflictID, target.IssueID, field, conflictsResolveAccept)
		return nil
	},
}

// conflictsDismissCmd marks a conflict as dismissed (acknowledged, not patched).
var conflictsDismissCmd = &cobra.Command{
	Use:   "dismiss <conflict-id>",
	Short: "Dismiss a conflict without applying a resolution",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conflictID := args[0]

		entries, conflictsPath, err := readConflicts()
		if err != nil {
			return err
		}

		target, err := findConflictEntry(entries, conflictID)
		if err != nil {
			return err
		}
		if target.Resolved {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Conflict %s is already resolved/dismissed.\n", conflictID)
			return nil
		}

		target.Resolved = true
		if err := writeConflicts(conflictsPath, entries); err != nil {
			return fmt.Errorf("failed to update conflicts file: %w", err)
		}

		// Persist dismissal to DB (non-fatal).
		if Store != nil {
			persistConflictDismissal(cmd.Context(), target)
		}

		if outputJSON {
			resp := map[string]interface{}{
				"id":       target.ID,
				"issue_id": target.IssueID,
				"status":   "dismissed",
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(resp)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(),
			"✅ Dismissed conflict %s (issue %s).\n", conflictID, target.IssueID)
		return nil
	},
}

// findConflictEntry returns a pointer to the ConflictEntry with the given ID.
func findConflictEntry(entries []merge.ConflictEntry, id string) (*merge.ConflictEntry, error) {
	for i := range entries {
		if entries[i].ID == id {
			return &entries[i], nil
		}
	}
	return nil, fmt.Errorf("conflict %q not found", id)
}

// persistConflictResolution upserts the conflict into conflict_records with status=resolved.
func persistConflictResolution(ctx context.Context, e *merge.ConflictEntry, resolution string) {
	now := time.Now().UTC()
	_, _ = Store.ExecContext(ctx, `
		INSERT INTO conflict_records (id, issue_id, field, local_val, remote_val, status, detected_at, resolved_at, resolution)
		VALUES (?, ?, ?, ?, ?, 'resolved', ?, ?, ?)
		ON DUPLICATE KEY UPDATE status='resolved', resolved_at=?, resolution=?`,
		e.ID, e.IssueID, e.Field, string(e.Local), string(e.Remote),
		e.DetectedAt, now, resolution,
		now, resolution,
	)
}

// persistConflictDismissal upserts the conflict into conflict_records with status=dismissed.
func persistConflictDismissal(ctx context.Context, e *merge.ConflictEntry) {
	now := time.Now().UTC()
	_, _ = Store.ExecContext(ctx, `
		INSERT INTO conflict_records (id, issue_id, field, local_val, remote_val, status, detected_at, resolved_at, resolution)
		VALUES (?, ?, ?, ?, ?, 'dismissed', ?, ?, 'dismissed')
		ON DUPLICATE KEY UPDATE status='dismissed', resolved_at=?, resolution='dismissed'`,
		e.ID, e.IssueID, e.Field, string(e.Local), string(e.Remote),
		e.DetectedAt, now,
		now,
	)
}

// importPendingConflicts bulk-inserts pending entries from conflicts.json to DB.
// Idempotent: uses INSERT IGNORE so existing records are not overwritten.
func importPendingConflicts(ctx context.Context, entries []merge.ConflictEntry) {
	for _, e := range entries {
		if e.Resolved {
			continue
		}
		_, _ = Store.ExecContext(ctx, `
			INSERT IGNORE INTO conflict_records (id, issue_id, field, local_val, remote_val, status, detected_at)
			VALUES (?, ?, ?, ?, ?, 'pending', ?)`,
			e.ID, e.IssueID, e.Field, string(e.Local), string(e.Remote), e.DetectedAt,
		)
	}
}

func init() {
	conflictsCmd.AddCommand(conflictsListCmd)
	conflictsCmd.AddCommand(conflictsResolveCmd)
	conflictsCmd.AddCommand(conflictsDismissCmd)
	rootCmd.AddCommand(conflictsCmd)

	conflictsResolveCmd.Flags().StringVar(&conflictsResolveAccept, "accept", "",
		"Resolution: 'ours' (current branch) or 'theirs' (other branch)")
	_ = conflictsResolveCmd.MarkFlagRequired("accept")
}
