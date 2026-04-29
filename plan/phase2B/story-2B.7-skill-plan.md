# Story 2B.7: /plan Skill — Generate Issues

Invokes the planner agent to turn a markdown document (PRD, spec, design doc) into a grava issue hierarchy via `grava-gen-issues`.

## File

`.claude/skills/plan/SKILL.md`

## Frontmatter

```yaml
---
name: plan
description: "Generate grava issues from a markdown document or folder."
user-invocable: true
---
```

## Usage

```
/plan <path-to-doc-or-folder>
```

Examples:
- `/plan docs/feature-spec.md`
- `/plan docs/epics/`
- `/plan plan/phase2B/`

## Setup

```bash
DOC_PATH="$ARGUMENTS"
[ -e "$DOC_PATH" ] || { echo "PLAN_FAILED: $DOC_PATH not found"; exit 1; }
```

## Workflow

Spawn planner agent via the **Agent tool**:

```
Agent({
  description: "Generate issues from $DOC_PATH",
  subagent_type: "planner",
  prompt: "Generate grava issues from document at: $DOC_PATH.
           Follow your workflow. Output PLANNER_DONE with stats."
})
```

Wait for `PLANNER_DONE` in the returned result.

## Output

On success:
```
PLAN_COMPLETE: Run /ship (no id) to discover and ship the next ready leaf-type issue from the new backlog
```

On failure:
```
PLAN_FAILED: <reason>
```

## Acceptance Criteria

- `/plan <path>` validates path exists before spawning planner
- Planner agent is spawned with `subagent_type: "planner"`
- Skill waits for `PLANNER_DONE` signal from planner agent
- On planner success, suggests next step: rerun `/ship` (no id) to drain the new backlog one issue at a time
- On missing path, exits with `PLAN_FAILED: ...not found`
- New issues visible in `grava ready` after completion

## Dependencies

- Story 2B.4 (planner agent)
- `.claude/skills/grava-gen-issues/` (exists)

## Signals Emitted

- `PLAN_COMPLETE` — planner returned PLANNER_DONE
- `PLAN_FAILED: <reason>` — validation or planner failure
