# Agent Team Guide

End-to-end reference for the grava pipeline: from a backlog of issues to merged PRs, driven by Claude Code agents.

---

## Overview

The agent team turns grava issues into merged code automatically. You write the spec; the pipeline codes, reviews, opens the PR, and tracks the merge — each stage in its own Claude agent with a clean signal contract between them.

```
Backlog → /ship → [coder] → [reviewer] → [pr-creator] → PR → watcher → merged
```

Three entry-point skills:

| Command | What it does |
|---------|-------------|
| `/ship [id]` | Ship one issue end-to-end |
| `/plan <doc>` | Break a spec document into a grava issue hierarchy |
| `/hunt [scope]` | Audit the codebase for bugs, file them as grava issues |

---

## Prerequisites

```bash
# 1. grava binary
grava version          # must be ≥ 0.2.0

# 2. GitHub CLI authenticated with repo scope
gh auth status         # must show "repo" in token scopes

# 3. Dolt server running
grava doctor           # confirms DB is reachable

# 4. jq installed
jq --version
```

### One-time setup per clone

```bash
# Install the commit-msg git hook (enables bug-hunt: token in commits)
./scripts/install-hooks.sh

# Add cron entries — paste into `crontab -e`
grava bootstrap --print-cron
```

---

## /ship — Ship an Issue

### Auto-discover and ship the top ready issue

```bash
/ship
```

Phase 0 reads `grava ready`, filters to `task` and `bug` type issues, picks the highest-priority one, validates the spec, and proceeds. Prints the selected issue + 2 alternatives for awareness.

### Ship a specific issue

```bash
/ship grava-abc123
```

### Flags

| Flag | When to use |
|------|-------------|
| `--force` | Bypass the precondition gate (spec heuristic misfired on a valid issue) |
| `--retry` | Re-run after a PR was closed without merge |
| `--retry --rebase-only` | Branch went stale but review already passed — rebase + re-PR, skip review |

---

## Pipeline Phases

### Phase 0: Discover + Precondition Gate

Validates the issue before spawning any agent. Gate checks:

1. Description is non-empty
2. Acceptance criteria present (`## Acceptance Criteria`, `## AC`, or `- [ ]` checkboxes)
3. No `code_review` label already (work pending review)

**Gate fails:**
```
PIPELINE_HALTED: grava-abc123 failed precondition — no acceptance criteria
Operator must intervene. Options:
  • Fix the spec on grava-abc123, then re-run /ship grava-abc123
  • Pick a different issue: /ship <other-id>
  • Bypass the gate: /ship grava-abc123 --force
```

Minimum spec that passes the gate:

```markdown
## Description
Add rate limiting to the /api/export endpoint to prevent abuse.

## Acceptance Criteria
- [ ] Requests over 100/min per user are rejected with 429
- [ ] Retry-After header is set on rejection
- [ ] Unit tests cover the limit boundary
```

### Phase 1: Code (coder agent)

The `coder` agent:
1. Checks spec exists before claiming
2. Runs `grava claim` → auto-provisions `.worktree/grava-abc123/` on branch `grava/grava-abc123`
3. Implements via TDD (RED → GREEN → REFACTOR)
4. Runs scoped tests + linter
5. Commits, labels `code_review`, writes `CODER_DONE: <sha>`

### Phase 2: Review (reviewer agent, up to 3 rounds)

The `reviewer` agent runs `grava-code-review` — 5-axis review (correctness, bugs, security, error handling, tests). Posts `[CRITICAL]`/`[HIGH]`/`[MEDIUM]` comments on the issue.

- **APPROVED** → proceeds to Phase 3
- **CHANGES_REQUESTED** → coder respawned with findings, commits `[round N]` footer
- **3 rounds exhausted** → `PIPELINE_HALTED`, `needs-human` label

### Phase 3: PR Creation (pr-creator agent)

