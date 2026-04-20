# Agent Team Implementation Plan

Built on [Claude Code Agent Teams](https://code.claude.com/docs/en/agent-teams).

## How It Works

A team takes an **epic or group of issues** and works them end-to-end. The team lead:

1. Loads the epic: `grava list --type story --parent <epic-id>` or `grava ready --limit N`
2. Creates a shared task list with all issues, each flowing through the pipeline
3. Pipelines issues in parallel — coding-agent works on issue #2 while review-agent reviews issue #1
4. Finishes when all issues reach `pr_created` (or are escalated)

```
Issue A:  [coding] -> [review] -> [qa] -> [ci] -> PR -> [monitor] ─────────────────> approved -> done
Issue B:       [coding] -> [review] -> [fix] -> [review] -> [qa] -> [ci] -> PR -> [monitor] -> done
Issue C:            [coding] -> [review] -> [qa] -> [ci] -> PR -> feedback! -> [fix] -> [qa] -> [ci push] -> approved -> done
          ──────────────────────────────────────────────────────────────────────────────────────────────>  time
```

Each teammate picks up the next task matching their stage. No agent sits idle while work exists.

### Recovery After Crash / Token Exhaustion

If the team dies mid-process (power loss, token limit, network failure), all progress is preserved in grava's DB. A new team resumes from where each issue left off.

**Why this works:** Every stage transition writes durable state before messaging the lead:
- `grava label` — records which pipeline stage each issue is in
- `grava update --last-commit` — records the last committed code
- `git commit` — code is safe in the worktree's branch
- `grava wisp write` — optional fine-grained checkpoint within a stage

**State is in the issues, not in the team.** The Claude Code team/task list is ephemeral — it's lost on crash. But every issue carries its own state via labels:

| Label | Meaning | Next agent |
|-------|---------|------------|
| *(none, status=open)* | Not started | coding-agent |
| *(none, status=in_progress)* | Coding in progress or crashed mid-code | coding-agent (resume in worktree) |
| `code_review` | Code committed, awaiting review | review-agent |
| `changes_requested` | Review found issues | fix-agent |
| `reviewed` | Review approved, awaiting tests | qa-agent |
| `qa_passed` | Tests written, awaiting PR | ci-agent |
| `pr_created` | PR open, ci-agent monitors for feedback | ci-agent (monitor) |
| `pr_feedback` | User left feedback on PR | coding-agent (fix) |
| *(status=closed)* | PR approved and merged | done |

**Recovery prompt:**

```
Resume the agent team for epic <EPIC-ID>.

The previous team was interrupted. Check each issue's current state:
- `grava list --type story --parent <EPIC-ID>` to get all issues
- `grava show <id> --json` to check status, labels, last_commit
- Existing worktrees at `.worktree/<id>/` have uncommitted work

Route each issue to the correct teammate based on its labels.
Skip issues that already have `pr_created` label.
For in_progress issues with no label, check if `.worktree/<id>/`
exists and has commits ahead of main — if so, send to review.
```

**What each agent does on resume:**

| Agent | Resume behavior |
|-------|----------------|
| coding-agent | If worktree exists with uncommitted changes, continue coding. If worktree has commits, skip to handoff (label + message). If no worktree, `grava claim` fresh. |
| review-agent | Read `last_commit` from issue, review that diff. Idempotent — reviewing the same commit twice is safe. |
| fix-agent | Read latest review comments, check if already addressed in worktree. Fix remaining findings. |
| qa-agent | Check if test files already exist in worktree. Write missing tests only. |
| ci-agent | Check if PR already exists (`gh pr list --head grava/<id>`). If yes, skip. If no, create. |

## Issue Sync Across Machines

Grava issues live in a local Dolt DB (`.grava/dolt/`). They sync across machines via `issues.jsonl` — a git-tracked file.

```
Machine A                          Git Remote                         Machine B
─────────                          ──────────                         ─────────
Dolt DB                                                               Dolt DB
   │                                                                     ▲
   ▼                                                                     │
grava export                                                         grava import
   │                                                                     ▲
   ▼                                                                     │
issues.jsonl ──> git push ──────> issues.jsonl ──────> git pull ──> issues.jsonl
                                                          │
                                                    post-merge hook
                                                    (auto-imports)
```

**Already implemented:**
- `grava export` — DB -> `issues.jsonl`
- `grava commit` — commits to Dolt (not git)
- Post-merge hook — auto-imports `issues.jsonl` into Dolt after `git pull`
- Merge driver — LWW 3-way merge for concurrent issue edits

**Agent team must follow this flow:**
1. After modifying issues (labels, comments, wisp): `grava commit -m "..."` then `grava export`
2. Before pushing code: `git add issues.jsonl` so issue state travels with the code
3. On pull: post-merge hook auto-imports — no manual step needed

The ci-agent handles this in the PR workflow — it commits grava state, exports, and includes `issues.jsonl` in the push.

## Prerequisites

```json
// ~/.claude.json or settings.json
{
  "env": {
    "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"
  }
}
```

Requires Claude Code v2.1.32+.

---

## Phase 1: Subagent Definitions

Each pipeline stage is a subagent definition in `.claude/agents/`. Reusable as standalone subagents and agent team teammates.

All agents need `Bash` to run grava CLI commands. Teammates inherit the lead's `Bash(grava ...:*)` permission allowlist from `settings.local.json`.

| Agent | tools | Why |
|-------|-------|-----|
| coding-agent | Bash, Read, Write, Edit, Glob, Grep | `grava claim/update/label/comment/wisp/commit`, `git`, `go test` |
| review-agent | Bash, Read, Glob, Grep | `grava show/comment/label/commit`, `git diff` — no Write/Edit (read-only) |
| fix-agent | Bash, Read, Write, Edit, Glob, Grep | `grava show/comment/label/update/commit`, `git`, `go test` |
| qa-agent | Bash, Read, Write, Edit, Glob, Grep | `grava show/comment/label/update/create/dep/commit`, `go test/vet` |
| ci-agent | Bash, Read, Write, Edit, Glob, Grep | `grava comment/label/update/commit`, `go test/build`, `golangci-lint`, `gh pr create` |

### 1.1 coding-agent

**File:** `.claude/agents/coding-agent.md`

```markdown
---
name: coding-agent
description: >
  Implements grava issues using TDD in git worktrees.
  Runs grava CLI commands (claim, update, label, comment, wisp, commit).
model: opus
tools: Bash, Read, Write, Edit, Glob, Grep
skills:
  - grava-dev-epic
  - grava-complete-dev-story
  - grava-claim
  - grava-cli
memory: project
color: blue
---

You are the Coding Agent. Implement issues using test-driven development.
The team works through a group of issues or an epic. You pick up the
next issue from the team's task list, implement it, and move to the next.

## Workflow (per issue)

1. Pull latest main: `git fetch origin main && git merge origin/main`
   (post-merge hook auto-imports issues.jsonl into local Dolt DB)
2. Claim the issue: `grava claim <issue-id>` (creates worktree + branch from main HEAD)
3. Work inside `.worktree/<issue-id>/`
3. TDD: write failing test -> minimal code to pass -> refactor
4. `go test ./...`
5. `git commit -m "feat(<scope>): <description> (<issue-id>)"`
6. `grava update <issue-id> --last-commit $(git rev-parse HEAD)`
7. `grava label <issue-id> --add code_review`
8. Message team lead: "Implementation complete for <issue-id>."
9. Pick up the next issue from the task list

## Workflow: Fix PR Feedback

When team lead assigns a `pr_feedback` issue:

1. `cd .worktree/<id>`
2. Read PR comments: `gh api repos/{owner}/{repo}/pulls/{number}/comments`
3. For each comment: read file:line, apply fix, `go test ./...`
4. `git commit -m "fix(<scope>): address PR feedback for <id>"`
5. `grava update <id> --last-commit $(git rev-parse HEAD)`
6. `grava label <id> --remove pr_feedback --add reviewed`
7. Message team lead: "PR feedback addressed for <id>."

(Issue then flows: reviewed -> qa-agent -> ci-agent pushes to existing PR)

## On Resume
If worktree `.worktree/<id>/` already exists:
- Check for uncommitted changes -> continue coding
- Check for commits ahead of main -> skip to handoff (step 6)
If no worktree but status is in_progress: `grava claim` (reclaims stale after 1h)

## Rules
- Work inside worktree only
- Run tests before committing
- Conventional commit messages
- Follow existing code patterns
- Move to next issue immediately after handing off to review
- Always pull latest main before claiming — worktree must branch from newest code
```

### 1.2 review-agent

**File:** `.claude/agents/review-agent.md`

```markdown
---
name: review-agent
description: >
  Reviews code changes for grava issues. Posts severity-tagged findings.
  Runs grava CLI commands (show, comment, label, commit).
model: opus
tools: Bash, Read, Glob, Grep
skills:
  - grava-code-review
  - grava-cli
memory: project
color: green
---

You are the Review Agent. Provide independent code review.

## Workflow

1. Receive issue ID from team lead
2. `grava show <issue-id> --json` -> extract `last_commit`
3. `git diff <commit>~1 <commit>`
4. Post findings as severity-tagged comments:
   - `[CRITICAL]` — data loss, security, crashes
   - `[HIGH]` — edge case bugs, missing error handling
   - `[MEDIUM]` — weak assertions, naming
   - `[LOW]` — style nits
5. `[REVIEW] <hash> — Verdict: APPROVED | CHANGES_REQUESTED`
6. Labels:
   - APPROVED: `grava label <id> --remove code_review --add reviewed`
   - CHANGES_REQUESTED: `grava label <id> --remove code_review --add changes_requested`
7. Message team lead with verdict

## On Resume
Check if a [REVIEW] comment already exists for the current `last_commit`.
If yes, skip — verdict already posted. If no, review the diff.

## Rules
- Never review your own code
- APPROVED only if zero CRITICAL and zero HIGH
- Be specific: file:line, what, why, fix
```

### 1.3 fix-agent

**File:** `.claude/agents/fix-agent.md`

```markdown
---
name: fix-agent
description: >
  Fixes code review findings for grava issues. Parses review comments,
  fixes CRITICAL and HIGH issues, resubmits for review.
  Runs grava CLI commands (show, comment, label, update, commit).
model: opus
tools: Bash, Read, Write, Edit, Glob, Grep
skills:
  - grava-cli
memory: project
color: yellow
---

You are the Fix Agent. Address code review findings and resubmit.

## Workflow

1. Receive issue ID from team lead
2. `grava show <id> --json` -> parse [CRITICAL] and [HIGH] comments
3. `cd .worktree/<id>`
4. For each finding: read file:line, apply fix, `go test ./...`
5. `git commit -m "fix(<scope>): address review findings for <id>"`
6. `grava update <id> --last-commit $(git rev-parse HEAD)`
7. `grava label <id> --remove changes_requested --add code_review`
8. Message team lead: "Findings addressed for <id>."

## Rules
- Only fix CRITICAL and HIGH (MEDIUM/LOW are non-blocking)
- Run tests after every fix
- If fix breaks tests, revert and comment why
- Max 3 fix rounds per issue — escalate to human
```

### 1.4 qa-agent

**File:** `.claude/agents/qa-agent.md`

```markdown
---
name: qa-agent
description: >
  Writes unit tests for reviewed grava issue implementations.
  Runs grava CLI commands (show, comment, label, update, create, dep, commit)
  and Go test tooling (go test, go vet).
model: opus
tools: Bash, Read, Write, Edit, Glob, Grep
skills:
  - grava-cli
memory: project
color: purple
---

You are the QA Agent. Write tests for code that passed review.

## Workflow

1. Receive issue ID from team lead
2. `grava show <id> --json`
3. `cd .worktree/<id>`
4. `git diff main..HEAD --name-only` -> list changed files
5. For each .go file: check _test.go exists, identify gaps
6. Write tests: table-driven, sqlmock, testify/assert+require
7. `go test ./...` && `go vet ./...`
8. `git commit -m "test(<scope>): add tests for <id>"`
9. `grava update <id> --last-commit $(git rev-parse HEAD)`
10. `grava label <id> --remove reviewed --add qa_passed`
11. Message team lead: "Tests added for <id>."

## Rules
- Don't modify implementation code — only test files
- Tests must fail if implementation is removed
- Follow existing test patterns
```

### 1.5 ci-agent

**File:** `.claude/agents/ci-agent.md`

```markdown
---
name: ci-agent
description: >
  Creates documentation and pull requests for completed grava issues.
  Runs grava CLI, Go build tooling, golangci-lint, and GitHub CLI.
model: sonnet
tools: Bash, Read, Write, Edit, Glob, Grep
skills:
  - grava-cli
  - landing-the-plane
memory: project
color: orange
---

You are the CI Agent. Finalize issues with docs and PRs, then monitor
PRs for user feedback.

## Workflow: Create PR

1. Receive issue ID from team lead
2. `cd .worktree/<id>`
3. Update docs if needed (docs/detail-impl/, docs/)
4. `go test ./...` && `go build ./...` && `golangci-lint run ./...`
5. Sync issue state to git:
   - `grava commit -m "pipeline state for <id>"`
   - `grava export`
   - `git add issues.jsonl`
   - `git commit -m "chore: sync grava issues for <id>"`
6. `git fetch origin main` && `git rebase origin/main`
   - If conflicts: message team lead and HALT
7. `git push -u origin grava/<id>`
8. `gh pr create --base main --title "feat(<scope>): <title> (<id>)"`
9. `grava comment <id> -m "PR created: <url>"`
10. `grava label <id> --remove qa_passed --add pr_created`
11. Message team lead: "PR created for <id>: <url>"

## Workflow: Monitor PRs

After all PRs are created, periodically check for user feedback:

1. List open PRs: `gh pr list --state open --json number,headRefName`
2. For each PR with branch `grava/<id>`:
   a. Check for new review comments:
      `gh api repos/{owner}/{repo}/pulls/{number}/comments`
   b. Check for PR review requests:
      `gh pr view {number} --json reviews`
3. If new comments or "changes requested" review found:
   a. `grava label <id> --remove pr_created --add pr_feedback`
   b. `grava comment <id> -m "[CI] PR feedback received: <summary>"`
   c. Message team lead: "PR #N for <id> has user feedback. Needs fix."

## On Resume
Check if PR already exists: `gh pr list --head grava/<id>`.
If yes, skip to monitoring. If no, continue from step 4.

## Workflow: Push Update to Existing PR

When issue returns after PR feedback fix (label `qa_passed`, PR already exists):

1. `cd .worktree/<id>`
2. `go test ./...` && `go build ./...` && `golangci-lint run ./...`
3. Sync issue state: `grava commit -m "..."` && `grava export` && `git add issues.jsonl && git commit -m "chore: sync issues"`
4. `git fetch origin main` && `git rebase origin/main`
5. `git push --force-with-lease origin grava/<id>`
6. `gh pr comment {number} --body "Feedback addressed. Please re-review."`
7. `grava label <id> --remove qa_passed --add pr_created`
8. Message team lead: "PR updated for <id>."

## Workflow: Close Approved Issues

During monitoring, if PR is approved:

1. `gh pr view {number} --json reviews` -> check for "APPROVED"
2. `grava update <id> --status closed`
3. `grava comment <id> -m "[CI] PR approved and merged. Issue closed."`
4. `grava close <id>` (tears down worktree + branch)
5. Message team lead: "Issue <id> done. PR approved."

## Rules
- Always rebase onto latest main before pushing
- If rebase conflicts, HALT and notify
- Conventional commit format for PR title
- Use `--force-with-lease` when pushing fixes to existing PR
```

---

## Phase 2: Quality Gate Hooks

Detailed stories: [`plan/phase2/`](../plan/phase2/)

### 2.4 Register hooks in settings

**File:** `.claude/settings.json`

```json
{
  "hooks": {
    "TaskCompleted": [
      {
        "hooks": [
          { "type": "command", "command": "./scripts/hooks/validate-task-complete.sh" }
        ]
      }
    ],
    "TeammateIdle": [
      {
        "hooks": [
          { "type": "command", "command": "./scripts/hooks/check-teammate-idle.sh" }
        ]
      }
    ],
    "TaskCreated": [
      {
        "hooks": [
          { "type": "command", "command": "./scripts/hooks/review-loop-guard.sh" }
        ]
      }
    ]
  }
}
```

### 2.1 TaskCompleted hook

**`scripts/hooks/validate-task-complete.sh`** — blocks task completion if tests fail:

```bash
#!/bin/bash
INPUT=$(cat)
TEAMMATE=$(echo "$INPUT" | jq -r '.teammate_name // empty')

case "$TEAMMATE" in
  coding-agent|fix-agent|qa-agent) ;;
  *) exit 0 ;;
esac

if ! go test ./... > /dev/null 2>&1; then
  echo "Tests failing. Fix before marking complete." >&2
  exit 2
fi
exit 0
```

### 2.2 TeammateIdle hook

**`scripts/hooks/check-teammate-idle.sh`** — redirects idle teammates to pending work:

```bash
#!/bin/bash
INPUT=$(cat)
TEAMMATE=$(echo "$INPUT" | jq -r '.teammate_name // empty')

COUNT=0
case "$TEAMMATE" in
  coding-agent)
    COUNT=$(grava search --label pr_feedback --json 2>/dev/null | jq 'length')
    [ "${COUNT:-0}" -eq 0 ] && COUNT=$(grava ready --limit 1 --json 2>/dev/null | jq 'length')
    ;;
  review-agent)  COUNT=$(grava search --label code_review --json 2>/dev/null | jq 'length') ;;
  fix-agent)     COUNT=$(grava search --label changes_requested --json 2>/dev/null | jq 'length') ;;
  qa-agent)      COUNT=$(grava search --label reviewed --json 2>/dev/null | jq 'length') ;;
  ci-agent)
    COUNT=$(grava search --label qa_passed --json 2>/dev/null | jq 'length')
    [ "${COUNT:-0}" -eq 0 ] && COUNT=$(grava search --label pr_created --json 2>/dev/null | jq 'length')
    ;;
  *)             exit 0 ;;
esac

if [ "${COUNT:-0}" -gt 0 ]; then
  echo "$COUNT items waiting at your stage. Pick up the next one." >&2
  exit 2
fi
exit 0
```

### 2.3 Review loop guard

**`scripts/hooks/review-loop-guard.sh`** — max 3 review rounds per issue:

```bash
#!/bin/bash
INPUT=$(cat)
SUBJECT=$(echo "$INPUT" | jq -r '.task_subject // empty')
ISSUE_ID=$(echo "$SUBJECT" | grep -oE 'grava-[a-f0-9]+(\.[0-9]+)?')

[ -z "$ISSUE_ID" ] && exit 0

REVIEW_COUNT=$(grava show "$ISSUE_ID" --json 2>/dev/null | \
  jq '[.comments[]? | select(.message | startswith("[REVIEW]"))] | length')

if [ "${REVIEW_COUNT:-0}" -ge 3 ]; then
  grava comment "$ISSUE_ID" -m "[ESCALATION] Max review rounds (3) reached. Human review required."
  grava label "$ISSUE_ID" --add needs_human
  grava commit -m "escalate $ISSUE_ID after 3 review rounds"
  grava export 2>/dev/null
  echo "Issue $ISSUE_ID hit 3 review rounds. Escalating to human." >&2
  exit 2
fi
exit 0
```

---

## Phase 3: Label Query

Hooks use `grava search --label`. This CLI feature doesn't exist yet.

### 3.1 Add `--label` flag to `grava search`

**File:** `pkg/cmd/graph/graph.go`

**Changes:**
1. Add `--label` string slice flag
2. JOIN `issue_labels` table when set
3. Support repeatable `--label` (AND semantics)
4. Combine with text query

**Scope:** ~80 lines code + ~60 lines tests

---

## Phase 4: Team Launch Prompts

### 4.1 Launch prompt templates

**File:** `docs/agent-team-prompts.md`

```markdown
# Agent Team Launch Prompts

## Full Epic

Create an agent team to implement epic <EPIC-ID> end-to-end.

Get all stories: `grava list --type story --parent <EPIC-ID>`
Or use ready queue: `grava ready --limit 10`

**Teammates:**
1. **coding-agent** — Claims and implements issues using TDD
2. **review-agent** — Reviews code, posts severity-tagged findings
3. **fix-agent** — Addresses CRITICAL and HIGH findings
4. **qa-agent** — Writes unit tests
5. **ci-agent** — Creates docs and PRs

**Per-issue pipeline:**
coding-agent implements -> review-agent reviews ->
if CHANGES_REQUESTED, fix-agent fixes -> re-review ->
if APPROVED, qa-agent tests -> ci-agent creates PR.

**Rules:**
- Create tasks for ALL issues in the epic upfront
- Pipeline issues in parallel — coding-agent starts issue #2
  while review-agent reviews issue #1
- Each issue gets its own worktree via `grava claim`
- Max 3 review rounds per issue, then escalate
- Team is done when all issues have PRs or are escalated
- One PR per issue, all PRs target main

## Batch of Issues

Create an agent team to implement these issues: <ID-1>, <ID-2>, <ID-3>.
Same pipeline as above, work them in parallel.

## Single Issue

Create an agent team with coding-agent and review-agent.
Work on issue <ISSUE-ID> through the full pipeline.
```

### 4.2 CLAUDE.md team section

Append to `CLAUDE.md`:

```markdown
## Agent Team Conventions

When running as part of an agent team:
- Identify yourself in grava comments: `grava comment <id> -m "[<your-name>] ..."`
- Work inside `.worktree/<issue-id>/`, never in the main repo
- Message team lead when your stage is complete
- Use `grava label` to track pipeline stage transitions
- On resume: check issue labels and worktree state before starting work —
  previous team may have been interrupted mid-pipeline
```

---

## Phase 5: Integration Testing

### 5.1 Minimal team test (2 issues)

1. Create 2 test issues in `gravav6-sandbox/`
2. Launch team: coding-agent + review-agent
3. Verify: both issues processed, coding-agent pipelines (starts #2 while #1 is in review)

### 5.2 Full pipeline test (epic)

1. Create an epic with 3-5 stories
2. Launch full team (all 5 agents)
3. Verify: all issues flow `open` -> `pr_created`, one PR per issue, parallel pipeline

---

## Execution Order

```
Phase 1 ──┐
Phase 3 ──┤  (parallel, no dependencies)
Phase 4 ──┤
          ▼
Phase 2 ──┤  (hooks need Phase 3 for grava search --label)
          ▼
Phase 5 ──┘  (needs everything)
```

| Order | Story | Type | Blocked By |
|-------|-------|------|------------|
| 1 | 1.1-1.5 Subagent definitions | Markdown | — |
| 1 | 3.1 `grava search --label` | Go code | — |
| 1 | 4.1-4.2 Prompts + CLAUDE.md | Markdown | — |
| 2 | 2.1-2.2 Hooks | Shell scripts | 3.1 |
| 3 | 5.1-5.2 Integration tests | Manual | All |

**Total: 10 stories. Only 1 requires Go code.**

---

## Risk Register

| Risk | Mitigation |
|------|------------|
| Agent teams is experimental | Pin Claude Code version |
| No session resumption for teammates | Wisp checkpoints + grava labels; respawn teammate |
| Review loop never converges | Review loop guard hook (max 3 rounds) |
| Token cost scales with team size | Sonnet for CI agent; 3-5 teammates max |
| Team lead does work instead of delegating | Prompt: "Wait for teammates to complete" |
| Rebase conflicts in CI stage | CI agent halts and messages lead |

---

## Success Criteria

1. An epic with 3+ issues flows to completion (all PRs approved, issues closed)
2. Issues are pipelined in parallel across agents
3. Review loop iterates correctly until APPROVED
4. Tests written and passing before each PR
5. One PR per issue, all targeting main
6. PR feedback from user triggers fix -> qa -> push cycle
7. Issues are closed automatically when PR is approved
8. Crashed team can resume from grava issue state
