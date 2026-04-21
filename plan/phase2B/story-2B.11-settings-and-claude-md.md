# Story 2B.11: Register Hooks + Update CLAUDE.md

Two deliverables: merge new hooks into `.claude/settings.json`, then append Agent Team documentation sections to `CLAUDE.md`.

## Files

- `.claude/settings.json` (merge, don't replace)
- `CLAUDE.md` (append sections)

## Part A: Settings.json Merge

Add PostToolUse and Stop hooks. Merge with the existing `TaskCompleted`, `TeammateIdle`, `TaskCreated` hooks from Phase 2.

### Before (existing Phase 2 state)

```json
{
  "hooks": {
    "TaskCompleted": [ { "hooks": [ { "type": "command", "command": "./scripts/hooks/validate-task-complete.sh" } ] } ],
    "TeammateIdle": [ { "hooks": [ { "type": "command", "command": "./scripts/hooks/check-teammate-idle.sh" } ] } ],
    "TaskCreated": [ { "hooks": [ { "type": "command", "command": "./scripts/hooks/review-loop-guard.sh" } ] } ]
  }
}
```

### After

```json
{
  "hooks": {
    "TaskCompleted": [ { "hooks": [ { "type": "command", "command": "./scripts/hooks/validate-task-complete.sh" } ] } ],
    "TeammateIdle": [ { "hooks": [ { "type": "command", "command": "./scripts/hooks/check-teammate-idle.sh" } ] } ],
    "TaskCreated": [ { "hooks": [ { "type": "command", "command": "./scripts/hooks/review-loop-guard.sh" } ] } ],
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [ { "type": "command", "command": "./scripts/hooks/sync-pipeline-status.sh" } ]
      }
    ],
    "Stop": [
      { "hooks": [ { "type": "command", "command": "./scripts/hooks/warn-in-progress.sh" } ] }
    ]
  }
}
```

### Optional: Custom Worktree Hook

If adopting `claude-code-custom-worktree.md`, also merge:

```json
"WorktreeCreate": [
  { "hooks": [ { "type": "command", "command": "bash \"$CLAUDE_PROJECT_DIR\"/.claude/hooks/worktree.sh", "timeout": 30 } ] }
],
"WorktreeRemove": [
  { "hooks": [ { "type": "command", "command": "bash \"$CLAUDE_PROJECT_DIR\"/.claude/hooks/worktree.sh", "timeout": 15 } ] }
]
```

## Part B: CLAUDE.md Append

Append these sections to the existing `CLAUDE.md` (below existing content — do not replace):

```markdown
## Agent Team

| Command | Description | Skills Used |
|---------|-------------|-------------|
| `/ship <id>` | Single-issue pipeline (code → review → PR → merge) | grava-claim, grava-dev-epic, grava-code-review |
| `/ship-all [team]` | Autopilot — drain backlog, create PRs, wait for merge | grava-next-issue + above |
| `/plan <doc>` | Generate issues from PRD/spec markdown | grava-gen-issues |
| `/hunt [scope]` | Audit codebase, file bugs as issues | grava-bug-hunt |

## Skill ↔ Agent Map

| Skill | Owned By | Purpose |
|-------|----------|---------|
| grava-cli (mental-model) | all agents | First-load context primer |
| grava-claim | coder | Atomic claim with prerequisite checks |
| grava-dev-epic | coder | Full TDD workflow + DoD |
| grava-code-review | reviewer | 5-axis review with severity classification |
| grava-next-issue | orchestrator (`/ship-all`) | Discover loop with stale detection |
| grava-bug-hunt | bug-hunter | Parallel codebase audit |
| grava-gen-issues | planner | Doc → issue hierarchy with deps |

## Pipeline Signals (agent ↔ orchestrator contract)

| Signal | Emitter | Meaning |
|--------|---------|---------|
| `CODER_DONE: <sha>` | coder | grava-dev-epic completed, code_review label set |
| `CODER_HALTED: <reason>` | coder | TDD or context loading hit blocker |
| `REVIEWER_APPROVED` | reviewer | grava-code-review verdict APPROVED |
| `REVIEWER_BLOCKED: <findings>` | reviewer | grava-code-review verdict CHANGES_REQUESTED |
| `PR_CREATED: <url>` | orchestrator | PR opened, polling for merge begins |
| `PR_COMMENTS_RESOLVED: <round>` | orchestrator | Coder fixed PR feedback, pushed to branch |
| `PR_MERGED` | orchestrator | PR merged upstream, closing issue now |
| `PIPELINE_COMPLETE: <id>` | orchestrator | PR merged + `grava close` done; team advances |
| `PIPELINE_HALTED: <reason>` | orchestrator | Human intervention needed |
| `PIPELINE_FAILED: <reason>` | orchestrator | PR creation failed or PR closed without merge |
| `PLANNER_DONE` | planner | grava-gen-issues created N issues |
| `BUG_HUNT_COMPLETE` | bug-hunter | grava-bug-hunt filed N bug issues |

## Context Passing (how agents receive state)

Claude Code agents do NOT inherit environment variables from the parent.
All context is passed via the Agent tool's `prompt` parameter.

| Context | How It's Passed | Example |
|---------|-----------------|---------|
| Issue ID | In `prompt` string | `"Implement issue grava-abc123..."` |
| Commit SHA | In `prompt` string (from prior agent result) | `"Last commit: a1b2c3d..."` |
| Review findings | Appended to `prompt` on re-spawn | `"Fix these findings:\n..."` |
| Team name | In `prompt` string | `"You are on team alpha. ..."` |
| Worktree | grava-provisioned at `.worktree/<id>` | Agent `cd .worktree/$ISSUE_ID` after claim |

Agents read shared state from the grava DB via CLI (`grava show`, `grava wisp read`).
This is the crash-recovery mechanism — wisps persist across sessions.
```

## Acceptance Criteria

### Settings

- Existing `TaskCompleted` / `TeammateIdle` / `TaskCreated` hooks unchanged
- New `PostToolUse` with matcher `Bash` → runs `sync-pipeline-status.sh`
- New `Stop` → runs `warn-in-progress.sh`
- `jq` still parses the file (no syntax errors)
- `./scripts/hooks/*.sh` paths are executable (`chmod +x` already applied by earlier stories)

### CLAUDE.md

- New "Agent Team" table lists all 4 slash commands
- "Skill ↔ Agent Map" lists all 7 skills with ownership
- "Pipeline Signals" table covers all 12 signals
- "Context Passing" table explains prompt-based vs wisp-based state
- Existing CLAUDE.md content (Grava Project sections) is preserved

## Dependencies

- Stories 2B.9 (sync-pipeline-status.sh) and 2B.10 (warn-in-progress.sh) — scripts must exist before hooks register them
- Stories 2B.1–2B.8 — signals and skills referenced in CLAUDE.md must be implemented

## Test Plan

- After merge: start a session, run any Bash command → hook fires silently
- Emit a line containing `CODER_DONE: abc` inside a worktree → wisp updates (verify via `grava wisp read`)
- Claim an issue, leave it in_progress, exit Claude Code → Stop hook prints warning with team tag
- Open CLAUDE.md → all new sections render correctly (tables not broken)
