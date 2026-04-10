# Story 5.3: Claude Integration with Custom Path

Status: ready-for-dev

## Story

As a user of Claude Code,
I want Grava to ensure that `claude --worktree` uses the project's `.worktree/` folder instead of its default location,
Maintaining a unified environment for all isolated sessions.

## Acceptance Criteria

1. **AC#1 -- Worktree Redirection**
   If `grava claim --launch` is used, Grava MUST attempt to override Claude's default worktree path.
   *Strategy:* If Claude does not support a global `worktreeDir` setting, Grava's `--launch` command will:
     1. Claim the issue.
     2. Create a symlink from `.claude/worktrees/<id>` to `.worktree/<id>` (if possible) OR
     3. Instruct the user to run Claude from within the already-provisioned `.worktree/<id>`.

2. **AC#2 -- Identity Compliance**
   The branch name used by Claude MUST be reconciled to `grava/<id>` for consistency with Grava's default behavior.

3. **AC#3 -- Settings Sync**
   The `worktree` block in `.claude/settings.json` (from Story 5.5) MUST be validated as active when a Claude session starts.

## Tasks / Subtasks

- [ ] Task 1: Refine Claude Launch Sequence
  - [ ] 1.1 Research if `claude` supports a path override flag (e.g. `claude --worktree <id> --path ./.worktree`).
  - [ ] 1.2 Implement the most robust redirection strategy.

## Dev Notes

### Symlink Fallback
If redirection fails, we may need to use a `WorktreeCreate` hook in Claude's settings to move the folder after creation.

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-05-worktree.md#Story-5.3]
