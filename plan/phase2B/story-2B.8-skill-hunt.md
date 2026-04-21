# Story 2B.8: /hunt Skill — Bug Audit

Invokes the bug-hunter agent to audit the codebase and file bugs as grava issues via `grava-bug-hunt`.

## File

`.claude/skills/hunt/SKILL.md`

## Frontmatter

```yaml
---
name: hunt
description: "Audit the codebase for bugs and file them as grava issues."
user-invocable: true
---
```

## Usage

```
/hunt [scope]
```

Scope options:
- `(none)` → since last tag (default)
- `recent` → last 20 commits
- `all` → full codebase
- `<package-path>` → targeted package (e.g. `./internal/auth`)

Examples:
- `/hunt`
- `/hunt recent`
- `/hunt ./internal/store`

## Setup

```bash
SCOPE="${ARGUMENTS:-since-last-tag}"
```

## Workflow

Spawn bug-hunter agent via the **Agent tool**:

```
Agent({
  description: "Bug hunt: $SCOPE",
  subagent_type: "bug-hunter",
  prompt: "Audit the codebase for bugs with scope: $SCOPE.
           Follow your workflow. Output BUG_HUNT_COMPLETE with stats."
})
```

Wait for `BUG_HUNT_COMPLETE` in the returned result.

## Output

```
HUNT_COMPLETE: Run /ship-all to fix the new bugs in priority order
```

## Acceptance Criteria

- `/hunt` (no args) defaults to "since-last-tag" scope
- Bug-hunter agent spawned with `subagent_type: "bug-hunter"` and the scope in prompt
- Skill waits for `BUG_HUNT_COMPLETE` signal
- New bug issues visible in `grava ready` after completion
- Suggests `/ship-all` as next step

## Dependencies

- Story 2B.3 (bug-hunter agent)
- `.claude/skills/grava-bug-hunt/` (exists)

## Signals Emitted

- `HUNT_COMPLETE` — bug-hunter returned BUG_HUNT_COMPLETE
