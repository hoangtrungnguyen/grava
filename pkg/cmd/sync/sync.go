// Package synccmd contains the sync commands (commit, import, export).
//
// Note: package name is synccmd (not sync) to avoid collision with stdlib sync.
package synccmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/spf13/cobra"
)

// IssueJSONLRecord is the canonical flat JSONL format written to issues.jsonl.
// Each line in issues.jsonl represents one issue with all related data embedded.
// This is the format understood by the merge driver, grava export, and grava import.
type IssueJSONLRecord struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Type          string            `json:"type"`
	Priority      int               `json:"priority"`
	Status        string            `json:"status"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	CreatedBy     string            `json:"created_by"`
	UpdatedBy     string            `json:"updated_by"`
	AgentModel    *string           `json:"agent_model,omitempty"`
	Metadata      json.RawMessage   `json:"metadata,omitempty"`
	AffectedFiles json.RawMessage   `json:"affected_files,omitempty"`
	Ephemeral     bool              `json:"ephemeral,omitempty"`
	Labels        []string          `json:"labels,omitempty"`
	Comments      []CommentRecord   `json:"comments,omitempty"`
	Dependencies  []DepRecord       `json:"dependencies,omitempty"`
	WispEntries   []WispEntryRecord `json:"wisp_entries,omitempty"`
}

// CommentRecord is a single issue comment embedded in IssueJSONLRecord.
type CommentRecord struct {
	ID         int       `json:"id"`
	Message    string    `json:"message"`
	Actor      string    `json:"actor,omitempty"`
	AgentModel *string   `json:"agent_model,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// DepRecord is a single dependency embedded in IssueJSONLRecord.
type DepRecord struct {
	FromID     string  `json:"from_id"`
	ToID       string  `json:"to_id"`
	Type       string  `json:"type"`
	CreatedBy  string  `json:"created_by,omitempty"`
	UpdatedBy  string  `json:"updated_by,omitempty"`
	AgentModel *string `json:"agent_model,omitempty"`
}

// WispEntryRecord is a single wisp key-value entry embedded in IssueJSONLRecord.
type WispEntryRecord struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	WrittenBy string    `json:"written_by,omitempty"`
	WrittenAt time.Time `json:"written_at"`
}

// ExportItem represents the legacy wrapped export format.
// Kept for backward compatibility with existing tooling that reads the old format.
type ExportItem struct {
	Type string          `json:"type"` // "issue" or "dependency"
	Data json.RawMessage `json:"data"`
}

// IssueExportData is the legacy issue export model (used by ExportItem).
type IssueExportData struct {
	ID            string          `json:"id"`
	Title         string          `json:"title"`
	Description   string          `json:"description"`
	Type          string          `json:"issue_type"`
	Priority      int             `json:"priority"`
	Status        string          `json:"status"`
	Metadata      json.RawMessage `json:"metadata"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	CreatedBy     string          `json:"created_by"`
	UpdatedBy     string          `json:"updated_by"`
	AgentModel    *string         `json:"agent_model"`
	AffectedFiles json.RawMessage `json:"affected_files"`
	Ephemeral     bool            `json:"ephemeral"`
}

// DependencyExportData is the dependency export model.
type DependencyExportData struct {
	FromID     string  `json:"from_id"`
	ToID       string  `json:"to_id"`
	Type       string  `json:"type"`
	CreatedBy  string  `json:"created_by"`
	UpdatedBy  string  `json:"updated_by"`
	AgentModel *string `json:"agent_model"`
}

// AddCommands registers all sync commands with the root cobra.Command.
func AddCommands(root *cobra.Command, d *cmddeps.Deps) {
	root.AddCommand(newCommitCmd(d))
	root.AddCommand(newExportCmd(d))
	root.AddCommand(newImportCmd(d))
}

func newCommitCmd(d *cmddeps.Deps) *cobra.Command {
	var commitMessage string
	cmd := &cobra.Command{
		Use:   "commit -m <message>",
		Short: "Commit changes to the Dolt database",
		Long:  `Commit all staged changes (all modified issues and dependencies) to the Dolt version history.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if commitMessage == "" {
				return fmt.Errorf("commit message is required (use -m)")
			}

			_, err := (*d.Store).Exec("CALL DOLT_ADD('-A')")
			if err != nil {
				return fmt.Errorf("failed to stage changes: %w", err)
			}

			var hash string
			err = (*d.Store).QueryRow("CALL DOLT_COMMIT('-m', ?)", commitMessage).Scan(&hash)
			if err != nil {
				return fmt.Errorf("failed to commit: %w", err)
			}

			cmd.Printf("✅ Committed changes. Hash: %s\n", hash)
			return nil
		},
	}

	cmd.Flags().StringVarP(&commitMessage, "message", "m", "", "Commit message")
	_ = cmd.MarkFlagRequired("message")
	return cmd
}

