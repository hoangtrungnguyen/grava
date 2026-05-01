---
name: ship
description: "Single-issue pipeline: code → review → PR (handoff to watcher). Use when user says /ship <id>."
user-invocable: true
---

# /ship Skill — Single-Issue Pipeline

Orchestrates one issue through the full pipeline: code → review → PR → handoff.
Spawns coder + reviewer + pr-creator agents. After PR creation, ownership transfers
to `scripts/pr-merge-watcher.sh`; `/ship` exits.

## Usage

```
/ship [issue-id] [--retry] [--rebase-only] [--force]
```

Examples:
- `/ship grava-abc123` — explicit ID, skip discovery
- `/ship` — discover next ready leaf-type issue from queue, then ship it
- `/ship grava-abc123 --retry` — retry a previously rejected PR
- `/ship grava-abc123 --retry --rebase-only` — rebase-only retry (branch went stale)
- `/ship grava-abc123 --force` — bypass precondition gate

## Setup

```bash
# Parse positional ID and flags. Order-tolerant.
ISSUE_ID=""
RETRY=0
REBASE_ONLY=0
FORCE=0
for arg in $ARGUMENTS; do
  case "$arg" in
    --retry)        RETRY=1 ;;
    --rebase-only)  REBASE_ONLY=1 ;;
    --force)        FORCE=1 ;;
    --*)            echo "PIPELINE_FAILED: unknown flag $arg"; exit 1 ;;
    *)              [ -z "$ISSUE_ID" ] && ISSUE_ID="$arg" ;;
  esac
done

# --rebase-only requires --retry
if [ "$REBASE_ONLY" = "1" ] && [ "$RETRY" = "0" ]; then
  echo "PIPELINE_FAILED: --rebase-only requires --retry"
  exit 1
fi

# --retry requires an explicit ID (no auto-discover when retrying)
if [ "$RETRY" = "1" ] && [ -z "$ISSUE_ID" ]; then
  echo "PIPELINE_FAILED: --retry requires <issue-id>"
  exit 1
fi

# gh preflight before any work
./scripts/preflight-gh.sh || exit 1

# When an ID is supplied, validate it exists
if [ -n "$ISSUE_ID" ]; then
  grava show "$ISSUE_ID" --json >/dev/null 2>&1 || { echo "PIPELINE_FAILED: $ISSUE_ID not found"; exit 1; }
fi
```

## Helper — last-line signal parse

Used by every Phase below. Matches signals only when they are the final non-empty
line of the agent result.

```bash
last_line() {
  printf '%s' "$1" | awk 'NF{l=$0} END{print l}'
}

parse_signal() {
  # Echoes "<NAME>|<TAIL>" or "INVALID|<reason>"
  local line="$1"
  case "$line" in
    "CODER_DONE: "*)            echo "CODER_DONE|${line#CODER_DONE: }" ;;
    "CODER_HALTED: "*)          echo "CODER_HALTED|${line#CODER_HALTED: }" ;;
    "REVIEWER_APPROVED")        echo "REVIEWER_APPROVED|" ;;
    "REVIEWER_BLOCKED: "*)      echo "REVIEWER_BLOCKED|${line#REVIEWER_BLOCKED: }" ;;
    "REVIEWER_BLOCKED")         echo "REVIEWER_BLOCKED|" ;;
    "PR_CREATED: "*)            echo "PR_CREATED|${line#PR_CREATED: }" ;;
    "PR_FAILED: "*)             echo "PR_FAILED|${line#PR_FAILED: }" ;;
    *)                          echo "INVALID|no signal in last line" ;;
  esac
}
```

## Heartbeat

Once per phase iteration:

```bash
grava wisp write "$ISSUE_ID" orchestrator_heartbeat "$(date -u +%s)"
```

`grava doctor` flags issues whose `orchestrator_heartbeat` is older than 30 minutes
while `pipeline_phase` is non-terminal.

## Re-entry detection

`/ship` may be invoked on an issue already partway through the pipeline. Read `pipeline_phase`:

