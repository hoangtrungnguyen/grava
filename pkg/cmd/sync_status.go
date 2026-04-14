package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// SyncStatusResult holds the data reported by grava sync-status.
type SyncStatusResult struct {
	FileExists      bool   `json:"file_exists"`
	FileHash        string `json:"file_hash"`        // SHA-256 of current issues.jsonl; "" if unreadable
	StoredHash      string `json:"stored_hash"`      // hash from .grava/last_import_hash; "" if missing
	FileChanged     bool   `json:"file_changed"`     // true when file_hash != stored_hash
	DoltAvailable   bool   `json:"dolt_available"`
	DoltUncommitted int    `json:"dolt_uncommitted"` // rows in dolt_status; -1 if unavailable
	Status          string `json:"status"`           // "in_sync" | "file_changed" | "dolt_dirty" | "file_changed_and_dolt_dirty" | "never_imported" | "no_file"
}

var syncStatusCmd = &cobra.Command{
	Use:   "sync-status",
	Short: "Show sync state between issues.jsonl and Dolt",
	Long: `Reports whether issues.jsonl and the Dolt database are in sync.

Checks performed:
  A — Content Hash: compares the current issues.jsonl SHA-256 with the hash
      stored in .grava/last_import_hash from the last successful hook sync.
  C — Dolt Uncommitted: queries dolt_status for any staged/unstaged changes
      that would be overwritten by the next hook sync.

Exit codes:
  0 — in_sync, no_file, or never_imported (informational, not errors)
  1 — file_changed, dolt_dirty, or file_changed_and_dolt_dirty`,
	RunE: func(cmd *cobra.Command, args []string) error {
		issuesPath := resolveIssuesFilePath()
		result := computeSyncStatus(issuesPath)

		if outputJSON, _ := cmd.Root().PersistentFlags().GetBool("json"); outputJSON {
			b, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
		} else {
			printSyncStatus(cmd, result, issuesPath)
		}

		// Non-zero exit when diverged so CI/scripts can detect problems.
		// never_imported and no_file are informational — not error states.
		if result.Status != "in_sync" && result.Status != "no_file" && result.Status != "never_imported" {
			return fmt.Errorf("sync status: %s", result.Status)
		}
		return nil
	},
}

// computeSyncStatus gathers all status fields. It is separate from the cobra
// RunE so tests can call it directly without constructing a cobra context.
func computeSyncStatus(issuesPath string) SyncStatusResult {
	r := SyncStatusResult{DoltUncommitted: -1}

	// Check file presence.
	if _, err := os.Stat(issuesPath); os.IsNotExist(err) {
		r.Status = "no_file"
		return r
	}
	r.FileExists = true

	// Check A: content hash.
	r.StoredHash = readLastImportHash()
	currentHash, err := hashFile(issuesPath)
	if err == nil {
		r.FileHash = currentHash
		r.FileChanged = currentHash != r.StoredHash
	}

	if r.StoredHash == "" && r.FileHash == "" {
		r.Status = "never_imported"
		return r
	}

	// Check C: Dolt uncommitted changes.
	// DoltAvailable is only set true when both the connection AND the
	// dolt_status query succeed; this keeps the struct in a consistent state
	// (no "available but count unknown" sentinel).
	store, err := connectDBFn()
	if err == nil {
		defer store.Close() //nolint:errcheck
		rows, queryErr := store.Query("SELECT COUNT(*) FROM dolt_status")
		if queryErr == nil {
			defer rows.Close() //nolint:errcheck
			var count int
			if rows.Next() {
				_ = rows.Scan(&count)
			}
			r.DoltAvailable = true
			r.DoltUncommitted = count
		}
	}

	// Derive overall status.
	doltDirty := r.DoltAvailable && r.DoltUncommitted > 0
	switch {
	case r.StoredHash == "":
		r.Status = "never_imported"
	case r.FileChanged && doltDirty:
		r.Status = "file_changed_and_dolt_dirty"
	case r.FileChanged:
		r.Status = "file_changed"
	case doltDirty:
		r.Status = "dolt_dirty"
	default:
		r.Status = "in_sync"
	}
	return r
}

func printSyncStatus(cmd *cobra.Command, r SyncStatusResult, issuesPath string) {
	w := cmd.OutOrStdout()

	_, _ = fmt.Fprintln(w, "Sync Status")
	_, _ = fmt.Fprintln(w, "-----------")

	if !r.FileExists {
		_, _ = fmt.Fprintf(w, "File:  %s — not found\n", issuesPath)
		_, _ = fmt.Fprintln(w, "\nStatus: no_file")
		return
	}
	_, _ = fmt.Fprintf(w, "File:  %s\n", issuesPath)

	// Check A line.
	switch {
	case r.StoredHash == "" && r.FileHash == "":
		_, _ = fmt.Fprintln(w, "Hash:  unable to read (file or .grava not accessible)")
	case r.StoredHash == "":
		_, _ = fmt.Fprintf(w, "Hash:  %s  [never imported]\n", abbrev(r.FileHash))
	case r.FileHash == "":
		_, _ = fmt.Fprintf(w, "Hash:  unreadable  [stored: %s]\n", abbrev(r.StoredHash))
	case !r.FileChanged:
		_, _ = fmt.Fprintf(w, "Hash:  %s  [matches last import]\n", abbrev(r.FileHash))
	default:
		_, _ = fmt.Fprintf(w, "Hash:  %s  [CHANGED — stored: %s]\n", abbrev(r.FileHash), abbrev(r.StoredHash))
	}

	// Check C line.
	switch {
	case !r.DoltAvailable:
		_, _ = fmt.Fprintln(w, "Dolt:  unavailable (DB not reachable)")
	case r.DoltUncommitted == 0:
		_, _ = fmt.Fprintln(w, "Dolt:  clean (no uncommitted changes)")
	default:
		_, _ = fmt.Fprintf(w, "Dolt:  %d uncommitted change(s) — run 'grava commit' first\n", r.DoltUncommitted)
	}

	_, _ = fmt.Fprintf(w, "\nStatus: %s\n", r.Status)
}

// abbrev returns the first 12 characters of a hex hash for compact display.
func abbrev(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12] + "..."
}

func init() {
	rootCmd.AddCommand(syncStatusCmd)
}
