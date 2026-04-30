---
name: grava-next-issue
description: 'Continuously pick up and implement ready grava issues until the backlog is empty. Use when the user says "next issue", "do next task", "keep going", "work through the backlog", "implement next issue", "what should I work on next", "grab the next one", "drain the backlog", or wants to start working without specifying a particular issue. Also trigger when the user says "grava next", just "next" in the context of working through issues, or "keep working". This is the autopilot for grinding through ready issues.'
---

# Grava Next Issue

Continuously find, claim, and implement ready issues until nothing is left. This is the autopilot — one command kicks off a loop that drains the backlog.

## The Loop

Repeat until no ready issues remain:

### 1. Discover

The coder hand-off (`grava-dev-task`) only implements **leaf-type** issues (`task`, `bug`). `grava ready` does not have a `--type` flag, so filter client-side — otherwise autopilot can land on a `story` / `epic` and HALT immediately, eating into the consecutive-HALT stop budget.

```bash
# Pull the priority-ordered ready queue, then keep only leaf types the dev skill can implement.
grava ready --limit 10 --json | jq '[.[] | select(.Node.Type == "task" or .Node.Type == "bug")]'
```

If results come back:
- Show the top 3 candidates briefly (ID, title, type, priority).
- Auto-select #1 (highest effective priority, oldest age as tiebreaker).
- If the user wants to pick a different one, let them.

If no ready issues, check for any open work across all types:

```bash
grava list --status open --json
```

If open issues exist but none are ready, they're likely blocked. Report what's blocked and why. If nothing is open at all, check for in-progress work:

```bash
grava list --status in_progress --json
```

**Stop the loop** and report a status summary:
- "Backlog drained" if nothing is open or in_progress
- List any in_progress issues so the user knows what's still being worked on
- Run `grava stats --json` and include key numbers (total, open, closed, in_progress)

### 2. Claim

Follow the `/grava-claim` skill to claim the selected issue:
- Read issue description, identify required services/dependencies
- Verify each prerequisite is reachable
- Run `grava claim <id>` atomically
- Write initial wisp: `grava wisp write <id> pipeline_phase claimed` — this is the canonical orchestrator-state key (matches the `sync-pipeline-status.sh` hook's phase order). Do NOT use `status` here; that key is unread by the hook.

**Before claiming**, check for stale state:
- If the issue has a `code_review` label or existing implementation comments, it may have been worked on previously. Run `grava show <id> --json` and check `comments` and `labels`. If it looks already-implemented, skip it and move to the next candidate.
- If `grava claim` fails due to a heartbeat lock from a dead/crashed agent, note the stale lock and skip to the next candidate. Do not attempt to force-clear locks.

If the claim fails (prerequisites not met, stale lock, already-implemented), try the next candidate from step 1. If all candidates fail, report blockers and **stop the loop** — don't spin on unclaimable work.

### 3. Implement

Follow the `/grava-dev-task` workflow. The autopilot's discover (loop Step 1) and claim (loop Step 2) already handled the dev skill's Step 1 (Resolve ID) and the claim portion of Step 3. The dev skill enters at **Step 3 via the resume-detection branch** — it sees `status=in_progress` + `assignee=current-actor`, reads the wisp, and skips re-claim. Step 2's spec-presence gate still runs against the now-claimed issue; if it HALTs after this loop's claim, run `grava stop <id>` to roll the claim back before continuing.

**Important:** Work in the issue's worktree directory if one was created by `grava claim`. Check with `grava show <id> --json` for worktree path info.

Note: `grava dep list <id>` may return an error instead of an empty array if no dependencies exist. Treat this as "no dependencies" and continue.

This handles the full cycle:
- Load context (issue details, dep tree, related issues, project context)
- Plan (subtask decomposition if needed)
- TDD implementation (red-green-refactor)
- Validation (test suite, code quality, acceptance criteria)
- Definition of Done checklist
- Commit, label `code_review`, summary

### 4. Next

After completing the issue, immediately loop back to step 1. Do not pause, do not wait for user input between issues.

Print a brief transition line:
```
--- Issue <id> complete. Checking for next ready issue... ---
```

## Stop Conditions

The loop ends when any of these occur:
- **Backlog empty** — `grava ready` returns nothing and no open issues remain (any type)
- **All candidates blocked** — every ready issue fails prerequisite checks, has stale locks, or is already implemented
- **HALT from grava-dev-task** — implementation hits an unrecoverable blocker (missing deps, ambiguous requirements, 3 consecutive failures, scope mismatch)
- **User interrupts** — user stops the process manually

When stopping, print a summary of the session:
```
--- Session Complete ---
Issues completed: <count>
  - <id>: <title>
  - <id>: <title>
Issues skipped: <count>
  - <id>: <reason>
In progress (other agents): <count>
  - <id>: <title>
Issues remaining in ready queue: <grava ready count or "0">
Stopped because: <reason>
```
