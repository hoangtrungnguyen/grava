# Grava Agent Team — Full Implementation Plan (v2: Skills-Integrated)

> **Goal:** A production-ready multi-agent team for the Grava workflow, where each agent **delegates to existing skills** instead of inlining logic. Skills are the reusable instruction library; agents are the orchestration layer.

---

## 1. Overview

### Design Principle

> **Agents orchestrate, skills execute.**

Every skill in `.claude/skills/` already encodes domain expertise (TDD workflow, code review checklist, severity classification, etc.). Agents are thin wrappers that:
1. Receive context via their `prompt` parameter (no env vars — Claude Code doesn't pass them)
2. Read and follow the right skill at the right phase
3. Translate skill outputs into pipeline signals (`CODER_DONE`, `REVIEWER_APPROVED`, etc.)

> **API Note:** Agents are spawned via the **Agent tool** with `subagent_type` matching
> the agent's `name` field. Context flows through the `prompt` string and the grava DB
> (wisps for crash recovery).
>
> **Worktree ownership:** `grava claim <id>` provisions `.worktree/<id>` with branch
> `grava/<id>`. We do **NOT** use Claude Code's `isolation: "worktree"` param — grava
> owns worktrees. Agents `cd .worktree/<id>` after claim and work there.
> This gives persistent, predictable worktrees that survive across agent invocations
> (required for review-round-2 re-spawn and PR comment fix loops).
>
> **Relation to `claude-code-custom-worktree.md`:** That guide adds `WorktreeCreate` /
> `WorktreeRemove` hooks so `claude -w <name>` creates worktrees under `.worktree/`.
> The pipeline deliberately bypasses this — agents call `grava claim` directly for
> full lifecycle control (status, wisps, atomic claim). The custom worktree hook
> remains useful for ad-hoc exploration outside the pipeline (see Section 9).

This means a 20-line agent definition instead of 80, and one source of truth per concern.

### Team Topology

```
┌──────────────────────────────────────────────────────────────┐
│                       ORCHESTRATOR                           │
│              /ship <id>  or  /ship-all (autopilot)          │
└──┬──────────┬──────────┬──────────┬──────────┬──────────────┘
   │          │          │          │          │
   ▼          ▼          ▼          ▼          ▼
[planner] [bug-hunter] [coder] [reviewer]
   │          │          │          │
   │          │          │          │
   ▼          ▼          ▼          ▼
gen-issues bug-hunt   claim+      code-       → PR
                     dev-epic    review      (gh pr create)

           ┌────────────────────────┐
           │   Grava DB (shared)    │
           │  via .grava/redirect   │
           └────────────────────────┘
```

### Skill ↔ Agent Mapping

| Agent | Primary Skill(s) | Triggered By |
|-------|------------------|--------------|
| `orchestrator` | `grava-next-issue` (loop) | `/ship`, `/ship-all` |
| `planner` | `grava-gen-issues` | User uploads PRD/spec |
| `coder` | `grava-claim` → `grava-dev-epic` | Orchestrator Phase 1 |
| `reviewer` | `grava-code-review` | Orchestrator Phase 2 |
| (orchestrator) | `gh pr create` | Phase 3 — PR creation |
| `bug-hunter` | `grava-bug-hunt` | Periodic / manual |

All agents preload `grava-cli` via the `skills: [grava-cli]` frontmatter field.
This injects the CLI mental model into each agent's context automatically.

---

## 2. File Structure

```
.claude/
├── agents/
│   ├── coder.md            ← invokes grava-claim + grava-dev-epic
│   ├── reviewer.md         ← invokes grava-code-review
│   ├── bug-hunter.md       ← invokes grava-bug-hunt
│   └── planner.md          ← invokes grava-gen-issues
├── skills/
│   ├── ship/SKILL.md       ← single-issue pipeline orchestrator
│   ├── ship-all/SKILL.md   ← autopilot via grava-next-issue
│   ├── plan/SKILL.md       ← invoke planner agent
│   ├── hunt/SKILL.md       ← invoke bug-hunter agent
│   └── (existing skills)   ← DO NOT MODIFY
├── hooks/
│   └── worktree.sh         ← OPTIONAL: from claude-code-custom-worktree.md
│                             (ad-hoc `claude -w <name>` redirect; see Section 9)
├── settings.json           ← add PostToolUse + Stop hooks
│                             (and optional WorktreeCreate/Remove if hook installed)
└── ...

scripts/hooks/              ← matches existing hook location
├── sync-pipeline-status.sh ← NEW: PostToolUse signal → wisp sync
├── warn-in-progress.sh     ← NEW: Stop hook for orphan warning
├── validate-task-complete.sh  ← existing
├── check-teammate-idle.sh     ← existing
└── review-loop-guard.sh       ← existing

.worktree/                  ← in .gitignore
├── grava-<id>/             ← grava claim (pipeline)
└── <scratch-name>/         ← optional: claude -w <scratch-name>

CLAUDE.md                   ← add Agent Team + Skill Map sections
```

---

## 3. Agent Definitions

All agents are kept minimal — they delegate to skills.

### 3.1 Coder Agent

**`.claude/agents/coder.md`**

```markdown
---
name: coder
description: >
  Claims a grava issue and implements it end-to-end via TDD.
  Delegates all work to grava-claim and grava-dev-epic skills.
model: sonnet
tools: Read, Write, Edit, Bash, Glob, Grep
skills: [grava-cli]
maxTurns: 100
---

You are the coder agent in the Grava pipeline.

## Input

You receive `ISSUE_ID` in your initial prompt from the orchestrator.
Optionally you receive `RESUME: true` if the issue is already claimed (re-spawn for
review fixes or PR comment fixes) — in that case skip Phase A.

The `skills: [grava-cli]` frontmatter pre-loads the CLI mental model automatically.

## Worktree Convention

After claim, `.worktree/$ISSUE_ID/` exists on branch `grava/$ISSUE_ID`.
**All file edits, tests, and commits happen inside `.worktree/$ISSUE_ID/`.**
`cd .worktree/$ISSUE_ID` before any `go test`, `git commit`, or `git push`.

## Workflow

### Phase A: Claim (skip if RESUME)
Invoke the **`grava-claim`** skill with $ISSUE_ID.
Read: `.claude/skills/grava-claim/SKILL.md`

The skill handles: prerequisite checks, atomic claim, worktree provision, wisp heartbeat.
If the skill reports the claim failed (stale lock, prerequisites unmet, already-implemented):
- Output: `CODER_HALTED: <reason from skill>`
- Stop. Do NOT proceed to implementation.

If RESUME, verify worktree exists: `[ -d ".worktree/$ISSUE_ID" ]` — else HALT.

### Phase B: Implement
`cd .worktree/$ISSUE_ID`

Invoke the **`grava-dev-epic`** skill, starting from **Step 2 (Load Context)**.
Read: `.claude/skills/grava-dev-epic/SKILL.md` and `.claude/skills/grava-dev-epic/workflow.md`

The skill handles: context loading, planning, TDD red-green-refactor, validation,
Definition of Done checklist, commit on branch `grava/$ISSUE_ID`, label `code_review`, summary.

### Phase C: Signal
Once `grava-dev-epic` completes Step 7 (commit + label code_review):
- Read the recorded commit hash: `grava show $ISSUE_ID --json | jq -r '.last_commit'`
- Output: `CODER_DONE: <commit-sha>`

## HALT Conditions

If `grava-dev-epic` triggers any HALT condition (missing deps, ambiguous requirements,
3 consecutive failures, regressions you can't fix):
- Write wisp: `grava wisp write $ISSUE_ID coder_halted "<specific reason>"`
- Run: `grava stop $ISSUE_ID`
- Output: `CODER_HALTED: <reason>`
- Stop.

## Anti-Patterns

- Do NOT re-implement TDD logic — `grava-dev-epic` owns it
- Do NOT skip the wisp checkpoints from `grava-dev-epic` — they enable crash recovery
- Do NOT close the issue yourself — leave it `in_progress` with `code_review` label
- Your FINAL message MUST contain exactly one signal: `CODER_DONE: <sha>` or `CODER_HALTED: <reason>`
```

---

### 3.2 Reviewer Agent

**`.claude/agents/reviewer.md`**

```markdown
---
name: reviewer
description: >
  Reviews a grava issue's last_commit. Delegates to grava-code-review skill.
  Translates skill verdict into pipeline signal.
model: sonnet
tools: Read, Bash, Glob, Grep
skills: [grava-cli]
maxTurns: 30
---

You are the reviewer agent in the Grava pipeline. You review, you do not implement.

## Input

You receive `ISSUE_ID` in your initial prompt from the orchestrator.
The `skills: [grava-cli]` frontmatter pre-loads the CLI mental model automatically.

## Pre-flight Check

Verify the issue has a `last_commit` recorded:

```bash
LAST_COMMIT=$(grava show $ISSUE_ID --json | jq -r '.last_commit // empty')
if [ -z "$LAST_COMMIT" ]; then
  echo "REVIEWER_BLOCKED: no last_commit recorded on $ISSUE_ID"
  exit 1
fi
```

## Workflow

Invoke the **`grava-code-review`** skill with $ISSUE_ID.
Read: `.claude/skills/grava-code-review/SKILL.md`

The skill handles:
- Fetching the commit and changed files
- 5-axis review (correctness, bugs, security, error handling, tests, style)
- Severity classification (CRITICAL/HIGH/MEDIUM/LOW)
- Posting one comment per non-empty severity
- Posting `[REVIEW]` summary with verdict
- Applying `reviewed` or `changes_requested` label
- Committing grava state

## Signal Translation

Read the verdict from the `[REVIEW]` summary comment the skill just posted:

```bash
VERDICT=$(grava show $ISSUE_ID --json | \
  jq -r '.comments | map(select(.message | startswith("[REVIEW]"))) | last | .message' | \
  grep -oE 'Verdict: (APPROVED|CHANGES_REQUESTED)' | awk '{print $2}')
```

Then output one of:
- `REVIEWER_APPROVED` — if verdict is APPROVED
- `REVIEWER_BLOCKED` — if verdict is CHANGES_REQUESTED

When BLOCKED, also extract and emit the CRITICAL/HIGH findings so the coder agent
can act on them in the next round:

```bash
grava show $ISSUE_ID --json | \
  jq -r '.comments | map(select(.message | startswith("[CRITICAL]") or startswith("[HIGH]"))) | .[].message'
```

## Anti-Patterns

- Do NOT re-implement the severity classification — `grava-code-review` owns it
- Do NOT post review comments directly — the skill posts them in the correct format
- Do NOT approve when CRITICAL or HIGH findings exist (the skill enforces this)
- Your FINAL message MUST contain exactly one signal: `REVIEWER_APPROVED` or `REVIEWER_BLOCKED: <findings>`
```

---

### 3.3 PR Creation (Orchestrator Step — No Separate Agent)

After `REVIEWER_APPROVED`, the orchestrator creates a pull request directly.
No agent needed — it's a simple `gh pr create` call.

```bash
# Push the feature branch
git push -u origin $FEATURE_BRANCH

# Create PR with grava context
gh pr create \
  --title "$ISSUE_ID: $ISSUE_TITLE" \
  --body "$(cat <<EOF
## Grava Issue: $ISSUE_ID

$ISSUE_DESCRIPTION

## Review
- Reviewed by: reviewer agent
- Verdict: APPROVED
- Commit: $APPROVED_SHA

## Test Plan
- [x] Unit tests pass in worktree
- [x] Code review approved
- [ ] Merge to main + full test suite

---
*Created by agent team pipeline*
EOF
)"
```

The orchestrator then records the PR:

```bash
PR_URL=$(gh pr view --json url -q '.url')
grava comment $ISSUE_ID -m "PR created: $PR_URL"
grava label $ISSUE_ID --add pr-created
grava wisp write $ISSUE_ID pr_url "$PR_URL"
grava commit -m "pr created for $ISSUE_ID"
```

**Signal:** `PR_CREATED: <pr-url>`

**Issue is NOT closed here.** It closes when the PR is merged (see Section 8).

---

### 3.4 Bug Hunter Agent

**`.claude/agents/bug-hunter.md`**

```markdown
---
name: bug-hunter
description: >
  Periodic codebase audit. Finds bugs across packages and files them as grava issues.
  Delegates to grava-bug-hunt skill which runs parallel review sub-agents.
model: sonnet
tools: Read, Bash, Glob, Grep, Agent
skills: [grava-cli]
maxTurns: 50
---

You are the bug-hunter agent. You find bugs and file them — you do NOT fix them.

## Input

You receive a `SCOPE` in your initial prompt: "since-last-tag" (default), "recent", "all", or a package path.
The `skills: [grava-cli]` frontmatter pre-loads the CLI mental model automatically.

## Workflow

Invoke the **`grava-bug-hunt`** skill.
Read: `.claude/skills/grava-bug-hunt/SKILL.md`

The skill handles:
- Scope determination (default: changes since last tag)
- Parallel review sub-agents (one per package group)
- Severity classification (CRITICAL/HIGH/MEDIUM)
- User confirmation before issue creation
- `grava create` for each approved finding
- `grava commit` to snapshot the new issues

## Output

After the skill completes:
```
BUG_HUNT_COMPLETE
Files reviewed: <N>
Bugs found: <N> (critical=<X> high=<Y> medium=<Z>)
Issues created: <list of grava-XXXX IDs>
```

## Pipeline Integration

The bugs you file land in `grava ready` and get picked up by `/ship-all`.
You do NOT implement fixes yourself — that's the coder agent's job.

## When to Run

- Weekly cron / scheduled task
- After every major merge to main
- On user request: `/hunt`
- Before a release tag
```

---

### 3.5 Planner Agent

**`.claude/agents/planner.md`**

```markdown
---
name: planner
description: >
  Turns markdown specs/PRDs/design docs into a full grava issue hierarchy.
  Delegates to grava-gen-issues skill.
model: sonnet
tools: Read, Bash, Glob, Grep, Write
skills: [grava-cli]
maxTurns: 50
---

You are the planner agent. You create work items — you do NOT implement them.

## Input

You receive a `DOC_PATH` in your initial prompt: path to a markdown file or folder.
The `skills: [grava-cli]` frontmatter pre-loads the CLI mental model automatically.

## Setup

```bash
grava doctor    # confirm DB is up
```

## Workflow

Use the `DOC_PATH` from your prompt.

Invoke the **`grava-gen-issues`** skill.
Read: `.claude/skills/grava-gen-issues/SKILL.md`

The skill handles:
- Document ingestion + completeness validation
- Asking the user to fill gaps (services, APIs, libraries)
- Building the epic → story → task hierarchy
- Priority assignment based on critical-path heuristics
- Dependency edge creation
- Plan presentation + user approval gate
- Sequential creation in dependency order
- Manifest file generation

## Output

After the skill completes:
```
PLANNER_DONE
Source: <document-path>
Created: <N> issues (<E> epics, <S> stories, <T> tasks, <U> subtasks)
Dependencies: <D> edges
Needs clarification: <C> items
Manifest: tracker/gen-<doc-name>-<YYYY-MM-DD>.md
```

## Pipeline Integration

After planning completes, the user can invoke `/ship-all` to drain the new backlog.
You do NOT implement anything yourself.
```

---

## 4. Orchestrator Skills

### 4.1 Single-Issue Pipeline

**`.claude/skills/ship/SKILL.md`**

```markdown
---
name: ship
description: "Single-issue pipeline: code → review → PR. Use when user says /ship <id>."
user-invocable: true
---

# Ship Pipeline

Usage: /ship <issue-id>

## Setup

```bash
ISSUE_ID="$ARGUMENTS"    # from /ship invocation
grava show $ISSUE_ID --json  # verify issue exists and is open/in_progress
```

## Phase 1: Code

Spawn coder agent via the **Agent tool**:

```
Agent({
  description: "Implement $ISSUE_ID",
  subagent_type: "coder",
  prompt: "Claim and implement issue $ISSUE_ID. Follow phases A→C.
           Work inside .worktree/$ISSUE_ID on branch grava/$ISSUE_ID.
           Output CODER_DONE: <sha> or CODER_HALTED: <reason>."
})
```

**No `isolation` param** — grava-claim provisions `.worktree/$ISSUE_ID` with branch
`grava/$ISSUE_ID` and the coder cd's there. Context (ISSUE_ID) is passed in the
`prompt` string — no env vars needed.

Parse the returned result:
- Contains `CODER_DONE: <sha>` → extract SHA, proceed to Phase 2
- Contains `CODER_HALTED` → output `PIPELINE_HALTED: coder — <reason>` and stop

## Phase 2: Review (max 3 rounds)

For round in 1..3:

Spawn reviewer agent via the **Agent tool**:

```
Agent({
  description: "Review $ISSUE_ID round N",
  subagent_type: "reviewer",
  prompt: "Review issue $ISSUE_ID. The last commit is <sha from coder>.
           Output REVIEWER_APPROVED or REVIEWER_BLOCKED: <findings>."
})
```

If `REVIEWER_APPROVED` → proceed to Phase 3.

If `REVIEWER_BLOCKED`:
- Capture the CRITICAL/HIGH findings from reviewer output
- Write wisp: `grava wisp write $ISSUE_ID review_round_$N "blocked"`
- Re-spawn coder agent with findings appended to the prompt:
  ```
  Agent({
    description: "Fix review findings for $ISSUE_ID",
    subagent_type: "coder",
    prompt: "RESUME: true. Issue $ISSUE_ID was BLOCKED. Fix these findings:\n<findings>\n
             Worktree at .worktree/$ISSUE_ID already exists — cd there and continue
             on branch grava/$ISSUE_ID. Skip grava-claim (Phase A), go to grava-dev-epic (Phase B).
             Output CODER_DONE: <sha> or CODER_HALTED: <reason>."
  })
  ```
- Loop back to reviewer if CODER_DONE

If 3 rounds exhausted:
```bash
grava wisp write $ISSUE_ID pipeline_halted "review loop exhausted (3 rounds)"
grava label $ISSUE_ID --add needs-human
grava stop $ISSUE_ID
```
Output: `PIPELINE_HALTED: $ISSUE_ID needs human review` and stop.

## Phase 3: Create PR

After `REVIEWER_APPROVED`, create a pull request from the feature branch.
The branch is always `grava/$ISSUE_ID` (grava's convention, created by `grava claim`).

```bash
FEATURE_BRANCH="grava/$ISSUE_ID"   # grava claim's convention

# Push must happen from the worktree (where commits live)
cd .worktree/$ISSUE_ID
git push -u origin "$FEATURE_BRANCH"
cd - > /dev/null

ISSUE_TITLE=$(grava show $ISSUE_ID --json | jq -r '.title')
TITLE_PREFIX="${TEAM_NAME:+[$TEAM_NAME] }"   # empty if solo

gh pr create \
  --head "$FEATURE_BRANCH" \
  --title "${TITLE_PREFIX}${ISSUE_ID}: ${ISSUE_TITLE}" \
  --body "Grava issue: $ISSUE_ID
Team: ${TEAM_NAME:-solo}
Reviewed: APPROVED
Commit: $APPROVED_SHA"

PR_URL=$(gh pr view "$FEATURE_BRANCH" --json url -q '.url')
PR_NUMBER=$(gh pr view "$FEATURE_BRANCH" --json number -q '.number')

grava comment $ISSUE_ID -m "PR created: $PR_URL"
grava label $ISSUE_ID --add pr-created
grava wisp write $ISSUE_ID pr_url "$PR_URL"
grava wisp write $ISSUE_ID pr_number "$PR_NUMBER"
grava commit -m "pr created for $ISSUE_ID"
```

Output: `PR_CREATED: $PR_URL`

## Phase 4: Wait for PR Merge (resolve comments if any)

Poll until the PR is merged or closed. If reviewers leave comments,
the team resolves them — up to **`MAX_PR_FIX_ROUNDS` (default 3)** times.
After that, halt for human intervention.

```bash
MAX_PR_FIX_ROUNDS=3
FIX_ROUND=0
LAST_SEEN_COMMENT_ID=""   # track the newest comment id we've already addressed

while true; do
  STATE=$(gh pr view "$FEATURE_BRANCH" --json state -q '.state')

  if [ "$STATE" = "MERGED" ]; then
    break
  elif [ "$STATE" = "CLOSED" ]; then
    grava wisp write $ISSUE_ID pr_closed "closed without merge"
    grava label $ISSUE_ID --add pr-rejected
    # Output: PIPELINE_FAILED: PR closed without merge
    PIPELINE_RESULT="PIPELINE_FAILED: PR closed without merge"
    break
  fi

  # Pull only NEW review comments (since last round)
  COMMENTS_JSON=$(gh api "repos/{owner}/{repo}/pulls/$PR_NUMBER/comments" 2>/dev/null)
  NEW_COMMENTS=$(echo "$COMMENTS_JSON" | jq -r --arg last "$LAST_SEEN_COMMENT_ID" '
    [.[] | select(.in_reply_to_id == null) | select(.id > ($last | tonumber? // 0))]
  ')
  COMMENT_COUNT=$(echo "$NEW_COMMENTS" | jq 'length')
  REVIEW_DECISION=$(gh pr view "$FEATURE_BRANCH" --json reviewDecision -q '.reviewDecision')

  if [ "$COMMENT_COUNT" -gt 0 ] || [ "$REVIEW_DECISION" = "CHANGES_REQUESTED" ]; then
    FIX_ROUND=$((FIX_ROUND + 1))
    if [ $FIX_ROUND -gt $MAX_PR_FIX_ROUNDS ]; then
      grava wisp write $ISSUE_ID pr_fix_exhausted "$MAX_PR_FIX_ROUNDS rounds"
      grava label $ISSUE_ID --add needs-human
      PIPELINE_RESULT="PIPELINE_HALTED: PR comment fix loop exhausted ($MAX_PR_FIX_ROUNDS rounds)"
      break
    fi

    FEEDBACK=$(echo "$NEW_COMMENTS" | jq -r '.[] | "[\(.path):\(.line // .original_line)] \(.body)"')

    # Re-spawn coder to fix (see Agent call in Phase 2 re-spawn pattern)
    # Orchestrator runs:
    #   Agent({
    #     description: "Fix PR comments for $ISSUE_ID (round $FIX_ROUND/$MAX_PR_FIX_ROUNDS)",
    #     subagent_type: "coder",
    #     prompt: "RESUME: true. Issue $ISSUE_ID PR comments to resolve:\n$FEEDBACK\n
    #              Worktree .worktree/$ISSUE_ID exists. Skip Phase A. Fix, commit, push
    #              to branch grava/$ISSUE_ID. Output CODER_DONE: <sha> or CODER_HALTED."
    #   })
    CODER_RESULT="<parsed from Agent tool result>"

    case "$CODER_RESULT" in
      *CODER_DONE*)
        grava wisp write $ISSUE_ID pr_comments_resolved "round $FIX_ROUND"
        LAST_SEEN_COMMENT_ID=$(echo "$COMMENTS_JSON" | jq -r '[.[].id] | max')
        # Output signal: PR_COMMENTS_RESOLVED: $FIX_ROUND
        ;;
      *CODER_HALTED*)
        grava wisp write $ISSUE_ID pr_comments_halted "round $FIX_ROUND: $CODER_RESULT"
        grava label $ISSUE_ID --add needs-human
        PIPELINE_RESULT="PIPELINE_HALTED: coder could not resolve PR comments at round $FIX_ROUND"
        break 2
        ;;
    esac
  fi

  sleep 30   # poll every 30 seconds
done
```

After successful merge:

```bash
grava close $ISSUE_ID --actor pipeline
grava comment $ISSUE_ID -m "PR merged: $PR_URL"
grava commit -m "closed $ISSUE_ID (PR merged)"
PIPELINE_RESULT="PIPELINE_COMPLETE: $ISSUE_ID"
```

Output: final `PIPELINE_RESULT` — `PIPELINE_COMPLETE` on success,
`PIPELINE_HALTED` if fix loop exhausted, `PIPELINE_FAILED` if PR closed or creation failed.

The team now moves to the next issue.

If `gh pr create` (Phase 3) fails:
```bash
grava wisp write $ISSUE_ID pr_failed "<reason>"
grava label $ISSUE_ID --add pr-failed
```
Output: `PIPELINE_FAILED: pr creation failed`
```

---

### 4.2 Autopilot — Drain the Backlog

**`.claude/skills/ship-all/SKILL.md`**

```markdown
---
name: ship-all
description: "Autopilot — drain the entire grava backlog through the full pipeline."
user-invocable: true
---

# Ship All — Autopilot

Usage: `/ship-all [team-name] [--epic <id>] [--label <name>]`

Examples:
- `/ship-all` — solo team (team name = hostname or "solo")
- `/ship-all alpha` — team alpha
- `/ship-all bravo --epic grava-epic-001` — team bravo, scoped to one epic
- `/ship-all charlie --label backend` — team charlie, scoped by label

This wraps the `grava-next-issue` skill's discover loop with the
coder → reviewer → PR pipeline from /ship.

## Setup

Parse $ARGUMENTS:

```bash
# First positional arg = team name. Remaining = flags.
set -- $ARGUMENTS
TEAM_NAME="${1:-solo}"
shift 2>/dev/null || true

SCOPE_EPIC=""; SCOPE_LABEL=""
while [ $# -gt 0 ]; do
  case "$1" in
    --epic)  SCOPE_EPIC="$2"; shift 2 ;;
    --label) SCOPE_LABEL="$2"; shift 2 ;;
    *) shift ;;
  esac
done

echo "Team: $TEAM_NAME  Epic: ${SCOPE_EPIC:-any}  Label: ${SCOPE_LABEL:-any}"
```

## Workflow

Read `.claude/skills/grava-next-issue/SKILL.md` for loop semantics.

Repeat until stop condition met:

### 1. Discover (from grava-next-issue skill)

```bash
grava ready --limit 3 --json
```

Apply scope filters if set:
- `SCOPE_EPIC` → filter candidates to children of that epic (grava tree)
- `SCOPE_LABEL` → filter to issues with that label

Apply the skill's stale-state check on each candidate:
- Skip if `code_review` label present
- Skip if implementation comments exist
- Skip if claim fails due to stale heartbeat lock (other team has it)

If all 3 fail → fetch next batch. If still empty:
```bash
grava list --status open --json
grava list --status in_progress --json
grava stats --json
```
Print the skill's session summary format and stop.

### 2. Ship the chosen issue

For the first claimable candidate, record team ownership then run the full
/ship pipeline inline:

```bash
grava wisp write $ISSUE_ID team "$TEAM_NAME"
grava commit -m "team $TEAM_NAME took $ISSUE_ID"
```

Then:
- Spawn `coder` agent — works inside `.worktree/$ISSUE_ID` (grava-provisioned)
  - Prompt prefix: `"You are on team $TEAM_NAME. ..."`
- Spawn `reviewer` agent (up to 3 rounds)
- Create PR via `gh pr create` from `grava/$ISSUE_ID` branch
  - PR title prefix: `[$TEAM_NAME]` for traceability
- Wait for PR merge with comment-fix loop (max 3 rounds)
- All via the **Agent tool** with `subagent_type` — same as /ship Phases 1-4

### 3. Loop

After /ship completes (success, halt, or fail):
- Print transition line: `--- Issue <id> done. Checking for next... ---`
- Loop back to Discover

## Stop Conditions

- Backlog drained (no claimable issues)
- 3 consecutive PIPELINE_HALTED (avoid infinite halt loop)
- User interrupt

## Final Summary

```
--- /ship-all Complete ---
Issues with PR: <count>
  - <id>: <title> → <pr-url>
Issues halted: <count>
  - <id>: <reason>
Issues skipped (stale): <count>
Stopped because: <reason>
```
```

---

### 4.3 Planning Command

**`.claude/skills/plan/SKILL.md`**

```markdown
---
name: plan
description: "Generate grava issues from a markdown document or folder."
user-invocable: true
---

# Plan — Generate Issues

Usage: /plan <path-to-doc-or-folder>

Invokes the planner agent which delegates to grava-gen-issues skill.

```bash
DOC_PATH="$ARGUMENTS"
[ -e "$DOC_PATH" ] || { echo "PLAN_FAILED: $DOC_PATH not found"; exit 1; }
```

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

## Suggested Next Step

After planning completes:
```
PLAN_COMPLETE: Run /ship-all to begin draining the new backlog
```
```

---

### 4.4 Bug Hunt Command

**`.claude/skills/hunt/SKILL.md`**

```markdown
---
name: hunt
description: "Audit the codebase for bugs and file them as grava issues."
user-invocable: true
---

# Hunt — Bug Audit

Usage: /hunt [scope]

Scope options:
- (none) → since last tag (default)
- recent → last 20 commits
- all → full codebase
- <package-path> → targeted package

Invokes the bug-hunter agent which delegates to grava-bug-hunt skill.

```bash
SCOPE="${ARGUMENTS:-since-last-tag}"
```

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

## Suggested Next Step

```
HUNT_COMPLETE: Run /ship-all to fix the new bugs in priority order
```
```

---

## 5. Hooks

### 5.1 Settings

Add to existing `.claude/settings.json` (merge with current hooks):

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "./scripts/hooks/sync-pipeline-status.sh"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "./scripts/hooks/warn-in-progress.sh"
          }
        ]
      }
    ]
  }
}
```

> **Note:** This follows the existing `settings.json` hook format used by
> `TaskCompleted`, `TeammateIdle`, and `TaskCreated` hooks. Merge — don't replace.

---

### 5.2 Status Sync Hook

**`scripts/hooks/sync-pipeline-status.sh`**

Bash script matching the existing hook convention in `scripts/hooks/`.

```bash
#!/bin/bash
# PostToolUse hook — captures pipeline signals in Bash output and
# syncs them to grava wisps for crash recovery.
#
# Hook input (JSON on stdin) includes tool_name, tool_input, tool_output, cwd.
# The issue ID is extracted from the agent's worktree cwd (.worktree/<id>)
# since parallel teams have multiple in-progress issues — we cannot use
# `.[0]` on `grava list --status in_progress`.

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
OUTPUT=$(echo "$INPUT" | jq -r '.tool_output // empty')
CWD=$(echo "$INPUT" | jq -r '.cwd // empty')
TOOL_CMD=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# Only process Bash tool output
[ "$TOOL_NAME" = "Bash" ] || exit 0
[ -n "$OUTPUT" ] || exit 0