// resolveDefaultExportPath returns the path to issues.jsonl in the git repo root.
// Falls back to "issues.jsonl" relative to CWD when not in a git repo.
func resolveDefaultExportPath() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "issues.jsonl"
	}
	return strings.TrimSpace(string(out)) + "/issues.jsonl"
}

func newExportCmd(d *cmddeps.Deps) *cobra.Command {
	var (
		outputPath         string
		exportIncludeWisps bool
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export issues to issues.jsonl (canonical flat format)",
		Long: `Export all issues to issues.jsonl in the canonical flat JSONL format.

Each line is one JSON object containing the full issue with embedded labels,
comments, dependencies, and wisp entries. This is the format consumed by the
merge driver and imported by 'grava import'.

By default writes to issues.jsonl in the git repository root.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve output path: --output flag → git root/issues.jsonl
			path := outputPath
			if path == "" {
				path = resolveDefaultExportPath()
			}

			f, err := os.Create(path)
			if err != nil {
				return fmt.Errorf("failed to create %s: %w", path, err)
			}
			defer f.Close() //nolint:errcheck

			ctx := cmd.Context()
			count, err := exportFlatJSONL(ctx, *d.Store, f, exportIncludeWisps)
			if err != nil {
				return err
			}

			if *d.OutputJSON {
				resp := map[string]interface{}{
					"exported_path": path,
					"issue_count":   count,
					"exported_at":   time.Now().UTC().Format(time.RFC3339),
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(resp)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Exported %d issue(s) to %s\n", count, path)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path (default: <git-root>/issues.jsonl)")
	cmd.Flags().BoolVar(&exportIncludeWisps, "include-wisps", false, "Include ephemeral Wisp issues")
	return cmd
}

// exportFlatJSONL queries all non-tombstone issues with their labels, comments,
// dependencies, and wisp entries, and writes one JSON line per issue to w.
// Returns the number of issues written.
func exportFlatJSONL(ctx context.Context, store dolt.Store, w io.Writer, includeWisps bool) (int, error) {
	// 1. Fetch issues
	whereClause := "status != 'tombstone'"
	if !includeWisps {
		whereClause += " AND ephemeral = 0"
	}
	issueRows, err := store.QueryContext(ctx,
		"SELECT id, title, description, issue_type, priority, status, metadata, created_at, updated_at,"+
			" created_by, updated_by, agent_model, affected_files, ephemeral"+
			" FROM issues WHERE "+whereClause+" ORDER BY created_at, id",
	)
	if err != nil {
		return 0, fmt.Errorf("failed to query issues: %w", err)
	}
	defer issueRows.Close() //nolint:errcheck

	issues := make([]IssueJSONLRecord, 0, 64)
	for issueRows.Next() {
		var rec IssueJSONLRecord
		var metaBytes, filesBytes []byte
		if err := issueRows.Scan(
			&rec.ID, &rec.Title, &rec.Description, &rec.Type, &rec.Priority, &rec.Status,
			&metaBytes, &rec.CreatedAt, &rec.UpdatedAt, &rec.CreatedBy, &rec.UpdatedBy,
			&rec.AgentModel, &filesBytes, &rec.Ephemeral,
		); err != nil {
			return 0, fmt.Errorf("failed to scan issue: %w", err)
		}
		if len(metaBytes) > 0 {
			rec.Metadata = json.RawMessage(metaBytes)
		}
		if len(filesBytes) > 0 {
			rec.AffectedFiles = json.RawMessage(filesBytes)
		}
		issues = append(issues, rec)
	}
	if err := issueRows.Err(); err != nil {
		return 0, fmt.Errorf("issue scan error: %w", err)
	}
	if len(issues) == 0 {
		return 0, nil
	}

	// Build index by ID for efficient lookup
	idxByID := make(map[string]int, len(issues))
	for i, rec := range issues {
		idxByID[rec.ID] = i
	}

	// 2. Fetch all labels
	labelRows, err := store.QueryContext(ctx,
		"SELECT issue_id, label FROM issue_labels ORDER BY issue_id, created_at")
	if err != nil {
		return 0, fmt.Errorf("failed to query labels: %w", err)
	}
	defer labelRows.Close() //nolint:errcheck
	for labelRows.Next() {
		var issueID, label string
		if scanErr := labelRows.Scan(&issueID, &label); scanErr == nil {
			if idx, ok := idxByID[issueID]; ok {
				issues[idx].Labels = append(issues[idx].Labels, label)
			}
		}
	}
	if err := labelRows.Err(); err != nil {
		return 0, fmt.Errorf("label scan error: %w", err)
	}

	// 3. Fetch all comments
	commentRows, err := store.QueryContext(ctx,
		"SELECT issue_id, id, message, actor, agent_model, created_at FROM issue_comments ORDER BY issue_id, created_at")
	if err != nil {
		return 0, fmt.Errorf("failed to query comments: %w", err)
	}
	defer commentRows.Close() //nolint:errcheck
	for commentRows.Next() {
		var issueID string
		var c CommentRecord
		var actor *string
		if scanErr := commentRows.Scan(&issueID, &c.ID, &c.Message, &actor, &c.AgentModel, &c.CreatedAt); scanErr == nil {
			if actor != nil {
				c.Actor = *actor
			}
			if idx, ok := idxByID[issueID]; ok {
				issues[idx].Comments = append(issues[idx].Comments, c)
			}
		}
	}
	if err := commentRows.Err(); err != nil {
		return 0, fmt.Errorf("comment scan error: %w", err)
	}

	// 4. Fetch all dependencies (keyed by from_id)
	depRows, err := store.QueryContext(ctx,
		"SELECT from_id, to_id, type, created_by, updated_by, agent_model FROM dependencies ORDER BY from_id, to_id")
	if err != nil {
		return 0, fmt.Errorf("failed to query dependencies: %w", err)
	}
	defer depRows.Close() //nolint:errcheck
	for depRows.Next() {
		var dep DepRecord
		var createdBy, updatedBy *string
		if scanErr := depRows.Scan(&dep.FromID, &dep.ToID, &dep.Type, &createdBy, &updatedBy, &dep.AgentModel); scanErr == nil {
			if createdBy != nil {
				dep.CreatedBy = *createdBy
			}
			if updatedBy != nil {
				dep.UpdatedBy = *updatedBy
			}
			if idx, ok := idxByID[dep.FromID]; ok {
				issues[idx].Dependencies = append(issues[idx].Dependencies, dep)
			}
		}
	}
	if err := depRows.Err(); err != nil {
		return 0, fmt.Errorf("dependency scan error: %w", err)
	}

	// 5. Fetch wisp entries
	wispRows, err := store.QueryContext(ctx,
		"SELECT issue_id, key_name, value, written_by, written_at FROM wisp_entries ORDER BY issue_id, key_name")
	if err != nil {
		return 0, fmt.Errorf("failed to query wisp entries: %w", err)
	}
	defer wispRows.Close() //nolint:errcheck
	for wispRows.Next() {
		var issueID string
		var we WispEntryRecord
		if scanErr := wispRows.Scan(&issueID, &we.Key, &we.Value, &we.WrittenBy, &we.WrittenAt); scanErr == nil {
			if idx, ok := idxByID[issueID]; ok {
				issues[idx].WispEntries = append(issues[idx].WispEntries, we)
			}
		}
	}
	if err := wispRows.Err(); err != nil {
		return 0, fmt.Errorf("wisp scan error: %w", err)
	}

	// 6. Write one JSON line per issue
	enc := json.NewEncoder(w)
	for i := range issues {
		if err := enc.Encode(&issues[i]); err != nil {
			return i, fmt.Errorf("failed to write issue %s: %w", issues[i].ID, err)
		}
	}
	return len(issues), nil
}

// ImportResult holds the counts from a successful import operation.
type ImportResult struct {
	Imported int `json:"imported"`
	Updated  int `json:"updated"`
	Skipped  int `json:"skipped"`
}

// importFlatJSONL reads flat IssueJSONLRecord lines from r and upserts them into
// store within a single transaction. All-or-nothing: rolls back on any error.
func importFlatJSONL(ctx context.Context, store dolt.Store, r io.Reader, overwrite bool) (ImportResult, error) {
	tx, err := store.BeginTx(ctx, nil)
	if err != nil {
		return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK", "failed to start transaction", err)
	}
	defer tx.Rollback() //nolint:errcheck

	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var result ImportResult

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec IssueJSONLRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK", "failed to parse JSON line", err)
		}
		if rec.ID == "" {
			continue // skip malformed lines with no ID
		}

		metaBytes := []byte("{}")
		if len(rec.Metadata) > 0 {
			metaBytes = rec.Metadata
		}
		filesBytes := []byte("[]")
		if len(rec.AffectedFiles) > 0 {
			filesBytes = rec.AffectedFiles
		}

		var issueQuery string
		if overwrite {
			issueQuery = `INSERT INTO issues (id, title, description, issue_type, priority, status, metadata,
				created_at, updated_at, created_by, updated_by, agent_model, affected_files, ephemeral)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON DUPLICATE KEY UPDATE
				title=VALUES(title), description=VALUES(description), issue_type=VALUES(issue_type),
				priority=VALUES(priority), status=VALUES(status), metadata=VALUES(metadata),
				updated_at=VALUES(updated_at), updated_by=VALUES(updated_by),
				agent_model=VALUES(agent_model), affected_files=VALUES(affected_files),
				ephemeral=VALUES(ephemeral)`
		} else {
			issueQuery = `INSERT IGNORE INTO issues (id, title, description, issue_type, priority, status, metadata,
				created_at, updated_at, created_by, updated_by, agent_model, affected_files, ephemeral)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		}

		res, err := tx.ExecContext(ctx, issueQuery,
			rec.ID, rec.Title, rec.Description, rec.Type, rec.Priority, rec.Status, string(metaBytes),
			rec.CreatedAt, rec.UpdatedAt, rec.CreatedBy, rec.UpdatedBy, rec.AgentModel,
			string(filesBytes), rec.Ephemeral,
		)
		if err != nil {
			return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK", fmt.Sprintf("failed to upsert issue %s", rec.ID), err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			result.Skipped++
		} else if affected == 2 && overwrite {
			result.Updated++
		} else {
			result.Imported++
		}

		// Upsert labels
		for _, label := range rec.Labels {
			if _, execErr := tx.ExecContext(ctx,
				`INSERT IGNORE INTO issue_labels (issue_id, label) VALUES (?, ?)`,
				rec.ID, label,
			); execErr != nil {
				return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK",
					fmt.Sprintf("failed to upsert label %q for issue %s", label, rec.ID), execErr)
			}
		}

		// Upsert comments (by id)
		for _, c := range rec.Comments {
			if c.ID > 0 {
				if _, execErr := tx.ExecContext(ctx,
					`INSERT INTO issue_comments (id, issue_id, message, actor, agent_model, created_at)
					 VALUES (?, ?, ?, ?, ?, ?)
					 ON DUPLICATE KEY UPDATE message=VALUES(message), actor=VALUES(actor)`,
					c.ID, rec.ID, c.Message, nullableStr(c.Actor), c.AgentModel, c.CreatedAt,
				); execErr != nil {
					return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK",
						fmt.Sprintf("failed to upsert comment %d for issue %s", c.ID, rec.ID), execErr)
				}
			}
		}

		// Upsert dependencies
		for _, dep := range rec.Dependencies {
			if _, execErr := tx.ExecContext(ctx,
				`INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by, agent_model)
				 VALUES (?, ?, ?, ?, ?, ?)
				 ON DUPLICATE KEY UPDATE type=VALUES(type)`,
				dep.FromID, dep.ToID, dep.Type,
				nullableStr(dep.CreatedBy), nullableStr(dep.UpdatedBy), dep.AgentModel,
			); execErr != nil {
				return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK",
					fmt.Sprintf("failed to upsert dependency %s→%s", dep.FromID, dep.ToID), execErr)
			}
		}

		// Upsert wisp entries (by issue_id + key_name)
		for _, we := range rec.WispEntries {
			if _, execErr := tx.ExecContext(ctx,
				`INSERT INTO wisp_entries (issue_id, key_name, value, written_by, written_at)
				 VALUES (?, ?, ?, ?, ?)
				 ON DUPLICATE KEY UPDATE value=VALUES(value), written_by=VALUES(written_by), written_at=VALUES(written_at)`,
				rec.ID, we.Key, we.Value, we.WrittenBy, we.WrittenAt,
			); execErr != nil {
				return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK",
					fmt.Sprintf("failed to upsert wisp entry %q for issue %s", we.Key, rec.ID), execErr)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK", "error reading import file", err)
	}

	if err := tx.Commit(); err != nil {
		return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK", "failed to commit transaction", err)
	}

	return result, nil
}