The `pr-creator` agent:
1. Verifies HEAD matches the approved SHA
2. Runs `scripts/pre-merge-check.sh` (merge conflict probe + `go build` against merged tree)
3. Pushes branch, runs `gh pr create`
4. Writes `pr_url`, `pr_number` wisps, labels `pr-created`

### Phase 4: Handoff

`/ship` exits. Ownership moves to `scripts/pr-merge-watcher.sh` (runs every 5 min via cron).

The watcher:
- **PR merged** → `grava close`, `pipeline_phase=complete`
- **PR closed without merge** → distils rejection into issue description, sets `pipeline_phase=failed`, labels `pr-rejected`
- **New PR comments** → writes `pr_new_comments` wisp, signals re-entry on next `/ship <id>`

---

## Re-entry Patterns

### PR has new review comments

The watcher detects new comments and sets a wisp. Re-run `/ship <id>`:

```bash
/ship grava-abc123
# → detects pr_new_comments wisp
# → spawns coder to fix comments [round N]
# → waits for CI, clears wisp, re-arms watcher
```

### PR was closed without merge

```bash
/ship grava-abc123
# PIPELINE_FAILED: PR closed without merge.
#   Reason: reviewer-rejected (see issue description for full notes)
# Recovery options:
#   /ship grava-abc123 --retry              — re-implement with rejection notes
#   /ship grava-abc123 --retry --rebase-only — rebase stale branch, skip review
#   grava close grava-abc123 --force        — abandon
```

Retry is capped at `MAX_PR_RETRIES=2`. Over-cap → `needs-human` label.

### Issue already in-progress (crash recovery)

```bash
/ship grava-abc123
# → reads pipeline_phase wisp to determine where to resume
# → coder reads step wisp for skill-internal checkpoint
```

---

## /plan — Generate Issues from a Document

```bash
/plan docs/feature-spec.md
/plan plan/phase2B/
/plan docs/epics/
```

The `planner` agent reads the document, asks you to fill in any gaps (unknown services, APIs, libraries), presents the proposed issue hierarchy for approval, then creates issues in dependency order.

**Output:**
```
PLANNER_DONE
Source: docs/feature-spec.md
Created: 14 issues (1 epic, 3 stories, 10 tasks, 0 subtasks)
Dependencies: 8 edges
Manifest: tracker/gen-feature-spec-2026-05-01.md
```

After planning, drain the backlog:
```bash
/ship    # ships the top ready task
/ship    # ships the next one
```

### When the operator defers a gap

If you answer "I don't know yet" or "skip" to a required question, the planner stops:
```
PLANNER_NEEDS_INPUT: docs/feature-spec.md missing 2 items (payment gateway API, auth provider)
```

Find stalled plans:
```bash
grava list --label planner-needs-input
```

---

## /hunt — Bug Audit

```bash
/hunt                       # since last git tag (default)
/hunt recent                # last 20 commits
/hunt all                   # full codebase
/hunt ./internal/auth       # targeted package
```

The `bug-hunter` agent runs parallel sub-agents per package group, classifies findings as CRITICAL/HIGH/MEDIUM, asks you to approve before filing, then creates grava issues.

**Output:**
```
BUG_HUNT_COMPLETE
Files reviewed: 48
Bugs found: 5 (critical=1 high=2 medium=2)
Issues created: [grava-f3a1, grava-f3a2, grava-f3a3, grava-f3a4, grava-f3a5]
```

Drain the bugs:
```bash
/ship    # picks the highest-priority ready bug
```

### Automatic hunt triggers

- **Commit token:** include `bug-hunt: <scope>` anywhere in a commit message body → enqueued for the next hourly cron run
- **Hourly cron:** `scripts/run-pending-hunts.sh` drains the queue
- **Nightly cron:** full `since-last-tag` scan at 02:00

---

## Monitoring

### Check pipeline state

```bash
# List all in-progress issues
grava list --status in_progress

# Read pipeline phase of a specific issue
grava wisp read grava-abc123 pipeline_phase

# Check all wisps on an issue
grava wisp read grava-abc123

# Full health check (stale heartbeats, orphan worktrees, schema version)
grava doctor
```

