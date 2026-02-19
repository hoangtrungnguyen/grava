package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	importFile         string
	importOverwrite    bool
	importSkipExisting bool // Default is true if overwrite is false?
	// Wait, standard behavior usually: error on duplicate.
	// But CLI flags often offer --force or --skip.
	// Let's say: default = error on duplicate.
	// --skip-existing = ignore duplicates.
	// --overwrite = replace duplicates.
)

// importCmd represents the import command
var importCmd = &cobra.Command{
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
		defer f.Close()

		ctx := context.Background()
		tx, err := Store.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}
		defer tx.Rollback()

		scanner := bufio.NewScanner(f)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 10*1024*1024) // 10MB limit

		var count, skipped, updated int

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var item ExportItem
			if err := json.Unmarshal(line, &item); err != nil {
				return fmt.Errorf("failed to parse JSON line: %w", err)
			}

			switch item.Type {
			case "issue":
				var i IssueExportData
				if err := json.Unmarshal(item.Data, &i); err != nil {
					return fmt.Errorf("failed to parse issue data: %w", err)
				}

				baseQuery := `INTO issues (
					id, title, description, issue_type, priority, status, metadata,
					created_at, updated_at, created_by, updated_by, agent_model,
					affected_files, ephemeral
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

				var query string
				if importSkipExisting {
					query = "INSERT IGNORE " + baseQuery
				} else {
					query = "INSERT " + baseQuery
				}

				if importOverwrite {
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
					return fmt.Errorf("failed to insert issue %s: %w", i.ID, err)
				}

				affected, _ := res.RowsAffected()
				if affected == 0 {
					skipped++
				} else if affected == 2 && importOverwrite {
					updated++
				} else {
					count++
				}

			case "dependency":
				var d DependencyExportData
				if err := json.Unmarshal(item.Data, &d); err != nil {
					return fmt.Errorf("failed to parse dependency data: %w", err)
				}

				baseQuery := `INTO dependencies (
					from_id, to_id, type, created_by, updated_by, agent_model
				) VALUES (?, ?, ?, ?, ?, ?)`

				var query string
				if importSkipExisting {
					query = "INSERT IGNORE " + baseQuery
				} else {
					query = "INSERT " + baseQuery
				}

				if importOverwrite {
					query += ` ON DUPLICATE KEY UPDATE
						created_by=VALUES(created_by), updated_by=VALUES(updated_by), agent_model=VALUES(agent_model)`
				}

				_, err := tx.ExecContext(ctx, query,
					d.FromID, d.ToID, d.Type, d.CreatedBy, d.UpdatedBy, d.AgentModel,
				)
				// Foreign key failure handling?
				// If issue not found, this will fail.
				// We should probably fail loud to alert user of inconsistency.
				if err != nil {
					return fmt.Errorf("failed to insert dependency %s->%s: %w", d.FromID, d.ToID, err)
				}
				// Counters logic for deps is simpler/less critical
			}
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error reading import file: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		cmd.Printf("✅ Imported %d items (Updated: %d, Skipped: %d)\n", count, updated, skipped)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.Flags().StringVarP(&importFile, "file", "f", "", "Input file (required)")
	importCmd.Flags().BoolVar(&importOverwrite, "overwrite", false, "Overwrite existing IDs (upsert)")
	importCmd.Flags().BoolVar(&importSkipExisting, "skip-existing", false, "Skip existing IDs (ignore duplicates)")
}