```bash
PHASE=$(grava wisp read "$ISSUE_ID" pipeline_phase 2>/dev/null)
case "$PHASE" in
  pr_awaiting_merge)
    NEW_COMMENTS_FLAG=$(grava wisp read "$ISSUE_ID" pr_new_comments 2>/dev/null)
    if [ -n "$NEW_COMMENTS_FLAG" ]; then
      goto_phase4_resume=1
    else
      echo "PIPELINE_INFO: $ISSUE_ID still awaiting merge (no new comments). Watcher will re-flag."
      exit 0
    fi
    ;;
  failed)
    if [ "$RETRY" = "1" ]; then
      goto_retry_block=1
    else
      PR_URL=$(grava wisp read "$ISSUE_ID" pr_url 2>/dev/null)
      REASON=$(grava wisp read "$ISSUE_ID" pr_close_reason 2>/dev/null)
      echo "PIPELINE_FAILED: $ISSUE_ID PR closed without merge."
      echo "  PR: ${PR_URL:-<unknown>}"
      echo "  Reason: ${REASON:-unknown} (see issue description for full notes)"
      echo "Recovery options:"
      echo "  /ship $ISSUE_ID --retry              — distil PR feedback, full retry (Phase 1→2→3)"
      echo "  /ship $ISSUE_ID --retry --rebase-only — skip review, rebase + re-PR"
      echo "  grava close $ISSUE_ID --force        — abandon"
      exit 0
    fi
    ;;
  complete)
    echo "PIPELINE_COMPLETE: $ISSUE_ID (already done)"
    exit 0
    ;;
esac
```

## Phase 0: Discover + Precondition Gate

### 0.1 — Discover (only when no ID supplied)

```bash
AUTO_PICKED=0
CANDIDATES_JSON='[]'
COUNT=0

if [ -z "$ISSUE_ID" ]; then
  CANDIDATES_JSON=$(grava ready --limit 10 --json 2>/dev/null \
    | jq '[.[] | select(.Node.Type == "task" or .Node.Type == "bug")]')

  COUNT=$(echo "$CANDIDATES_JSON" | jq 'length')
  if [ "$COUNT" -eq 0 ]; then
    echo "PIPELINE_INFO: ready queue empty (no task/bug). Run /plan or unblock parents first."
    exit 0
  fi

  TOP_ID=$(echo "$CANDIDATES_JSON" | jq -r '.[0].Node.ID')
  TOP_TITLE=$(echo "$CANDIDATES_JSON" | jq -r '.[0].Node.Title')

  echo "PIPELINE_INFO: discovered $COUNT ready leaf issue(s). Auto-selecting top:"
  echo "  → $TOP_ID — $TOP_TITLE"
  echo "$CANDIDATES_JSON" | jq -r '.[1:3][] | "    alt: \(.Node.ID) — \(.Node.Title)"'

  ISSUE_ID="$TOP_ID"
  AUTO_PICKED=1
fi
```

### 0.2 — Precondition gate (always runs unless `--force`)

```bash
if [ "$FORCE" = "1" ]; then
  echo "PIPELINE_INFO: --force set; bypassing precondition gate for $ISSUE_ID"
else
  TOP_JSON=$(grava show "$ISSUE_ID" --json 2>/dev/null)
  PRECOND_FAIL=""

  DESC=$(echo "$TOP_JSON" | jq -r '.description // ""')
  [ -z "$DESC" ] && PRECOND_FAIL="missing description"

  if [ -z "$PRECOND_FAIL" ]; then
    if ! echo "$DESC" | grep -qiE '(acceptance criteria|## ?ac|^- ?\[ \])'; then
      PRECOND_FAIL="no acceptance criteria"
    fi
  fi

  if [ -z "$PRECOND_FAIL" ]; then
    LABELS=$(echo "$TOP_JSON" | jq -r '.labels[]? // ""' | tr '\n' ' ')
    echo "$LABELS" | grep -qw "code_review" && PRECOND_FAIL="already has code_review label (work pending review)"
  fi

  if [ -n "$PRECOND_FAIL" ]; then
    echo "PIPELINE_HALTED: $ISSUE_ID failed precondition — $PRECOND_FAIL"
    echo "Operator must intervene. Options:"
    echo "  • Fix the spec on $ISSUE_ID, then re-run /ship $ISSUE_ID"
    if [ "$AUTO_PICKED" = "1" ] && [ "$COUNT" -gt 1 ]; then
      echo "  • Pick a different candidate:"
      echo "$CANDIDATES_JSON" | jq -r '.[1:3][] | "      /ship \(.Node.ID)   # \(.Node.Title)"'
    elif [ "$AUTO_PICKED" = "0" ]; then
      echo "  • Pick a different issue: /ship <other-id>"
    fi
    echo "  • Bypass the gate (only if you've verified the spec): /ship $ISSUE_ID --force"
    exit 0
  fi
fi
```

