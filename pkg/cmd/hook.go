package cmd

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmd/reserve"
	synccmd "github.com/hoangtrungnguyen/grava/pkg/cmd/sync"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/hoangtrungnguyen/grava/pkg/grava"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Git hook dispatch",
	Long:  `Parent command for Git hook handlers. Used by grava-managed hook shims.`,
}

// hookDryRun is bound to the --dry-run flag on hookRunCmd.
// When true, sync handlers print what would be imported without touching the DB.
var hookDryRun bool

var hookRunCmd = &cobra.Command{
	Use:   "run <hook-name> [args...]",
	Short: "Run a named Git hook handler",
	Long: `Dispatch a named Git hook handler.

Called automatically by grava-managed Git hook shims (installed by 'grava install').
Supported hooks:
  post-merge      Sync Dolt if issues.jsonl changed during the merge
  post-checkout   Sync Dolt if issues.jsonl changed on the new branch
  pre-commit      Validate issues.jsonl format before allowing the commit
  prepare-commit-msg  No-op placeholder

Use --dry-run to preview what would be synced without touching the database.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		hookName := args[0]
		hookArgs := args[1:]
		switch hookName {
		case "post-merge":
			return runPostMerge(cmd, hookDryRun)
		case "post-checkout":
			return runPostCheckout(cmd, hookArgs, hookDryRun)
		case "pre-commit":
			return runPreCommit(cmd)
		case "prepare-commit-msg":
			return nil // placeholder, no-op
		default:
			return nil // unknown hooks are silently ignored
		}
	},
}

// connectDBFn is the hook used by syncFromFile to obtain a Dolt store.
// It is overridden in tests to inject a mock store without touching real Dolt.
var connectDBFn = func() (dolt.Store, error) { return tryConnectDB() }

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

// hashFile returns the SHA-256 hex digest of the file at path.
func hashFile(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// importHashPath resolves the path to .grava/last_import_hash.
func importHashPath() (string, error) {
	gravaDir, err := grava.ResolveGravaDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(gravaDir, "last_import_hash"), nil
}

// readLastImportHash returns the hash stored after the last successful sync,
// or "" when no hash has been recorded yet.
func readLastImportHash() string {
	path, err := importHashPath()
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// writeLastImportHash persists hash to .grava/last_import_hash after a
// successful sync. Errors are silently ignored: a missing hash file only means
// the next hook invocation will recheck Dolt status instead of short-circuiting.
func writeLastImportHash(hash string) {
	path, err := importHashPath()
	if err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(hash), 0644) //nolint:gosec
}

// hasDoltUncommittedChanges returns true when dolt_status reports any
// staged or unstaged rows — i.e., there are working-set changes that an
// import --overwrite would silently discard.
//
// All errors are treated as "no changes" (fail-open) so hook execution is
// never blocked on non-Dolt backends or when dolt_status is unavailable.
func hasDoltUncommittedChanges(store dolt.Store) bool {
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

// countJSONLRecords counts the lines in a JSONL file that contain valid JSON objects.
// Blank lines and non-JSON content are excluded so the dry-run count matches
// what importFlatJSONL would actually process.
func countJSONLRecords(path string) (int, error) {
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return 0, err
	}
	defer f.Close() //nolint:errcheck

	var n int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if json.Unmarshal([]byte(line), &obj) == nil {
			n++
		}
	}
	return n, scanner.Err()
}

// syncFromFile connects to the DB, runs the A+C safety checks, imports
// issues.jsonl (upsert), and updates the stored content hash.
//
// When dryRun is true the function counts and prints what would be synced
// without connecting to the database or modifying any state.
//
// Safety checks (live mode only):
//
//	A — Content Hash: if the current issues.jsonl hash matches the hash stored
//	    from the last sync, the file content is unchanged and the sync is
//	    skipped to avoid redundant work.
//	C — Dolt Commit Tracking: if Dolt has uncommitted working-set changes,
//	    the sync is skipped to prevent silently overwriting them.
//
// On DB connection failure the function prints a warning and returns nil so
// Git operations are never blocked.
func syncFromFile(cmd *cobra.Command, trigger string, dryRun bool) error {
	issuesPath := resolveIssuesFilePath()
	if _, err := os.Stat(issuesPath); os.IsNotExist(err) {
		return nil // no file — nothing to sync
	}

	if dryRun {
		count, err := countJSONLRecords(issuesPath)
		if err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
				"grava hook [dry-run]: failed to read %s: %v\n", issuesPath, err)
			return nil
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(),
			"grava hook [dry-run]: would sync up to %d issues from issues.jsonl after %s\n",
			count, trigger)
		return nil
	}

	// Check A: skip if we already imported this exact file content.
	currentHash, hashErr := hashFile(issuesPath)
	if hashErr == nil && currentHash == readLastImportHash() {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(),
			"grava hook: issues.jsonl unchanged since last sync, skipping (%s)\n", trigger)
		return nil
	}

	store, err := connectDBFn()
	if err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
			"grava hook: DB unavailable (%s), skipping sync: %v\n", trigger, err)
		return nil
	}
	defer store.Close() //nolint:errcheck

	// Check C: skip if Dolt has uncommitted changes that would be overwritten.
	if hasDoltUncommittedChanges(store) {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
			"grava hook: skipping sync after %s — Dolt has uncommitted changes that would be overwritten.\n"+
				"  Commit first: 'grava commit -m <msg>', then re-run: 'grava hook run %s'\n",
			trigger, trigger)
		return nil
	}

	result, err := synccmd.SyncIssuesFile(context.Background(), store, issuesPath)
	if err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
			"grava hook: sync failed after %s: %v\n", trigger, err)
		return nil
	}

	// Persist the hash so subsequent hook calls with the same content are skipped.
	if hashErr == nil {
		writeLastImportHash(currentHash)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(),
		"grava: synced %d issues from issues.jsonl after %s\n",
		result.Imported+result.Updated, trigger)
	return nil
}

func runPostMerge(cmd *cobra.Command, dryRun bool) error {
	if !issuesChangedInMerge() {
		return nil
	}
	return syncFromFile(cmd, "merge", dryRun)
}

func runPostCheckout(cmd *cobra.Command, args []string, dryRun bool) error {
	// Git passes: prev-head new-head is-branch-checkout
	if len(args) < 2 {
		return nil
	}
	prevHead, newHead := args[0], args[1]
	if !issuesChangedInCheckout(prevHead, newHead) {
		return nil
	}
	return syncFromFile(cmd, "checkout", dryRun)
}

func runPreCommit(cmd *cobra.Command) error {
	// 1. Check file reservations: block commits to paths held by other agents.
	if err := checkReservationConflicts(cmd); err != nil {
		return err
	}

	// 2. Validate issues.jsonl format (existing behavior).
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

// stagedFilesFn is the function used to get staged files. Overridden in tests.
var stagedFilesFn = getStagedFiles

// getStagedFiles returns the list of staged file paths via git diff --cached.
func getStagedFiles() ([]string, error) {
	out, err := exec.Command("git", "diff", "--cached", "--name-only").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get staged files: %w", err)
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\n"), nil
}

// checkReservationConflicts queries active exclusive leases and blocks if any
// staged paths are reserved by another agent. DB connection failures are
// non-fatal (fail-open) so hooks never block on infrastructure issues.
func checkReservationConflicts(cmd *cobra.Command) error {
	staged, err := stagedFilesFn()
	if err != nil || len(staged) == 0 {
		return nil // no staged files or can't determine — allow commit
	}

	store, err := connectDBFn()
	if err != nil {
		// DB unavailable — fail open, don't block commits
		return nil
	}
	defer store.Close() //nolint:errcheck

	actor := viper.GetString("actor")
	if actor == "" || actor == "unknown" {
		// No identity configured — check ALL exclusive leases to be safe.
		// Without a unique actor, we can't distinguish "own" vs "other" leases.
		actor = ""
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	conflicts, err := reserve.CheckStagedConflicts(ctx, store, staged, actor)
	if err != nil {
		// Query failure — fail open
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
			"grava hook pre-commit: reservation check failed (allowing commit): %v\n", err)
		return nil
	}

	if len(conflicts) == 0 {
		return nil
	}

	// Block the commit with structured output.
	first := conflicts[0]
	errMsg := fmt.Sprintf("Path %s is reserved by %s until %s. Release or wait.",
		first.Path, first.AgentID, first.ExpiresTS.UTC().Format(time.RFC3339))

	result := map[string]interface{}{
		"code":      "FILE_RESERVATION_BLOCK",
		"message":   errMsg,
		"conflicts": conflicts,
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	return fmt.Errorf("grava hook pre-commit: %s\n%s", errMsg, string(b))
}

func init() {
	hookCmd.AddCommand(hookRunCmd)
	rootCmd.AddCommand(hookCmd)
	hookRunCmd.Flags().BoolVar(&hookDryRun, "dry-run", false,
		"Preview what would be synced without touching the database")
}
