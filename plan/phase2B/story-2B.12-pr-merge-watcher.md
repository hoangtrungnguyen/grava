# Story 2B.12: PR Merge Watcher (async)

Async cron / launchd script that owns the **post-handoff** phase of the pipeline. `/ship` exits after PR creation (story 2B.5 Phase 3) and writes `pipeline_phase=pr_awaiting_merge`. This watcher polls outside any Claude Code conversation, so the merge wait does not bloat conversation context (the previous inline `while true; sleep 120` blew the 5-minute prompt cache after ~30 minutes of polling).

## File

`scripts/pr-merge-watcher.sh`

## Run cadence

- Every 5 minutes via `cron` (Linux) or `launchd` (macOS).
- One process per repo. PID file at `.grava/pr-merge-watcher.pid` to prevent overlap.

## Inputs

Reads from grava DB:
- `grava list --label pr-created --json` — issues currently awaiting merge
- Per-issue wisps: `pr_number`, `pr_url`, `pr_awaiting_merge_since`, `pr_last_seen_comment_id`

Writes to grava DB on state transitions:
- `pr_new_comments` — JSON of unseen comments (signals `/ship` re-entry)
- `pr_last_seen_comment_id` — high-water mark
- `pr_merged_at` — timestamp on merge
- `pr_stale` — set after `MAX_PR_WAIT_HOURS=72`
- `pipeline_phase` — `pr_merged` or `complete` (after `grava close`)

## Logic

```bash
#!/bin/bash
# scripts/pr-merge-watcher.sh — async PR merge tracker.
# Run via cron every 5 min: */5 * * * * cd /path/to/repo && ./scripts/pr-merge-watcher.sh

set -u

REPO_ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
cd "$REPO_ROOT" || exit 1

PIDFILE=".grava/pr-merge-watcher.pid"
mkdir -p .grava
if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  exit 0    # previous run still active
fi
echo $$ > "$PIDFILE"
trap 'rm -f "$PIDFILE"' EXIT

MAX_PR_WAIT_HOURS=72
NOW=$(date -u +%s)

ISSUES=$(grava list --label pr-created --json 2>/dev/null)
[ -n "$ISSUES" ] || exit 0

echo "$ISSUES" | jq -r '.[].id' | while read -r ID; do
  PR_NUMBER=$(grava wisp read "$ID" pr_number 2>/dev/null)
  PR_URL=$(grava wisp read "$ID" pr_url 2>/dev/null)
  [ -n "$PR_NUMBER" ] || continue

  STATE=$(gh pr view "$PR_NUMBER" --json state -q '.state' 2>/dev/null)

  case "$STATE" in
    MERGED)
      grava wisp write "$ID" pr_merged_at "$NOW"
      grava wisp write "$ID" pipeline_phase pr_merged
      grava label "$ID" --remove pr-created
      grava close "$ID" --actor watcher
      grava wisp write "$ID" pipeline_phase complete
      grava commit -m "watcher: $ID merged + closed"
      continue
      ;;
    CLOSED)
      # First-time CLOSED detection — distil rejection reason + record on issue.
      # Gated by `pr_rejection_recorded` wisp so re-runs of the watcher don't
      # double-write the description.
      ALREADY_RECORDED=$(grava wisp read "$ID" pr_rejection_recorded 2>/dev/null)
      if [ -z "$ALREADY_RECORDED" ]; then
        # Pull review history + closer identity. `gh pr view --json` returns
        # arrays even when empty, so jq is safe.
        REVIEWS_JSON=$(gh pr view "$PR_NUMBER" --json reviews,closedBy,author 2>/dev/null)
        CHANGES_REQUESTED=$(echo "$REVIEWS_JSON" | jq -r '
          [.reviews[]? | select(.state == "CHANGES_REQUESTED") | .body] | join("\n\n---\n\n")
        ' | head -c 4096)
        CLOSED_BY=$(echo "$REVIEWS_JSON" | jq -r '.closedBy.login // "unknown"')
        AUTHOR=$(echo "$REVIEWS_JSON" | jq -r '.author.login // ""')
        LAST_COMMENT=$(gh pr view "$PR_NUMBER" --json comments 2>/dev/null \
          | jq -r '.comments[-1].body // ""' | head -c 1024)

        # Categorize. Order matters: reviewer veto outranks self-close.
        if [ -n "$CHANGES_REQUESTED" ]; then
          REASON="reviewer-rejected"
        elif [ "$CLOSED_BY" = "$AUTHOR" ]; then
          REASON="author-abandoned"
        else
          REASON="unknown"
        fi

        # Compose the rejection note. Markdown so it renders in any future
        # `grava show` viewer.
        STAMP=$(date -u +%FT%TZ)
        NOTES=$(cat <<EOF

## PR Rejection Notes ($STAMP)

PR: $PR_URL
Closed by: $CLOSED_BY
Reason category: $REASON

### Reviewer feedback (CHANGES_REQUESTED bodies)
${CHANGES_REQUESTED:-_none recorded_}

### Closing comment
${LAST_COMMENT:-_none_}
EOF
)

        # Write to issue description (story 2B.0c provides --description-append).
        # Also drop a chronological comment for visibility in any timeline view.
        printf '%s\n' "$NOTES" | grava update "$ID" --description-append-from-stdin
        grava comment "$ID" -m "PR closed without merge ($REASON). See description for full notes."

        # Persist for the /ship --retry flow (Phase 5).
        grava wisp write "$ID" pr_rejection_notes "$NOTES"
        grava wisp write "$ID" pr_close_reason "$REASON"
        grava wisp write "$ID" pr_closed_at "$NOW"
        grava wisp write "$ID" pr_rejection_recorded "1"
      fi

      grava wisp write "$ID" pipeline_phase failed
      grava label "$ID" --add pr-rejected
      grava label "$ID" --remove pr-created
      grava commit -m "watcher: $ID PR closed without merge"
      continue
      ;;
  esac

  # State is OPEN. Check stale cap.
  SINCE=$(grava wisp read "$ID" pr_awaiting_merge_since 2>/dev/null || echo "$NOW")
  AGE_HRS=$(( (NOW - SINCE) / 3600 ))
  if [ "$AGE_HRS" -ge "$MAX_PR_WAIT_HOURS" ]; then
    grava wisp write "$ID" pr_stale "true"
    grava label "$ID" --add needs-human
    grava commit -m "watcher: $ID stale (>${MAX_PR_WAIT_HOURS}h)"
    continue
  fi

  # Check for new review comments + CHANGES_REQUESTED
  COMMENTS_JSON=$(gh api "repos/{owner}/{repo}/pulls/$PR_NUMBER/comments" 2>/dev/null)
  LAST_SEEN=$(grava wisp read "$ID" pr_last_seen_comment_id 2>/dev/null || echo 0)
  NEW=$(echo "$COMMENTS_JSON" | jq -c --argjson last "$LAST_SEEN" '
    [.[] | select(.in_reply_to_id == null) | select(.id > $last)]
  ')
  NEW_COUNT=$(echo "$NEW" | jq 'length')
  REVIEW_DECISION=$(gh pr view "$PR_NUMBER" --json reviewDecision -q '.reviewDecision' 2>/dev/null)

  if [ "$NEW_COUNT" -gt 0 ] || [ "$REVIEW_DECISION" = "CHANGES_REQUESTED" ]; then
    HIGHEST=$(echo "$COMMENTS_JSON" | jq -r '[.[].id] | max // 0')
    grava wisp write "$ID" pr_new_comments "$NEW"
    grava wisp write "$ID" pr_last_seen_comment_id "$HIGHEST"
    grava commit -m "watcher: $ID new PR comments ($NEW_COUNT)"
    # Best-effort notify hook (no-op if not configured)
    [ -x scripts/hooks/notify-pr-comments.sh ] && scripts/hooks/notify-pr-comments.sh "$ID" "$PR_URL"
  fi
done

exit 0
```