# Resolve ISSUE_ID from cwd if we're inside a grava worktree.
# Pattern: any path containing /.worktree/<id>(/...)
ISSUE_ID=""
if [[ "$CWD" =~ /\.worktree/([a-zA-Z0-9_-]+)(/|$) ]]; then
  ISSUE_ID="${BASH_REMATCH[1]}"
fi

# Fallback 1: extract from an explicit `grava <cmd> <id>` in the bash command
if [ -z "$ISSUE_ID" ] && [ -n "$TOOL_CMD" ]; then
  ISSUE_ID=$(echo "$TOOL_CMD" | grep -oE 'grava[[:space:]]+(claim|show|comment|label|wisp|stop|close|commit)[[:space:]]+[a-zA-Z0-9_-]+' | head -1 | awk '{print $3}')
fi

# Fallback 2: if signals present but no ID resolved, scan tool_output
# for an embedded grava-id pattern (agents often echo their issue id)
if [ -z "$ISSUE_ID" ]; then
  ISSUE_ID=$(echo "$OUTPUT" | grep -oE 'grava-[a-zA-Z0-9]{4,}' | head -1)
fi

# If still no ID, nothing to sync — exit silently
[ -n "$ISSUE_ID" ] || exit 0

# Match pipeline signals → grava wisp state
case "$OUTPUT" in
  *CODER_DONE*)           grava wisp write "$ISSUE_ID" pipeline_phase coding_complete 2>/dev/null ;;
  *CODER_HALTED*)         grava wisp write "$ISSUE_ID" pipeline_phase coding_halted 2>/dev/null ;;
  *REVIEWER_APPROVED*)    grava wisp write "$ISSUE_ID" pipeline_phase review_approved 2>/dev/null ;;
  *REVIEWER_BLOCKED*)     grava wisp write "$ISSUE_ID" pipeline_phase review_blocked 2>/dev/null ;;
  *PR_CREATED*)           grava wisp write "$ISSUE_ID" pipeline_phase pr_created 2>/dev/null ;;
  *PR_COMMENTS_RESOLVED*) grava wisp write "$ISSUE_ID" pipeline_phase pr_comments_resolved 2>/dev/null ;;
  *PR_MERGED*)            grava wisp write "$ISSUE_ID" pipeline_phase pr_merged 2>/dev/null ;;
  *PIPELINE_HALTED*)      grava wisp write "$ISSUE_ID" pipeline_phase halted_human_needed 2>/dev/null ;;
  *PIPELINE_FAILED*)      grava wisp write "$ISSUE_ID" pipeline_phase failed 2>/dev/null ;;
  *PIPELINE_COMPLETE*)    grava wisp write "$ISSUE_ID" pipeline_phase complete 2>/dev/null ;;