// nullableStr returns nil if s is empty, otherwise &s. Used for nullable string
// DB columns where an empty Go string should be stored as SQL NULL.
func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ExportFlatJSONL queries all non-tombstone issues and writes one JSON line per
// issue to w (including labels, comments, dependencies, and wisp entries).
// This is the exported version of exportFlatJSONL for use by sandbox scenarios.
func ExportFlatJSONL(ctx context.Context, store dolt.Store, w io.Writer, includeWisps bool) (int, error) {
	return exportFlatJSONL(ctx, store, w, includeWisps)
}

// ImportFlatJSONL reads flat IssueJSONLRecord lines from r and upserts them
// into store. This is the exported version for use by sandbox scenarios.
func ImportFlatJSONL(ctx context.Context, store dolt.Store, r io.Reader, overwrite bool) (ImportResult, error) {
	return importFlatJSONL(ctx, store, r, overwrite)
}

// SyncIssuesFile opens the flat JSONL file at path and upserts all records into
// store (overwrite=true). It is used by git hook handlers to sync after a
// merge or checkout that changed issues.jsonl.
func SyncIssuesFile(ctx context.Context, store dolt.Store, path string) (ImportResult, error) {
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return ImportResult{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck
	return importFlatJSONL(ctx, store, f, true)
}

// ValidateJSONL reads all lines from r and verifies each is parseable as a flat
// IssueJSONLRecord with a non-empty id field. Returns the first parse error.
// Used by pre-commit to reject malformed issues.jsonl before a commit lands.
func ValidateJSONL(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec IssueJSONLRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}
		if rec.ID == "" {
			return fmt.Errorf("line %d: missing required field 'id'", lineNum)
		}
	}
	return scanner.Err()
}

