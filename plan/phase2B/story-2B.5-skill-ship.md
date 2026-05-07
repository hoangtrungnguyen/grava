# Story 2B.5: /ship Skill — Single-Issue Pipeline

Orchestrator skill that takes one issue through the full pipeline: code → review → PR → handoff. Spawns coder + reviewer + pr-creator agents. After PR creation, ownership transfers to `pr-merge-watcher.sh` (story 2B.12); `/ship` exits — the merge wait and PR-comment fix loop run async.

`/ship` also owns **issue discovery**: when invoked without an ID, it reads the ready queue (`grava ready`), filters to leaf-type work (`task` / `bug`), and picks the top candidate. The previously separate `grava-next-issue` skill is no longer wired into the pipeline — collapsing discover + ship into one entry point eliminates the trigger overlap that fired both skills on the same prompt.

## File

`.claude/skills/ship/SKILL.md`

## Frontmatter

```yaml
---
name: ship
description: "Single-issue pipeline: code → review → PR (handoff to watcher). Use when user says /ship <id>."
user-invocable: true
---
```

## Usage

```
/ship [issue-id]
```

Examples:
- `/ship grava-abc123` — explicit ID, skip discovery
- `/ship` — discover next ready leaf-type issue from queue, then ship it

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

# --retry requires an explicit ID (no auto-discover when retrying — the operator
# is being deliberate about which rejected issue to revive)
if [ "$RETRY" = "1" ] && [ -z "$ISSUE_ID" ]; then
  echo "PIPELINE_FAILED: --retry requires <issue-id>"
  exit 1
fi

# --force bypasses the Phase 0 precondition gate. Allowed with or without an
# explicit ID (auto-pick + force is rare but legal: "I trust whatever's on top").
# The gate's job is to catch malformed specs before spawning the coder; --force
# is the deliberate "I know what I'm doing" override.

# gh preflight before any work
./scripts/preflight-gh.sh || exit 1

# When no ID supplied, fall through to Phase 0 (discover) below.
# When an ID is supplied, validate it exists.
if [ -n "$ISSUE_ID" ]; then
  grava show "$ISSUE_ID" --json >/dev/null 2>&1 || { echo "PIPELINE_FAILED: $ISSUE_ID not found"; exit 1; }
fi
```

The `scripts/preflight-gh.sh` body (kept here for reference; lives in repo per Fix 10):

```bash
#!/bin/bash
if ! gh auth status >/dev/null 2>&1; then
  cat <<EOF >&2
PIPELINE_FAILED: GitHub auth missing
Fix: gh auth login --web --git-protocol https
Or:  gh auth login --with-token < ~/.gh-token
EOF
  exit 1
fi
SCOPES=$(gh auth status 2>&1 | grep -oE "Token scopes: .*" || echo "")
echo "$SCOPES" | grep -q "repo" || { echo "PIPELINE_FAILED: token missing 'repo' scope" >&2; exit 1; }
```

### Re-entry detection

`/ship` may be invoked on an issue already partway through the pipeline (crash recovery, or `pr-merge-watcher` flagged it). Read `pipeline_phase`:

```bash
PHASE=$(grava wisp read "$ISSUE_ID" pipeline_phase 2>/dev/null)
case "$PHASE" in
  pr_awaiting_merge)
    # Watcher has flagged new comments / CHANGES_REQUESTED — jump straight to Phase 4
    NEW_COMMENTS_FLAG=$(grava wisp read "$ISSUE_ID" pr_new_comments 2>/dev/null)
    if [ -n "$NEW_COMMENTS_FLAG" ]; then
      goto_phase4_resume=1
    else
      echo "PIPELINE_INFO: $ISSUE_ID still awaiting merge (no new comments). Watcher will re-flag."
      exit 0
    fi
    ;;
  failed)
    # PR was closed without merge. Watcher recorded rejection notes on the issue
    # description and labelled `pr-rejected` (story 2B.12). Default behavior:
    # halt informational. Operator decides; auto-retry is unsafe (same code →
    # likely same rejection). Explicit `--retry` opts into the retry block.
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
      echo "  /ship $ISSUE_ID --retry --rebase-only — skip review, rebase + re-PR (only if last review was APPROVED but branch went stale)"
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

