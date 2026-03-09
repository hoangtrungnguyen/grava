# Add --json output flag Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a global `--json` flag to the Grava CLI to enable machine-readable output for all commands.

**Architecture:** 
1. Add a persistent `--json` flag to the root command.
2. Update the `RunE` function of each command to check for this flag.
3. If `--json` is set, collect the result data into a struct/map, marshal it to JSON, and print it to stdout.
4. Ensure human-readable output (tables, etc.) is bypassed when JSON mode is active.

**Tech Stack:** 
- Go (Standard Library `encoding/json`)
- Cobra (CLI Framework)

---

### Task 1: Setup Global JSON Flag

**Files:**
- Modify: `pkg/cmd/root.go:20-25` (Add global variable)
- Modify: `pkg/cmd/root.go:85-95` (Add persistent flag)

**Step 1: Add global variable `outputJSON`**

```go
var (
	cfgFile    string
	dbURL      string
	actor      string
	agentModel string
	Store      dolt.Store
	outputJSON bool // Add this
)
```

**Step 2: Add persistent flag `--json`**

```go
rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output in JSON format")
```

**Step 3: Verify build**

Run: `go build ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add pkg/cmd/root.go
git commit -m "feat(cli): Add global --json flag (grava-de78.8)"
```

---

### Task 2: Implement JSON Output for `list` command

**Files:**
- Modify: `pkg/cmd/list.go`

**Step 1: Define Issue JSON struct**

```go
type IssueListItem struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    Type      string    `json:"type"`
    Priority  int       `json:"priority"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
}
```

**Step 2: Update `RunE` to handle JSON**

```go
        // ... after scanning rows into variables ...
        item := IssueListItem{
            ID:        id,
            Title:     title,
            Type:      iType,
            Priority:  priority,
            Status:    status,
            CreatedAt: createdAt,
        }
        
        if outputJSON {
            results = append(results, item)
        } else {
            // ... existing tabwriter logic ...
        }
```

**Step 3: Print JSON if flag is set**

```go
    if outputJSON {
        b, _ := json.MarshalIndent(results, "", "  ")
        fmt.Fprintln(cmd.OutOrStdout(), string(b))
        return nil
    }
```

**Step 4: Verify with a mock test**

Create a temporary test or run `./grava list --json` (ensure database is up).

**Step 5: Commit**

```bash
git add pkg/cmd/list.go
git commit -m "feat(cli): Implement JSON output for list command (grava-de78.8)"
```

---

### Task 3: Implement JSON Output for `show` command

**Files:**
- Modify: `pkg/cmd/show.go`

**Step 1: Update `show` command to support JSON**

Collect all scanned fields into an `IssueDetail` struct and output as JSON if `outputJSON` is true.

**Step 2: Commit**

```bash
git add pkg/cmd/show.go
git commit -m "feat(cli): Implement JSON output for show command (grava-de78.8)"
```

---

### Task 4: Implement JSON Output for `create` and `subtask` commands

**Files:**
- Modify: `pkg/cmd/create.go`
- Modify: `pkg/cmd/subtask.go`

**Step 1: Update `create` and `subtask` to return JSON**

Instead of `"✅ Created issue: [ID]"`, return `{"id": "[ID]", "status": "created"}`.

**Step 2: Commit**

```bash
git add pkg/cmd/create.go pkg/cmd/subtask.go
git commit -m "feat(cli): Implement JSON output for create/subtask commands (grava-de78.8)"
```

---

### Task 5: Implement JSON Output for `update`, `comment`, `label`, `assign`, `dep`

**Files:**
- Modify: Several command files in `pkg/cmd/`

**Step 1: Consistently return JSON for modification commands**

Return `{"id": "[ID]", "status": "updated", "field": "[FIELD]"}` or similar.

**Step 2: Commit**

```bash
git commit -a -m "feat(cli): Implement JSON output for update/management commands (grava-de78.8)"
```

---

### Task 6: Final Verification & Documentation

**Step 1: Run all tests**

Run: `go test ./...`
Expected: PASS

**Step 2: Update `docs/CLI_REFERENCE.md`**

Add `--json` flag to the global flags section.

**Step 3: Commit**

```bash
git add docs/CLI_REFERENCE.md
git commit -m "docs: Update CLI reference for --json flag (grava-de78.8)"
```