esac

exit 0
```

---

### 5.3 In-Progress Warning Hook

**`scripts/hooks/warn-in-progress.sh`**

```bash
#!/bin/bash
# Stop hook — warns when a session ends with in-progress issues.
# With parallel teams, multiple issues may be in_progress; surface ALL of them
# along with their team wisp (if set) so the user knows which terminals own what.
# Matches existing hook convention in scripts/hooks/.

ISSUES=$(grava list --status in_progress --json 2>/dev/null)
[ $? -eq 0 ] || exit 0

COUNT=$(echo "$ISSUES" | jq 'length')
[ "$COUNT" -gt 0 ] || exit 0

echo "Warning: Session ending with $COUNT in-progress issue(s):" >&2

echo "$ISSUES" | jq -r '.[].id' | while read -r id; do
  team=$(grava wisp read "$id" team 2>/dev/null || echo "unknown")
  title=$(grava show "$id" --json 2>/dev/null | jq -r '.title // "?"')
  echo "   [$team] $id: $title" >&2
done

echo "Run \`grava stop <id>\` to release, or \`grava doctor\` for orphan check." >&2
exit 0
```

---

## 6. CLAUDE.md Updates

Append these sections to the existing `CLAUDE.md`:

```markdown
## Agent Team

| Command | Description | Skills Used |
|---------|-------------|-------------|
| `/ship <id>` | Single-issue pipeline (code → review → PR) | grava-claim, grava-dev-epic, grava-code-review |
| `/ship-all` | Autopilot — drain backlog, create PRs | grava-next-issue + above |
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
| `REVIEWER_BLOCKED` | reviewer | grava-code-review verdict CHANGES_REQUESTED |
| `PR_CREATED: <url>` | orchestrator | PR opened, polling for merge begins |
| `PR_COMMENTS_RESOLVED: <round>` | orchestrator | Coder fixed PR feedback, pushed to branch |
| `PR_MERGED` | orchestrator | PR merged upstream, closing issue now |
| `PIPELINE_COMPLETE: <id>` | orchestrator | PR merged + `grava close` done; team advances |
| `PIPELINE_HALTED: <reason>` | orchestrator | Human intervention needed (review loop or PR-fix loop exhausted) |
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
| Approval signal | Literal string in `prompt` | `"REVIEWER_APPROVED"` |
| Worktree | grava-provisioned at `.worktree/<issue-id>` via `grava claim` | Agent `cd .worktree/$ISSUE_ID` after claim |

