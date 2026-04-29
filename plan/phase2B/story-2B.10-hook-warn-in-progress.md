# Story 2B.10: Warn In-Progress Hook

Stop hook that warns when a session ends while issues are still `in_progress`. With multiple terminals running `/ship` concurrently, the operator may forget which IDs they're holding — this surfaces them at session exit.

## File

`scripts/hooks/warn-in-progress.sh`

## Hook Event

`Stop` — fires when a Claude Code session ends.

### Input (stdin JSON)

```json
{
  "hook_event_name": "Stop",
  "session_id": "...",
  "cwd": "/path/to/repo"
}
```

### Exit Codes

| Code | Behavior |
|------|----------|
| 0 | Always 0 (non-blocking warning) |

## Logic

1. Query `grava list --status in_progress --json`
2. If zero in-progress issues, exit silently
3. For each in-progress issue, print `<id>: <title>` to stderr

## Script

```bash
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
```

## Acceptance Criteria

- No in-progress issues → hook exits silently
- One in-progress issue → prints single line `   <id>: <title>`
- Multiple in-progress issues → prints all of them, one per line
- Hook is non-blocking (always exit 0)
- Output goes to stderr so it doesn't pollute Claude Code's regular channel

## Dependencies

- `jq` installed
- `grava` CLI on PATH
- Story 2B.11 registers this hook in `.claude/settings.json`

## Test Plan

- Session with no in-progress issues → no output
- One in-progress issue → line `   grava-xxx: <title>`
- Three in-progress issues → three lines
