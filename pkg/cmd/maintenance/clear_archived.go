package maintenance

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/spf13/cobra"
)

// clearArchivedIssues permanently deletes all archived issues from the database.
// Called by `grava clear` (no date flags). Tombstone records are written to the
// deletions table before hard delete. FK CASCADE handles cleanup of related rows.
func clearArchivedIssues(cmd *cobra.Command, d *cmddeps.Deps) error {
	store := *d.Store
	ctx := context.Background()

	// Collect archived issue IDs for tombstone records and audit events
	rows, err := store.Query("SELECT id FROM issues WHERE status = 'archived'")
	if err != nil {
		return gravaerrors.New("DB_UNREACHABLE", "failed to query archived issues", err)
	}
	defer rows.Close() //nolint:errcheck

	var archivedIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to scan archived issue", err)
		}
		archivedIDs = append(archivedIDs, id)
	}
	if err := rows.Err(); err != nil {
		return gravaerrors.New("DB_UNREACHABLE", "error reading archived issues", err)
	}

	if len(archivedIDs) == 0 {
		if *d.OutputJSON {
			resp := map[string]any{"status": "unchanged", "purged": 0}
			b, _ := json.MarshalIndent(resp, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
			return nil
		}
		cmd.Println("No archived issues to clear.")
		return nil
	}

	// Build audit events — one per purged issue
	auditEvents := make([]dolt.AuditEvent, 0, len(archivedIDs))
	for _, id := range archivedIDs {
		auditEvents = append(auditEvents, dolt.AuditEvent{
			IssueID:   id,
			EventType: dolt.EventClear,
			Actor:     *d.Actor,
			Model:     *d.AgentModel,
			OldValue:  map[string]any{"status": "archived"},
			NewValue:  map[string]any{"action": "hard_delete"},
		})
	}

	// Execute tombstone + delete inside a single audited transaction
	err = dolt.WithAuditedTx(ctx, store, auditEvents, func(tx *sql.Tx) error {
		now := time.Now()

		// Insert tombstone records into deletions table
		for _, id := range archivedIDs {
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO deletions (id, deleted_at, reason, actor) VALUES (?, ?, ?, ?)",
				id, now, "cleared archived issue", *d.Actor,
			); err != nil {
				return gravaerrors.New("DB_UNREACHABLE",
					fmt.Sprintf("failed to record tombstone for %s", id), err)
			}
		}

		// Hard delete all archived issues (FK CASCADE handles related rows)
		if _, err := tx.ExecContext(ctx, "DELETE FROM issues WHERE status = 'archived'"); err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to delete archived issues", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	purged := len(archivedIDs)

	if *d.OutputJSON {
		resp := map[string]any{
			"status": "cleared",
			"purged": purged,
			"ids":    archivedIDs,
		}
		b, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
		return nil
	}

	cmd.Printf("🗑️  Purged %d archived issue(s).\n", purged)
	return nil
}
