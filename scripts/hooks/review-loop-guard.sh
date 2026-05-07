#!/bin/bash
INPUT=$(cat)
SUBJECT=$(echo "$INPUT" | jq -r '.task_subject // empty')
CWD=$(echo "$INPUT" | jq -r '.cwd // empty')
DESCRIPTION=$(echo "$INPUT" | jq -r '.task_description // empty')
ISSUE_ID=$(echo "$SUBJECT" | grep -oE 'grava-[a-f0-9]+(\.[0-9]+)?')
[ -z "$ISSUE_ID" ] && ISSUE_ID=$(echo "$DESCRIPTION" | grep -oE 'grava-[a-f0-9]+(\.[0-9]+)?')

# Ensure grava commands run in the correct project directory
if [ -n "$CWD" ] && [ -d "$CWD" ]; then
  cd "$CWD" || exit 0
fi

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
