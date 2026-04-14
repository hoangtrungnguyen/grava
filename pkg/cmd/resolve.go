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

var resolveCmd = &cobra.Command{
	Use:   "resolve",
	Short: "Manage merge conflict resolutions",
	Long:  `List and resolve schema-level merge conflicts recorded in .grava/conflicts.json.`,
}

var resolveListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending merge conflicts",
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, path, err := readConflicts()
		if err != nil {
			return err
		}
		// Count unresolved entries first so we can print "no conflicts" when all
		// are resolved, even if the file exists and has entries.
		var pending []merge.ConflictEntry
		for _, e := range entries {
			if !e.Resolved {
				pending = append(pending, e)
			}
		}
		if len(pending) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No pending conflicts.")
			return nil
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Conflicts in %s:\n\n", path)
		for _, e := range pending {
			field := e.Field
			if field == "" {
				field = "(whole issue)"
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"  ID: %s\n  Issue: %s  Field: %s\n  Local:  %s\n  Remote: %s\n\n",
				e.ID, e.IssueID, field, string(e.Local), string(e.Remote))
		}
		return nil
	},
}

var resolvePickChoice string

var resolvePickCmd = &cobra.Command{
	Use:   "pick <conflict-id>",
	Short: "Resolve a conflict by choosing local or remote",
	Long: `Apply a resolution to a pending conflict.

Choices:
  local   — keep the value from the current branch (%A)
  remote  — take the value from the other branch (%B)

After picking, the conflicted field in issues.jsonl is updated and the
conflict is marked resolved in .grava/conflicts.json.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if resolvePickChoice != "local" && resolvePickChoice != "remote" {
			return fmt.Errorf("--choice must be 'local' or 'remote'")
		}
		conflictID := args[0]

		entries, conflictsPath, err := readConflicts()
		if err != nil {
			return err
		}

		var target *merge.ConflictEntry
		for i := range entries {
			if entries[i].ID == conflictID {
				target = &entries[i]
				break
			}
		}
		if target == nil {
			return fmt.Errorf("conflict %q not found", conflictID)
		}
		if target.Resolved {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Conflict %s is already resolved.\n", conflictID)
			return nil
		}

		// Determine the winning value.
		var winner json.RawMessage
		if resolvePickChoice == "local" {
			winner = target.Local
		} else {
			winner = target.Remote
		}

		// Apply the resolution to issues.jsonl.
		if err := applyConflictResolution(target, winner); err != nil {
			return fmt.Errorf("failed to apply resolution: %w", err)
		}

		// Mark the conflict resolved.
		target.Resolved = true
		if err := writeConflicts(conflictsPath, entries); err != nil {
			return fmt.Errorf("failed to update conflicts file: %w", err)
		}

		field := target.Field
		if field == "" {
			field = "(whole issue)"
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(),
			"✅ Resolved conflict %s: issue %s field %q → %s\n",
			conflictID, target.IssueID, field, resolvePickChoice)
		return nil
	},
}

// readConflicts reads .grava/conflicts.json and returns entries + path.
func readConflicts() ([]merge.ConflictEntry, string, error) {
	gravaDir, err := grava.ResolveGravaDir()
	if err != nil {
		return nil, "", fmt.Errorf("not in a grava repository: %w", err)
	}
	path := filepath.Join(gravaDir, "conflicts.json")

	b, err := os.ReadFile(path) //nolint:gosec
	if os.IsNotExist(err) {
		return nil, path, nil
	}
	if err != nil {
		return nil, path, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var entries []merge.ConflictEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, path, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return entries, path, nil
}

// writeConflicts writes entries to the given path as pretty JSON.
func writeConflicts(path string, entries []merge.ConflictEntry) error {
	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644) //nolint:gosec
}

// applyConflictResolution patches issues.jsonl by replacing the conflicted
// field with the chosen winner value and removing the _conflict marker.
func applyConflictResolution(target *merge.ConflictEntry, winner json.RawMessage) error {
	const issuesFile = "issues.jsonl"

	b, err := os.ReadFile(issuesFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", issuesFile, err)
	}

	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	var out []string

	for _, line := range lines {
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			// Pass through unparseable lines unchanged.
			out = append(out, line)
			continue
		}

		issueID, _ := obj["id"].(string)
		if issueID != target.IssueID {
			// Not the issue we're resolving — keep unchanged.
			reMarshaled, err := merge.MarshalSorted(obj)
			if err != nil {
				out = append(out, line)
			} else {
				out = append(out, string(reMarshaled))
			}
			continue
		}

		if target.Field == "" {
			// Whole-issue conflict — replace the line with the winner value.
			out = append(out, string(winner))
			continue
		}

		// Field-level conflict — replace the conflicted field.
		var winnerVal interface{}
		if err := json.Unmarshal(winner, &winnerVal); err != nil {
			return fmt.Errorf("failed to parse winner value: %w", err)
		}
		if winnerVal == nil {
			// Winner chose to delete the field.
			delete(obj, target.Field)
		} else {
			obj[target.Field] = winnerVal
		}

		reMarshaled, err := merge.MarshalSorted(obj)
		if err != nil {
			return fmt.Errorf("failed to marshal resolved issue: %w", err)
		}
		out = append(out, string(reMarshaled))
	}

	result := strings.Join(out, "\n") + "\n"
	return os.WriteFile(issuesFile, []byte(result), 0644) //nolint:gosec
}

func init() {
	resolveCmd.AddCommand(resolveListCmd)
	resolveCmd.AddCommand(resolvePickCmd)
	rootCmd.AddCommand(resolveCmd)

	resolvePickCmd.Flags().StringVar(&resolvePickChoice, "choice", "", "Resolution: 'local' or 'remote'")
	_ = resolvePickCmd.MarkFlagRequired("choice")
}
