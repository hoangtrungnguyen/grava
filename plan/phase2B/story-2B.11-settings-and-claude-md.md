# Story 2B.11: Register Hooks + Update CLAUDE.md

Two deliverables: merge new hooks into `.claude/settings.json`, then append Agent Team documentation sections to `CLAUDE.md`. The Pipeline Signals table covers all current signals (including `PR_FAILED`, `PR_COMMENTS_RESOLVED`, `PIPELINE_HANDOFF`, `PLANNER_NEEDS_INPUT`) and stamps the protocol as v1 (lightweight stub; enforcement deferred until a v2 rename forces it). Out-of-Claude-Code setup — `pr-merge-watcher` cron (story 2B.12) and `install-hooks.sh` (story 2B.15) — is documented here too.

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
| `/ship <id>` | Single-issue pipeline (code → review → PR → handoff) | grava-dev-task, grava-code-review |
| `/ship <id> --force` | Same as above, but bypasses the Phase 0 precondition gate (use when the spec is fine but the AC heuristic mis-fires) | grava-dev-task, grava-code-review |
| `/ship <id> --retry` | Re-run a previously-rejected PR with rejection feedback as input | grava-dev-task, grava-code-review |
| `/ship <id> --retry --rebase-only` | Rebase stale-but-approved branch onto `main` and open a fresh PR (no review re-run) | grava-dev-task |
| `/plan <doc>` | Generate issues from PRD/spec markdown | grava-gen-issues |
| `/hunt [scope]` | Audit codebase, file bugs as issues | grava-bug-hunt |

> Backlog drain: rerun `/ship` (no id) — Phase 0 inside the skill discovers the next ready leaf-type issue (`task` / `bug`) and ships it. One terminal per issue. Previously `/ship-all` autopilot — archived.

> PR rejection recovery: when watcher detects a `CLOSED` (un-merged) PR, it appends a "PR Rejection Notes" section to the issue description, sets wisp `pr_close_reason`, and labels the issue `pr-rejected`. Operator runs `/ship <id> --retry` to re-enter the pipeline; capped at `MAX_PR_RETRIES=2`, then `needs-human`.

## Skill ↔ Agent Map

| Skill | Owned By | Purpose |
|-------|----------|---------|
| grava-cli (mental-model) | all agents | First-load context primer |
| grava-dev-task | coder | Spec-check → atomic claim → full TDD workflow + DoD |
| grava-code-review | reviewer | 5-axis review with severity classification |
| grava-bug-hunt | bug-hunter | Parallel codebase audit |
| grava-gen-issues | planner | Doc → issue hierarchy with deps |
| (no skill) | pr-creator | Push branch + `gh pr create` + template |
| (inline in `/ship` Phase 0) | orchestrator | Discover next ready leaf-type issue (`grava ready --json` filter) — replaces standalone `grava-next-issue` skill in the pipeline |

## Pipeline Signals (agent ↔ orchestrator contract)

> **Signal protocol version: v1.** Last-line-only parse — orchestrator and the `sync-pipeline-status` hook reject signals that don't appear as the final non-empty line of an agent result. Future breaking changes (renames, removed names) bump to v2 and add a `SIGNAL_PROTO: v2` preamble check.

| Signal | Emitter | Meaning |
|--------|---------|---------|
| `CODER_DONE: <sha>` | coder | grava-dev-task completed, code_review label set |
| `CODER_HALTED: <reason>` | coder | TDD or context loading hit blocker |
| `REVIEWER_APPROVED` | reviewer | grava-code-review verdict APPROVED |
| `REVIEWER_BLOCKED: <findings>` | reviewer | grava-code-review verdict CHANGES_REQUESTED |
| `PR_CREATED: <url>` | pr-creator agent | PR opened |
| `PR_FAILED: <reason>` | pr-creator agent | Push or `gh pr create` failed |
| `PR_COMMENTS_RESOLVED: <round>` | orchestrator | Coder fixed PR feedback, pushed to branch |
| `PR_MERGED` | pr-merge-watcher | PR merged on GitHub; watcher closed the grava issue |
| `PIPELINE_HANDOFF: <id> ...` | orchestrator | `/ship` exiting; pr-merge-watcher owns from here |
| `PIPELINE_COMPLETE: <id>` | watcher (via wisp) / orchestrator on re-entry | PR merged + `grava close` done |
| `PIPELINE_HALTED: <reason>` | orchestrator | Human intervention needed |
| `PIPELINE_FAILED: <reason>` | orchestrator | Signal parse failure or PR closed without merge |
| `PIPELINE_INFO: <reason>` | orchestrator | Re-entry no-op (e.g. still awaiting merge) |
| `PLANNER_DONE` | planner | grava-gen-issues created N issues |
| `PLANNER_NEEDS_INPUT: <summary>` | planner | Generation paused on missing info; operator clarifies, planner resumes |
| `BUG_HUNT_COMPLETE` | bug-hunter | grava-bug-hunt filed N bug issues |

## Wisp Keys (canonical state vocabulary)

Every wisp written by an agent, skill, or hook MUST use one of the keys below. Drift (e.g. writing `status` instead of `pipeline_phase`) produces dead-letter writes that `grava doctor` and `sync-pipeline-status.sh` cannot read.