## Helper — last-line signal parse

Used by every Phase below. Matches signals only when they are the final non-empty line of the agent result.

```bash
last_line() {
  printf '%s' "$1" | awk 'NF{l=$0} END{print l}'
}

parse_signal() {
  # Echoes "<NAME>|<TAIL>" or "INVALID|<reason>"
  # Scope: only the 7 signals /ship itself consumes (CODER_*, REVIEWER_*, PR_CREATED/FAILED).
  # Do NOT extend with PIPELINE_*, BUG_HUNT_*, PLANNER_* — those are emitted BY /ship or by
  # sibling skills, not consumed by /ship. Full protocol: agent-team-implementation-plan.md §4.
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

`grava doctor` flags issues whose `orchestrator_heartbeat` is older than 30 minutes while `pipeline_phase` is non-terminal.

> **Shared writer contract.** `/ship` is one of two heartbeat writers. The other is `grava-dev-task` — see story 2B.0d, which adds inline `orchestrator_heartbeat` writes at every workflow checkpoint inside the skill (RED, GREEN, REFACTOR, validation, commit). Without that, the seed written here would freeze for the entire Phase 1 window and `grava doctor` would false-flag every long-running coder. The two writers cooperate: `/ship` seeds, `grava-dev-task` advances during Phase 1, `/ship` resumes advancing during Phase 2 and Phase 3 iterations.

## Phase 0: Discover + Precondition Gate

Two concerns, run in this order: (1) resolve `$ISSUE_ID` if it wasn't supplied, (2) gate the chosen issue against minimum-spec requirements before spawning any agent. The gate runs for **both** auto-pick and explicit `/ship <id>` — it is bypassable only via `--force`. Rationale: the cost of a bad-spec coder spawn (wasted Agent tokens + a HALT three minutes later from `grava-dev-task` Step 2's spec-presence check) is higher than the cost of one extra `grava show` + jq pass at the orchestrator. Defense-in-depth: gate first, skill second.

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

  # Auto-pick — no prompt, no confirmation. Print the selection (and 2 alts for
  # operator awareness).
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

  # 1. Description must be non-empty
  DESC=$(echo "$TOP_JSON" | jq -r '.description // ""')
  [ -z "$DESC" ] && PRECOND_FAIL="missing description"

  # 2. Acceptance criteria must be present (heuristic: AC heading or checkbox
  #    list in description — grava JSON has no separate `acceptance_criteria`
  #    field today; the regex matches "Acceptance Criteria", "## AC", "## ac",
  #    and unchecked checkboxes "- [ ]" anywhere in the description)
  if [ -z "$PRECOND_FAIL" ]; then
    if ! echo "$DESC" | grep -qiE '(acceptance criteria|## ?ac|^- ?\[ \])'; then
      PRECOND_FAIL="no acceptance criteria"
    fi
  fi

  # 3. No `code_review` label (defensive — ready filter should exclude these
  #    via the open-only status filter, but a label-only edge case shouldn't
  #    spawn a coder)
  if [ -z "$PRECOND_FAIL" ]; then
    LABELS=$(echo "$TOP_JSON" | jq -r '.labels[]? // ""' | tr '\n' ' ')
    echo "$LABELS" | grep -qw "code_review" && PRECOND_FAIL="already has code_review label (work pending review)"
  fi

  if [ -n "$PRECOND_FAIL" ]; then
    echo "PIPELINE_HALTED: $ISSUE_ID failed precondition — $PRECOND_FAIL"
    echo "Operator must intervene. Options:"
    echo "  • Fix the spec on $ISSUE_ID, then re-run /ship $ISSUE_ID"
    if [ "$AUTO_PICKED" = "1" ] && [ "$COUNT" -gt 1 ]; then
      echo "  • Pick a different candidate from this discovery:"
      echo "$CANDIDATES_JSON" | jq -r '.[1:3][] | "      /ship \(.Node.ID)   # \(.Node.Title)"'
    elif [ "$AUTO_PICKED" = "0" ]; then
      echo "  • Pick a different issue: /ship <other-id>"
    fi
    echo "  • Bypass the gate (only if you've verified the spec is acceptable): /ship $ISSUE_ID --force"
    exit 0
  fi
fi
```

