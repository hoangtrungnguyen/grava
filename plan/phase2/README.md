# Phase 2: Quality Gate Hooks

Hook scripts that enforce rules at agent team task boundaries.

## Stories

| Story | File | Description |
|-------|------|-------------|
| [2.1](story-2.1-task-completed-hook.md) | `scripts/hooks/validate-task-complete.sh` | Block task completion if tests fail |
| [2.2](story-2.2-teammate-idle-hook.md) | `scripts/hooks/check-teammate-idle.sh` | Redirect idle teammates to pending work |
| [2.3](story-2.3-review-loop-guard.md) | `scripts/hooks/review-loop-guard.sh` | Max 3 review rounds per issue |
| [2.4](story-2.4-register-hooks.md) | `.claude/settings.json` | Register all hooks in settings |

## Dependency

Phase 3 (`grava search --label`) must be implemented first — the idle hook uses it.
