# Story 2B.5: /ship Skill — Single-Issue Pipeline

Orchestrator skill that takes one issue through the full pipeline: code → review → PR → merge. Spawns coder + reviewer agents, creates PR, polls for merge while handling PR comments.

## File

`.claude/skills/ship/SKILL.md`

## Frontmatter

```yaml
---
name: ship
description: "Single-issue pipeline: code → review → PR. Use when user says /ship <id>."
user-invocable: true
---
```

## Usage

```
/ship <issue-id>
```

Example: `/ship grava-abc123`

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

**No `isolation` param** — grava-claim provisions `.worktree/$ISSUE_ID` with branch `grava/$ISSUE_ID`.

Parse the returned result:
- Contains `CODER_DONE: <sha>` → extract SHA, proceed to Phase 2
- Contains `CODER_HALTED` → output `PIPELINE_HALTED: coder — <reason>` and stop

## Phase 2: Review (max 3 rounds)

For round in 1..3:

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
- Capture CRITICAL/HIGH findings
- `grava wisp write $ISSUE_ID review_round_$N "blocked"`
- Re-spawn coder with findings:

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

Loop back to reviewer if CODER_DONE.

If 3 rounds exhausted:
```bash
grava wisp write $ISSUE_ID pipeline_halted "review loop exhausted (3 rounds)"
grava label $ISSUE_ID --add needs-human
grava stop $ISSUE_ID
```
Output: `PIPELINE_HALTED: $ISSUE_ID needs human review` and stop.

## Phase 3: Create PR

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

If `gh pr create` fails:
```bash
grava wisp write $ISSUE_ID pr_failed "<reason>"
grava label $ISSUE_ID --add pr-failed
```
Output: `PIPELINE_FAILED: pr creation failed`

## Phase 4: Wait for PR Merge (resolve comments if any)

Poll until PR is merged or closed. Resolve comments up to 3 rounds.

```bash
MAX_PR_FIX_ROUNDS=3
FIX_ROUND=0
LAST_SEEN_COMMENT_ID=""

while true; do
  STATE=$(gh pr view "$FEATURE_BRANCH" --json state -q '.state')

  if [ "$STATE" = "MERGED" ]; then
    break
  elif [ "$STATE" = "CLOSED" ]; then
    grava wisp write $ISSUE_ID pr_closed "closed without merge"
    grava label $ISSUE_ID --add pr-rejected
    PIPELINE_RESULT="PIPELINE_FAILED: PR closed without merge"
    break
  fi

  # Pull only NEW review comments
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

    # Re-spawn coder to fix — orchestrator runs:
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

After merge:

```bash
grava close $ISSUE_ID --actor pipeline
grava comment $ISSUE_ID -m "PR merged: $PR_URL"
grava commit -m "closed $ISSUE_ID (PR merged)"
PIPELINE_RESULT="PIPELINE_COMPLETE: $ISSUE_ID"
```

## Acceptance Criteria

- `/ship <id>` runs end-to-end for a happy-path issue → `PIPELINE_COMPLETE`
- Phase 2 review loop caps at 3 rounds → `PIPELINE_HALTED` with `needs-human` label
- Phase 3 uses branch `grava/<id>` (not auto-detected) for `gh pr create`
- Phase 4 polls every 30s, detects `MERGED` / `CLOSED` states
- PR comment loop caps at `MAX_PR_FIX_ROUNDS=3` → halts cleanly
- On final success: issue status = `closed`, worktree removed by `grava close`
- All wisp writes snapshot via `grava commit`

## Dependencies

- Stories 2B.1 (coder), 2B.2 (reviewer)
- `gh` CLI authenticated
- Story 2B.9 hook (captures signals → wisps in real time)

## Signals Emitted

- `PR_CREATED: <url>` — Phase 3 complete
- `PR_COMMENTS_RESOLVED: <round>` — coder pushed a fix
- `PR_MERGED` — detected merge in poll
- `PIPELINE_COMPLETE: <id>` — success terminal
- `PIPELINE_HALTED: <reason>` — review or PR-fix loop exhausted
- `PIPELINE_FAILED: <reason>` — PR creation failed or PR closed without merge
