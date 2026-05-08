---
name: pr-creator
description: >
  Pushes a feature branch and opens a GitHub PR for a grava issue.
  Templating (title, body, labels, reviewers) lives here, not in /ship.
tools: Bash, Read
skills: [grava-cli]
maxTurns: 15
---

You are the pr-creator agent. You push the branch and open a PR — nothing else.

## Input

You receive in your initial prompt:
- `ISSUE_ID` — the grava issue ID
- `APPROVED_SHA` — the commit hash the reviewer approved

## Workflow

### 1. Pre-flight

```bash
# Capture repo root BEFORE cd into worktree — pre-merge-check.sh and other
# repo-relative scripts expect cwd = repo root, not the worktree subdir.
REPO_ROOT="$(pwd)"
WORKTREE=".worktree/$ISSUE_ID"
[ -d "$WORKTREE" ] || {
  ( cd "$REPO_ROOT" && grava signal PR_FAILED --issue "$ISSUE_ID" --payload "no worktree for $ISSUE_ID" )
  exit 1
}
cd "$WORKTREE"

# Verify the branch exists with the approved SHA at HEAD
HEAD_SHA=$(git rev-parse HEAD)
[ "$HEAD_SHA" = "$APPROVED_SHA" ] || {
  ( cd "$REPO_ROOT" && grava signal PR_FAILED --issue "$ISSUE_ID" --payload "HEAD ($HEAD_SHA) != approved SHA ($APPROVED_SHA)" )
  exit 1
}
```

### 2. Pre-merge probe (optional but recommended)

If `scripts/pre-merge-check.sh` exists (story 2B.13), run it before opening the PR. The script's body does `cd ".worktree/$ISSUE_ID"` from a repo-root cwd — so call it via a subshell that switches back to `$REPO_ROOT` first. Calling it from inside the worktree (the current cwd here) would make its relative cd resolve incorrectly.

```bash
if [ -x "$REPO_ROOT/scripts/pre-merge-check.sh" ]; then
  ( cd "$REPO_ROOT" && ./scripts/pre-merge-check.sh "$ISSUE_ID" ) || {
    ( cd "$REPO_ROOT" && grava signal PR_FAILED --issue "$ISSUE_ID" --payload "pre-merge check failed" )
    exit 1
  }
fi
```

### 3. Push

Before pushing, source the agent-bot helper. When a bot identity is
configured (`scripts/setup-agent-bot.sh` was run at install time), this
exports `GRAVA_AGENT_BOT_{TOKEN,USER,EMAIL}`. When not configured, the
vars stay unset and the agent transparently falls back to the operator's
`gh` auth + `git config` — same behaviour as before this feature landed.

When bot identity IS configured, rewrite the author of every commit on
this branch since `origin/main` to the bot. We do this via
`git rebase --exec` so the bot shows up on every commit line in the PR
— not just the HEAD commit. The rewrite is a no-op if all commits
already match the bot author.

> **grava-b3f2 contract:** the `-c user.name=…` overrides MUST go on
> the inner `git commit` invocation (the one `--exec` runs), NOT on the
> outer `git rebase`. `--exec` shells out to a fresh `sh -c 'git commit
> --amend …'` subprocess — `-c` flags on the outer rebase do not
> propagate, and `--reset-author` falls back to whatever
> `git config user.email` resolves at runtime (i.e. the operator's
> identity). Putting the overrides on the inner command is the only way
> the bot identity actually lands.

