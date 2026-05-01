#!/bin/bash
# Local merge-conflict + smoke-test probe. Returns non-zero on conflict.
# Called: ./scripts/pre-merge-check.sh <issue-id>
# Must be invoked from repo root — first action is cd into worktree (relative path).

set -u
ISSUE_ID="${1:?usage: pre-merge-check.sh <issue-id>}"
WORKTREE=".worktree/$ISSUE_ID"

[ -d "$WORKTREE" ] || { echo "PIPELINE_FAILED: no worktree for $ISSUE_ID"; exit 1; }

cd "$WORKTREE" || exit 1

git fetch origin main >/dev/null 2>&1 || true

BASE=$(git merge-base HEAD origin/main)
CONFLICT=$(git merge-tree "$BASE" HEAD origin/main | grep -c '<<<<<<' || true)

if [ "$CONFLICT" -gt 0 ]; then
  echo "PIPELINE_HALTED: would conflict with main ($CONFLICT files)"
  exit 2
fi

# Best-effort smoke compile against the merged tree
TMP_PROBE="../.merge-probe-$$"
git worktree add --detach "$TMP_PROBE" HEAD >/dev/null 2>&1 || exit 0
(
  cd "$TMP_PROBE" || exit 0
  git merge --no-commit --no-ff origin/main >/dev/null 2>&1 || { echo "merge failed in probe"; exit 3; }
  go build ./... 2>&1 || exit 3
)
PROBE_RC=$?
git worktree remove "$TMP_PROBE" --force >/dev/null 2>&1 || true

[ "$PROBE_RC" -eq 0 ] || { echo "PIPELINE_HALTED: build fails when merged with main"; exit 3; }
echo "pre-merge OK"
exit 0