### Pipeline phases (in order)

```
claimed → coding_complete → review_blocked → review_approved
        → pr_created → pr_awaiting_merge → pr_comments_resolved
        → pr_merged → complete
```

Terminal (requires human): `halted_human_needed`, `coding_halted`, `failed`

### Issues needing attention

```bash
grava list --label needs-human      # halted, requires intervention
grava list --label pr-rejected      # closed PRs awaiting --retry
grava list --label code_review      # pending review (reviewer not yet spawned)
grava list --label pr-created       # PRs open, watcher tracking
grava list --label planner-needs-input   # stalled plans
```

---

## Wisp Reference

Key wisps written by the pipeline (read with `grava wisp read <id> <key>`):

| Key | Written by | Meaning |
|-----|-----------|---------|
| `pipeline_phase` | `/ship`, hook | Current pipeline stage |
| `orchestrator_heartbeat` | `/ship`, coder | Unix timestamp — `grava doctor` flags if >30min stale |
| `step` | coder skill | Internal TDD checkpoint (`claimed`, `context-loaded`, `validated`, `complete`) |
| `pr_url` | pr-creator | GitHub PR URL |
| `pr_number` | pr-creator | GitHub PR number |
| `pr_new_comments` | watcher | JSON of unseen PR comments (triggers re-entry) |
| `pr_close_reason` | watcher | `reviewer-rejected` / `author-abandoned` / `unknown` |
| `pr_rejection_notes` | watcher | Full markdown distillation of why PR was closed |
| `pr_retry_count` | `/ship --retry` | Retry count (cap: 2) |
| `coder_halted` | coder | Halt reason for human triage |

---

## Manual Operations

### Release a stuck claim

```bash
grava stop grava-abc123      # sets status back to open, clears assignee
```

### Abandon an issue

```bash
grava close grava-abc123 --force    # closes and removes worktree
```

### Delete a wisp (reset a phase manually)

```bash
grava wisp delete grava-abc123 pipeline_phase
```

### View full issue timeline

```bash
grava history grava-abc123
grava show grava-abc123 --json | jq '.comments'
```

---

## Parallel Terminals

You can run multiple issues in parallel — one terminal per issue:

```bash
# Terminal 1
/ship grava-abc123

# Terminal 2
/ship grava-def456
```

Signals in different terminals update different wisps because each `grava signal` call passes `--issue $ISSUE_ID` (or auto-detects it from the `.worktree/<id>/` cwd). The Stop hook warns on exit if any issues are still in-progress.

---

## Cron Setup Reference

```cron
# PR merge watcher — every 5 min
*/5 * * * * cd /path/to/grava && ./scripts/pr-merge-watcher.sh >> .grava/watcher.log 2>&1

# Hunt queue drain — hourly
0 * * * * cd /path/to/grava && ./scripts/run-pending-hunts.sh

# Nightly bug scan
0 2 * * * cd /path/to/grava && claude -p "/hunt since-last-tag" >> .grava/hunt.log 2>&1
```

Generate with the correct paths for your clone:
```bash
grava bootstrap --print-cron
```

---

## Quick Reference Card

```
BACKLOG MANAGEMENT
  /plan <doc>               Generate issues from a spec
  /hunt [scope]             File bugs from codebase audit
  grava ready               Show what's ready to ship

SHIPPING
  /ship                     Auto-pick + ship next ready task/bug
  /ship <id>                Ship a specific issue
  /ship <id> --force        Skip precondition gate
  /ship <id> --retry        Re-run after PR rejection
  /ship <id> --retry --rebase-only   Rebase stale approved branch

MONITORING
  grava doctor              Health check
  grava list --status in_progress    Active issues
  grava list --label needs-human     Blocked, needs you
  grava wisp read <id> pipeline_phase

RECOVERY
  grava stop <id>           Release stuck claim
  grava close <id> --force  Abandon issue
  grava wisp delete <id> <key>       Reset a wisp
```
