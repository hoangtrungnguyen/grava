# Story 2B.13: Pre-Merge Cross-Branch Regression Check

Catches "two branches green alone, broken when merged" before the PR is mergeable. With N parallel terminals modifying overlapping code, each branch passes its own CI but the merge breaks main. Two-pronged: a local merge-tree probe inside `/ship` (or the watcher) and a GitHub Action that runs the full test suite against the merged-with-main result.

## Files

- `scripts/pre-merge-check.sh` — local probe; called from `/ship` Phase 3 setup and from `pr-merge-watcher.sh` before declaring merge-ready
- `.github/workflows/pre-merge-check.yml` — GH Action on every push to `grava/**`

## Local probe (scripts/pre-merge-check.sh)

```bash
#!/bin/bash
# Local merge-conflict + smoke-test probe. Returns non-zero on conflict.
# Called: ./scripts/pre-merge-check.sh <issue-id>

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
TMP_BR="prelaunch-$ISSUE_ID-$$"
git worktree add --detach "../.merge-probe-$$" HEAD >/dev/null 2>&1 || exit 0
(
  cd "../.merge-probe-$$" || exit 0
  git merge --no-commit --no-ff origin/main >/dev/null 2>&1 || { echo "merge failed in probe"; exit 3; }
  go build ./... 2>&1 || exit 3
)
PROBE_RC=$?
git worktree remove "../.merge-probe-$$" --force >/dev/null 2>&1 || true

[ "$PROBE_RC" -eq 0 ] || { echo "PIPELINE_HALTED: build fails when merged with main"; exit 3; }
echo "pre-merge OK"
exit 0
```

Callers:
- `pr-creator` agent (story 2B.14) — runs this **before** `gh pr create`. Invocation pattern `( cd "$REPO_ROOT" && ./scripts/pre-merge-check.sh "$ISSUE_ID" )` because the script's first action is `cd ".worktree/$ISSUE_ID"` (relative — must be invoked from repo root). On exit≠0, the agent emits `PR_FAILED: pre-merge check failed`.
- `pr-merge-watcher.sh` (story 2B.12) — runs once per cycle on issues whose PR is `OPEN` and have no `pr_new_comments`. Failure surfaces as wisp `pre_merge_failed` + `needs-human` label. Watcher already runs from repo root (line 37 `cd "$REPO_ROOT"`).

> Note: `/ship` Phase 3 does NOT call this directly. The probe is delegated to `pr-creator` so the PR-creation agent owns the full pre-PR contract (push, probe, gh create, label) in one place.

## GitHub Action (.github/workflows/pre-merge-check.yml)

```yaml
name: pre-merge-check
on:
  push:
    branches:
      - 'grava/**'

jobs:
  merged-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - name: Configure git
        run: |
          git config user.email ci@grava.local
          git config user.name "grava ci"
      - name: Merge main into branch (probe)
        run: |
          git fetch origin main
          git merge --no-commit --no-ff origin/main || {
            echo "::error::merge with main produced conflicts"
            exit 1
          }
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Test merged result
        run: go test -race ./...
```

Branch protection on `main` should require this check before merge. Document in `docs/branch-protection.md`.

## Post-merge regression hunt

After PR merge to `main`, `pr-merge-watcher.sh` (story 2B.12) optionally enqueues a `/hunt recent` run (via the bug-hunter scheduler — story 2B.15). Out of scope for this story; tracked there.

## Acceptance Criteria

- `scripts/pre-merge-check.sh <id>` returns 0 on clean merge, 2 on conflict, 3 on build break
- Script must be invoked from repo root cwd (its first action is `cd ".worktree/$ISSUE_ID"` — relative). Callers that may already be inside the worktree must use a subshell: `( cd "$REPO_ROOT" && ./scripts/pre-merge-check.sh "$ISSUE_ID" )`. The `pr-creator` agent does this; the watcher runs from repo root already.
- `pr-creator` agent (story 2B.14) runs it before `gh pr create`; exit 2 or 3 → emits `PR_FAILED: pre-merge check failed`. `/ship` does not call this directly — the probe is delegated to the PR-creation agent so all pre-PR concerns live in one prompt.
- GH Action runs on every push to `grava/**` and tests the merged-with-main result, not branch HEAD
- Branch-protection rule references the action's check name (`merged-test`) — README docs the click path
- Cleanup: probe worktree always removed even if build fails (trap or explicit cleanup)

## Dependencies

- Story 2B.5 (Phase 3 caller)
- Story 2B.12 (watcher caller)
- Repo: `go.mod` and `go test -race ./...` known-green on `main`
- Branch-protection access on the GitHub repo (manual one-time admin task)

## Out of Scope

- Multi-language repos (only Go probed). Extend conditionally in the script.
- Post-merge `/hunt` trigger — story 2B.15 owns this.
