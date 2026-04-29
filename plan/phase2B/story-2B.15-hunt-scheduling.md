# Story 2B.15: Bug-Hunt Scheduling

Story 2B.3 (`bug-hunter` agent) declares "weekly cron / after major merge / on request" but never specifies the scheduling mechanism. This story owns the triggers.

## Files

- `.git/hooks/commit-msg` — scans message for `bug-hunt: <scope>` token
- `scripts/install-hooks.sh` — installs the commit-msg hook (so it lives in version control even though `.git/hooks/` doesn't)
- `scripts/run-pending-hunts.sh` — cron worker; drains the pending-hunts queue
- cron entries — hourly drain + nightly `since-last-tag`

## Triggers (3 paths)

| Trigger | Mechanism | Scope |
|---------|-----------|-------|
| Per-commit, opt-in | `commit-msg` hook reads `bug-hunt: <scope>` token, enqueues | Whatever scope follows the token |
| Hourly drain | cron runs `run-pending-hunts.sh` | Drains `.grava/pending-hunts.txt` (file-based queue) |
| Nightly tag-window | cron runs `claude -p "/hunt since-last-tag"` at 02:00 | Default scope |

> **Why a file, not a wisp.** Wisps are issue-scoped — the grava CLI rejects sentinel / non-issue ids (e.g. `_global`, `grava-global`). A pipeline-wide queue does not belong to any one issue, so it lives in the filesystem under `.grava/` (same neighbourhood as pidfiles and logs). Append-on-write + atomic-rename-on-drain gives the same semantics the wisp version was reaching for, with no CLI extension needed.

## File: `.git/hooks/commit-msg`

```bash
#!/bin/bash
# commit-msg hook — enqueue bug hunts when developers tag commits.
# Token format anywhere in the commit body:  bug-hunt: <scope>
# Scope examples: ./internal/auth, recent, all

set -u
MSG_FILE="$1"
SCOPES=$(grep -oE 'bug-hunt:[[:space:]]*[^[:space:]]+' "$MSG_FILE" | awk -F: '{print $2}' | tr -d ' ')

[ -z "$SCOPES" ] && exit 0

# The hook runs from the repo root via git, so this relative path resolves
# correctly. Use $GIT_DIR/.. as a more robust anchor when running under worktrees.
ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0
QUEUE="$ROOT/.grava/pending-hunts.txt"
mkdir -p "$ROOT/.grava"

# Append one scope per line — drain script reads line-by-line, atomic rename
# is sufficient to avoid lost writes from concurrent commits in worktrees.
for s in $SCOPES; do
  printf '%s\n' "$s" >> "$QUEUE"
done

exit 0
```

## File: `scripts/install-hooks.sh`

`commit-msg` lives in `.git/hooks/` which git does NOT version. This script installs it from a tracked source under `scripts/git-hooks/commit-msg`.

```bash
#!/bin/bash
# Idempotent installer for repo-local git hooks.
set -e
ROOT=$(git rev-parse --show-toplevel)
SRC="$ROOT/scripts/git-hooks"
DST="$ROOT/.git/hooks"

mkdir -p "$DST"
for hook in "$SRC"/*; do
  name=$(basename "$hook")
  cp "$hook" "$DST/$name"
  chmod +x "$DST/$name"
  echo "installed: $name"
done
```

A `make setup` target should run this. New clones run `./scripts/install-hooks.sh` once.

## File: `scripts/run-pending-hunts.sh`

```bash
#!/bin/bash
# Cron worker — drains .grava/pending-hunts.txt and dispatches one /hunt
# per scope via `claude -p`. Hourly cadence.

set -u
REPO_ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
cd "$REPO_ROOT" || exit 1

PIDFILE=".grava/run-pending-hunts.pid"
mkdir -p .grava
if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  exit 0
fi
echo $$ > "$PIDFILE"
trap 'rm -f "$PIDFILE"' EXIT

QUEUE=".grava/pending-hunts.txt"
[ -s "$QUEUE" ] || exit 0

# Atomic drain: rename the queue out of the way before dispatching. Concurrent
# commit-msg hooks that fire mid-dispatch land in a fresh QUEUE and are picked
# up next cycle. Duplicate hunts are wasteful but safe; a crash mid-dispatch
# loses at most one cycle of scopes, recovered by the next commit token.
DRAIN=".grava/pending-hunts.draining.$$"
mv "$QUEUE" "$DRAIN" 2>/dev/null || exit 0

# Deduplicate so two commits enqueueing the same scope don't dispatch twice.
sort -u "$DRAIN" | while read -r SCOPE; do
  [ -z "$SCOPE" ] && continue
  echo "[$(date -u +%FT%TZ)] dispatching /hunt $SCOPE"
  claude -p "/hunt $SCOPE" >> .grava/hunt.log 2>&1 || \
    echo "warn: /hunt $SCOPE returned non-zero"
done

rm -f "$DRAIN"
exit 0
```

## cron lines

```cron
# Hourly drain
0 * * * * cd /path/to/grava && ./scripts/run-pending-hunts.sh

# Nightly default scope
0 2 * * * cd /path/to/grava && claude -p "/hunt since-last-tag" >> .grava/hunt.log 2>&1
```

## Queue schema (file-based, not wisp)

| Path | Owner | Lifetime | Format |
|------|-------|----------|--------|
| `.grava/pending-hunts.txt` | commit-msg writes; cron drains via atomic rename | until next cron run | one scope token per line; deduped on drain |

> Why not a wisp: the grava CLI rejects non-issue wisp namespaces (no `_global` / `grava-global` sentinel). The pipeline-wide queue therefore lives in the filesystem under `.grava/` — same convention as pidfiles and logs. Add `.grava/pending-hunts.txt` to `.gitignore` (the `.grava/` directory is already gitignored).

## Update to story 2B.3

Add a "When to Run" section pointing here:

> Triggers (story 2B.15):
> 1. Commit message contains `bug-hunt: <scope>` → enqueued, picked up within ~1h
> 2. Cron-driven nightly run with scope `since-last-tag`
> 3. Manual: user runs `/hunt [scope]`

## Acceptance Criteria

- `scripts/install-hooks.sh` is idempotent — re-running does not corrupt existing hooks
- `commit-msg` hook is non-blocking (exit 0 always) — never breaks a commit
- A commit message containing `bug-hunt: ./internal/auth` appends `./internal/auth` as a new line in `.grava/pending-hunts.txt`
- Multiple `bug-hunt:` tokens in one message are all enqueued (one per line)
- `run-pending-hunts.sh` atomic-renames the queue file before dispatching (drain is exclusive of concurrent enqueues)
- Drain dedupes scopes — two commits enqueueing the same scope dispatch only once
- Pidfile prevents overlapping cron runs
- Empty / missing queue file → drain exits 0 in <1s
- Nightly `claude -p "/hunt since-last-tag"` invocation runs at 02:00 local
- `make setup` (separate ticket) calls install-hooks.sh
- No `grava wisp` write or read references `_global` / sentinel ids — pipeline-wide state lives in `.grava/` files only

## Dependencies

- Story 2B.3 (bug-hunter agent — invoked by `/hunt`)
- Story 2B.8 (`/hunt` skill)
- `claude` CLI on PATH (headless invocation)
- `.grava/` directory writable (already gitignored; pipeline assumes it exists)

## Out of Scope

- launchd plists (mac equivalent of cron) — same logic, different unit file
- Slack/Telegram notification of completed hunts — separate notification story