Agents read shared state from the grava DB via CLI (`grava show`, `grava wisp read`).
This is the crash-recovery mechanism — wisps persist across sessions.
```

---

## 7. End-to-End Flow Examples

### Example 1: Spec → Backlog → Ship

```bash
# 1. User uploads a PRD
/plan docs/feature-spec.md
# → planner invokes grava-gen-issues
# → user approves the plan
# → 49 issues created
# → PLANNER_DONE

# 2. Drain the new backlog
/ship-all
# → orchestrator uses grava-next-issue to discover
# → for each issue:
#     coder (grava-claim → grava-dev-epic) → CODER_DONE
#     reviewer (grava-code-review) → REVIEWER_APPROVED or BLOCKED loop
#     gh pr create → PR_CREATED
#     wait for PR merge → issue closed
#     next issue
# → /ship-all completes when backlog drained
```

### Example 2: Bug Hunt → Fix → Ship

```bash
# 1. Weekly audit
/hunt
# → bug-hunter invokes grava-bug-hunt
# → 8 bugs filed (3 critical, 5 medium)
# → BUG_HUNT_COMPLETE

# 2. Auto-fix in priority order
/ship-all
# → orchestrator picks up critical bugs first (priority sort)
# → coder fixes each
# → reviewer validates
# → PR created for each
```

### Example 3: Single Issue with Review Loop

```bash
/ship grava-abc123
# Round 1: coder implements, reviewer finds 2 CRITICAL issues → BLOCKED
# Round 2: coder fixes, reviewer finds 1 HIGH → BLOCKED
# Round 3: coder fixes, reviewer APPROVED
# PR created
# Issue closes when PR is merged
```

### Example 4: Recovery After Crash

```bash
# Session crashed mid-pipeline
grava doctor
# Shows: grava-abc123 in_progress, last wisp: pipeline_phase=coding_complete

