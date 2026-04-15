package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hoangtrungnguyen/grava/pkg/grava"
	"github.com/hoangtrungnguyen/grava/pkg/merge"
	"github.com/spf13/cobra"
)

// mergeDriverCmd implements `grava merge-driver <ancestor> <current> <other>`.
// Git invokes this with positional arguments: %O (ancestor), %A (current/result), %B (other).
// This is the canonical FR-22 merge driver registered as `grava-merge` in .git/config.
//
// The older `grava merge-slot` command (flag-based) is retained for backward compatibility.
var mergeDriverCmd = &cobra.Command{
	Use:   "merge-driver <ancestor> <current> <other>",
	Short: "Schema-aware 3-way merge driver for issues.jsonl (invoked by Git as grava-merge)",
	Long: `Executes a schema-aware 3-way merge for JSONL files tracked by Git.

Git registers and invokes this command as the 'grava-merge' driver:
  driver = grava merge-driver %O %A %B

Arguments:
  ancestor  ancestor version file path (%O)
  current   current branch file path (%A) — merged result written here
  other     other branch file path (%B)

--dry-run outputs the merged JSONL to stdout without modifying the current (%A) file.
Exit code 1 signals Git that conflicts remain (unless --dry-run is used).`,
	Args: cobra.ExactArgs(3),
	RunE: mergeDriverRunE,
}

var mergeDriverDryRun bool

func init() {
	rootCmd.AddCommand(mergeDriverCmd)
	mergeDriverCmd.Flags().BoolVar(&mergeDriverDryRun, "dry-run", false,
		"Output merge plan to stdout without writing to the current (%A) file")
}

func mergeDriverRunE(cmd *cobra.Command, args []string) error {
	ancestorPath := args[0]
	currentPath := args[1]
	otherPath := args[2]

	ancestorBytes, err := os.ReadFile(ancestorPath)
	if err != nil {
		return fmt.Errorf("merge-driver: read ancestor %s: %w", ancestorPath, err)
	}
	currentBytes, err := os.ReadFile(currentPath)
	if err != nil {
		return fmt.Errorf("merge-driver: read current %s: %w", currentPath, err)
	}
	otherBytes, err := os.ReadFile(otherPath)
	if err != nil {
		return fmt.Errorf("merge-driver: read other %s: %w", otherPath, err)
	}

	result, err := merge.ProcessMergeWithLWW(
		string(ancestorBytes),
		string(currentBytes),
		string(otherBytes),
	)
	if err != nil {
		return fmt.Errorf("merge-driver: merge failed: %w", err)
	}

	if mergeDriverDryRun {
		// Dry-run: show merge plan on stdout, no file writes, no conflict exit.
		fmt.Fprintln(cmd.OutOrStdout(), strings.TrimRight(result.Merged, "\n"))
		if result.HasGitConflict {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "dry-run: conflicts detected (would exit 1 in live mode)")
		}
		return nil
	}

	// Write merged result back to the current (%A) file.
	if err := os.WriteFile(currentPath, []byte(result.Merged), 0o644); err != nil { //nolint:gosec
		return fmt.Errorf("merge-driver: write result to %s: %w", currentPath, err)
	}

	// Write conflict records and emit Notifier alert (best-effort).
	if len(result.ConflictRecords) > 0 {
		if gravaDir, gravaErr := grava.ResolveGravaDir(); gravaErr == nil {
			conflictsPath := filepath.Join(gravaDir, "conflicts.json")
			if b, marshalErr := json.MarshalIndent(result.ConflictRecords, "", "  "); marshalErr == nil {
				_ = os.WriteFile(conflictsPath, b, 0o644) //nolint:gosec
			}
		}
		_ = Notifier.Send("merge-conflict",
			fmt.Sprintf("%d conflict record(s) detected. Run 'grava resolve list' to view.", len(result.ConflictRecords)))
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
			"⚠️  %d conflict record(s) written. Run 'grava resolve list'.\n", len(result.ConflictRecords))
	}

	if result.HasGitConflict {
		// Non-zero exit tells Git that conflicts remain.
		conflictExitFn(1)
	}

	return nil
}
