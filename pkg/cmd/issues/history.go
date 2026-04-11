package issues

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/spf13/cobra"
)

// HistoryEntry is the JSON output model for a single event in the issue progression log.
type HistoryEntry struct {
	EventType string         `json:"event_type"`
	Actor     string         `json:"actor"`
	Timestamp time.Time      `json:"timestamp"`
	Details   map[string]any `json:"details"`
}

// issueHistory retrieves the ordered progression log of an issue from the events table.
// If since is non-empty, only events after that timestamp are returned.
// Returns ISSUE_NOT_FOUND if the issue does not exist.
// If the issue exists but has no events, returns an empty slice (AC#5).
func issueHistory(ctx context.Context, store dolt.Store, issueID, since string) ([]HistoryEntry, error) {
	// Verify issue exists (AC#6: non-existent issue returns error).
	var existingID string
	row := store.QueryRowContext(ctx, "SELECT id FROM issues WHERE id = ?", issueID)
	if err := row.Scan(&existingID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, gravaerrors.New("ISSUE_NOT_FOUND",
				fmt.Sprintf("issue %s not found", issueID), err)
		}
		return nil, gravaerrors.New("DB_UNREACHABLE",
			fmt.Sprintf("failed to read issue %s", issueID), err)
	}

	// Build query with optional date filter.
	query := `SELECT event_type, actor, old_value, new_value, timestamp
	          FROM events WHERE issue_id = ?`
	args := []any{issueID}

	if since != "" {
		sinceTime, err := parseSinceDate(since)
		if err != nil {
			return nil, gravaerrors.New("INVALID_DATE",
				fmt.Sprintf("invalid --since date %q: expected YYYY-MM-DD or RFC3339", since), err)
		}
		query += " AND timestamp >= ?"
		args = append(args, sinceTime)
	}

	query += " ORDER BY timestamp ASC, id ASC"

	rows, err := store.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, gravaerrors.New("DB_UNREACHABLE", "failed to read history", err)
	}
	defer rows.Close() //nolint:errcheck

	entries := make([]HistoryEntry, 0, 100)
	for rows.Next() {
		var eventType, actor string
		var oldValueJSON, newValueJSON sql.NullString
		var ts time.Time

		if err := rows.Scan(&eventType, &actor, &oldValueJSON, &newValueJSON, &ts); err != nil {
			return nil, gravaerrors.New("DB_UNREACHABLE", "failed to scan history row", err)
		}

		details := mergeEventDetails(oldValueJSON, newValueJSON)

		entries = append(entries, HistoryEntry{
			EventType: eventType,
			Actor:     actor,
			Timestamp: ts,
			Details:   details,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, gravaerrors.New("DB_UNREACHABLE", "error reading history rows", err)
	}

	return entries, nil
}

// parseSinceDate tries RFC3339, then date-only format.
func parseSinceDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("cannot parse %q as date", s)
}

func parseJSONToMap(s sql.NullString) (map[string]any, error) {
	if !s.Valid || s.String == "" || s.String == "{}" {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s.String), &m); err == nil {
		return m, nil
	}
	// Fallback for valid JSON that is not an object (e.g. array, string)
	var fallback any
	if err := json.Unmarshal([]byte(s.String), &fallback); err == nil {
		return map[string]any{"value": fallback}, nil
	}
	// Return error for corrupted JSON
	return nil, gravaerrors.New("CORRUPTED_JSON",
		fmt.Sprintf("invalid JSON in events table: %s", s.String), nil)
}

// mergeEventDetails combines old_value and new_value JSON into a single details map.
// Returns new_value fields, with old_value fields prefixed with "old_" when both exist.
// If JSON parsing fails, includes error details in the map.
func mergeEventDetails(oldJSON, newJSON sql.NullString) map[string]any {
	details := make(map[string]any)

	oldMap, oldErr := parseJSONToMap(oldJSON)
	newMap, newErr := parseJSONToMap(newJSON)

	// Record any parsing errors in output for visibility.
	if oldErr != nil {
		details["_old_value_error"] = oldErr.Error()
	}
	if newErr != nil {
		details["_new_value_error"] = newErr.Error()
	}

	// If both exist, show old_ prefixed and new values.
	if len(oldMap) > 0 && len(newMap) > 0 {
		for k, v := range oldMap {
			details["old_"+k] = v
		}
		for k, v := range newMap {
			details[k] = v
		}
		return details
	}

	// If only new_value exists, use it directly.
	if len(newMap) > 0 {
		return newMap
	}

	// If only old_value exists, use it directly.
	if len(oldMap) > 0 {
		return oldMap
	}

	return details
}

func newHistoryCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history <issue-id>",
		Short: "Show the event progression log of an issue",
		Long:  `Display the ordered audit trail for an issue: status changes, claims, wisp writes, comments, and label changes.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			issueID := args[0]
			since, _ := cmd.Flags().GetString("since")

			entries, err := issueHistory(ctx, *d.Store, issueID, since)
			if err != nil {
				if *d.OutputJSON {
					return writeJSONError(cmd, err)
				}
				return err
			}

			if *d.OutputJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(entries)
			}

			// Human-readable output.
			if len(entries) == 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No events found for issue %s\n", issueID)
				return nil
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "History for %s (%d events):\n\n", issueID, len(entries))
			for _, e := range entries {
				detailStr := formatDetails(e.Details)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s  %-14s  %-20s  %s\n",
					e.Timestamp.Format("2006-01-02 15:04:05"),
					e.EventType,
					e.Actor,
					detailStr,
				)
			}
			return nil
		},
	}
	cmd.Flags().String("since", "", "Filter events after this date (YYYY-MM-DD or RFC3339)")
	return cmd
}

// formatDetails renders a details map as a compact key=value string with sorted keys.
func formatDetails(d map[string]any) string {
	if len(d) == 0 {
		return ""
	}
	keys := make([]string, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		b, err := json.Marshal(d[k])
		if err != nil {
			parts = append(parts, k+"=<error>")
			continue
		}
		parts = append(parts, k+"="+string(b))
	}
	return fmt.Sprintf("{%s}", strings.Join(parts, ", "))
}

