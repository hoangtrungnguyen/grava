# Plan: `grava sync` Command

**Created:** 2026-02-18  
**Status:** Planned  
**Ticket:** TASK-1-8-SEARCH-AND-MAINTENANCE

---

## Context & Scope

The full daemon-based sync server (Epic 3) was deliberately archived — it required cloud
infrastructure, a background daemon, and a RemotesAPI endpoint. That complexity is out of
scope for the current MVP.

`grava sync` is the **lightweight alternative**: a CLI command that wraps Dolt's native
`pull` and `push` operations via `os/exec`. It gives users a single, memorable command
to synchronise their local Dolt repo with a configured remote, without any daemon or
persistent process.

---

## User Story

**As a** developer or AI agent  
**I want to** run `grava sync` to pull remote changes and push local commits  
**So that** my local issue database stays in sync with the team's shared remote

---

## Acceptance Criteria (from TASK-1-8)

- [ ] `grava sync` synchronises the local database with remote
- [ ] Command returns proper exit codes and error messages
- [ ] Help documentation available

---

## Design Decisions

### 1. Execution strategy — `os/exec` over SQL

Dolt sync (`pull`, `push`, `commit`) is a **VCS operation**, not a SQL operation.
It must be invoked as a subprocess against the Dolt repo directory (`.grava/dolt/`),
not through the MySQL-compatible SQL interface.

The command will:
1. Resolve the Dolt repo path (default: `.grava/dolt/`, overridable via `--repo` flag)
2. Run `dolt pull [remote] [branch]`
3. Run `dolt push [remote] [branch]`
4. Stream stdout/stderr directly to the user

### 2. No auto-commit

`grava sync` will **not** auto-commit uncommitted local changes. If the user has
uncommitted mutations, Dolt will report them naturally. A future `--commit` flag
could be added to auto-commit with a generated message, but that is out of scope here.

### 3. Remote and branch configuration

Resolution order (highest priority first):

| Source | Example |
|---|---|
| CLI flags | `--remote origin --branch main` |
| `.grava.yaml` config | `sync.remote: origin` |
| Dolt repo defaults | whatever `dolt remote` shows |

### 4. `Store` interface is NOT used

Because sync is a filesystem/VCS operation, it does **not** go through the `dolt.Store`
interface. The command bypasses `PersistentPreRunE` (which sets up the DB connection)
by checking `cmd.Name() == "sync"` in the root guard — similar to how `init` is handled.

---

## Implementation Plan

### Step 1 — `pkg/cmd/sync.go`

```go
package cmd

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

var (
    syncRemote string
    syncBranch string
    syncRepo   string
)

var syncCmd = &cobra.Command{
    Use:   "sync",
    Short: "Synchronise local Dolt database with remote",
    Long: `Sync runs 'dolt pull' followed by 'dolt push' against the configured remote.

The Dolt repo is expected at .grava/dolt/ by default.
Override with --repo or the GRAVA_DOLT_REPO environment variable.

Examples:
  grava sync
  grava sync --remote origin --branch main
  grava sync --repo /path/to/dolt/repo`,
    RunE: func(cmd *cobra.Command, args []string) error {
        // Resolve repo path
        repo := syncRepo
        if repo == "" {
            repo = viper.GetString("dolt_repo")
        }
        if repo == "" {
            repo = filepath.Join(".grava", "dolt")
        }

        // Verify repo exists
        if _, err := os.Stat(repo); os.IsNotExist(err) {
            return fmt.Errorf("dolt repo not found at %q — run `grava init` first", repo)
        }

        remote := syncRemote
        if remote == "" {
            remote = viper.GetString("sync.remote")
        }
        if remote == "" {
            remote = "origin"
        }

        branch := syncBranch
        if branch == "" {
            branch = viper.GetString("sync.branch")
        }
        if branch == "" {
            branch = "main"
        }

        // Pull
        cmd.Printf("⬇️  Pulling from %s/%s...\n", remote, branch)
        if err := runDolt(repo, cmd, "pull", remote, branch); err != nil {
            return fmt.Errorf("dolt pull failed: %w", err)
        }

        // Push
        cmd.Printf("⬆️  Pushing to %s/%s...\n", remote, branch)
        if err := runDolt(repo, cmd, "push", remote, branch); err != nil {
            return fmt.Errorf("dolt push failed: %w", err)
        }

        cmd.Println("✅ Sync complete.")
        return nil
    },
}

// runDolt executes a dolt subcommand in the given repo directory,
// streaming output to the cobra command's writer.
func runDolt(repoDir string, cmd *cobra.Command, args ...string) error {
    c := exec.Command("dolt", args...)
    c.Dir = repoDir
    c.Stdout = cmd.OutOrStdout()
    c.Stderr = cmd.ErrOrStderr()
    return c.Run()
}

func init() {
    rootCmd.AddCommand(syncCmd)
    syncCmd.Flags().StringVar(&syncRemote, "remote", "", "Dolt remote name (default: origin)")
    syncCmd.Flags().StringVar(&syncBranch, "branch", "", "Branch to sync (default: main)")
    syncCmd.Flags().StringVar(&syncRepo,   "repo",   "", "Path to the Dolt repo directory")
}
```

### Step 2 — Bypass DB connection in `root.go`

Add `"sync"` to the guard in `PersistentPreRunE`:

```go
if cmd.Name() == "help" || cmd.Name() == "init" || cmd.Name() == "sync" {
    return nil
}
```

### Step 3 — Tests (`commands_test.go`)

Because `sync` shells out to `dolt`, unit tests must use a **fake `dolt` binary** on
`$PATH` or test the error paths (repo not found, pull fails, push fails) without
actually running Dolt.

Recommended approach: use `os.MkdirTemp` to create a fake repo dir, and inject a
`runDoltFn` variable (default `runDolt`) that tests can swap for a stub.

Test cases:
| Test | Scenario | Expected |
|---|---|---|
| `TestSyncCmdRepoNotFound` | `.grava/dolt` does not exist | error: "run `grava init` first" |
| `TestSyncCmdPullFails` | stub returns error on `pull` | error: "dolt pull failed" |
| `TestSyncCmdPushFails` | stub returns error on `push` | error: "dolt push failed" |
| `TestSyncCmdSuccess` | stub succeeds for both | output contains "Sync complete" |

### Step 4 — Docs (`CLI_REFERENCE.md`)

Add a `### sync` section covering:
- Usage + flags (`--remote`, `--branch`, `--repo`)
- Config file keys (`sync.remote`, `sync.branch`, `dolt_repo`)
- Prerequisites (Dolt must be installed, remote must be configured)
- Example output

---

## Out of Scope (deferred to Epic 3 if ever revived)

- Auto-commit before push
- Conflict resolution UI
- Background daemon / debounce batching
- RemotesAPI / gRPC transport
- Multi-remote fan-out

---

## Dependencies

- `dolt` binary must be on `$PATH` at runtime
- Dolt remote must be configured in the repo (`dolt remote add origin <url>`)
- No new Go dependencies required
