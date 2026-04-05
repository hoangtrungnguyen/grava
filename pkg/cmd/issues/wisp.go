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

// WispWriteParams holds the input for wispWrite.
type WispWriteParams struct {
	IssueID string
	Key     string
	Value   string
	Actor   string
}

// WispWriteResult is the JSON output for a successful wisp write.
type WispWriteResult struct {
	IssueID   string `json:"issue_id"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	WrittenBy string `json:"written_by"`
}

// WispEntry is the JSON output for a single wisp entry.
type WispEntry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	WrittenBy string    `json:"written_by"`
	WrittenAt time.Time `json:"written_at"`
}

// wispWrite upserts a key-value pair into the wisp_entries table and updates
// wisp_heartbeat_at on the issue row. Wrapped in WithAuditedTx for atomicity.
func wispWrite(ctx context.Context, store dolt.Store, params WispWriteParams) (WispWriteResult, error) {
	err := dolt.WithAuditedTx(ctx, store, []dolt.AuditEvent{
		{
			IssueID:   params.IssueID,
			EventType: dolt.EventWispWrite,
			Actor:     params.Actor,
			NewValue:  map[string]any{"key": params.Key, "value": params.Value},
		},
	}, func(tx *sql.Tx) error {
		// Verify issue exists.
		var issueID string
		row := tx.QueryRowContext(ctx, "SELECT id FROM issues WHERE id = ?", params.IssueID)
		if err := row.Scan(&issueID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return gravaerrors.New("ISSUE_NOT_FOUND",
					fmt.Sprintf("issue %s not found", params.IssueID), err)
			}
			return gravaerrors.New("DB_UNREACHABLE",
				fmt.Sprintf("failed to read issue %s", params.IssueID), err)
		}

		// Upsert the wisp entry.
		_, err := tx.ExecContext(ctx,
			`INSERT INTO wisp_entries (issue_id, key_name, value, written_by)
			 VALUES (?, ?, ?, ?)
			 ON DUPLICATE KEY UPDATE value = VALUES(value), written_by = VALUES(written_by), written_at = NOW()`,
			params.IssueID, params.Key, params.Value, params.Actor,
		)
		if err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to write wisp entry", err)
		}

		// Update wisp_heartbeat_at on the issue row.
		_, err = tx.ExecContext(ctx,
			"UPDATE issues SET wisp_heartbeat_at = NOW() WHERE id = ?",
			params.IssueID,
		)
		if err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to update wisp_heartbeat_at", err)
		}

		return nil
	})
	if err != nil {
		return WispWriteResult{}, err
	}
	return WispWriteResult{
		IssueID:   params.IssueID,
		Key:       params.Key,
		Value:     params.Value,
		WrittenBy: params.Actor,
	}, nil
}

// wispRead reads one or all wisp entries for an issue.
// If key is non-empty, returns a single *WispEntry; otherwise returns []WispEntry.
// Returns ISSUE_NOT_FOUND if the issue does not exist.
// Returns WISP_NOT_FOUND if a specific key is requested but not present.
func wispRead(ctx context.Context, store dolt.Store, issueID, key string) (any, error) {
	// Verify issue exists.
	var existingID string
	row := store.QueryRow("SELECT id FROM issues WHERE id = ?", issueID)
	if err := row.Scan(&existingID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, gravaerrors.New("ISSUE_NOT_FOUND",
				fmt.Sprintf("issue %s not found", issueID), err)
		}
		return nil, gravaerrors.New("DB_UNREACHABLE",
			fmt.Sprintf("failed to read issue %s", issueID), err)
	}

	if key != "" {
		// Single key lookup.
		var entry WispEntry
		r := store.QueryRow(
			`SELECT key_name, value, written_by, written_at
			 FROM wisp_entries WHERE issue_id = ? AND key_name = ?`,
			issueID, key,
		)
		if err := r.Scan(&entry.Key, &entry.Value, &entry.WrittenBy, &entry.WrittenAt); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, gravaerrors.New("WISP_NOT_FOUND",
					fmt.Sprintf("no wisp entry found for key %q on issue %s", key, issueID), err)
			}
			return nil, gravaerrors.New("DB_UNREACHABLE", "failed to read wisp entry", err)
		}
		return &entry, nil
	}

	// All entries.
	rows, err := store.Query(
		`SELECT key_name, value, written_by, written_at
		 FROM wisp_entries WHERE issue_id = ? ORDER BY written_at`,
		issueID,
	)
	if err != nil {
		return nil, gravaerrors.New("DB_UNREACHABLE", "failed to read wisp entries", err)
	}
	defer rows.Close() //nolint:errcheck

	entries := []WispEntry{}
	for rows.Next() {
		var e WispEntry
		if err := rows.Scan(&e.Key, &e.Value, &e.WrittenBy, &e.WrittenAt); err != nil {
			return nil, gravaerrors.New("DB_UNREACHABLE", "failed to scan wisp entry", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, gravaerrors.New("DB_UNREACHABLE", "error reading wisp rows", err)
	}
	return entries, nil
}

// newWispCmd builds the `grava wisp` parent command with write and read subcommands.
func newWispCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wisp",
		Short: "Read and write ephemeral wisp state for an issue",
		Long:  `Wisp stores key-value checkpoints on an issue so agents can resume after a crash.`,
	}
	cmd.AddCommand(newWispWriteCmd(d))
	cmd.AddCommand(newWispReadCmd(d))
	return cmd
}

func newWispWriteCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "write <issue-id> <key> <value>",
		Short: "Write a key-value pair to an issue's wisp store",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			actor, _ := cmd.Flags().GetString("actor")
			if actor == "" {
				actor = *d.Actor
			}
			result, err := wispWrite(ctx, *d.Store, WispWriteParams{
				IssueID: args[0],
				Key:     args[1],
				Value:   args[2],
				Actor:   actor,
			})
			if err != nil {
				if *d.OutputJSON {
					return writeJSONError(cmd, err)
				}
				return err
			}
			if *d.OutputJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Wisp written: %s[%s] = %q (by %s)\n",
				result.IssueID, result.Key, result.Value, result.WrittenBy)
			return nil
		},
	}
	cmd.Flags().String("actor", "", "Override the actor identity for this write")
	return cmd
}

func newWispReadCmd(d *cmddeps.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "read <issue-id> [key]",
		Short: "Read wisp entries for an issue",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			issueID := args[0]
			key := ""
			if len(args) == 2 {
				key = args[1]
			}
			result, err := wispRead(ctx, *d.Store, issueID, key)
			if err != nil {
				if *d.OutputJSON {
					return writeJSONError(cmd, err)
				}
				return err
			}
			if *d.OutputJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			switch v := result.(type) {
			case *WispEntry:
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s = %q (by %s at %s)\n",
					issueID, v.Key, v.Value, v.WrittenBy, v.WrittenAt.Format(time.RFC3339))
			case []WispEntry:
				if len(v) == 0 {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "(no wisp entries)")
					return nil
				}
				for _, e := range v {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s = %q (by %s at %s)\n",
						e.Key, e.Value, e.WrittenBy, e.WrittenAt.Format(time.RFC3339))
				}
			}
			return nil
		},
	}
}