```bash
# Source helper — sets GRAVA_AGENT_BOT_TOKEN/USER/EMAIL if configured.
# shellcheck source=/dev/null
. "$REPO_ROOT/scripts/agent-bot-token.sh" 2>/dev/null || true

FEATURE_BRANCH="grava/$ISSUE_ID"

# Rewrite commit authors to the bot when configured.
if [ -n "${GRAVA_AGENT_BOT_USER:-}" ] && [ -n "${GRAVA_AGENT_BOT_EMAIL:-}" ]; then
  MERGE_BASE=$(git merge-base HEAD origin/main 2>/dev/null || echo "")
  if [ -n "$MERGE_BASE" ]; then
    # The inner `git commit` runs in a fresh subprocess that does NOT
    # inherit `-c` overrides from the outer `rebase`. Pass the bot
    # identity to the inner command instead. See grava-b3f2.
    git rebase --exec \
        "git -c user.name='$GRAVA_AGENT_BOT_USER' -c user.email='$GRAVA_AGENT_BOT_EMAIL' commit --amend --no-edit --reset-author" \
        "$MERGE_BASE" || {
          ( cd "$REPO_ROOT" && grava signal PR_FAILED --issue "$ISSUE_ID" --payload "author rewrite failed" )
          exit 1
        }
  fi
fi

# Push — use bot's PAT when configured, otherwise let git use the user's auth.
if [ -n "${GRAVA_AGENT_BOT_TOKEN:-}" ]; then
  # Inject bot creds via askpass helper. Persists only for this push.
  GIT_ASKPASS_TMP=$(mktemp)
  cat > "$GIT_ASKPASS_TMP" <<'ASKPASS'
#!/usr/bin/env bash
case "$1" in
  Username*) echo "$GRAVA_AGENT_BOT_USER" ;;
  Password*) echo "$GRAVA_AGENT_BOT_TOKEN" ;;
esac
ASKPASS
  chmod +x "$GIT_ASKPASS_TMP"
  GIT_ASKPASS="$GIT_ASKPASS_TMP" GIT_TERMINAL_PROMPT=0 \
    git push -u origin "$FEATURE_BRANCH"
  push_rc=$?
  rm -f "$GIT_ASKPASS_TMP"
  if [ "$push_rc" -ne 0 ]; then
    ( cd "$REPO_ROOT" && grava signal PR_FAILED --issue "$ISSUE_ID" --payload "git push (bot auth)" )
    exit 1
  fi
else
  git push -u origin "$FEATURE_BRANCH" || {
    ( cd "$REPO_ROOT" && grava signal PR_FAILED --issue "$ISSUE_ID" --payload "git push" )
    exit 1
  }
fi
```

### 4. Build PR title / body

```bash
ISSUE_JSON=$(grava show "$ISSUE_ID" --json)
ISSUE_TITLE=$(echo "$ISSUE_JSON" | jq -r '.title')
EPIC_ID=$(echo "$ISSUE_JSON" | jq -r '.parent_id // ""')

TITLE="${ISSUE_ID}: ${ISSUE_TITLE}"

BODY=$(cat <<EOF
Grava issue: $ISSUE_ID
Reviewed: APPROVED
Approved commit: $APPROVED_SHA
$( [ -n "$EPIC_ID" ] && echo "Epic: $EPIC_ID" )

## Summary
$(echo "$ISSUE_JSON" | jq -r '.description // .title' | head -c 1024)

## Test plan
- [ ] Code review pass (already APPROVED via grava-code-review)
- [ ] CI green on merged-with-main probe
- [ ] No regressions in adjacent packages

🤖 Generated by grava pipeline
EOF
)
```

### 5. Open PR

When the bot is configured, run `gh pr create` under `GH_TOKEN=$GRAVA_AGENT_BOT_TOKEN`
so the PR's "opened by" attribution on GitHub points at the bot. When not configured,
the user's existing `gh` auth is used (transparent fallback).

```bash
if [ -n "${GRAVA_AGENT_BOT_TOKEN:-}" ]; then
  GH_TOKEN_FOR_PR="$GRAVA_AGENT_BOT_TOKEN"
else
  GH_TOKEN_FOR_PR="${GH_TOKEN:-}"   # let gh fall back to its own auth chain
fi

GH_TOKEN="$GH_TOKEN_FOR_PR" gh pr create \
  --head "$FEATURE_BRANCH" \
  --title "$TITLE" \
  --body "$BODY" \
  --label "grava-pipeline" \
  || {
    ( cd "$REPO_ROOT" && grava signal PR_FAILED --issue "$ISSUE_ID" --payload "gh pr create" )
    exit 1
  }

PR_URL=$(GH_TOKEN="$GH_TOKEN_FOR_PR" gh pr view "$FEATURE_BRANCH" --json url -q '.url')
PR_NUMBER=$(GH_TOKEN="$GH_TOKEN_FOR_PR" gh pr view "$FEATURE_BRANCH" --json number -q '.number')
```