# Resume from review phase
grava show grava-abc123 --json | jq '.last_commit'
# Re-run from where it left off
/ship grava-abc123
# Coder agent sees code_review label + last_commit → skips to reviewer
```

---

## 8. Parallel Teams (Multi-Terminal)

### Concept

Run N agent teams in N terminals, all pulling from the same grava backlog.
Each team works in its own worktree on feature branches.
Pipeline is **code → review → PR → wait for merge → close → next**.

```
Terminal 1                Terminal 2                Terminal 3
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│ /ship-all alpha  │      │ /ship-all bravo  │      │ /ship-all charlie│
│                  │      │                  │      │                  │
│ discover → claim │      │ discover → claim │      │ discover → claim │
│ coder (worktree) │      │ coder (worktree) │      │ coder (worktree) │
│ reviewer         │      │ reviewer         │      │ reviewer         │
│ gh pr create     │      │ gh pr create     │      │ gh pr create     │
│ wait for merge   │      │ wait for merge   │      │ wait for merge   │
│ close + next...  │      │ close + next...  │      │ close + next...  │
└────────┬─────────┘      └────────┬─────────┘      └────────┬─────────┘
         │                         │                         │
         └─────────────────────────┼─────────────────────────┘
                                   ▼
                  ┌────────────────────────────────┐
                  │         Grava DB (shared)       │
                  │  .grava/dolt/ — atomic claims   │
                  │  feature branches — per issue   │
                  └────────────────────────────────┘
                                   │
                          (later, manually)
                                   ▼
                  ┌────────────────────────────────┐
                  │  Merge branches + resolve       │
                  │  conflicts + test + deploy      │
                  └────────────────────────────────┘
