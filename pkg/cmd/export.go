package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	exportFile           string
	exportFormat         string
	exportIncludeWisps   bool
	exportSkipTombstones bool
)

// ExportItem represents a standard export format for issues and dependencies
type ExportItem struct {
	Type string          `json:"type"` // "issue" or "dependency"
	Data json.RawMessage `json:"data"`
}

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

type DependencyExportData struct {
	FromID     string  `json:"from_id"`
	ToID       string  `json:"to_id"`
	Type       string  `json:"type"`
	CreatedBy  string  `json:"created_by"`
	UpdatedBy  string  `json:"updated_by"`
	AgentModel *string `json:"agent_model"`
}

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export issues and dependencies",
	Long: `Export issues and dependencies to a file (default: stdout) in JSONL format.
Useful for backups, migrations, or seeding test data.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var out io.Writer = cmd.OutOrStdout()
		if exportFile != "" {
			f, err := os.Create(exportFile)
			if err != nil {
				return fmt.Errorf("failed to create export file: %w", err)
			}
			defer f.Close()
			out = f
		}

		// 1. Export Issues
		// Cleanup logic for WHERE clause construction
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

		rows, err := Store.Query(issueQuery, params...)
		if err != nil {
			return fmt.Errorf("failed to query issues: %w", err)
		}
		defer rows.Close()

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

			// Handle JSON fields
			if len(metadataBytes) > 0 {
				i.Metadata = json.RawMessage(metadataBytes)
			} else {
				i.Metadata = json.RawMessage("{}") // Default empty JSON object
			}
			if len(filesBytes) > 0 {
				i.AffectedFiles = json.RawMessage(filesBytes)
			} else {
				i.AffectedFiles = json.RawMessage("[]") // Default empty JSON array
			}

			data, err := json.Marshal(i)
			if err != nil {
				return fmt.Errorf("failed to marshal issue data: %w", err)
			}

			item := ExportItem{
				Type: "issue",
				Data: data,
			}
			if err := enc.Encode(item); err != nil {
				return fmt.Errorf("failed to write export item: %w", err)
			}
		}

		// 2. Export Dependencies
		// We only export dependencies where both ends exist in the export?
		// Or dump all dependencies and let import handle validity?
		// For simplicity, dump all. Or filter if we filtered issues.
		// If we filtered issues (e.g. no wisps), we should filter deps too.
		// BUT usually export is full backup. Default behavior should be "export everything consistent".

		depQuery := `SELECT from_id, to_id, type, created_by, updated_by, agent_model FROM dependencies`
		// If we filter issues, we might want to filter deps.
		// "dependencies" table doesn't have ephemeral flag, relies on issues being ephemeral.
		// If we skip wisps, we should skip deps involving wisps.
		// This requires a JOIN.

		if !exportIncludeWisps {
			depQuery = `SELECT d.from_id, d.to_id, d.type, d.created_by, d.updated_by, d.agent_model 
			            FROM dependencies d
			            JOIN issues i1 ON d.from_id = i1.id
			            JOIN issues i2 ON d.to_id = i2.id
			            WHERE i1.ephemeral = 0 AND i2.ephemeral = 0`
		}
		// Similarly for tombstones
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

		depRows, err := Store.Query(depQuery)
		if err != nil {
			return fmt.Errorf("failed to query dependencies: %w", err)
		}
		defer depRows.Close()

		for depRows.Next() {
			var d DependencyExportData
			if err := depRows.Scan(
				&d.FromID, &d.ToID, &d.Type, &d.CreatedBy, &d.UpdatedBy, &d.AgentModel,
			); err != nil {
				return fmt.Errorf("failed to scan dependency: %w", err)
			}

			data, err := json.Marshal(d)
			if err != nil {
				return fmt.Errorf("failed to marshal dependency data: %w", err)
			}

			item := ExportItem{
				Type: "dependency",
				Data: data,
			}
			if err := enc.Encode(item); err != nil {
				return fmt.Errorf("failed to write export item: %w", err)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)

	exportCmd.Flags().StringVarP(&exportFile, "file", "f", "", "Output file (default: stdout)")
	exportCmd.Flags().StringVar(&exportFormat, "format", "jsonl", "Output format (only jsonl supported)")
	exportCmd.Flags().BoolVar(&exportIncludeWisps, "include-wisps", false, "Include ephemeral Wisp issues")
	exportCmd.Flags().BoolVar(&exportSkipTombstones, "skip-tombstones", false, "Exclude soft-deleted issues")
}
