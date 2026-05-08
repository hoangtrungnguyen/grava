#!/usr/bin/env bash
# finalize-pr.sh — atomic post-PR bookkeeping for the pr-creator agent.
#
# Usage:
#   scripts/agent-bot/finalize-pr.sh <issue-id> <pr-number> <pr-url>
#
# Runs, in order:
#   1. grava wisp write <id> pr_number <num>
#   2. grava wisp write <id> pr_url <url>
#   3. grava wisp write <id> pr_awaiting_merge_since <unix-ts>
#   4. grava signal PR_CREATED --issue <id> --payload <url> --actor pr-creator
#      (the signal CLI rejects with SIGNAL_PRECONDITION_UNMET if steps 1-3
#      didn't write — defense in depth, see grava-fddd / pkg/cmd/issues/signal.go)
#   5. grava label <id> --add pr-created
#   6. grava commit -m "pr-creator: finalize <id> PR #<num>"
#   7. Self-verify: re-read pipeline_phase, pr_url, label list. Exit 0 only
#      if all match expectations; print a summary either way.
#
# Replaces the prose Step 6/7/8 contract in .claude/agents/pr-creator.md
# (regression tracked in grava-adfb, grava-fddd). Single command means the
# agent cannot "forget" half the steps.
#
# Failure of any step → print which step failed, exit 1.
#
# Environment:
#   GRAVA_BIN — override the grava binary path (default: grava on PATH).
#   Useful for tests that want to inject a mock.

set -uo pipefail

GRAVA_BIN="${GRAVA_BIN:-grava}"

usage() {
  cat >&2 <<'USAGE'
Usage: finalize-pr.sh <issue-id> <pr-number> <pr-url>
USAGE
  exit 2
}

if [ "$#" -ne 3 ]; then
  usage
fi

ISSUE_ID="$1"
PR_NUMBER="$2"
PR_URL="$3"

if [ -z "$ISSUE_ID" ] || [ -z "$PR_NUMBER" ] || [ -z "$PR_URL" ]; then
  usage
fi

NOW=$(date -u +%s)

# step <step-num> <description> <command...> — runs the command and exits 1
# on failure with a clear message. Stdout/stderr from the command are
# preserved so the agent transcript shows what actually went wrong.
step() {
  local n="$1"; local desc="$2"; shift 2
  if ! "$@"; then
    printf 'finalize-pr.sh: step %s (%s) FAILED\n' "$n" "$desc" >&2
    exit 1
  fi
}

# Steps 1-3: bookkeeping wisps the watcher reads. MUST land before step 4
# because `grava signal PR_CREATED` enforces them as preconditions.
step 1 "wisp write pr_number"               "$GRAVA_BIN" wisp write "$ISSUE_ID" pr_number "$PR_NUMBER"
step 2 "wisp write pr_url"                  "$GRAVA_BIN" wisp write "$ISSUE_ID" pr_url "$PR_URL"
step 3 "wisp write pr_awaiting_merge_since" "$GRAVA_BIN" wisp write "$ISSUE_ID" pr_awaiting_merge_since "$NOW"

# Step 4: emit the signal. Advances pipeline_phase to pr_created and
# (re-)writes pr_url as auxiliary. Will fail with SIGNAL_PRECONDITION_UNMET
# if any of steps 1-3 didn't actually persist.
step 4 "signal PR_CREATED" "$GRAVA_BIN" signal PR_CREATED \
  --issue "$ISSUE_ID" --payload "$PR_URL" --actor pr-creator

# Step 5: label so the watcher's poll discovers this issue. Watcher cron
# polls `grava list --label pr-created`; ordering after the wisps prevents
# a race where the watcher fires between the label and the wisp writes
# (grava-6dd0).
step 5 "label --add pr-created" "$GRAVA_BIN" label "$ISSUE_ID" --add pr-created

# Step 6: dolt audit-log snapshot.
step 6 "grava commit" "$GRAVA_BIN" commit -m "pr-creator: finalize $ISSUE_ID PR #$PR_NUMBER"

# Step 7: self-verify by reading state back. Each read is independent; we
# collect ALL mismatches before deciding the verdict so the operator sees
# every gap at once.
PHASE=$("$GRAVA_BIN" wisp read "$ISSUE_ID" pipeline_phase 2>/dev/null || true)
WISP_URL=$("$GRAVA_BIN" wisp read "$ISSUE_ID" pr_url 2>/dev/null || true)
WISP_NUMBER=$("$GRAVA_BIN" wisp read "$ISSUE_ID" pr_number 2>/dev/null || true)
WISP_SINCE=$("$GRAVA_BIN" wisp read "$ISSUE_ID" pr_awaiting_merge_since 2>/dev/null || true)
HAS_LABEL=$("$GRAVA_BIN" show "$ISSUE_ID" --json 2>/dev/null \
  | jq -r '.labels // [] | contains(["pr-created"])' 2>/dev/null || echo "false")

VERIFY_FAIL=""
if [ "$PHASE" != "pr_created" ] && [ "$PHASE" != "pr_awaiting_merge" ]; then
  VERIFY_FAIL="${VERIFY_FAIL:+$VERIFY_FAIL; }pipeline_phase=${PHASE:-<unset>} (expected pr_created)"
fi
if [ "$WISP_URL" != "$PR_URL" ]; then
  VERIFY_FAIL="${VERIFY_FAIL:+$VERIFY_FAIL; }pr_url=${WISP_URL:-<unset>} (expected $PR_URL)"
fi
if [ "$WISP_NUMBER" != "$PR_NUMBER" ]; then
  VERIFY_FAIL="${VERIFY_FAIL:+$VERIFY_FAIL; }pr_number=${WISP_NUMBER:-<unset>} (expected $PR_NUMBER)"
fi
if [ -z "$WISP_SINCE" ]; then
  VERIFY_FAIL="${VERIFY_FAIL:+$VERIFY_FAIL; }pr_awaiting_merge_since unset"
fi
if [ "$HAS_LABEL" != "true" ]; then
  VERIFY_FAIL="${VERIFY_FAIL:+$VERIFY_FAIL; }pr-created label missing"
fi

if [ -n "$VERIFY_FAIL" ]; then
  printf 'finalize-pr.sh: step 7 (self-verify) FAILED: %s\n' "$VERIFY_FAIL" >&2
  exit 1
fi

cat <<SUMMARY
finalize-pr.sh: ✅ $ISSUE_ID PR #$PR_NUMBER finalized
  pipeline_phase = $PHASE
  pr_url         = $WISP_URL
  pr_number      = $WISP_NUMBER
  pr_awaiting_merge_since = $WISP_SINCE
  label pr-created = present
SUMMARY
exit 0
