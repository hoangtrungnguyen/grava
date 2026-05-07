#!/bin/bash
INPUT=$(cat)
TEAMMATE=$(echo "$INPUT" | jq -r '.teammate_name // empty')
CWD=$(echo "$INPUT" | jq -r '.cwd // empty')

# Only validate code-producing agents
case "$TEAMMATE" in
  coding-agent|fix-agent|qa-agent) ;;
  *) exit 0 ;;
esac

# Run tests in the agent's working directory (worktree)
if [ -n "$CWD" ] && [ -d "$CWD" ]; then
  cd "$CWD" || exit 0
fi

# Detect changed packages for targeted testing
CHANGED_PKGS=$(git diff --name-only main -- '*.go' 2>/dev/null | xargs -I{} dirname {} | sort -u | sed 's|^|./|' | paste -sd ' ' -)

if [ -n "$CHANGED_PKGS" ]; then
  TEST_OUTPUT=$(go test $CHANGED_PKGS 2>&1)
else
  TEST_OUTPUT=$(go test ./... 2>&1)
fi

if [ $? -ne 0 ]; then
  echo "Tests failing. Fix before marking complete." >&2
  echo "$TEST_OUTPUT" | tail -20 >&2
  exit 2
fi
exit 0