Why filter to leaf types: epics and phases never get implemented directly — they spawn child issues. Skipping them avoids handing the coder a non-actionable item.

Why the gate runs on explicit IDs too (not just auto-pick): explicit `/ship <id>` is no longer a "trust the operator" bypass. Rationale: typos and stale memory are common ("I meant grava-abc, not grava-acb") — an explicit ID with a missing description still costs a coder spawn + HALT cycle. The gate check is ~50ms of jq; the override is one extra flag (`--force`) for the cases where the operator genuinely knows the spec is acceptable despite weak heuristics.

Why this is in `/ship`, not a separate skill: the previous architecture had `grava-next-issue` as its own skill that discovered, then `/ship` consumed the result. Two skills triggered on the same user prompt ("ship the next one"), causing trigger overlap and ambiguous wisp ownership. Collapsing into one skill makes the entry point unambiguous and the wisp writer obvious.

## Phase 1: Code

Seed `pipeline_phase` before spawning the coder. `grava-dev-task` only writes the skill-internal `step` wisp; `pipeline_phase` is the orchestrator's vocabulary and `/ship` owns it. Without this seed, `pipeline_phase` stays unset through the entire implementation window, and `grava doctor` cannot flag heartbeat-stale work during the longest phase of the pipeline.

```bash
grava wisp write "$ISSUE_ID" pipeline_phase claimed
```

```
Agent({
  description: "Implement $ISSUE_ID",
  subagent_type: "coder",
  prompt: "Claim and implement issue $ISSUE_ID via grava-dev-task.
           grava-dev-task will pre-check the spec, atomically claim, and provision .worktree/$ISSUE_ID on branch grava/$ISSUE_ID.
           Output CODER_DONE: <sha> or CODER_HALTED: <reason> as the LAST non-empty line."
})
```

Parse the result with `parse_signal "$(last_line "$AGENT_RESULT")"`:
- `CODER_DONE|<sha>` → save SHA, proceed to Phase 2
- `CODER_HALTED|<reason>` → `PIPELINE_HALTED: coder — <reason>`, exit
- `INVALID|<reason>` → `PIPELINE_FAILED: signal parse failed in Phase 1 — <reason>`, exit

**No `isolation` param** — `grava-dev-task` Step 3 calls `grava claim`, which auto-provisions `.worktree/$ISSUE_ID` on branch `grava/$ISSUE_ID`.

## Phase 2: Review (max 3 rounds)

```bash
MAX_REVIEW_ROUNDS=3
APPROVED_SHA=""

for ROUND in $(seq 1 $MAX_REVIEW_ROUNDS); do
  grava wisp write "$ISSUE_ID" orchestrator_heartbeat "$(date -u +%s)"

  # Spawn reviewer (pseudo-code; actual call uses Agent tool)
  REVIEWER_RESULT=$(Agent reviewer "Review issue $ISSUE_ID. Last commit: $LAST_SHA.
    Output REVIEWER_APPROVED or REVIEWER_BLOCKED: <findings> as the LAST non-empty line.")

  PARSED=$(parse_signal "$(last_line "$REVIEWER_RESULT")")
  NAME="${PARSED%%|*}"; TAIL="${PARSED#*|}"

  case "$NAME" in
    REVIEWER_APPROVED)
      APPROVED_SHA="$LAST_SHA"
      break
      ;;
    REVIEWER_BLOCKED)
      grava wisp write "$ISSUE_ID" review_round_$ROUND "blocked"

      # Fix 4 — distil findings: only [CRITICAL] / [HIGH], cap at 2KB
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

      # Fix 4 — pass ROUND to coder so commit footer becomes [round N]
      CODER_RESULT=$(Agent coder "RESUME: true. ROUND: $ROUND. Issue $ISSUE_ID was BLOCKED.
        $RESPAWN_CTX
        Worktree .worktree/$ISSUE_ID and claim already exist — skip the claim step in grava-dev-task and resume at edit/commit.
        Commit message MUST end with [round $ROUND].
        Output CODER_DONE: <sha> or CODER_HALTED: <reason> as the LAST non-empty line.")

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

## Phase 3: Create PR (delegated to pr-creator agent — Fix 9)

The PR template, label list, and reviewer assignment all live in the `pr-creator` agent body (story 2B.14). `/ship` only delegates:

```bash
grava wisp write "$ISSUE_ID" orchestrator_heartbeat "$(date -u +%s)"

