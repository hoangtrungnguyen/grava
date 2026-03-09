# Git Merge Driver Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform Git from a text-based version control system into a schema-aware database manager for Grava using a custom JSONL merge driver.

**Architecture:** 
- `grava merge-slot`: The core driver that accepts `%O`, `%A`, and `%B` paths from Git, parses JSONL into memory, maps by Issue ID, and executes a 3-way merge on specific fields. Exits with 0 on success, or 1 on unresolvable conflicts.
- `grava resolve`: An interactive CLI tool that parses merge conflict blocks (or special conflict JSON structure) and prompts the user to select Local, Remote, or manually edit the conflict.
- `grava install` (or `grava init` augmentation): An automated setup command that registers the merge driver in `.git/config`, sets up `issues.jsonl merge=grava` in `.gitattributes`, and generates `pre-commit`, `post-merge`, and `post-checkout` hooks.
- Git Hooks: 
  - `pre-commit`: runs `grava export --file issues.jsonl` and `git add issues.jsonl`.
  - `post-merge` and `post-checkout`: runs `grava import --file issues.jsonl --overwrite`.

**Tech Stack:** Go, Cobra CLI, Git, JSONL

---

### Task 1: Core Command Scaffolding & Hook Templates

**Files:**
- Create: `pkg/cmd/merge_slot.go`
- Create: `pkg/cmd/resolve.go`
- Create: `pkg/cmd/install.go`

**Step 1: Write the failing tests for command registration**

```go
// in pkg/cmd/commands_test.go
func TestNewCommands(t *testing.T) {
    // Add tests verifying merge-slot, resolve, and install commands exist
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/cmd -run TestNewCommands -v`
Expected: FAIL due to missing commands

**Step 3: Write minimal implementation**

Create `pkg/cmd/merge_slot.go` with basic `cobra.Command` that takes `--ancestor`, `--current`, `--other`, and `--output` flags.
Create `pkg/cmd/resolve.go` with basic `cobra.Command`.
Create `pkg/cmd/install.go` with basic `cobra.Command` that writes git hooks and config.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/cmd -run TestNewCommands -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/cmd/merge_slot.go pkg/cmd/resolve.go pkg/cmd/install.go pkg/cmd/commands_test.go
git commit -m "feat: scaffold commands for git merge driver"
```

---

### Task 2: Implement Three-Way Merge Logic (`merge-slot`)

**Files:**
- Create: `pkg/merge/merge.go`
- Create: `pkg/merge/merge_test.go`
- Modify: `pkg/cmd/merge_slot.go`

**Step 1: Write the failing tests**

```go
// in pkg/merge/merge_test.go
func TestThreeWayMerge(t *testing.T) {
    // Inject mock JSONL for Ancestor, Current, Other
    // Verify non-conflicting field edits are merged
    // Verify true conflicts return an error and a conflict marker
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/merge -v`
Expected: FAIL since package doesn't exist yet

**Step 3: Write minimal implementation**

Implement `Func ProcessMerge(ancestor, current, other string) (merged string, hasConflict bool, err error)` in `pkg/merge/merge.go`. Map issues by ID, compare field-by-field. If field mod in Local but not Remote = Local, etc. If both modified differently, create a conflict struct. 

Integrate it into `pkg/cmd/merge_slot.go` to parse input files, execute merge, and write back to `--output` (or original `%A` file). Return exit code 1 if `hasConflict` is true.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/merge -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/merge/ pkg/cmd/merge_slot.go
git commit -m "feat: implement JSONL three-way merge logic"
```

---

### Task 3: Implement Automated Setup (`install`)

**Files:**
- Modify: `pkg/cmd/install.go`
- Create: `pkg/cmd/install_test.go`

**Step 1: Write the failing test**

```go
// in pkg/cmd/install_test.go
func TestInstallConfiguresGit(t *testing.T) {
    // create temp git repo
    // run install command logic 
    // read .git/config and .gitattributes to ensure presence of Grava driver
    // verify hooks exist in .git/hooks/
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/cmd -run TestInstallConfiguresGit -v`

**Step 3: Write minimal implementation**

In `pkg/cmd/install.go`:
- Use `exec.Command("git", "config", "merge.grava.name", "Grava JSONL merge driver")`
- Set `merge.grava.driver = "grava merge-slot --ancestor %O --current %A --other %B --output %A"`
- Append `issues.jsonl merge=grava` to `.gitattributes` or `.grava/.gitattributes`
- Create hook files in `.git/hooks/pre-commit`, `.git/hooks/post-merge`, `.git/hooks/post-checkout` with execution bits set (`chmod +x`).

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/cmd -run TestInstallConfiguresGit -v`

**Step 5: Commit**

```bash
git add pkg/cmd/install.go pkg/cmd/install_test.go
git commit -m "feat: automated setup for git hooks and config"
```

---

### Task 4: Implement Human Overseer (`resolve`)

**Files:**
- Modify: `pkg/cmd/resolve.go`
- Create: `pkg/merge/resolve_test.go`

**Step 1: Write failing test**

```go
// Verify parsing of conflict markers and resolution
```

**Step 2: Run test to verify it fails**

Run tests.

**Step 3: Write minimal implementation**

In `pkg/cmd/resolve.go`, write logic to scan `issues.jsonl` for the conflict markers left by `merge-slot`. Use an interactive prompt (e.g., `survey` or fmt scan) to ask the user to pick `[L] Local` or `[R] Remote`. Once resolved, serialize back to clean JSONL and tell the user to `git add`.

**Step 4: Run test to verify it passes**

Run tests.

**Step 5: Commit**

```bash
git add pkg/cmd/resolve.go pkg/merge/
git commit -m "feat: interactive conflict resolution CLI"
```

---

### Task 5: Integration Testing in Real Git Repo

**Files:**
- Create: `e2e/git_driver_test.go` or `scripts/test_git_driver.sh`

**Step 1: Write the test script**

Create a script `scripts/test_git_driver.sh` that:
1. `git init testrepo`
2. Runs `grava install`
3. Creates a base issue, commits.
4. Branches `FeatureA`, changes field A, commits.
5. Branches `FeatureB` from base, changes field B, commits.
6. Merges `FeatureA` into `FeatureB`.
7. Asserts merge success and `issues.jsonl` contains both field A and field B changes.
8. Creates a conflict (modify same field), merges, asserts it stops and leaves a conflict marker.

**Step 2: Run test script**

Run: `bash scripts/test_git_driver.sh`

**Step 3: Fix issues and verify passes**

Adjust any code in `grava` to ensure the e2e test behaves perfectly. 

**Step 4: Commit**

```bash
git add scripts/test_git_driver.sh
git commit -m "test: add integration test for git merge driver"
```