```

### Pipeline (Unified)

Every team runs the same pipeline. There is no solo/team mode distinction.

```
code → review → create PR → wait for merge → close issue → next issue
```

After `REVIEWER_APPROVED`, the orchestrator creates a PR then polls
until the PR is merged. Only after merge does the team move on.

### Issue Lifecycle

```
open → claimed (in_progress) → PR created → PR merged → closed → next issue
```

The team waits for the PR to merge before moving on.
The orchestrator polls `gh pr view` every 30 seconds until merge or rejection.
After merge, the orchestrator runs `grava close` inline — no async mechanism needed.

### Launch

```bash
# Terminal 1
claude          # then: /ship-all alpha

# Terminal 2
claude          # then: /ship-all bravo

# Terminal 3
claude          # then: /ship-all charlie
```

Target a specific epic or label:

```bash
/ship-all alpha --epic grava-epic-001
/ship-all bravo --label backend
```

### Team Identity

The team name is passed to each agent's prompt for audit:

```
Agent({
  description: "[alpha] Implement $ISSUE_ID",
  subagent_type: "coder",
  prompt: "You are on team alpha. Claim and implement issue $ISSUE_ID.
           After claim, cd .worktree/$ISSUE_ID. ..."
})
```

Recorded in wisps for tracking:

```bash
grava wisp write $ISSUE_ID team "$TEAM_NAME"
```

### How Contention Is Resolved

| Scenario | What Happens |
|----------|-------------|
| Two teams claim same issue | `grava claim` is atomic — loser skips to next candidate |
| Two teams modify same file | Each in its own worktree — no conflict during work. Conflicts resolve at merge time. |
| Team finishes but branch can't merge cleanly | That's expected. Human resolves at merge time. |

### Merge & Deploy (human-driven, outside pipeline)

PRs are the pipeline's output. Merge and deploy are separate:

```bash
# See what's ready to merge
grava list --label pr-created --json
gh pr list --state open

