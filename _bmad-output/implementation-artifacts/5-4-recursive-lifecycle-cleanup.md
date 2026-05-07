# Story 5.4: Recursive Lifecycle Cleanup

Status: ready-for-dev

## Story

As a developer,
I want `grava close` to safely delete my isolated directory and branch,
Preventing "worktree rot" in my local filesystem.

## Acceptance Criteria

1. **AC#1 -- Worktree Teardown (Standard)**
   When `grava close <id>` is run in a standard Grava worktree:
   - Verify all changes are committed/pushed (block if dirty).
   - Resolve to the parent directory.
   - Run `git worktree remove --force <path>`.
   - Update issue status to `done`.

2. **AC#2 -- Branch Deletion**
   Upon successful `close`, the branch `grava/<id>` MUST be deleted from the local repository.

3. **AC#3 -- Pause Behavior (Stop)**
   `grava stop <id>` MUST delete the worktree directory but **keep** the branch and Wisp state for future resumption.

4. **AC#4 -- Claude Environment Safety**
   If `grava close` is run inside a `.claude/worktrees/` directory:
   - It MUST WARN the user that Claude manages this directory.
   - It MUST instruct the user to `exit` and allow Claude to perform the cleanup.

## Tasks / Subtasks

- [ ] Task 1: Implement Teardown Logic
  - [ ] 1.1 Implement safe directory deletion using `git worktree remove`.
  - [ ] 1.2 Implement branch cleanup.

- [ ] Task 2: Safety Checks
  - [ ] 2.1 Add `IsDirty()` check to the worktree before closing.

## Dev Notes

### Git Worktree Remove
The command `git worktree remove` is available in Git 2.17+. Since we are on Linux (Ubuntu Noble), this is safely available.

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-05-worktree.md#Story-5.4]
