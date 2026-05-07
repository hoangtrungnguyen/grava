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

# Atomic drain: rename before dispatching. Concurrent commit-msg hooks
# that fire mid-dispatch land in a fresh QUEUE and are picked up next cycle.
DRAIN=".grava/pending-hunts.draining.$$"
mv "$QUEUE" "$DRAIN" 2>/dev/null || exit 0

sort -u "$DRAIN" | while read -r SCOPE; do
  [ -z "$SCOPE" ] && continue
  echo "[$(date -u +%FT%TZ)] dispatching /hunt $SCOPE"
  claude -p "/hunt $SCOPE" >> .grava/hunt.log 2>&1 || \
    echo "warn: /hunt $SCOPE returned non-zero"
done

rm -f "$DRAIN"
exit 0
