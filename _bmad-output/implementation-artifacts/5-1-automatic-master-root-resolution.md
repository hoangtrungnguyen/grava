# Story 5.1: Automatic Master-Root Resolution

Status: ready-for-dev

## Story

As a developer working in an isolated worktree,
I want Grava to automatically find its master database when running inside a Claude worktree,
So that my progress is synced to the main project regardless of my current directory.

## Acceptance Criteria

1. **AC#1 -- Parent Database Discovery**
   When `grava` is executed from a subdirectory (like `.worktree/issue-123/`),
   Then it MUST recursively walk up parent directories to find the `.grava/` folder and its `issues.db`.

2. **AC#2 -- Environment Context Awareness**
   `grava --context` MUST show the identified "Workspace Root" vs the "Current Directory".
   If no `.grava/` folder is found all the way to the system root, it MUST return a standard `GRAVA_NOT_INITIALIZED` error.

3. **AC#3 -- Global Database Lock Awareness**
   All writes to the database (audit logs, wisp entries) MUST use the absolute path of the `issues.db` found in AC#1 to ensure consistency across multiple concurrent worktrees.

4. **AC#4 -- Identity Standard Verification**
   If running in a worktree, Grava MUST verify the current branch matches the naming standard (`grava/<id>`). If it follows the Claude default (`worktree-<id>`), Grava MUST prompt or automatically rename it for consistency.

## Tasks / Subtasks

- [ ] Task 1: Implement Recursive Root Discovery
  - [ ] 1.1 Create a helper function `FindWorkspaceRoot()` that walks up from `os.Getwd()`.
  - [ ] 1.2 Update the database connection logic to use the discovered root.

- [ ] Task 2: Update CLI Context Command
  - [ ] 2.1 Add workspace info to the diagnostic output.

## Dev Notes

### Path Security
Be careful with symlinks. The root discovery should resolve paths to ensure it doesn't get stuck in a loop.

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-05-worktree.md#Story-5.1]
