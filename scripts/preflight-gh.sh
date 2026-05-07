#!/bin/bash
# Pre-flight check for GitHub CLI authentication and required scopes.
# Called by /ship skill before any agent spawn.

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