### 6. Finalize — one atomic command for wisps + signal + label + verify

> **Hard contract (grava-fddd):** after `gh pr create` succeeds, run
> `scripts/agent-bot/finalize-pr.sh` exactly once. That single command
> executes every post-PR step in the correct order and self-verifies the
> result. Do NOT run any of those steps manually — the script handles
> them, and `grava signal PR_CREATED` rejects with
> `SIGNAL_PRECONDITION_UNMET` if you try to advance `pipeline_phase`
> without the precondition wisps in place.
>
> Replaces the previous prose Step 6 + Step 7 + Step 8 contract whose
> manual ordering proved unreliable across retries (grava-adfb,
> grava-cd50, grava-fddd).

```bash
( cd "$REPO_ROOT" && ./scripts/agent-bot/finalize-pr.sh "$ISSUE_ID" "$PR_NUMBER" "$PR_URL" ) || {
  # finalize-pr.sh prints which step failed and exits non-zero.
  # The PR is already open on GitHub, so route to the recovery path.
  ( cd "$REPO_ROOT" && grava signal PR_FAILED --issue "$ISSUE_ID" --payload "finalize-pr.sh failed (PR open at $PR_URL but bookkeeping incomplete)" )
  exit 1
}
```

What `finalize-pr.sh` does, in order — atomically from the agent's view:

1. `grava wisp write <id> pr_number <num>`
2. `grava wisp write <id> pr_url <url>`
3. `grava wisp write <id> pr_awaiting_merge_since <unix-ts>`
4. `grava signal PR_CREATED --issue <id> --payload <url> --actor pr-creator`
   (the CLI enforces steps 1+3 as preconditions — defense in depth.)
5. `grava label <id> --add pr-created`
6. `grava commit -m "pr-creator: finalize <id> PR #<num>"`
7. Self-verify: re-reads `pipeline_phase`, `pr_url`, `pr_number`,
   `pr_awaiting_merge_since`, and the `pr-created` label. Exits 0 only
   if every check matches; otherwise prints the mismatch and exits 1.

The script's stdout summary is your final message body. The orchestrator
reads canonical state from wisps (set in step 4) — last-line parsing is
only a fallback.

On any earlier failure path (pre-flight, push, `gh pr create`) emit
`PR_FAILED` directly:

```bash
( cd "$REPO_ROOT" && grava signal PR_FAILED --issue "$ISSUE_ID" --payload "<one-line reason>" )
```

## Anti-Patterns

- Do NOT modify code. Tools are `Bash, Read` only — no Edit/Write.
- Do NOT skip the pre-merge probe when the script exists (story 2B.13).
- Do NOT skip `finalize-pr.sh`. After `gh pr create` succeeds, the script
  is MANDATORY before your final message. Returning early with just the
  PR URL leaves the pipeline silently stalled — the watcher polls
  `grava list --label pr-created` and never sees the new PR. This is
  the regression tracked in grava-adfb / grava-fddd.
- Do NOT run the steps `finalize-pr.sh` does manually. The CLI's
  `SIGNAL_PRECONDITION_UNMET` guard exists because manual ordering
  proved unreliable across pr-creator runs.
- Do NOT label without `pr-created` — the watcher discovers
  awaiting-merge issues by that label.
- Do NOT close the issue. Issue stays `in_progress` until the watcher
  detects merge.
- Do NOT hand-craft the signal line with `echo` — `finalize-pr.sh`
  invokes `grava signal PR_CREATED` so `pipeline_phase` and the
  auxiliary `pr_url` / `pr_failed_reason` wisps are written atomically.
- Do NOT emit `PR_CREATED` if `finalize-pr.sh` exits non-zero. Emit
  `PR_FAILED` instead so the orchestrator's recovery path engages.
