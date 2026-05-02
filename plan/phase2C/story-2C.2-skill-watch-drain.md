# Story 2C.2: /watch-drain Skill — Periodic Issue Watcher

Background-poll daemon that checks the grava database for ready leaf-type issues on a configurable interval. When issues are found it invokes `/ship-loop` to drain them, then resumes polling. Runs until explicitly stopped or a fatal error limit is reached.

Two delivery modes in one story:
- **Skill (`/watch-drain`)** — user invokes once in a Claude Code session; the skill loops with `sleep` between checks.
- **Cron script (`scripts/issue-drain-watcher.sh`)** — shell script that checks once and exits; cron provides the recurrence. Use this for unattended / always-on operation.

Both modes are specified here. The skill is the primary user-facing entry point; the cron script is recommended for production use.

## File

`.claude/skills/watch-drain/SKILL.md` (skill)
`scripts/issue-drain-watcher.sh` (cron companion)

## Frontmatter

```yaml
---
name: watch-drain
description: "Autopilot daemon: poll grava for ready issues on an interval, invoke /ship-loop when found."
user-invocable: true
---
```

## Usage

```
/watch-drain [--interval <seconds>] [--label <name>] [--epic <id>] [--max-errors N]
```

Examples:
- `/watch-drain` — poll every 300 s (5 min), drain when issues found
- `/watch-drain --interval 60` — poll every 60 s
- `/watch-drain --label backend` — scope drain to `backend`-labelled issues
- `/watch-drain --epic grava-epic-001` — scope to one epic's subtree

## Setup

```bash
POLL_INTERVAL=300    # seconds between polls when queue is empty
MAX_ERRORS=3         # fatal error limit (consecutive drain failures)
SCOPE_LABEL=""
SCOPE_EPIC=""

for arg in $ARGUMENTS; do
  case "$arg" in
    --interval)    shift; POLL_INTERVAL="${1:-300}" ;;
    --max-errors)  shift; MAX_ERRORS="${1:-3}" ;;
    --label)       shift; SCOPE_LABEL="${1:-}" ;;
    --epic)        shift; SCOPE_EPIC="${1:-}" ;;
    --*)           echo "WATCH_FAILED: unknown flag $arg"; exit 1 ;;
  esac
done

# preflight once at start, not on every poll cycle
./scripts/preflight-gh.sh || exit 1

CONSECUTIVE_ERRORS=0
TOTAL_DRAIN_RUNS=0
TOTAL_ISSUES_SHIPPED=0
echo "watch-drain started. Poll interval: ${POLL_INTERVAL}s. Scope: label=${SCOPE_LABEL:-any} epic=${SCOPE_EPIC:-any}"
```

## Poll Helper

```bash
count_ready_issues() {
  local q
  q=$(grava ready --limit 50 --json 2>/dev/null \
    | jq '[.[] | select(.Node.Type == "task" or .Node.Type == "bug")]')

  if [ -n "$SCOPE_EPIC" ]; then
    q=$(echo "$q" | jq --arg e "$SCOPE_EPIC" '[.[] | select(.Node.ParentID == $e)]')
  fi
  if [ -n "$SCOPE_LABEL" ]; then
    q=$(echo "$q" | jq --arg l "$SCOPE_LABEL" '[.[] | select(.Node.Labels[]? == $l)]')
  fi

  echo "$q" | jq 'length'
}
```

## Main Loop

```bash
while true; do
  TS=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
  READY=$(count_ready_issues)

  if [ "$READY" -eq 0 ]; then
    echo "[$TS] No ready issues. Next check in ${POLL_INTERVAL}s."
    sleep "$POLL_INTERVAL"
    continue
  fi

  echo "[$TS] $READY ready issue(s) detected. Starting /ship-loop..."

  # Build scope args to forward to /ship-loop
  SCOPE_ARGS=""
  [ -n "$SCOPE_LABEL" ] && SCOPE_ARGS="$SCOPE_ARGS --label $SCOPE_LABEL"
  [ -n "$SCOPE_EPIC" ]  && SCOPE_ARGS="$SCOPE_ARGS --epic $SCOPE_EPIC"

  LOOP_RESULT=$(Skill("ship-loop", "$SCOPE_ARGS"))
  LAST=$(printf '%s' "$LOOP_RESULT" | awk 'NF{l=$0} END{print l}')

  TOTAL_DRAIN_RUNS=$((TOTAL_DRAIN_RUNS + 1))

  case "$LAST" in
    "PIPELINE_INFO: ready queue empty"*)
      # Normal drain completion
      CONSECUTIVE_ERRORS=0
      # Extract shipped count from summary line if present
      SHIPPED_THIS_RUN=$(printf '%s' "$LOOP_RESULT" | grep -oP 'Shipped.*: \K[0-9]+' | head -1 || echo "?")
      TOTAL_ISSUES_SHIPPED=$((TOTAL_ISSUES_SHIPPED + ${SHIPPED_THIS_RUN:-0}))
      echo "[$TS] Drain complete (shipped this run: ${SHIPPED_THIS_RUN:-?}). Resuming polling in ${POLL_INTERVAL}s."
      sleep "$POLL_INTERVAL"
      ;;

    "PIPELINE_INFO: "*)
      # Unexpected info (e.g., discovered issue vanished mid-run) — treat as soft error
      CONSECUTIVE_ERRORS=0
      echo "[$TS] Drain info: ${LAST#PIPELINE_INFO: }. Resuming polling in ${POLL_INTERVAL}s."
      sleep "$POLL_INTERVAL"
      ;;

    "PIPELINE_FAILED: "*)
      CONSECUTIVE_ERRORS=$((CONSECUTIVE_ERRORS + 1))
      echo "[$TS] Drain failed: ${LAST#PIPELINE_FAILED: } (consecutive errors: $CONSECUTIVE_ERRORS/$MAX_ERRORS)"
      if [ "$CONSECUTIVE_ERRORS" -ge "$MAX_ERRORS" ]; then
        STOP_REASON="$CONSECUTIVE_ERRORS consecutive drain failures"
        break
      fi
      sleep "$POLL_INTERVAL"
      ;;

    *)
      # /ship-loop ended mid-summary (PIPELINE_HALTED from internal stop condition is normal)
      CONSECUTIVE_ERRORS=0
      echo "[$TS] Drain ended (last: $LAST). Resuming polling in ${POLL_INTERVAL}s."
      sleep "$POLL_INTERVAL"
      ;;
  esac
done
```

