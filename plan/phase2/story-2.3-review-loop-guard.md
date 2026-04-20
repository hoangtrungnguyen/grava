# Story 2.3: Review Loop Guard

Prevent infinite review loops by capping at 3 rounds per issue.

## File

`scripts/hooks/review-loop-guard.sh`

## Hook Event

`TaskCreated` — fires when a task is being created. Intercepts re-review task creation after 3 rounds.

### Input (stdin JSON)

```json
{
  "hook_event_name": "TaskCreated",
  "task_id": "task-007",
  "task_subject": "Review grava-abc1 (round 4)",
  "task_description": "Re-review after fix",
  "teammate_name": "review-agent",
  "team_name": "my-team",
  "session_id": "abc123",
  "cwd": "/path/to/repo"
}
```

### Exit Codes

| Code | Behavior |
|------|----------|
| 0 | Task creation proceeds |
| 2 | Rolls back task creation. Stderr fed back to the agent |

## Logic

1. Read `task_subject` from stdin
2. Extract grava issue ID (pattern: `grava-[a-f0-9]+(\.[0-9]+)?`)
3. If no issue ID found, exit 0 (not a review task)
4. Count existing `[REVIEW]` comments via `grava show <id> --json`
5. If count >= 3:
   - Post escalation comment
   - Add `needs_human` label
   - Export issue state so other machines see the escalation
   - Exit 2 to block the task
6. Otherwise exit 0

## Script

```bash
#!/bin/bash
INPUT=$(cat)
SUBJECT=$(echo "$INPUT" | jq -r '.task_subject // empty')
ISSUE_ID=$(echo "$SUBJECT" | grep -oE 'grava-[a-f0-9]+(\.[0-9]+)?')

[ -z "$ISSUE_ID" ] && exit 0

REVIEW_COUNT=$(grava show "$ISSUE_ID" --json 2>/dev/null | \
  jq '[.comments[]? | select(.message | startswith("[REVIEW]"))] | length')

if [ "${REVIEW_COUNT:-0}" -ge 3 ]; then
  grava comment "$ISSUE_ID" -m "[ESCALATION] Max review rounds (3) reached. Human review required."
  grava label "$ISSUE_ID" --add needs_human
  grava commit -m "escalate $ISSUE_ID after 3 review rounds"
  grava export 2>/dev/null
  echo "Issue $ISSUE_ID hit 3 review rounds. Escalating to human." >&2
  exit 2
fi
exit 0
```

## Acceptance Criteria

- Task creation blocked after 3 review rounds
- `needs_human` label added to the issue
- Escalation comment posted
- Issue state exported to `issues.jsonl` for cross-machine sync
- Tasks unrelated to reviews are not affected
- Issues with < 3 reviews proceed normally