# Review and merge PRs (resolve conflicts as needed)
gh pr merge <pr-number> --merge

# Issues auto-close on merge (via GitHub Actions or periodic check)

# Deploy when ready (after merging a batch, a sprint, etc.)
go test -race ./...
gcloud run deploy grava --source . --region asia-southeast1 --quiet
```

### Testing

| What | When | Owned By |
|------|------|----------|
| Unit tests per issue | During coding (TDD) | coder agent |
| Test validation hook | On task completion | `validate-task-complete.sh` |
| Code review test check | During review | reviewer agent |
| Cross-team regression | After PR merge to main | CI / human at merge time |
| Full suite before deploy | Before `gcloud deploy` | CI / human |

Each team's tests pass in their worktree. Cross-team regressions are caught
when PRs merge to main — that's the standard git workflow.

### Final Summary Format

```
--- /ship-all [$TEAM_NAME] Complete ---
Issues with PR created: <count>
  - <id>: <title> → PR: <pr-url>
Issues halted: <count>
  - <id>: <reason>
Issues skipped (claimed by other team): <count>
Stopped because: <reason>
```

---

## 9. Worktree Management (grava-owned + optional claude hook)

### Two paths, one directory

Both grava and Claude Code can produce git worktrees. Both land under `.worktree/`.
The pipeline uses grava; Claude's custom hook (from `claude-code-custom-worktree.md`)
is available for ad-hoc work outside the pipeline.

| Path | Creator | Trigger | Branch | Lifecycle owner |
|------|---------|---------|--------|-----------------|
| Pipeline | `grava claim <id>` | coder Phase A | `grava/<id>` | grava (`grava close` removes) |
| Ad-hoc | Claude worktree hook | `claude -w <name>` | `worktree-<name>` | Claude hook (`WorktreeRemove`) |

Directory layout:

```
grava/
├── .worktree/
│   ├── grava-abc123/        ← grava claim (pipeline)  on branch grava/grava-abc123
│   ├── grava-def456/        ← grava claim (pipeline)  on branch grava/grava-def456
│   ├── scratch-refactor/    ← claude -w scratch-refactor  on branch worktree-scratch-refactor
│   └── spike-auth/          ← claude -w spike-auth        on branch worktree-spike-auth
└── .claude/
    ├── settings.json        ← hooks registered (both WorktreeCreate AND PostToolUse)
    └── hooks/
        └── worktree.sh      ← from claude-code-custom-worktree.md