## Exit Summary

```bash
echo ""
echo "--- watch-drain stopped ---"
echo "Reason: ${STOP_REASON:-user interrupt}"
echo "Total drain runs:     $TOTAL_DRAIN_RUNS"
echo "Total issues shipped: $TOTAL_ISSUES_SHIPPED"
```

## Cron Companion — `scripts/issue-drain-watcher.sh`

For unattended operation: the script checks once and exits; cron provides the schedule. Unlike the skill (which blocks a Claude Code session), the script spawns an independent `claude -p` process per drain cycle so the session cost is pay-per-use.

```bash
#!/bin/bash
# issue-drain-watcher.sh — run via cron; checks once, triggers drain if needed.
# Add to crontab: */10 * * * * cd /path/to/grava && ./scripts/issue-drain-watcher.sh >> .grava/drain-watcher.log 2>&1

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

READY=$(grava ready --limit 5 --json 2>/dev/null \
  | jq '[.[] | select(.Node.Type == "task" or .Node.Type == "bug")] | length')

TS=$(date -u '+%Y-%m-%dT%H:%M:%SZ')

if [ "$READY" -eq 0 ]; then
  echo "[$TS] No ready issues. Skipping drain."
  exit 0
fi

echo "[$TS] $READY ready issue(s). Triggering drain..."

# Guard: skip if a drain is already running (lock file)
LOCK_FILE=".grava/drain-watcher.lock"
if [ -f "$LOCK_FILE" ]; then
  LOCK_PID=$(cat "$LOCK_FILE" 2>/dev/null || echo "")
  if [ -n "$LOCK_PID" ] && kill -0 "$LOCK_PID" 2>/dev/null; then
    echo "[$TS] Drain already running (pid $LOCK_PID). Skipping."
    exit 0
  fi
  echo "[$TS] Stale lock (pid $LOCK_PID gone). Clearing."
  rm -f "$LOCK_FILE"
fi

# Spawn drain in background, record PID as lock
claude --print "/ship-loop" >> ".grava/drain.log" 2>&1 &
DRAIN_PID=$!
echo "$DRAIN_PID" > "$LOCK_FILE"
echo "[$TS] Drain started (pid $DRAIN_PID). Lock: $LOCK_FILE"
```

Add cron entry (or launchd on macOS):
```cron
# Issue drain watcher — every 10 min
*/10 * * * * cd /path/to/grava && ./scripts/issue-drain-watcher.sh >> .grava/drain-watcher.log 2>&1
```

The lock file prevents overlapping drain runs. The cron script exits immediately after spawning; the drain itself runs in the background.

## Stop Conditions (skill mode)

| Condition | Behaviour |
|-----------|-----------|
| Queue empty after drain | Sleep `POLL_INTERVAL`, re-poll |
| `PIPELINE_FAILED` from `/ship-loop` | Increment error counter; after `MAX_ERRORS` consecutive failures, stop |
| `PIPELINE_HALTED` from `/ship-loop` (internal stop) | Not a watcher error — loop halted its own budget; watcher resumes polling |
| User interrupt (Ctrl-C) | Print summary, exit |

## Acceptance Criteria

**Skill (`/watch-drain`)**
- Polls `grava ready --json` every `POLL_INTERVAL` seconds; default 300 s
- When `count_ready_issues > 0`: invokes `/ship-loop` (with forwarded scope args), logs start + completion
- After drain: resumes polling with the same interval
- When queue is empty on poll: logs "No ready issues", sleeps, re-polls — does NOT invoke `/ship-loop`
- `MAX_ERRORS` consecutive `PIPELINE_FAILED` responses → prints summary, exits
- Individual `PIPELINE_HALTED` from `/ship-loop` does NOT increment error counter (halt = spec problem, not watcher error)
- `--interval`, `--label`, `--epic`, `--max-errors` flags parsed correctly
- `gh` auth missing → exits before first poll
- `preflight-gh.sh` runs once at startup, not on every poll cycle

**Cron script (`scripts/issue-drain-watcher.sh`)**
- Script is idempotent: if no ready issues, exits 0 without spawning Claude
- Lock file prevents concurrent drain runs; stale lock (dead PID) is cleared automatically
- Lock file path: `.grava/drain-watcher.lock`
- Log output goes to `.grava/drain-watcher.log` (cron redirect) + `.grava/drain.log` (drain output)
- Script exits immediately after spawning drain process (non-blocking)

## Dependencies

- Story 2C.1 (`/ship-loop` skill — must be implemented first)
- `scripts/preflight-gh.sh` (from 2B.5)
- `grava ready --json` with leaf-type filtering
- `claude --print` CLI available in PATH (for cron script)
- `jq` on PATH

## Signals Emitted

- `WATCH_FAILED: <reason>` — startup validation failure (unknown flag, preflight)
- No new pipeline signals; watcher is a consumer only
