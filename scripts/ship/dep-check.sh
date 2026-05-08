#!/usr/bin/env bash
# scripts/ship/dep-check.sh — fail-safe dependency precondition gate for /ship.
#
# Replaces the inline `BLOCKERS_JSON=$(grava blocked ... 2>/dev/null || echo "[]")`
# pattern that swallowed errors silently (grava-223a). When `grava blocked`
# fails — DB down, schema migration mid-flight, malformed JSON — we MUST
# halt the pipeline rather than proceed without a check.
#
# Exit codes (consumed by ship/SKILL.md Phase 0.2):
#   0 — no open blockers, /ship may proceed
#   1 — fail-safe abort (CLI errored, malformed JSON, missing arg)
#   2 — at least one open (non-closed/non-tombstone) blocker; summary on stderr
#
# stderr on rc=1 starts with "PIPELINE_FAILED:" so the orchestrator can
# surface it verbatim. stderr on rc=2 starts with "BLOCKED:" plus the
# comma-joined blocker ids.
#
# Defensive jq filter on rc=2 preserves the grava-cd50 (layer 5) belt-and-
# suspenders: even if `grava blocked` ever stops filtering closed/tombstone
# server-side, we filter again here.

set -uo pipefail

ISSUE_ID="${1:-}"

if [ -z "$ISSUE_ID" ]; then
  echo "PIPELINE_FAILED: dep-check called without issue id" >&2
  echo "Usage: dep-check.sh <issue-id>" >&2
  exit 1
fi

# Capture stderr separately so we can surface a useful diagnostic when grava
# itself errors. Using process substitution keeps stderr bytes intact.
STDERR_FILE=$(mktemp)
# shellcheck disable=SC2064  # we want $STDERR_FILE expanded now
trap "rm -f \"$STDERR_FILE\"" EXIT

BLOCKERS_JSON=$(grava blocked "$ISSUE_ID" --json 2>"$STDERR_FILE")
RC=$?

if [ $RC -ne 0 ]; then
  STDERR_TAIL=$(head -c 200 "$STDERR_FILE" 2>/dev/null || true)
  echo "PIPELINE_FAILED: dep-check failed (grava blocked exit=$RC); aborting to fail-safe" >&2
  if [ -n "$STDERR_TAIL" ]; then
    echo "  grava stderr: $STDERR_TAIL" >&2
  fi
  exit 1
fi

# Validate JSON shape before letting jq filter run. A non-array (e.g. error
# object, plain text, truncated stream) must be treated as fail-safe — NOT
# silently coerced to an empty array.
if ! printf '%s' "$BLOCKERS_JSON" | jq -e 'type == "array"' >/dev/null 2>&1; then
  PREVIEW=$(printf '%s' "$BLOCKERS_JSON" | head -c 200)
  echo "PIPELINE_FAILED: dep-check returned non-array JSON; aborting to fail-safe" >&2
  echo "  payload preview: $PREVIEW" >&2
  exit 1
fi

# Defensive: filter out closed/tombstone deps. `grava blocked` already does
# this server-side under default flags, but we belt-and-suspenders here so a
# future caller passing --all (archaeology mode) doesn't accidentally halt.
OPEN_IDS=$(printf '%s' "$BLOCKERS_JSON" \
  | jq -r '[.[] | select(.status != "closed" and .status != "tombstone") | .id] | join(", ")')

if [ -n "$OPEN_IDS" ]; then
  COUNT=$(printf '%s' "$BLOCKERS_JSON" \
    | jq '[.[] | select(.status != "closed" and .status != "tombstone")] | length')
  echo "BLOCKED: $COUNT unresolved blocker(s): $OPEN_IDS" >&2
  exit 2
fi

exit 0