## Phase 1: Code

```bash
grava wisp write "$ISSUE_ID" pipeline_phase claimed

# Spawn coder agent (via Agent tool)
Agent({
  description: "Implement $ISSUE_ID",
  subagent_type: "coder",
  prompt: "Claim and implement issue $ISSUE_ID via grava-dev-task.
           grava-dev-task will pre-check the spec, atomically claim, and provision .worktree/$ISSUE_ID on branch grava/$ISSUE_ID.
           Output CODER_DONE: <sha> or CODER_HALTED: <reason> as the LAST non-empty line."
})
```

Parse result with `parse_signal "$(last_line "$AGENT_RESULT")"`:
- `CODER_DONE|<sha>` → save SHA, proceed to Phase 2
- `CODER_HALTED|<reason>` → `PIPELINE_HALTED: coder — <reason>`, exit
- `INVALID|<reason>` → `PIPELINE_FAILED: signal parse failed in Phase 1 — <reason>`, exit

**No `isolation` param** — `grava-dev-task` Step 3 calls `grava claim`, which auto-provisions `.worktree/$ISSUE_ID`.

## Phase 2: Review (max 3 rounds)

```bash
MAX_REVIEW_ROUNDS=3
APPROVED_SHA=""

for ROUND in $(seq 1 $MAX_REVIEW_ROUNDS); do
  grava wisp write "$ISSUE_ID" orchestrator_heartbeat "$(date -u +%s)"

  Agent({
    description: "Review $ISSUE_ID round $ROUND",
    subagent_type: "reviewer",
    prompt: "Review issue $ISSUE_ID. Last commit: $LAST_SHA.
             Output REVIEWER_APPROVED or REVIEWER_BLOCKED: <findings> as the LAST non-empty line."
  })

  PARSED=$(parse_signal "$(last_line "$REVIEWER_RESULT")")
  NAME="${PARSED%%|*}"; TAIL="${PARSED#*|}"

  case "$NAME" in
    REVIEWER_APPROVED)
      APPROVED_SHA="$LAST_SHA"
      break
      ;;
    REVIEWER_BLOCKED)
      grava wisp write "$ISSUE_ID" review_round_$ROUND "blocked"

      FINDINGS=$(grava show "$ISSUE_ID" --json | \
        jq -r '.comments | map(select(.message | startswith("[CRITICAL]") or startswith("[HIGH]"))) | .[].message')
      FINDINGS_BYTES=$(printf '%s' "$FINDINGS" | wc -c)

      RESPAWN_CTX=""
      if [ "$FINDINGS_BYTES" -gt 2048 ]; then
        FINDINGS_PATH=".worktree/$ISSUE_ID/.review-round-$ROUND.md"
        mkdir -p "$(dirname "$FINDINGS_PATH")"
        printf '%s\n' "$FINDINGS" > "$FINDINGS_PATH"
        RESPAWN_CTX="FINDINGS_PATH: $FINDINGS_PATH (read this file)"
      else
        RESPAWN_CTX="FINDINGS:\n$FINDINGS"
      fi

      Agent({
        description: "Fix $ISSUE_ID review round $ROUND",
        subagent_type: "coder",
        prompt: "RESUME: true. ROUND: $ROUND. Issue $ISSUE_ID was BLOCKED.
          $RESPAWN_CTX
          Worktree .worktree/$ISSUE_ID and claim already exist — skip the claim step in grava-dev-task and resume at edit/commit.
          Commit message MUST end with [round $ROUND].
          Output CODER_DONE: <sha> or CODER_HALTED: <reason> as the LAST non-empty line."
      })

      PARSED=$(parse_signal "$(last_line "$CODER_RESULT")")
      NAME="${PARSED%%|*}"; TAIL="${PARSED#*|}"
      case "$NAME" in
        CODER_DONE)   LAST_SHA="$TAIL"; continue ;;
        CODER_HALTED) echo "PIPELINE_HALTED: coder halted at review round $ROUND — $TAIL"; exit 0 ;;
        *)            echo "PIPELINE_FAILED: signal parse failed in Phase 2 round $ROUND"; exit 1 ;;
      esac
      ;;
    *)
      echo "PIPELINE_FAILED: signal parse failed in Phase 2 round $ROUND — $TAIL"
      exit 1
      ;;
  esac
done

if [ -z "$APPROVED_SHA" ]; then
  grava wisp write "$ISSUE_ID" pipeline_halted "review loop exhausted ($MAX_REVIEW_ROUNDS rounds)"
  grava label "$ISSUE_ID" --add needs-human
  grava stop "$ISSUE_ID"
  echo "PIPELINE_HALTED: $ISSUE_ID needs human review"
  exit 0
fi
```