| Key | Owner | Values | Read By |
|-----|-------|--------|---------|
| `pipeline_phase` | orchestrator (`/ship`) — seeds `claimed` at Phase 1 start, sets `pr_awaiting_merge` after PR; + `sync-pipeline-status.sh` hook (signal-driven advances) | `claimed` → `coding_complete` → `review_blocked` → `review_approved` → `pr_created` → `pr_awaiting_merge` → `pr_comments_resolved` → `pr_merged` → `complete`. Terminal: `halted_human_needed`, `coding_halted`, `planner_needs_input`. **Recoverable terminal:** `failed` — `/ship <id> --retry` resets back to `claimed` and re-enters Phase 1 (capped by `pr_retry_count`) | `/ship` re-entry, `pr-merge-watcher.sh`, `grava doctor` |
| `step` | `grava-dev-task` workflow checkpoints | `claimed`, `context-loaded`, `validated`, `complete` (skill-internal — opaque to orchestrator) | The skill itself on resume |
| `orchestrator_heartbeat` | `/ship` (seeds + writes per phase iteration) **AND** `grava-dev-task` skill (writes at every workflow checkpoint — see story 2B.0d) | UTC unix timestamp | `grava doctor` (stale-detection: >30 min while `pipeline_phase` non-terminal) |
| `pr_url`, `pr_number`, `pr_new_comments`, `pr_fix_round`, `pr_off_scope`, `pr_ci_log` | `pr-merge-watcher.sh`, `/ship` Phase 4 | URL / int / JSON / counter / paths / log | `/ship` re-entry |
| `pr_close_reason`, `pr_rejection_notes`, `pr_closed_at`, `pr_rejection_recorded` | `pr-merge-watcher.sh` CLOSED branch (one-shot per close) | category (`reviewer-rejected` / `author-abandoned` / `unknown`) / markdown / unix ts / `1` gate flag | `/ship --retry` Phase 5, human triage |
| `pr_retry_count` | `/ship --retry` Phase 5 | int 1..`MAX_PR_RETRIES` (=2); over-cap → `needs-human` label | `/ship --retry` re-entry guard |
| `coder_halted` | `coder` agent on HALT | reason string | Human triage |
| `current_task` | `grava-dev-task` workflow Step 4 | short description of in-flight unit | Skill resume |

**Rule of thumb:** `pipeline_phase` for orchestrator state (read by hooks), `step` for skill-internal checkpoint (read only by the skill itself on resume). When in doubt, use `pipeline_phase` — that's what crash recovery reads.

## Context Passing (how agents receive state)

Claude Code agents do NOT inherit environment variables from the parent.
All context is passed via the Agent tool's `prompt` parameter.

| Context | How It's Passed | Example |
|---------|-----------------|---------|
| Issue ID | In `prompt` string | `"Implement issue grava-abc123..."` |
| Commit SHA | In `prompt` string (from prior agent result) | `"Last commit: a1b2c3d..."` |
| Review findings | Appended to `prompt` on re-spawn | `"Fix these findings:\n..."` |
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

- New "Agent Team" table lists 6 invocations (`/ship`, `/ship <id> --force`, `/ship <id> --retry`, `/ship <id> --retry --rebase-only`, `/plan`, `/hunt`) — `/ship` (no id) handles backlog drain via inline Phase 0 discovery
- "Skill ↔ Agent Map" lists 5 pipeline-active skills (`grava-cli`, `grava-dev-task`, `grava-code-review`, `grava-bug-hunt`, `grava-gen-issues`) plus inline-discover row for `/ship` Phase 0. `grava-claim` folded into `grava-dev-task` Step 3; `grava-next-issue` retired from pipeline (kept for ad-hoc terminal use, not invoked by agents)
- "Pipeline Signals" table covers all 15 signals (including `PR_MERGED` from the watcher)
- "Wisp Keys" table documents canonical state-vocabulary (`pipeline_phase`, `step`, `orchestrator_heartbeat`, PR-tracking, PR-rejection (`pr_close_reason`, `pr_rejection_notes`, `pr_closed_at`, `pr_rejection_recorded`), `pr_retry_count`, …) — no `team` / `team_history` keys
- `pipeline_phase` row notes `failed` as **recoverable** via `/ship <id> --retry` (capped by `pr_retry_count`); other terminal states still require manual intervention
- "Context Passing" table explains prompt-based vs wisp-based state
- Existing CLAUDE.md content (Grava Project sections) is preserved

## Out-of-Claude-Code setup (documented here; not in settings.json)

Two scheduled jobs and one git-hooks installer must run on each developer's machine:

```bash
# One-time after fresh clone
./scripts/install-hooks.sh           # story 2B.15 — installs commit-msg

# cron (or launchd equivalent on macOS)
*/5 * * * * cd /path/to/grava && ./scripts/pr-merge-watcher.sh   # story 2B.12
0   * * * * cd /path/to/grava && ./scripts/run-pending-hunts.sh  # story 2B.15
0   2 * * * cd /path/to/grava && claude -p "/hunt since-last-tag"
```

Document this in CLAUDE.md alongside the Agent Team table — appended block:

```markdown
## Pipeline Setup (one-time per clone)

After cloning, run:
\`\`\`bash
./scripts/install-hooks.sh
\`\`\`

Then add the cron entries listed in `plan/phase2B/story-2B.11-...md` for the merge watcher and hunt scheduler.
```

## Dependencies

- Stories 2B.9 (sync-pipeline-status.sh) and 2B.10 (warn-in-progress.sh) — scripts must exist before hooks register them
- Story 2B.12 (pr-merge-watcher.sh) — referenced in setup docs
- Story 2B.15 (install-hooks.sh, run-pending-hunts.sh) — referenced in setup docs
- Stories 2B.1–2B.8, 2B.14 — signals and skills referenced in CLAUDE.md must be implemented

## Test Plan

- After merge: start a session, run any Bash command → hook fires silently
- Emit a line containing `CODER_DONE: abc` inside a worktree → wisp updates (verify via `grava wisp read`)
- Claim an issue, leave it in_progress, exit Claude Code → Stop hook prints warning
- Open CLAUDE.md → all new sections render correctly (tables not broken)
