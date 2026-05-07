package issues

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/hoangtrungnguyen/grava/pkg/graph"
	"github.com/spf13/cobra"
)

// DropParams holds all inputs for the dropIssue named function.
type DropParams struct {
	ID    string
	Force bool
	Actor string
	Model string
}

// DropResult is the JSON output model for the drop command (NFR5).
type DropResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// dropIssue transitions an issue's status to archived (soft-delete).
// If the issue is in_progress, the --force flag is required.
// Status transition is routed through the graph layer (dag.SetNodeStatus).
// All user-facing errors are returned as *gravaerrors.GravaError.
func dropIssue(ctx context.Context, store dolt.Store, params DropParams) (DropResult, error) {
	// Pre-read: validate existence and current status
	var currentStatus string
	row := store.QueryRow("SELECT status FROM issues WHERE id = ?", params.ID)
	if err := row.Scan(&currentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DropResult{}, gravaerrors.New("ISSUE_NOT_FOUND",
				fmt.Sprintf("Issue %s not found", params.ID), nil)
		}
		return DropResult{}, gravaerrors.New("DB_UNREACHABLE", "failed to read issue", err)
	}

	// Guard: in_progress requires --force
	if currentStatus == "in_progress" && !params.Force {
		return DropResult{}, gravaerrors.New("ISSUE_IN_PROGRESS",
			"Cannot drop an active issue. Use --force to override.", nil)
	}

	// Already archived — idempotent
	if currentStatus == "archived" {
		return DropResult{ID: params.ID, Status: "archived"}, nil
	}

	// Status transition via graph layer (manages its own tx + audit)
	dag, err := graph.LoadGraphFromDB(store)
	if err != nil {
		return DropResult{}, gravaerrors.New("DB_UNREACHABLE", "failed to load graph", err)
	}
	dag.SetSession(params.Actor, params.Model)
	if err := dag.SetNodeStatus(params.ID, graph.StatusArchived); err != nil {
		return DropResult{}, gravaerrors.New("DB_UNREACHABLE", "failed to archive issue", err)
	}

	return DropResult{ID: params.ID, Status: "archived"}, nil
}

// dropAllData performs a nuclear reset, deleting ALL data from every table.
// Tables are truncated in FK-safe order.
func dropAllData(ctx context.Context, store dolt.Store) error {
	tables := []string{
		"dependencies",
		"events",
		"work_sessions",
		"issue_labels",
		"issue_comments",
		"deletions",
		"child_counters",
		"issues",
	}

	tx, err := store.BeginTx(ctx, nil)
	if err != nil {
		return gravaerrors.New("DB_UNREACHABLE", "failed to start transaction", err)
	}
	defer tx.Rollback() //nolint:errcheck

	for _, table := range tables {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", table)); err != nil {
			return gravaerrors.New("DB_UNREACHABLE",
				fmt.Sprintf("failed to delete from %s", table), err)
		}
	}

	if err := tx.Commit(); err != nil {
		return gravaerrors.New("DB_COMMIT_FAILED", "failed to commit drop-all transaction", err)
	}
	return nil
}

// newDropCmd builds the `grava drop` cobra command.
// Modes:
//   - grava drop <id>           → per-issue archive (soft-delete)
//   - grava drop <id> --force   → archive even if in_progress
//   - grava drop --all --force  → nuclear reset (delete ALL data)
func newDropCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drop [id]",
		Short: "Archive an issue or delete all data",
		Long: `Drop archives a specific issue (soft-delete) or performs a nuclear reset.

Per-issue archive (soft-delete):
  grava drop <id>          # archive a done/open issue
  grava drop <id> --force  # archive even if in_progress

Nuclear reset (delete ALL data):
  grava drop --all --force  # deletes everything, no undo`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			all, _ := cmd.Flags().GetBool("all")

			// Nuclear reset path
			if all {
				if !force && !*d.OutputJSON {
					cmd.Print("⚠️  This will DELETE ALL DATA from the Grava database.\nType \"yes\" to confirm: ")
					scanner := bufio.NewScanner(StdinReader)
					scanner.Scan()
					if strings.TrimSpace(scanner.Text()) != "yes" {
						cmd.Println("Aborted. No data was deleted.")
						return fmt.Errorf("user cancelled drop operation")
					}
				}

				if err := dropAllData(cmd.Context(), *d.Store); err != nil {
					if *d.OutputJSON {
						return writeJSONError(cmd, err)
					}
					return err
				}

				if *d.OutputJSON {
					resp := map[string]string{"status": "dropped", "note": "All data deleted from every table"}
					b, _ := json.MarshalIndent(resp, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
					return nil
				}
				cmd.Println("💣 All Grava data has been dropped.")
				return nil
			}

			// Per-issue archive path
			if len(args) == 0 {
				return cmd.Usage()
			}

			result, err := dropIssue(cmd.Context(), *d.Store, DropParams{
				ID:    args[0],
				Force: force,
				Actor: *d.Actor,
				Model: *d.AgentModel,
			})
			if err != nil {
				if *d.OutputJSON {
					return writeJSONError(cmd, err)
				}
				return err
			}

			if *d.OutputJSON {
				b, _ := json.MarshalIndent(result, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
				return nil
			}

			cmd.Printf("📦 Archived issue %s\n", result.ID)
			return nil
		},
	}

	cmd.Flags().Bool("force", false, "Force archive of in_progress issues, or skip confirmation for --all")
	cmd.Flags().Bool("all", false, "Nuclear reset: delete ALL data from every table")
	return cmd
}
