#!/bin/bash
# Stop hook — warns when a session ends with in-progress issues.
# Multiple issues may be in_progress when several terminals run /ship at once.

ISSUES=$(grava list --status in_progress --json 2>/dev/null)
[ $? -eq 0 ] || exit 0

COUNT=$(echo "$ISSUES" | jq 'length')
[ "$COUNT" -gt 0 ] || exit 0

echo "Warning: Session ending with $COUNT in-progress issue(s):" >&2

echo "$ISSUES" | jq -r '.[] | "   \(.id): \(.title)"' >&2

echo "Run \`grava stop <id>\` to release, or \`grava doctor\` for orphan check." >&2
exit 0