## Phase 3: Create PR (delegated to pr-creator agent)

```bash
grava wisp write "$ISSUE_ID" orchestrator_heartbeat "$(date -u +%s)"

Agent({
  description: "Create PR for $ISSUE_ID",
  subagent_type: "pr-creator",
  prompt: "Create PR for $ISSUE_ID.
    Approved SHA: $APPROVED_SHA.
    Output PR_CREATED: <url> or PR_FAILED: <reason> as the LAST non-empty line."
})

PARSED=$(parse_signal "$(last_line "$PR_RESULT")")
NAME="${PARSED%%|*}"; TAIL="${PARSED#*|}"
case "$NAME" in
  PR_CREATED)
    PR_URL="$TAIL"
    grava wisp write "$ISSUE_ID" pipeline_phase pr_awaiting_merge
    ;;
  PR_FAILED)
    grava label "$ISSUE_ID" --add pr-failed
    echo "PIPELINE_FAILED: pr creation failed — $TAIL"
    exit 1
    ;;
  *)
    echo "PIPELINE_FAILED: signal parse failed in Phase 3 — $TAIL"
    exit 1
    ;;
esac
```

## Phase 4: Handoff to async watcher

```bash
echo "PR_CREATED: $PR_URL"
echo "PIPELINE_HANDOFF: $ISSUE_ID awaiting merge — pr-merge-watcher will track."
exit 0
```

### Re-entry path (when watcher flags new comments)

When `pr-merge-watcher.sh` detects new PR comments or `CHANGES_REQUESTED`, it sets
`grava wisp write <id> pr_new_comments "<json>"`. Re-entry from Setup jumps here.

