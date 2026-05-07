#!/bin/bash
INPUT=$(cat)
TEAMMATE=$(echo "$INPUT" | jq -r '.teammate_name // empty')
CWD=$(echo "$INPUT" | jq -r '.cwd // empty')

# Ensure grava commands run in the correct project directory
if [ -n "$CWD" ] && [ -d "$CWD" ]; then
  cd "$CWD" || exit 0
fi

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