PR_RESULT=$(Agent pr-creator "Create PR for $ISSUE_ID.
  Approved SHA: $APPROVED_SHA.
  Output PR_CREATED: <url> or PR_FAILED: <reason> as the LAST non-empty line.")

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

`/ship` does NOT poll for merge. The poll loop (previously inline `while true; sleep 120`) blew the conversation cache. Ownership of the awaiting-merge issue moves to `scripts/pr-merge-watcher.sh` (story 2B.12), which runs via cron / launchd.

```bash
echo "PR_CREATED: $PR_URL"
echo "PIPELINE_HANDOFF: $ISSUE_ID awaiting merge — pr-merge-watcher will track."
exit 0
```

### Re-entry path (only when watcher flags new comments)

When `pr-merge-watcher.sh` detects new PR comments or `CHANGES_REQUESTED`, it sets:

```
grava wisp write <id> pr_new_comments "<json>"
```

…and queues a notification. The user (or a `/resume <id>` skill) re-invokes `/ship <id>`. The Setup section's re-entry detection (above) jumps execution into the Phase-4 resume block below.

```bash
# --- phase4_resume (only when wisp shows new comments) ---
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

# off-scope detection BEFORE re-spawning coder
ORIG_FILES=$(cd ".worktree/$ISSUE_ID" && git diff --name-only "main...grava/$ISSUE_ID" | sort -u)
COMMENT_FILES=$(echo "$NEW_COMMENTS" | jq -r '.[].path' | sort -u)
OUT_OF_SCOPE=$(comm -23 <(echo "$COMMENT_FILES") <(echo "$ORIG_FILES"))
if [ -n "$OUT_OF_SCOPE" ]; then
  grava label "$ISSUE_ID" --add needs-human
  grava wisp write "$ISSUE_ID" pr_off_scope "$OUT_OF_SCOPE"
  echo "PIPELINE_HALTED: PR feedback off-scope — $OUT_OF_SCOPE"
  exit 0
fi

# distil + cap re-spawn prompt
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

CODER_RESULT=$(Agent coder "RESUME: true. ROUND: $FIX_ROUND. Resolve PR comments for $ISSUE_ID.
  $RESPAWN_CTX
  Worktree .worktree/$ISSUE_ID and claim already exist — skip the claim step in grava-dev-task.
  Fix, commit (footer [round $FIX_ROUND]), push to grava/$ISSUE_ID.
  Output CODER_DONE: <sha> or CODER_HALTED: <reason> as the LAST non-empty line.")

PARSED=$(parse_signal "$(last_line "$CODER_RESULT")")
NAME="${PARSED%%|*}"; TAIL="${PARSED#*|}"
case "$NAME" in
  CODER_DONE)
    # Fix 5 — wait for CI before declaring resolved
    if ! gh pr checks "$PR_NUMBER" --watch --fail-fast; then
      grava wisp write "$ISSUE_ID" ci_failed "round $FIX_ROUND"
      FAIL_LOG=$(gh run view --log-failed --json --jq '.jobs[].steps[].log' 2>/dev/null | head -c 2048)
      grava wisp write "$ISSUE_ID" pr_ci_log "$FAIL_LOG"
      # Loop will re-fire when watcher sees CI red as a comment-equivalent signal,
      # OR /ship re-entry adds CI log to FEEDBACK on next round.
      echo "PIPELINE_HALTED: CI failed at round $FIX_ROUND — see wisp pr_ci_log"
      exit 0
    fi
    # Bump pr_last_seen_comment_id BEFORE clearing pr_new_comments so the watcher
    # does not re-detect the same comments on its next cycle. Highest id in the
    # batch is the new "seen" watermark.
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

After eventual merge: `pr-merge-watcher.sh` (story 2B.12) runs `grava close $ISSUE_ID` and writes `pipeline_phase=complete` itself. `/ship` is not involved.

## Phase 5: PR Rejection Retry (--retry flag on failed state)

Entry only when Setup re-entry switch sets `goto_retry_block=1` (i.e. `pipeline_phase=failed` + operator passed `--retry`). The watcher (story 2B.12) has already:

- Distilled the rejection reason into `pr_close_reason` wisp
- Appended a `## PR Rejection Notes (<timestamp>)` section to the issue description (via `grava update --description-append`, story 2B.0c)
- Added label `pr-rejected` and removed `pr-created`

Two sub-modes:

- **Default (`--retry`)** — full re-run from Phase 1. Coder reads the updated issue (rejection notes are now in the description), edits, commits. Then Phase 2 review, Phase 3 PR. Use when reviewer asked for code changes.
- **`--retry --rebase-only`** — skip review (`Phase 2`). Coder rebases onto current `main`, re-pushes branch. Then Phase 3 opens a new PR. Use only when last review was APPROVED but the branch went stale (long-running PR, conflicting merge to main).

```bash
# --- retry block (--retry flag on failed state) ---
MAX_PR_RETRIES=2
RETRY_COUNT=$(grava wisp read "$ISSUE_ID" pr_retry_count 2>/dev/null || echo 0)
if [ "$RETRY_COUNT" -ge "$MAX_PR_RETRIES" ]; then
  grava label "$ISSUE_ID" --add needs-human
  echo "PIPELINE_HALTED: $ISSUE_ID retry cap reached ($MAX_PR_RETRIES). Manual intervention required."
  exit 0
fi

RETRY_COUNT=$((RETRY_COUNT + 1))
grava wisp write "$ISSUE_ID" pr_retry_count "$RETRY_COUNT"

# Read distilled rejection notes (watcher already wrote them to the issue
# description; also stored as wisp for direct prompt injection).
RETRY_FEEDBACK=$(grava wisp read "$ISSUE_ID" pr_rejection_notes 2>/dev/null)
PR_CLOSE_REASON=$(grava wisp read "$ISSUE_ID" pr_close_reason 2>/dev/null)

# Reset state: remove pr-rejected so watcher re-engages on the next PR creation;
# pipeline_phase goes back to claimed (the worktree + branch still exist from
# the original Phase 1 run — no re-claim needed).
grava label "$ISSUE_ID" --remove pr-rejected
grava wisp write "$ISSUE_ID" pipeline_phase claimed
grava wisp write "$ISSUE_ID" orchestrator_heartbeat "$(date -u +%s)"

if [ "$REBASE_ONLY" = "1" ]; then
  # Rebase-only path: coder pulls main, rebases, no logic changes. Reviewer
  # already approved this code; we just need a clean re-PR.
  CODER_RESULT=$(Agent coder "RETRY: true. RETRY_MODE: rebase-only. ROUND: $RETRY_COUNT.
    Issue $ISSUE_ID had a previously approved PR that was closed without merge.
    Reason: $PR_CLOSE_REASON.
    Worktree .worktree/$ISSUE_ID exists with branch grava/$ISSUE_ID.
    Do NOT make code changes. Pull origin/main, rebase grava/$ISSUE_ID onto it,
    resolve conflicts (use simplest merge), commit any conflict resolutions.
    Output CODER_DONE: <sha> or CODER_HALTED: <reason> as the LAST non-empty line.")
else
  # Full retry: coder treats the rejection notes as feedback and re-implements.
  RETRY_BYTES=$(printf '%s' "$RETRY_FEEDBACK" | wc -c)
  RESPAWN_CTX=""
  if [ "$RETRY_BYTES" -gt 2048 ]; then
    RETRY_PATH=".worktree/$ISSUE_ID/.retry-round-$RETRY_COUNT.md"
    printf '%s\n' "$RETRY_FEEDBACK" > "$RETRY_PATH"
    RESPAWN_CTX="REJECTION_NOTES_PATH: $RETRY_PATH (read this file)"
  else
    RESPAWN_CTX="REJECTION_NOTES:\n$RETRY_FEEDBACK"
  fi

  CODER_RESULT=$(Agent coder "RETRY: true. RETRY_MODE: full. ROUND: $RETRY_COUNT.
    Issue $ISSUE_ID had a previously rejected PR. Reason: $PR_CLOSE_REASON.
    The rejection notes are appended to the issue description AND passed inline below.
    $RESPAWN_CTX
    Worktree .worktree/$ISSUE_ID exists. Address the rejection feedback, commit, push.
    Commit message MUST end with [retry $RETRY_COUNT].
    Output CODER_DONE: <sha> or CODER_HALTED: <reason> as the LAST non-empty line.")
fi

PARSED=$(parse_signal "$(last_line "$CODER_RESULT")")
NAME="${PARSED%%|*}"; TAIL="${PARSED#*|}"
case "$NAME" in
  CODER_DONE)
    LAST_SHA="$TAIL"
    APPROVED_SHA="$LAST_SHA"   # rebase-only: skip Phase 2, treat as approved
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

After the retry block sets `goto_phase2=1` or `goto_phase3=1`, control flows back into the existing Phase 2 / Phase 3 logic above. On successful PR re-creation, the new PR is a fresh GitHub PR (the closed one stays closed) — watcher discovers it via the `pr-created` label and resumes normal post-handoff tracking.

## Acceptance Criteria

- `/ship` (no arg) discovers next ready leaf-type issue via `grava ready --json` filter (`task` / `bug` only) and proceeds; empty queue → `PIPELINE_INFO`, exit 0
- Phase 0 auto-picks the top candidate without prompting the operator. Selection + up-to-2 alternates are echoed for awareness; override is "Ctrl-C and re-run as `/ship <id>`". Rationale: `/ship` is the autopilot entry point — interactive prompts break unattended terminal use and parallel-terminal workflows.
- **Phase 0 precondition gate (always-on).** After resolving `$ISSUE_ID` (auto-pick OR explicit), validate: (a) description non-empty, (b) acceptance criteria present (AC heading or checkbox list), (c) no `code_review` label. Any failure → `PIPELINE_HALTED: <id> failed precondition — <reason>` and exit 0. The halt message lists three recovery paths: fix the spec and re-run, pick a different issue (with auto-discovery alternates listed when applicable), or `/ship <id> --force` to bypass. The gate is the cheap (~50ms jq) tier-1 check before the more expensive coder-spawn-then-HALT path; it runs identically for auto-pick and explicit IDs because typos and stale memory affect explicit invocations too. Rationale (over auto-pick-only): explicit-id was previously bypassed on the assumption that operators choose deliberately, but the cost of a malformed-spec coder spawn outweighs the saved jq pass. `--force` exists for the cases where the heuristic is wrong (unusual AC phrasing, intentionally-light spec) — the override is one extra flag, not the default.
- `/ship <id>` validates the supplied ID exists, then runs the precondition gate (same as auto-pick path)
- `/ship <id> --force` bypasses the precondition gate; emits `PIPELINE_INFO: --force set; bypassing precondition gate ...` for audit trail. `--force` is independent of `--retry` — they may be combined (rare; `--retry` already skips Phase 0 by routing through the retry block, so `--retry --force` is redundant but not an error).
- Phase 1 seeds `pipeline_phase=claimed` BEFORE spawning the coder so `grava doctor` can detect heartbeat-stale work during the implementation window. A pre-claim HALT in `grava-dev-task` produces `CODER_HALTED` → hook overwrites the seed to `coding_halted` (terminal); if the agent crashes mid-stream, the seed persists and surfaces as stale-claim — both behaviours intentional.
- `gh` auth missing → `PIPELINE_FAILED` before any agent spawn
- Phase 1/2/3 result parsing uses **last-line-only** matching — body prose with signal substrings does not trigger phase advance
- Phase 2 review loop caps at 3 rounds → `PIPELINE_HALTED` with `needs-human` label
- Phase 2 re-spawn prompts ≤2KB inline OR pass `FINDINGS_PATH`
- Phase 2 re-spawn passes `ROUND: N` so commit footer is `[round N]`
- Phase 3 delegates to `pr-creator` agent — no inline `gh pr create` in `/ship`
- Phase 3 sets `pipeline_phase=pr_awaiting_merge` and exits — no inline poll
- Re-entry path detects `pr_new_comments` wisp and resumes Phase 4 fix loop only on signal
- Phase 4 fix loop caps at `MAX_PR_FIX_ROUNDS=3` → halts cleanly
- Phase 4 detects off-scope feedback (file outside original diff) → halts with `needs-human`
- Phase 4 waits on `gh pr checks --watch --fail-fast` after each coder push
- Phase 4 bumps `pr_last_seen_comment_id` to highest processed comment id before clearing `pr_new_comments` — watcher does not re-detect resolved comments
- Heartbeat wisp `orchestrator_heartbeat` written once per phase iteration
- All wisp writes snapshot via `grava commit` (where appropriate)
- **Re-entry on `pipeline_phase=failed`** without `--retry` → emits `PIPELINE_FAILED` with PR url + close reason + recovery menu, exit 0. No agent spawn.
- **`/ship <id> --retry`** on `pipeline_phase=failed` enters Phase 5 retry block. Reads `pr_close_reason` + `pr_rejection_notes` wisps (written by watcher), bumps `pr_retry_count`, removes `pr-rejected` label, resets `pipeline_phase=claimed`, re-spawns coder with rejection feedback. Caps at `MAX_PR_RETRIES=2`; over-cap → `PIPELINE_HALTED`, `needs-human` label.
- **`/ship <id> --retry --rebase-only`** skips Phase 2 review, jumps coder→Phase 3 directly. Coder spawned with `RETRY_MODE=rebase-only` instruction (no logic changes; pull/rebase/resolve only). Use only when last review was APPROVED but branch went stale. `--rebase-only` without `--retry` → `PIPELINE_FAILED`.
- **`--retry` without explicit `<id>`** → `PIPELINE_FAILED: --retry requires <issue-id>`. (No auto-discover when retrying — the operator must be deliberate about which rejected issue to revive.)
- Retry commit footer: `[retry N]` (mirrors review-round `[round N]` convention).

## Dependencies

- Stories 2B.1 (coder), 2B.2 (reviewer), 2B.14 (pr-creator)
- Story 2B.12 (`pr-merge-watcher.sh`) — owns merge polling
- Story 2B.9 hook (captures signals → wisps)
- `scripts/preflight-gh.sh` (created with this story)
- `gh` CLI authenticated, `jq` on PATH

## Signals Emitted

- `PR_CREATED: <url>` — Phase 3 success
- `PR_COMMENTS_RESOLVED: <round>` — coder pushed a fix in re-entry
- `PIPELINE_HANDOFF: <id> ...` — Phase 4 ownership transferred to watcher
- `PIPELINE_HALTED: <reason>` — Phase 0 precondition fail (auto-pick OR explicit ID, unless `--force`), review/PR-fix loop exhausted, off-scope, or CI failure
- `PIPELINE_FAILED: <reason>` — signal parse failure or PR creation failed
- `PIPELINE_INFO: ...` — re-entry no-op
- `PIPELINE_COMPLETE: <id>` — only emitted when re-entered after watcher already wrote `pipeline_phase=complete`