```bash
# --- phase4_resume ---
MAX_PR_FIX_ROUNDS=3
FIX_ROUND=$(grava wisp read "$ISSUE_ID" pr_fix_round 2>/dev/null || echo 0)
PR_NUMBER=$(grava wisp read "$ISSUE_ID" pr_number)
PR_URL=$(grava wisp read "$ISSUE_ID" pr_url)
NEW_COMMENTS=$(grava wisp read "$ISSUE_ID" pr_new_comments)

FIX_ROUND=$((FIX_ROUND + 1))
if [ "$FIX_ROUND" -gt "$MAX_PR_FIX_ROUNDS" ]; then
  grava wisp write "$ISSUE_ID" pr_fix_exhausted "$MAX_PR_FIX_ROUNDS rounds"
  grava label "$ISSUE_ID" --add needs-human
  echo "PIPELINE_HALTED: PR comment fix loop exhausted ($MAX_PR_FIX_ROUNDS rounds)"
  exit 0
fi
grava wisp write "$ISSUE_ID" pr_fix_round "$FIX_ROUND"

# Off-scope detection BEFORE re-spawning coder
ORIG_FILES=$(cd ".worktree/$ISSUE_ID" && git diff --name-only "main...grava/$ISSUE_ID" | sort -u)
COMMENT_FILES=$(echo "$NEW_COMMENTS" | jq -r '.[].path' | sort -u)
OUT_OF_SCOPE=$(comm -23 <(echo "$COMMENT_FILES") <(echo "$ORIG_FILES"))
if [ -n "$OUT_OF_SCOPE" ]; then
  grava label "$ISSUE_ID" --add needs-human
  grava wisp write "$ISSUE_ID" pr_off_scope "$OUT_OF_SCOPE"
  echo "PIPELINE_HALTED: PR feedback off-scope — $OUT_OF_SCOPE"
  exit 0
fi

FEEDBACK=$(echo "$NEW_COMMENTS" | jq -r '.[] | "[\(.path):\(.line // .original_line)] \(.body)"')
FEEDBACK_BYTES=$(printf '%s' "$FEEDBACK" | wc -c)
RESPAWN_CTX=""
if [ "$FEEDBACK_BYTES" -gt 2048 ]; then
  FEEDBACK_PATH=".worktree/$ISSUE_ID/.pr-round-$FIX_ROUND.md"
  printf '%s\n' "$FEEDBACK" > "$FEEDBACK_PATH"
  RESPAWN_CTX="FINDINGS_PATH: $FEEDBACK_PATH"
else
  RESPAWN_CTX="FEEDBACK:\n$FEEDBACK"
fi

Agent({
  description: "Fix PR comments $ISSUE_ID round $FIX_ROUND",
  subagent_type: "coder",
  prompt: "RESUME: true. ROUND: $FIX_ROUND. Resolve PR comments for $ISSUE_ID.
    $RESPAWN_CTX
    Worktree .worktree/$ISSUE_ID and claim already exist — skip the claim step in grava-dev-task.
    Fix, commit (footer [round $FIX_ROUND]), push to grava/$ISSUE_ID.
    Output CODER_DONE: <sha> or CODER_HALTED: <reason> as the LAST non-empty line."
})

PARSED=$(parse_signal "$(last_line "$CODER_RESULT")")
NAME="${PARSED%%|*}"; TAIL="${PARSED#*|}"
case "$NAME" in
  CODER_DONE)
    # Wait for CI before declaring resolved
    if ! gh pr checks "$PR_NUMBER" --watch --fail-fast; then
      grava wisp write "$ISSUE_ID" ci_failed "round $FIX_ROUND"
      FAIL_LOG=$(gh run view --log-failed --json --jq '.jobs[].steps[].log' 2>/dev/null | head -c 2048)
      grava wisp write "$ISSUE_ID" pr_ci_log "$FAIL_LOG"
      echo "PIPELINE_HALTED: CI failed at round $FIX_ROUND — see wisp pr_ci_log"
      exit 0
    fi
    # Bump watermark BEFORE clearing pr_new_comments so watcher doesn't re-detect
    HIGHEST_COMMENT_ID=$(echo "$NEW_COMMENTS" | jq -r '[.[].id] | max // empty')
    if [ -n "$HIGHEST_COMMENT_ID" ]; then
      grava wisp write "$ISSUE_ID" pr_last_seen_comment_id "$HIGHEST_COMMENT_ID"
    fi
    grava wisp write "$ISSUE_ID" pr_comments_resolved "round $FIX_ROUND"
    grava wisp write "$ISSUE_ID" pipeline_phase pr_awaiting_merge
    grava wisp delete "$ISSUE_ID" pr_new_comments
    echo "PR_COMMENTS_RESOLVED: $FIX_ROUND"
    echo "PIPELINE_HANDOFF: $ISSUE_ID re-armed — watcher will re-track."
    exit 0
    ;;
  CODER_HALTED)
    grava wisp write "$ISSUE_ID" pr_comments_halted "round $FIX_ROUND: $TAIL"
    grava label "$ISSUE_ID" --add needs-human
    echo "PIPELINE_HALTED: coder could not resolve PR comments at round $FIX_ROUND"
    exit 0
    ;;
esac
```

## Phase 5: PR Rejection Retry (`--retry` flag on `failed` state)

Entry only when Setup re-entry switch sets `goto_retry_block=1`.