## Re-entry contract with /ship

When `pr_new_comments` is set on an issue, the user (or `/resume <id>` skill, or a notification hook) re-invokes `/ship <id>`. The /ship Setup section detects `pipeline_phase=pr_awaiting_merge` + non-empty `pr_new_comments` and jumps directly to the Phase-4 resume block (story 2B.5).

## Install

cron line (every 5 min):

```cron
*/5 * * * * cd /path/to/grava && ./scripts/pr-merge-watcher.sh >> .grava/watcher.log 2>&1
```

launchd plist (macOS): `~/Library/LaunchAgents/dev.grava.pr-merge-watcher.plist`. Suggested `StartInterval = 300`. Working directory = repo root.

A `make watcher-install` target should exist (out of scope for this story; track separately).

## Acceptance Criteria

- Single-instance: pidfile prevents overlapping runs
- `MERGED` PR → `grava close --actor watcher` runs, `pipeline_phase=complete`, label `pr-created` removed
- `CLOSED` PR (no merge) → `pipeline_phase=failed`, label `pr-rejected` added
- Open PR with new comments → writes `pr_new_comments` + bumps `pr_last_seen_comment_id` (atomic — full JSON, not just count)
- `MAX_PR_WAIT_HOURS=72` exceeded → labels `needs-human`, writes `pr_stale=true`
- No `/ship` invocation occurs from inside the watcher — it only writes wisps; the conversation re-entry is human-driven (or hook-driven)
- Empty `grava list --label pr-created` → script exits in <1s

## Dependencies

- `gh` CLI authenticated
- `jq`, `grava` on PATH
- Story 2B.5 Phase 3 wisp contract (`pr_number`, `pr_url`, `pr_awaiting_merge_since`)
- (Optional) `scripts/hooks/notify-pr-comments.sh` for desktop / Telegram notifications

## Test Plan

- Open PR for an issue → wisp `pipeline_phase=pr_awaiting_merge`, run watcher, no state change
- Merge PR via gh → next watcher run closes issue, writes `complete`
- Close PR without merge → next watcher run writes `failed`
- Add PR review comment → next run writes `pr_new_comments` JSON
- Set fake `pr_awaiting_merge_since` to 73h ago → next run labels `needs-human`
- Two watcher invocations within 1 second → second exits silently (pidfile)

## Signals (external)

The watcher writes wisps; it does **not** emit pipeline signals to Claude Code (no agent context to emit into). Signal-equivalents flow back into `/ship` re-entry via wisp values:

- `pr_new_comments` set → re-entry trigger
- `pipeline_phase=pr_merged` → re-entry sees "already done"
- `pipeline_phase=failed` + `pr-rejected` label → re-entry exits informational
