#!/bin/bash
# PostToolUse hook — captures pipeline signals in Bash output and
# syncs them to grava wisps for crash recovery.
# Last-line-only parse + forward-only phase order (terminal states overwrite).

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
OUTPUT=$(echo "$INPUT" | jq -r '.tool_output // empty')
CWD=$(echo "$INPUT" | jq -r '.cwd // empty')
TOOL_CMD=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# Only process Bash tool output
[ "$TOOL_NAME" = "Bash" ] || exit 0
[ -n "$OUTPUT" ] || exit 0

# Cheap pre-filter — skip output that obviously has no signal
echo "$OUTPUT" | grep -qE '(CODER_(DONE|HALTED)|REVIEWER_(APPROVED|BLOCKED)|PR_(CREATED|COMMENTS_RESOLVED|MERGED|FAILED)|PIPELINE_(COMPLETE|HALTED|FAILED)|PLANNER_NEEDS_INPUT)' || exit 0

# Resolve ISSUE_ID from cwd if we're inside a grava worktree.
ISSUE_ID=""
if [[ "$CWD" =~ /\.worktree/([a-zA-Z0-9_-]+)(/|$) ]]; then
  ISSUE_ID="${BASH_REMATCH[1]}"
fi

# Fallback 1: extract from explicit `grava <cmd> <id>` in the bash command
if [ -z "$ISSUE_ID" ] && [ -n "$TOOL_CMD" ]; then
  ISSUE_ID=$(echo "$TOOL_CMD" | grep -oE 'grava[[:space:]]+(claim|show|comment|label|wisp|stop|close|commit)[[:space:]]+[a-zA-Z0-9_-]+' | head -1 | awk '{print $3}')
fi

# Fallback 2: scan output for embedded grava-id
if [ -z "$ISSUE_ID" ]; then
  ISSUE_ID=$(echo "$OUTPUT" | grep -oE 'grava-[a-zA-Z0-9]{4,}' | head -1)
fi

# Nothing to sync — exit silently
[ -n "$ISSUE_ID" ] || exit 0

# Fix 1: last-line-only parse — signals must be the final non-empty line
LAST_LINE=$(printf '%s' "$OUTPUT" | awk 'NF{l=$0} END{print l}')

NEW_PHASE=""
case "$LAST_LINE" in
  "CODER_DONE: "*)            NEW_PHASE="coding_complete" ;;
  "CODER_HALTED: "*)          NEW_PHASE="coding_halted" ;;
  "REVIEWER_APPROVED")        NEW_PHASE="review_approved" ;;
  "REVIEWER_BLOCKED: "*|"REVIEWER_BLOCKED") NEW_PHASE="review_blocked" ;;
  "PR_CREATED: "*)            NEW_PHASE="pr_created" ;;
  "PR_FAILED: "*)             NEW_PHASE="failed" ;;
  "PR_COMMENTS_RESOLVED: "*)  NEW_PHASE="pr_comments_resolved" ;;
  "PR_MERGED")                NEW_PHASE="pr_merged" ;;
  "PIPELINE_COMPLETE: "*)     NEW_PHASE="complete" ;;
  "PIPELINE_HALTED: "*)       NEW_PHASE="halted_human_needed" ;;
  "PIPELINE_FAILED: "*)       NEW_PHASE="failed" ;;
  "PLANNER_NEEDS_INPUT: "*)   NEW_PHASE="planner_needs_input" ;;
  *) exit 0 ;;
esac

# Fix 8: idempotent forward-only write
PHASE_ORDER=(claimed coding_complete review_blocked review_approved pr_created pr_awaiting_merge pr_comments_resolved pr_merged complete)
idx_of() {
  local target="$1" i=0
  for p in "${PHASE_ORDER[@]}"; do
    [ "$p" = "$target" ] && { echo "$i"; return; }
    i=$((i+1))
  done
  echo -1
}

CURRENT=$(grava wisp read "$ISSUE_ID" pipeline_phase 2>/dev/null)
WRITE=0
case "$NEW_PHASE" in
  failed|halted_human_needed|coding_halted|planner_needs_input)
    # Terminal / out-of-band states overwrite anything
    WRITE=1
    ;;
  *)
    NEW_IDX=$(idx_of "$NEW_PHASE")
    CUR_IDX=$(idx_of "$CURRENT")
    # Allow forward moves. Unknown CURRENT (-1) → allow.
    if [ "$NEW_IDX" -ge 0 ] && { [ "$CUR_IDX" -lt 0 ] || [ "$NEW_IDX" -gt "$CUR_IDX" ]; }; then
      WRITE=1
    fi
    ;;
esac

if [ "$WRITE" -eq 1 ]; then
  grava wisp write "$ISSUE_ID" pipeline_phase "$NEW_PHASE" 2>/dev/null
fi

exit 0