// importIssues reads ExportItem-wrapped JSONL from r and imports issues and
// dependencies into store. Kept for backward compatibility with legacy exports.
// overwrite enables upsert on duplicate keys; skipExisting silently ignores duplicates.
// The two flags are mutually exclusive — callers must validate before calling.
func importIssues(ctx context.Context, store dolt.Store, r io.Reader, overwrite, skipExisting bool) (ImportResult, error) {
	if overwrite && skipExisting {
		return ImportResult{}, gravaerrors.New("INVALID_ARGS",
			"overwrite and skip-existing are mutually exclusive", nil)
	}
	tx, err := store.BeginTx(ctx, nil)
	if err != nil {
		return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK", "failed to start transaction", err)
	}
	defer tx.Rollback() //nolint:errcheck

	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var result ImportResult

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var item ExportItem
		if err := json.Unmarshal(line, &item); err != nil {
			return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK", "failed to parse JSON line", err)
		}

		switch item.Type {
		case "issue":
			var i IssueExportData
			if err := json.Unmarshal(item.Data, &i); err != nil {
				return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK", "failed to parse issue data", err)
			}

			baseQuery := `INTO issues (
				id, title, description, issue_type, priority, status, metadata,
				created_at, updated_at, created_by, updated_by, agent_model,
				affected_files, ephemeral
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

			var query string
			if skipExisting {
				query = "INSERT IGNORE " + baseQuery
			} else {
				query = "INSERT " + baseQuery
			}

			if overwrite {
				query += ` ON DUPLICATE KEY UPDATE
				title=VALUES(title), description=VALUES(description), issue_type=VALUES(issue_type),
				priority=VALUES(priority), status=VALUES(status), metadata=VALUES(metadata),
				updated_at=VALUES(updated_at),
				updated_by=VALUES(updated_by),
				agent_model=VALUES(agent_model), affected_files=VALUES(affected_files),
				ephemeral=VALUES(ephemeral)`
			}

			res, err := tx.ExecContext(ctx, query,
				i.ID, i.Title, i.Description, i.Type, i.Priority, i.Status, string(i.Metadata),
				i.CreatedAt, i.UpdatedAt, i.CreatedBy, i.UpdatedBy, i.AgentModel,
				string(i.AffectedFiles), i.Ephemeral,
			)
			if err != nil {
				return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK", fmt.Sprintf("failed to insert issue %s", i.ID), err)
			}

			affected, _ := res.RowsAffected()
			if affected == 0 {
				result.Skipped++
			} else if affected == 2 && overwrite {
				result.Updated++
			} else {
				result.Imported++
			}

		case "dependency":
			var dep DependencyExportData
			if err := json.Unmarshal(item.Data, &dep); err != nil {
				return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK", "failed to parse dependency data", err)
			}

			baseQuery := `INTO dependencies (
				from_id, to_id, type, created_by, updated_by, agent_model
			) VALUES (?, ?, ?, ?, ?, ?)`

			var query string
			if skipExisting {
				query = "INSERT IGNORE " + baseQuery
			} else {
				query = "INSERT " + baseQuery
			}

			if overwrite {
				query += ` ON DUPLICATE KEY UPDATE
				created_by=VALUES(created_by), updated_by=VALUES(updated_by), agent_model=VALUES(agent_model)`
			}

			res, err := tx.ExecContext(ctx, query,
				dep.FromID, dep.ToID, dep.Type, dep.CreatedBy, dep.UpdatedBy, dep.AgentModel,
			)
			if err != nil {
				return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK", fmt.Sprintf("failed to insert dependency %s->%s", dep.FromID, dep.ToID), err)
			}
			depAffected, _ := res.RowsAffected()
			if depAffected == 0 {
				result.Skipped++
			} else {
				result.Imported++
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK", "error reading import file", err)
	}

	if err := tx.Commit(); err != nil {
		return ImportResult{}, gravaerrors.New("IMPORT_ROLLED_BACK", "failed to commit transaction", err)
	}

	return result, nil
}

// doltHasUncommittedChanges returns true when dolt_status reports any
// staged or unstaged rows. Errors are treated as "no changes" (fail-open).
func doltHasUncommittedChanges(store dolt.Store) bool {
	rows, err := store.Query("SELECT COUNT(*) FROM dolt_status")
	if err != nil {
		return false
	}
	defer rows.Close() //nolint:errcheck
	var count int
	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return false
		}
	}
	return count > 0
}

func newImportCmd(d *cmddeps.Deps) *cobra.Command {
	var (
		importFile         string
		importOverwrite    bool
		importSkipExisting bool
	)

	cmd := &cobra.Command{
		Use:   "import [file]",
		Short: "Import issues from a flat JSONL file",
		Long: `Import issues from a flat JSONL file (issues.jsonl format).

When a positional <file> argument is given the command runs the Dual-Safety Check
(FR24): it hashes the file and aborts with IMPORT_CONFLICT when Dolt has
uncommitted local changes that would be silently overwritten.

The legacy --file flag accepts the old wrapped ExportItem format
({"type":"issue","data":{...}}) and skips the Dual-Safety Check.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// --- New path: positional <file> argument (flat JSONL + Dual-Safety) ---
			if len(args) == 1 {
				if importOverwrite || importSkipExisting {
					return fmt.Errorf("--overwrite and --skip-existing are only valid with --file; omit them for positional import")
				}

				filePath := args[0]

				f, err := os.Open(filePath) //nolint:gosec
				if os.IsNotExist(err) {
					return gravaerrors.New("FILE_NOT_FOUND",
						fmt.Sprintf("import file not found: %s", filePath), err)
				}
				if err != nil {
					return fmt.Errorf("failed to open import file: %w", err)
				}
				defer f.Close() //nolint:errcheck

				// Dual-Safety Check (FR24): abort if Dolt has uncommitted changes.
				if doltHasUncommittedChanges(*d.Store) {
					return gravaerrors.New("IMPORT_CONFLICT",
						"Dolt has uncommitted changes; commit or reset them before importing", nil)
				}

				result, err := importFlatJSONL(ctx, *d.Store, f, true)
				if err != nil {
					return err
				}

				if *d.OutputJSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]int{
						"imported": result.Imported,
						"updated":  result.Updated,
						"skipped":  result.Skipped,
					})
				}
				cmd.Printf("Imported %d, updated %d, skipped %d\n",
					result.Imported, result.Updated, result.Skipped)
				return nil
			}

			// --- Legacy path: --file flag (ExportItem wrapped format) ---
			if importFile == "" {
				return fmt.Errorf("provide a file as positional argument or use --file for the legacy format")
			}

			if importOverwrite && importSkipExisting {
				return fmt.Errorf("cannot use both --overwrite and --skip-existing")
			}

			f, err := os.Open(importFile) //nolint:gosec
			if err != nil {
				return fmt.Errorf("failed to open import file: %w", err)
			}
			defer f.Close() //nolint:errcheck

			result, err := importIssues(ctx, *d.Store, f, importOverwrite, importSkipExisting)
			if err != nil {
				return err
			}

			cmd.Printf("Imported %d items (Updated: %d, Skipped: %d)\n",
				result.Imported, result.Updated, result.Skipped)
			return nil
		},
	}

	cmd.Flags().StringVarP(&importFile, "file", "f", "", "Input file in legacy ExportItem format")
	cmd.Flags().BoolVar(&importOverwrite, "overwrite", false, "Overwrite existing IDs (upsert) [legacy --file only]")
	cmd.Flags().BoolVar(&importSkipExisting, "skip-existing", false, "Skip existing IDs [legacy --file only]")
	return cmd
}
