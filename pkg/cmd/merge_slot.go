package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/grava"
	"github.com/hoangtrungnguyen/grava/pkg/merge"
	"github.com/spf13/cobra"
)

var (
	mergeAncestor string
	mergeCurrent  string
	mergeOther    string
)

// conflictExitFn is the function called when merge conflicts are detected.
// Overridable in tests to avoid os.Exit terminating the test process.
var conflictExitFn = func(code int) { os.Exit(code) }

var mergeSlotCmd = &cobra.Command{
	Use:   "merge-slot",
	Short: "Three-way merge driver for JSONL issues files",
	Long: `Executes a schema-aware three-way merge for JSONL files tracked by git.

Git invokes this as a merge driver: grava merge-slot --ancestor %O --current %A --other %B
The merged result is written back to the current (%A) file path.
Exit code 1 signals git that conflicts remain.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if mergeAncestor == "" || mergeCurrent == "" || mergeOther == "" {
			return fmt.Errorf("--ancestor, --current, and --other are required")
		}

		ancestorBytes, err := os.ReadFile(mergeAncestor)
		if err != nil {
			return fmt.Errorf("failed to read ancestor file: %w", err)
		}
		currentBytes, err := os.ReadFile(mergeCurrent)
		if err != nil {
			return fmt.Errorf("failed to read current file: %w", err)
		}
		otherBytes, err := os.ReadFile(mergeOther)
		if err != nil {
			return fmt.Errorf("failed to read other file: %w", err)
		}

		merged, hasConflict, err := merge.ProcessMerge(
			string(ancestorBytes),
			string(currentBytes),
			string(otherBytes),
		)
		if err != nil {
			return fmt.Errorf("merge failed: %w", err)
		}

		// Write merged result back to the current (%A) file
		if err := os.WriteFile(mergeCurrent, []byte(merged), 0644); err != nil { //nolint:gosec
			return fmt.Errorf("failed to write merge result to %s: %w", mergeCurrent, err)
		}

		if hasConflict {
			// Extract conflict entries and write them to .grava/conflicts.json
			// so that 'grava resolve list' can display them. Best-effort: if
			// the .grava directory is not found, skip writing and continue.
			if entries, err := merge.ExtractConflicts(merged, time.Now().UTC()); err == nil && len(entries) > 0 {
				if gravaDir, err := grava.ResolveGravaDir(); err == nil {
					conflictsPath := filepath.Join(gravaDir, "conflicts.json")
					if b, err := json.MarshalIndent(entries, "", "  "); err == nil {
						_ = os.WriteFile(conflictsPath, b, 0644) //nolint:gosec
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
							"⚠️  %d conflict(s) written to %s\n", len(entries), conflictsPath)
						_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "   Run 'grava resolve list' to view and resolve them.")
					}
				}
			}
			// Non-zero exit tells git that conflicts remain
			conflictExitFn(1)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(mergeSlotCmd)

	mergeSlotCmd.Flags().StringVar(&mergeAncestor, "ancestor", "", "Ancestor version file path (%O)")
	mergeSlotCmd.Flags().StringVar(&mergeCurrent, "current", "", "Current version file path (%A) — result is written here")
	mergeSlotCmd.Flags().StringVar(&mergeOther, "other", "", "Other version file path (%B)")
}