```bash
# --- retry block ---
MAX_PR_RETRIES=2
RETRY_COUNT=$(grava wisp read "$ISSUE_ID" pr_retry_count 2>/dev/null || echo 0)
if [ "$RETRY_COUNT" -ge "$MAX_PR_RETRIES" ]; then
  grava label "$ISSUE_ID" --add needs-human
  echo "PIPELINE_HALTED: $ISSUE_ID retry cap reached ($MAX_PR_RETRIES). Manual intervention required."
  exit 0
fi

RETRY_COUNT=$((RETRY_COUNT + 1))
grava wisp write "$ISSUE_ID" pr_retry_count "$RETRY_COUNT"

RETRY_FEEDBACK=$(grava wisp read "$ISSUE_ID" pr_rejection_notes 2>/dev/null)
PR_CLOSE_REASON=$(grava wisp read "$ISSUE_ID" pr_close_reason 2>/dev/null)

grava label "$ISSUE_ID" --remove pr-rejected
grava wisp write "$ISSUE_ID" pipeline_phase claimed
grava wisp write "$ISSUE_ID" orchestrator_heartbeat "$(date -u +%s)"

if [ "$REBASE_ONLY" = "1" ]; then
  Agent({
    description: "Rebase $ISSUE_ID retry $RETRY_COUNT",
    subagent_type: "coder",
    prompt: "RETRY: true. RETRY_MODE: rebase-only. ROUND: $RETRY_COUNT.
      Issue $ISSUE_ID had a previously approved PR that was closed without merge.
      Reason: $PR_CLOSE_REASON.
      Worktree .worktree/$ISSUE_ID exists with branch grava/$ISSUE_ID.
      Do NOT make code changes. Pull origin/main, rebase grava/$ISSUE_ID onto it,
      resolve conflicts (use simplest merge), commit any conflict resolutions.
      Output CODER_DONE: <sha> or CODER_HALTED: <reason> as the LAST non-empty line."
  })
else
  RETRY_BYTES=$(printf '%s' "$RETRY_FEEDBACK" | wc -c)
  RESPAWN_CTX=""
  if [ "$RETRY_BYTES" -gt 2048 ]; then
    RETRY_PATH=".worktree/$ISSUE_ID/.retry-round-$RETRY_COUNT.md"
    printf '%s\n' "$RETRY_FEEDBACK" > "$RETRY_PATH"
    RESPAWN_CTX="REJECTION_NOTES_PATH: $RETRY_PATH (read this file)"
  else
    RESPAWN_CTX="REJECTION_NOTES:\n$RETRY_FEEDBACK"
  fi

  Agent({
    description: "Retry $ISSUE_ID round $RETRY_COUNT",
    subagent_type: "coder",
    prompt: "RETRY: true. RETRY_MODE: full. ROUND: $RETRY_COUNT.
      Issue $ISSUE_ID had a previously rejected PR. Reason: $PR_CLOSE_REASON.
      The rejection notes are appended to the issue description AND passed inline below.
      $RESPAWN_CTX
      Worktree .worktree/$ISSUE_ID exists. Address the rejection feedback, commit, push.
      Commit message MUST end with [retry $RETRY_COUNT].
      Output CODER_DONE: <sha> or CODER_HALTED: <reason> as the LAST non-empty line."
  })
fi

PARSED=$(parse_signal "$(last_line "$CODER_RESULT")")
NAME="${PARSED%%|*}"; TAIL="${PARSED#*|}"
case "$NAME" in
  CODER_DONE)
    LAST_SHA="$TAIL"
    APPROVED_SHA="$LAST_SHA"
    if [ "$REBASE_ONLY" = "0" ]; then
      goto_phase2=1   # full retry: run review loop
    else
      goto_phase3=1   # rebase-only: straight to PR creation
    fi
    ;;
  CODER_HALTED)
    grava label "$ISSUE_ID" --add needs-human
    echo "PIPELINE_HALTED: retry coder halted at round $RETRY_COUNT — $TAIL"
    exit 0
    ;;
  *)
    echo "PIPELINE_FAILED: signal parse failed in retry block — $TAIL"
    exit 1
    ;;
esac
```

After the retry block sets `goto_phase2=1` or `goto_phase3=1`, control flows back
into Phase 2 / Phase 3. After eventual merge, `pr-merge-watcher.sh` runs
`grava close $ISSUE_ID` and writes `pipeline_phase=complete`.

## Signals Emitted

- `PR_CREATED: <url>` — Phase 3 success
- `PR_COMMENTS_RESOLVED: <round>` — coder pushed a fix in re-entry
- `PIPELINE_HANDOFF: <id> ...` — Phase 4 ownership transferred to watcher
- `PIPELINE_HALTED: <reason>` — precondition fail, loop exhausted, off-scope, or CI failure
- `PIPELINE_FAILED: <reason>` — signal parse failure or PR creation failed
- `PIPELINE_INFO: ...` — re-entry no-op or discovery announcement
- `PIPELINE_COMPLETE: <id>` — re-entered after watcher wrote `pipeline_phase=complete`
