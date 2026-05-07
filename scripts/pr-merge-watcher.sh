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
      grava signal PR_MERGED --issue "$ID" --actor watcher
      grava label "$ID" --remove pr-created
      # grava-63f3: don't emit PIPELINE_COMPLETE if grava close fails for
      # any reason other than "already closed". Otherwise the pipeline
      # reports complete while the issue board still says in_progress.
      if ! grava close "$ID" --actor watcher 2>/dev/null; then
        CURRENT_STATUS=$(grava show "$ID" --json 2>/dev/null | jq -r '.status // ""')
        if [ "$CURRENT_STATUS" != "closed" ]; then
          echo "watcher: failed to close $ID (status=$CURRENT_STATUS) — leaving for next iteration"
          continue
        fi
        # Already closed by hand or by an earlier iteration — proceed.
      fi
      grava signal PIPELINE_COMPLETE --issue "$ID" --payload "$ID" --actor watcher
      grava commit -m "watcher: $ID merged + closed"
      continue
      ;;
    CLOSED)
      # First-time CLOSED detection — distil rejection reason + record on issue.
      # Gated by pr_rejection_recorded wisp so re-runs don't double-write.
      ALREADY_RECORDED=$(grava wisp read "$ID" pr_rejection_recorded 2>/dev/null)
      if [ -z "$ALREADY_RECORDED" ]; then
        REVIEWS_JSON=$(gh pr view "$PR_NUMBER" --json reviews,closedBy,author 2>/dev/null)
        CHANGES_REQUESTED=$(echo "$REVIEWS_JSON" | jq -r '
          [.reviews[]? | select(.state == "CHANGES_REQUESTED") | .body] | join("\n\n---\n\n")
        ' | head -c 4096)
        CLOSED_BY=$(echo "$REVIEWS_JSON" | jq -r '.closedBy.login // "unknown"')
        AUTHOR=$(echo "$REVIEWS_JSON" | jq -r '.author.login // ""')
        LAST_COMMENT=$(gh pr view "$PR_NUMBER" --json comments 2>/dev/null \
          | jq -r '.comments[-1].body // ""' | head -c 1024)

        if [ -n "$CHANGES_REQUESTED" ]; then
          REASON="reviewer-rejected"
        elif [ "$CLOSED_BY" = "$AUTHOR" ]; then
          REASON="author-abandoned"
        else
          REASON="unknown"
        fi

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

        # grava-97ec: guard the description write — if it fails (DB blip,
        # network), defer the rest of the recording until next iteration so
        # the rejection notes aren't silently lost. The pr_rejection_recorded
        # gate is NOT set yet at this point, so re-runs will retry cleanly.
        if ! printf '%s\n' "$NOTES" | grava update "$ID" --description-append-from-stdin; then
          echo "watcher: failed to record rejection notes for $ID — will retry next iteration"
          continue
        fi
        grava comment "$ID" -m "PR closed without merge ($REASON). See description for full notes."

        # Bookkeeping wisps (non-phase): rejection notes blob, close timestamp,
        # idempotency gate. pr_close_reason is intentionally NOT written here
        # — `grava signal PR_CLOSED --payload "$REASON"` below records it
        # atomically alongside pipeline_phase=failed.
        grava wisp write "$ID" pr_rejection_notes "$NOTES"
        grava wisp write "$ID" pr_closed_at "$NOW"
        grava wisp write "$ID" pr_rejection_recorded "1"

        # Atomic: pipeline_phase=failed + pr_close_reason aux wisp in one tx.
        # Scoped inside the first-time block so re-runs (ALREADY_RECORDED=1)
        # don't re-emit the signal with a blank payload, which would overwrite
        # pr_close_reason with "". Phase is already terminal `failed` after the
        # first emission; subsequent watcher iterations are correctly no-op.
        grava signal PR_CLOSED --issue "$ID" --payload "$REASON" --actor watcher
      fi

      grava label "$ID" --add pr-rejected
      grava label "$ID" --remove pr-created
      grava commit -m "watcher: $ID PR closed without merge"
      continue
      ;;
  esac

  # State is OPEN. Check stale cap.
  # grava-6ac8: `grava wisp read` exits 0 even when the wisp is missing
  # (just empty stdout per current CLI behavior), so the `|| echo "$NOW"`
  # fallback never fires. Without an explicit emptiness check, SINCE
  # becomes "" and the arithmetic below errors silently — PRs >72h never
  # get the needs-human label. Default explicitly when SINCE is empty.
  SINCE=$(grava wisp read "$ID" pr_awaiting_merge_since 2>/dev/null)
  [ -n "$SINCE" ] || SINCE="$NOW"
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
    [ -x scripts/hooks/notify-pr-comments.sh ] && scripts/hooks/notify-pr-comments.sh "$ID" "$PR_URL"
  fi
done

exit 0
