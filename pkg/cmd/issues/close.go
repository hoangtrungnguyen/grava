package issues

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/hoangtrungnguyen/grava/pkg/graph"
	"github.com/hoangtrungnguyen/grava/pkg/utils"
	"github.com/spf13/cobra"
)

// CloseResult is the JSON output model for the close command.
type CloseResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func newCloseCmd(d *cmddeps.Deps) *cobra.Command {
	var force bool
	var pruneBranch bool

	cmd := &cobra.Command{
		Use:   "close <issue-id>",
		Short: "Close an issue and tear down its worktree",
		Long: `Close an issue by removing its Git worktree (.worktree/<id>), cleaning up
any Claude symlink, and setting status to closed.

The branch (grava/<id>) is kept so sibling tasks that depend on this issue's
work can still merge from it. Pass --prune-branch to delete it explicitly.

Blocks if the worktree has uncommitted changes unless --force is used.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			issueID := args[0]

			cwd, err := os.Getwd()
			if err != nil {
				return gravaerrors.New("CWD_UNREACHABLE", "failed to get working directory", err)
			}

			// AC#4: Claude environment safety
			if utils.IsInsideClaudeWorktree(cwd) {
				return gravaerrors.New("CLAUDE_WORKTREE_DETECTED",
					"You are inside a Claude-managed worktree. Exit this session first and run `grava close` from the project root.", nil)
			}

			// AC#1: Check for dirty worktree (block if uncommitted changes)
			worktreeDir := filepath.Join(cwd, ".worktree", issueID)
			if _, err := os.Stat(worktreeDir); err == nil {
				if !force {
					dirty, err := utils.IsWorktreeDirty(cwd, issueID)
					if err != nil {
						return gravaerrors.New("WORKTREE_CHECK_FAILED",
							fmt.Sprintf("failed to check worktree status: %v", err), err)
					}
					if dirty {
						return gravaerrors.New("WORKTREE_DIRTY",
							fmt.Sprintf("worktree .worktree/%s has uncommitted changes. Commit or stash first, or use --force to override.", issueID), nil)
					}
				}

				// Remove worktree directory; keep the branch by default so
				// sibling tasks that block on this one can still merge from it.
				// Pass --prune-branch to also delete the branch.
				var cleanErr error
				if pruneBranch {
					cleanErr = utils.DeleteWorktree(cwd, issueID)
				} else {
					cleanErr = utils.RemoveWorktreeOnly(cwd, issueID)
				}
				if cleanErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  Worktree cleanup warning: %v\n", cleanErr)
				}
			}

			// Clean up Claude symlink if it exists
			claudeSymlink := filepath.Join(cwd, ".claude", "worktrees", issueID)
			if _, err := os.Lstat(claudeSymlink); err == nil {
				_ = os.Remove(claudeSymlink)
			}

			// Update status to closed via graph layer
			dag, err := graph.LoadGraphFromDB(*d.Store)
			if err != nil {
				return gravaerrors.New("DB_UNREACHABLE", "failed to load graph", err)
			}
			dag.SetSession(*d.Actor, *d.AgentModel)
			if err := dag.SetNodeStatus(issueID, graph.StatusClosed); err != nil {
				return gravaerrors.New("STATUS_UPDATE_FAILED",
					fmt.Sprintf("failed to close issue: %v", err), err)
			}

			// Advance pipeline_phase to "complete" so wisps stay consistent
			// with issue status. Soft failure: a stale phase is bad UX but
			// not worth blocking the close.
			if _, sigErr := signalRun(cmd.Context(), *d.Store, SignalParams{
				IssueID: issueID,
				Kind:    SignalPipelineComplete,
				Actor:   *d.Actor,
			}); sigErr != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  pipeline_phase update skipped: %v\n", sigErr)
			}

			result := CloseResult{ID: issueID, Status: "closed"}
			if *d.OutputJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Closed %s\n", issueID)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force close even with uncommitted changes")
	cmd.Flags().BoolVar(&pruneBranch, "prune-branch", false, "Also delete the grava/<id> branch (default: keep for sibling merges)")
	return cmd
}
