# Story 5.5: Configure Claude & Git on Init

Status: ready-for-dev

## Story

As a developer,
I want `grava init` to automatically configure my Git and Claude settings to use the `.worktree/` folder at the project root,
So that all isolated sessions are stored in a unified, consistent location.

## Acceptance Criteria

1. **AC#1 -- Unified Worktree Directory Setup**
   When `grava init` is run,
   Then it MUST create a hidden directory `.worktree/` at the project root.
   It MUST add `.worktree/` to the project's `.gitignore` automatically.

2. **AC#2 -- Git Config Injection**
   `grava init` MUST set a local Git config entry:
   `git config grava.worktreeDir .worktree`
   This serves as the source of truth for the binary.

3. **AC#3 -- Claude Settings Refinement**
   Grava MUST update `.claude/settings.json` with the `worktree` block:
   ```json
   {
     "worktree": {
       "symlinkDirectories": ["node_modules", ".cache"],
       "sparsePaths": []
     }
   }
   ```
   *Note:* Since Claude defaults to `.claude/worktrees/`, Grava MUST also check if it can configure Claude to use the root `.worktree/` (perhaps via a hook or by recommending the `--worktree` path in `grava claim`).

4. **AC#4 -- Pre-flight Validation**
   `grava init` MUST verify that Git version is 2.17+ (required for `git worktree remove`).

## Tasks / Subtasks

- [ ] Task 1: Implement Project Bootstrapper
  - [ ] 1.1 Create `.worktree/` directory.
  - [ ] 1.2 Update `.gitignore`.
  - [ ] 1.3 Set `git config` values.

- [ ] Task 2: Implement Claude Config Sync
  - [ ] 2.1 Refine the `settings.json` merging logic from Story 5.5 (previous iteration).

## Dev Notes

### Path Standardization
All Grava commands should use `git config --get grava.worktreeDir` to resolve the base path for claims.

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-05-worktree.md#Story-5.5]