```

### Why not merge the two paths?

`grava claim` does more than create a worktree:
- Atomic claim (prevents parallel teams racing)
- Status transition to `in_progress`
- Prerequisite / dependency check
- Wisp heartbeat for crash recovery
- Branch naming convention (`grava/<id>`) — enables `grava close` cleanup

The Claude hook is a thin git-worktree wrapper. Making `grava claim` go through it
adds a layer with no benefit; letting the hook call `grava claim` couples Claude's
UX to grava's internal state machine. Keep them independent.

### Gotchas

1. **Don't run `claude -w grava-<id>`** for an issue already in the pipeline —
   the hook would create branch `worktree-grava-<id>`, conflicting with
   `grava/grava-<id>`. Use `/ship <id>` or manual `grava claim <id>` instead.

2. **`.worktree/` must be in `.gitignore`** — already required by both guides.

3. **Ad-hoc worktrees don't sync with grava** — no issue status updates, no wisps.
   Fine for exploration; don't use for production issues.

### When to use each

| Use case | Tool |
|----------|------|
| Ship a grava issue through the pipeline | `/ship <id>` (uses grava claim) |
| Parallel team autopilot | `/ship-all <team>` (uses grava claim) |
| Quick code spike / scratch branch | `claude -w <scratch-name>` |
| Test a PR from another contributor | `claude -w pr-review-<num>` |
| Manual issue work outside pipeline | `grava claim <id>` then `cd .worktree/<id>` |

---

## 10. Implementation Order

| Step | Action | Validates |
|------|--------|-----------|
| 1 | Verify all 7 skills exist in `.claude/skills/` | Skill registry intact |
| 2 | Create 4 agent files (coder, reviewer, bug-hunter, planner) | Agents resolve skills correctly |
| 3 | Create 4 orchestrator skills (`ship`, `ship-all`, `plan`, `hunt`) in `.claude/skills/` | End-to-end pipeline |
| 4 | Add hooks (`sync-pipeline-status.sh`, `warn-in-progress.sh`) to `scripts/hooks/` | Wisp state tracking |
| 5 | Merge new hooks into existing `settings.json` | Hooks fire correctly |
| 6 | Append Agent Team + Skill Map sections to `CLAUDE.md` | Documentation |
| 7 | Smoke test: `/ship` on a low-risk issue | Full pipeline works |
| 8 | Smoke test: `/plan` on a small spec doc | Planner agent works |
| 9 | Smoke test: `/hunt` on a single package | Bug hunter works |
| 10 | Production: `/ship-all` end-to-end | Autopilot works |
| 11 | Add team identity + PR-merge closure to `/ship-all` | Multi-team support |
| 12 | Smoke test: 2 terminals running `/ship-all` on same backlog | Claim contention works |
| 13 | Stress test: 3 terminals, merge branches after, run tests | End-to-end parallel flow |

---

## 11. Failure Modes & Recovery

| Failure | Detected By | Recovery |
|---------|-------------|----------|
| Coder skill HALT | `CODER_HALTED` signal | Issue returned to `open`, human notified |
| Coder crashed mid-impl | `grava doctor` orphan check | Re-run `/ship <id>` — coder reads wisp + commit, resumes |
| Stale claim (heartbeat lock) | grava-next-issue stale check | Skipped automatically, next candidate tried |
| Already-implemented in queue | grava-next-issue stale check | Skipped (has `code_review` label) |
| Reviewer loop exhausted | 3 BLOCKED rounds | Issue labeled `needs-human`, pipeline halts |
| PR creation fails | `PIPELINE_FAILED: pr creation` | Check `gh auth`, push branch manually, re-run |
| PR comments unresolvable | `PIPELINE_HALTED: could not resolve PR comments` | Issue labeled `needs-human`, coder couldn't fix feedback |
| PR closed without merge | `PIPELINE_FAILED: PR closed without merge` | Re-open PR or re-run `/ship` from scratch |
| Planner gap (missing API) | grava-gen-issues validation | Skill blocks, asks user for clarification |
| Bug hunter empty | grava-bug-hunt finds nothing | Reports "0 bugs found" — codebase is clean |
| Claim contention (parallel teams) | `grava claim` returns "already claimed" | Team skips to next candidate automatically |
| Merge conflict (at merge time) | `git merge` fails | Human resolves — branches are preserved, no work lost |
| Cross-team regression (at merge time) | `go test` fails on merged main | Human fixes before deploy — each branch is still clean |

---

## 12. Maintenance

### When to Update Skills vs Agents

| Change Type | Update Skill | Update Agent |
|-------------|--------------|--------------|
| Add a TDD step | `grava-dev-epic/workflow.md` | No change needed |
| Add a review check | `grava-code-review/SKILL.md` | No change needed |
| Add new pipeline phase | New skill OR new agent | Both — new agent invokes new skill |
| Change PR template | (no skill) | `ship/SKILL.md` Phase 3 |
| Change signal vocabulary | All affected agents + CLAUDE.md | Yes |

The rule: **logic lives in skills, sequencing lives in agents**. If the change is *what* an agent does, it's a skill update. If the change is *when* it does it, it's an agent or orchestrator update.
