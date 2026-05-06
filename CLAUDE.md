# Claude Configuration for Grava Project

## Agent Workflows

This project uses custom agent workflows defined in the `.agent/workflows/` directory. Please reference these workflows when working on tasks.

### Available Workflows

- **are-u-ready**: Located at [`.agent/workflows/are-u-ready.md`](.agent/workflows/are-u-ready.md)
  - Protocol for validating readiness before starting tickets
  - Checks context, dependencies, and environment connections
  - Reference the `epic/` and `tracker/` directories for task context

## Project Structure

- `.agent/workflows/`: Custom agent workflows and protocols
- `tracker/`: Historical task tracking and completed work
- `epic/` or `docs/epics/`: Epic definitions and roadmaps
- `docs/`: Project documentation

## Working with Tasks

When working on tickets or tasks:
1. Check the `tracker/` directory for historical context
2. Reference `docs/epics/` for epic-level requirements
3. Follow workflows defined in `.agent/workflows/`
4. Use the Dolt database located at `.grava/dolt/`

## Database

This project uses Dolt as its database substrate. The database directory is `.grava/dolt/`. Use the following connection:
- Command: `dolt --data-dir .grava/dolt sql`
- Connection string: `root@tcp(127.0.0.1:3306)/grava?parseTime=true`

## Agent Team

| Command | Description | Skills Used |
|---------|-------------|-------------|
| `/ship <id>` | Single-issue pipeline (code → review → PR → handoff) | grava-dev-task, grava-code-review |
| `/ship <id> --force` | Same as above, but bypasses the Phase 0 precondition gate (use when the spec is fine but the AC heuristic mis-fires) | grava-dev-task, grava-code-review |
| `/ship <id> --retry` | Re-run a previously-rejected PR with rejection feedback as input | grava-dev-task, grava-code-review |
| `/ship <id> --retry --rebase-only` | Rebase stale-but-approved branch onto `main` and open a fresh PR (no review re-run) | grava-dev-task |
| `/plan <doc>` | Generate issues from PRD/spec markdown | grava-gen-issues |
| `/hunt [scope]` | Audit codebase, file bugs as issues | grava-bug-hunt |

> Backlog drain: rerun `/ship` (no id) — Phase 0 inside the skill discovers the next ready leaf-type issue (`task` / `bug`) and ships it. One terminal per issue.

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
| (inline in `/ship` Phase 0) | orchestrator | Discover next ready leaf-type issue (`grava ready --json` filter) |

## Pipeline Signals (agent ↔ orchestrator contract)

> **Signal protocol version: v2.** Agents call `grava signal <KIND> --issue $ID [--payload $V]` which writes `pipeline_phase` and any auxiliary triage wisps atomically inside one transaction. The orchestrator (`/ship`) reads canonical state via `grava wisp read` / `grava show --json`. The CLI also prints `<KIND>: <payload>` as its final stdout line so the orchestrator's stdout-fallback parser still resolves the kind in case of a wisp-write failure.

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
| `PLANNER_NEEDS_INPUT: <summary>` | planner | Generation paused on missing info |
| `BUG_HUNT_COMPLETE` | bug-hunter | grava-bug-hunt filed N bug issues |

## Wisp Keys (canonical state vocabulary)

| Key | Owner | Values | Read By |
|-----|-------|--------|---------|
| `pipeline_phase` | `grava signal` CLI (sole writer — orchestrator, agents, watcher all route through it) | `claimed` → `coding_complete` → `review_blocked` → `review_approved` → `pr_created` → `pr_awaiting_merge` → `pr_comments_resolved` → `pr_merged` → `complete`. Terminal: `halted_human_needed`, `coding_halted`, `planner_needs_input`. Recoverable: `failed` | `/ship` re-entry, `pr-merge-watcher.sh`, `grava doctor` |
| `step` | `grava-dev-task` workflow checkpoints | `claimed`, `context-loaded`, `validated`, `complete` (skill-internal) | The skill itself on resume |
| `orchestrator_heartbeat` | `/ship` (seeds + phase iterations) AND `grava-dev-task` (every workflow checkpoint) | UTC unix timestamp | `grava doctor` (stale-detection: >30 min while `pipeline_phase` non-terminal) |
| `pr_url`, `pr_number`, `pr_new_comments`, `pr_fix_round`, `pr_off_scope`, `pr_ci_log` | `pr-merge-watcher.sh`, `/ship` Phase 4 | URL / int / JSON / counter / paths / log | `/ship` re-entry |
| `pr_close_reason`, `pr_rejection_notes`, `pr_closed_at`, `pr_rejection_recorded` | `pr-merge-watcher.sh` CLOSED branch | category / markdown / unix ts / gate flag | `/ship --retry` Phase 5, human triage |
| `pr_retry_count` | `/ship --retry` Phase 5 | int 1..`MAX_PR_RETRIES` (=2) | `/ship --retry` re-entry guard |
| `coder_halted` | `coder` agent on HALT | reason string | Human triage |
| `current_task` | `grava-dev-task` workflow Step 4 | short description of in-flight unit | Skill resume |

## Context Passing (how agents receive state)

Claude Code agents do NOT inherit environment variables from the parent.
All context is passed via the Agent tool's `prompt` parameter.

| Context | How It's Passed | Example |
|---------|-----------------|---------|
| Issue ID | In `prompt` string | `"Implement issue grava-abc123..."` |
| Commit SHA | In `prompt` string (from prior agent result) | `"Last commit: a1b2c3d..."` |
| Review findings | Appended to `prompt` on re-spawn | `"Fix these findings:\n..."` |
| Worktree | grava-provisioned at `.worktree/<id>` | Agent `cd .worktree/$ISSUE_ID` after claim |

## Pipeline Setup (one-time per clone)

After cloning, run:
```bash
./scripts/install-hooks.sh
```

Then add these cron entries (or launchd equivalents on macOS):

```cron
# PR merge watcher — every 5 min
*/5 * * * * cd /path/to/grava && ./scripts/pr-merge-watcher.sh >> .grava/watcher.log 2>&1

# Hunt scheduler — hourly drain
0 * * * * cd /path/to/grava && ./scripts/run-pending-hunts.sh

# Nightly bug scan
0 2 * * * cd /path/to/grava && claude -p "/hunt since-last-tag" >> .grava/hunt.log 2>&1
```
