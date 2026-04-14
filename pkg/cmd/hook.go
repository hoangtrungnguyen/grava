package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	synccmd "github.com/hoangtrungnguyen/grava/pkg/cmd/sync"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Git hook dispatch",
	Long:  `Parent command for Git hook handlers. Used by grava-managed hook shims.`,
}

var hookRunCmd = &cobra.Command{
	Use:   "run <hook-name> [args...]",
	Short: "Run a named Git hook handler",
	Long: `Dispatch a named Git hook handler.

Called automatically by grava-managed Git hook shims (installed by 'grava install').
Supported hooks:
  post-merge      Sync Dolt if issues.jsonl changed during the merge
  post-checkout   Sync Dolt if issues.jsonl changed on the new branch
  pre-commit      Validate issues.jsonl format before allowing the commit
  prepare-commit-msg  No-op placeholder`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		hookName := args[0]
		hookArgs := args[1:]
		switch hookName {
		case "post-merge":
			return runPostMerge(cmd)
		case "post-checkout":
			return runPostCheckout(cmd, hookArgs)
		case "pre-commit":
			return runPreCommit(cmd)
		case "prepare-commit-msg":
			return nil // placeholder, no-op
		default:
			return nil // unknown hooks are silently ignored
		}
	},
}

// tryConnectDB attempts to connect to the Dolt database using the same
// resolution chain as PersistentPreRunE: --db-url flag → viper (config/env)
// → hardcoded default. Hook commands skip PersistentPreRunE so they must
// replicate this chain themselves.
func tryConnectDB() (dolt.Store, error) {
	url := dbURL // set by --db-url flag during cobra flag parsing
	if url == "" {
		url = viper.GetString("db_url")
	}
	if url == "" {
		url = "root@tcp(127.0.0.1:3306)/grava?parseTime=true"
	}
	return dolt.NewClient(url)
}

// issuesChangedInMerge reports whether issues.jsonl was modified in the most
// recent merge by comparing HEAD@{1} with HEAD.
func issuesChangedInMerge() bool {
	out, err := exec.Command("git", "diff", "--name-only", "HEAD@{1}", "HEAD", "--", "issues.jsonl").Output()
	if err != nil {
		// No reflog entry (fresh repo) or other error — be conservative and check
		// whether the file exists; treat it as changed so the sync still runs.
		_, statErr := os.Stat("issues.jsonl")
		return statErr == nil
	}
	return strings.TrimSpace(string(out)) != ""
}

// issuesChangedInCheckout reports whether issues.jsonl differs between the two
// provided git refs. Used by the post-checkout handler.
func issuesChangedInCheckout(prev, next string) bool {
	out, err := exec.Command("git", "diff", "--name-only", prev, next, "--", "issues.jsonl").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// syncFromFile connects to the DB, imports issues.jsonl (upsert), and reports
// the result. On DB connection failure it prints a warning and returns nil so
// the Git operation is never blocked.
func syncFromFile(cmd *cobra.Command, trigger string) error {
	if _, err := os.Stat("issues.jsonl"); os.IsNotExist(err) {
		return nil // no file — nothing to sync
	}

	store, err := tryConnectDB()
	if err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
			"grava hook: DB unavailable (%s), skipping sync: %v\n", trigger, err)
		return nil
	}
	defer store.Close() //nolint:errcheck

	result, err := synccmd.SyncIssuesFile(context.Background(), store, "issues.jsonl")
	if err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
			"grava hook: sync failed after %s: %v\n", trigger, err)
		return nil
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(),
		"grava: synced %d issues from issues.jsonl after %s\n",
		result.Imported+result.Updated, trigger)
	return nil
}

func runPostMerge(cmd *cobra.Command) error {
	if !issuesChangedInMerge() {
		return nil
	}
	return syncFromFile(cmd, "merge")
}

func runPostCheckout(cmd *cobra.Command, args []string) error {
	// Git passes: prev-head new-head is-branch-checkout
	if len(args) < 2 {
		return nil
	}
	prevHead, newHead := args[0], args[1]
	if !issuesChangedInCheckout(prevHead, newHead) {
		return nil
	}
	return syncFromFile(cmd, "checkout")
}

func runPreCommit(cmd *cobra.Command) error {
	if _, err := os.Stat("issues.jsonl"); os.IsNotExist(err) {
		return nil // file not present — nothing to validate
	}

	f, err := os.Open("issues.jsonl") //nolint:gosec
	if err != nil {
		return fmt.Errorf("grava hook pre-commit: failed to open issues.jsonl: %w", err)
	}
	defer f.Close() //nolint:errcheck

	if err := synccmd.ValidateJSONL(f); err != nil {
		return fmt.Errorf("grava hook pre-commit: issues.jsonl is malformed: %w", err)
	}
	return nil
}

func init() {
	hookCmd.AddCommand(hookRunCmd)
	rootCmd.AddCommand(hookCmd)
}
