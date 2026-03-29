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
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/spf13/cobra"
)

// ExportItem represents a standard export format for issues and dependencies.
type ExportItem struct {
	Type string          `json:"type"` // "issue" or "dependency"
	Data json.RawMessage `json:"data"`
}

// IssueExportData is the issue export model.
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

func newExportCmd(d *cmddeps.Deps) *cobra.Command {
	var (
		exportFile           string
		exportFormat         string
		exportIncludeWisps   bool
		exportSkipTombstones bool
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export issues and dependencies",
		Long: `Export issues and dependencies to a file (default: stdout) in JSONL format.
Useful for backups, migrations, or seeding test data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var out = cmd.OutOrStdout()
			if exportFile != "" {
				f, err := os.Create(exportFile)
				if err != nil {
					return fmt.Errorf("failed to create export file: %w", err)
				}
				defer f.Close() //nolint:errcheck
				out = f
			}

			whereClause := []string{}
			params := []any{}
			if !exportIncludeWisps {
				whereClause = append(whereClause, "ephemeral = 0")
			}
			if exportSkipTombstones {
				whereClause = append(whereClause, "status != 'tombstone'")
			}

			issueQuery := "SELECT id, title, description, issue_type, priority, status, metadata, created_at, updated_at, created_by, updated_by, agent_model, affected_files, ephemeral FROM issues"
			if len(whereClause) > 0 {
				issueQuery += " WHERE "
				for i, clause := range whereClause {
					if i > 0 {
						issueQuery += " AND "
					}
					issueQuery += clause
				}
			}

			rows, err := (*d.Store).Query(issueQuery, params...)
			if err != nil {
				return fmt.Errorf("failed to query issues: %w", err)
			}
			defer rows.Close() //nolint:errcheck

			enc := json.NewEncoder(out)

			for rows.Next() {
				var i IssueExportData
				var metadataBytes []byte
				var filesBytes []byte

				if err := rows.Scan(
					&i.ID, &i.Title, &i.Description, &i.Type, &i.Priority, &i.Status, &metadataBytes,
					&i.CreatedAt, &i.UpdatedAt, &i.CreatedBy, &i.UpdatedBy, &i.AgentModel,
					&filesBytes, &i.Ephemeral,
				); err != nil {
					return fmt.Errorf("failed to scan issue: %w", err)
				}

				if len(metadataBytes) > 0 {
					i.Metadata = json.RawMessage(metadataBytes)
				} else {
					i.Metadata = json.RawMessage("{}")
				}
				if len(filesBytes) > 0 {
					i.AffectedFiles = json.RawMessage(filesBytes)
				} else {
					i.AffectedFiles = json.RawMessage("[]")
				}

				data, err := json.Marshal(i)
				if err != nil {
					return fmt.Errorf("failed to marshal issue data: %w", err)
				}

				item := ExportItem{Type: "issue", Data: data}
				if err := enc.Encode(item); err != nil {
					return fmt.Errorf("failed to write export item: %w", err)
				}
			}

			depQuery := `SELECT from_id, to_id, type, created_by, updated_by, agent_model FROM dependencies`
			if !exportIncludeWisps {
				depQuery = `SELECT d.from_id, d.to_id, d.type, d.created_by, d.updated_by, d.agent_model
			            FROM dependencies d
			            JOIN issues i1 ON d.from_id = i1.id
			            JOIN issues i2 ON d.to_id = i2.id
			            WHERE i1.ephemeral = 0 AND i2.ephemeral = 0`
			}
			if exportSkipTombstones {
				if !exportIncludeWisps {
					depQuery += " AND i1.status != 'tombstone' AND i2.status != 'tombstone'"
				} else {
					depQuery = `SELECT d.from_id, d.to_id, d.type, d.created_by, d.updated_by, d.agent_model
				            FROM dependencies d
				            JOIN issues i1 ON d.from_id = i1.id
				            JOIN issues i2 ON d.to_id = i2.id
				            WHERE i1.status != 'tombstone' AND i2.status != 'tombstone'`
				}
			}

			depRows, err := (*d.Store).Query(depQuery)
			if err != nil {
				return fmt.Errorf("failed to query dependencies: %w", err)
			}
			defer depRows.Close() //nolint:errcheck

			for depRows.Next() {
				var dep DependencyExportData
				if err := depRows.Scan(
					&dep.FromID, &dep.ToID, &dep.Type, &dep.CreatedBy, &dep.UpdatedBy, &dep.AgentModel,
				); err != nil {
					return fmt.Errorf("failed to scan dependency: %w", err)
				}

				data, err := json.Marshal(dep)
				if err != nil {
					return fmt.Errorf("failed to marshal dependency data: %w", err)
				}

				item := ExportItem{Type: "dependency", Data: data}
				if err := enc.Encode(item); err != nil {
					return fmt.Errorf("failed to write export item: %w", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&exportFile, "file", "f", "", "Output file (default: stdout)")
	cmd.Flags().StringVar(&exportFormat, "format", "jsonl", "Output format (only jsonl supported)")
	cmd.Flags().BoolVar(&exportIncludeWisps, "include-wisps", false, "Include ephemeral Wisp issues")
	cmd.Flags().BoolVar(&exportSkipTombstones, "skip-tombstones", false, "Exclude soft-deleted issues")
	return cmd
}

// ImportResult holds the counts from a successful import operation.
type ImportResult struct {
	Imported int `json:"imported"`
	Updated  int `json:"updated"`
	Skipped  int `json:"skipped"`
}

// importIssues reads JSONL from r and imports issues and dependencies into store.
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

func newImportCmd(d *cmddeps.Deps) *cobra.Command {
	var (
		importFile         string
		importOverwrite    bool
		importSkipExisting bool
	)

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import issues and dependencies",
		Long: `Import issues and dependencies from a JSONL file.
By default, the command fails if an ID already exists.
Use --skip-existing to ignore duplicates, or --overwrite to update them.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if importFile == "" {
				return fmt.Errorf("input file is required (use --file)")
			}

			if importOverwrite && importSkipExisting {
				return fmt.Errorf("cannot use both --overwrite and --skip-existing")
			}

			f, err := os.Open(importFile)
			if err != nil {
				return fmt.Errorf("failed to open import file: %w", err)
			}
			defer f.Close() //nolint:errcheck

			ctx := cmd.Context()
			result, err := importIssues(ctx, *d.Store, f, importOverwrite, importSkipExisting)
			if err != nil {
				return err
			}

			cmd.Printf("Imported %d items (Updated: %d, Skipped: %d)\n", result.Imported, result.Updated, result.Skipped)
			return nil
		},
	}

	cmd.Flags().StringVarP(&importFile, "file", "f", "", "Input file (required)")
	cmd.Flags().BoolVar(&importOverwrite, "overwrite", false, "Overwrite existing IDs (upsert)")
	cmd.Flags().BoolVar(&importSkipExisting, "skip-existing", false, "Skip existing IDs (ignore duplicates)")
	return cmd
}
