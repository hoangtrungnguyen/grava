# Story 2.2: TeammateIdle Hook

Redirect idle teammates to pending work in their pipeline stage.

## File

`scripts/hooks/check-teammate-idle.sh`

## Hook Event

`TeammateIdle` — fires when a teammate finishes its current work and is about to go idle.

### Input (stdin JSON)

```json
{
  "hook_event_name": "TeammateIdle",
  "teammate_name": "review-agent",
  "team_name": "my-team",
  "idle_reason": "No more assigned tasks",
  "session_id": "abc123",
  "cwd": "/path/to/repo"
}
```

### Exit Codes

| Code | Behavior |
|------|----------|
| 0 | Teammate goes idle |
| 2 | Prevents idle. Stderr fed back to the agent — it continues working |

## Logic

1. Read `teammate_name` from stdin
2. Map teammate to its pipeline label(s):
   - `coding-agent` -> check `pr_feedback` (PR feedback) AND `grava ready` (new issues)
   - `review-agent` -> `code_review`
   - `fix-agent` -> `changes_requested`
   - `qa-agent` -> `reviewed`
   - `ci-agent` -> `qa_passed` AND `pr_created` (PRs to monitor)
3. Query grava to count pending work
4. If count > 0, exit 2 with message — agent picks up next task
5. If count = 0, exit 0 — let it idle

## Dependency

Requires Phase 3: `grava search --label` must be implemented first.

## Script

```bash
#!/bin/bash
INPUT=$(cat)
TEAMMATE=$(echo "$INPUT" | jq -r '.teammate_name // empty')

COUNT=0
case "$TEAMMATE" in
  coding-agent)
    COUNT=$(grava search --label pr_feedback --json 2>/dev/null | jq 'length')
    [ "${COUNT:-0}" -eq 0 ] && COUNT=$(grava ready --limit 1 --json 2>/dev/null | jq 'length')
    ;;
  review-agent)  COUNT=$(grava search --label code_review --json 2>/dev/null | jq 'length') ;;
  fix-agent)     COUNT=$(grava search --label changes_requested --json 2>/dev/null | jq 'length') ;;
  qa-agent)      COUNT=$(grava search --label reviewed --json 2>/dev/null | jq 'length') ;;
  ci-agent)
    COUNT=$(grava search --label qa_passed --json 2>/dev/null | jq 'length')
    [ "${COUNT:-0}" -eq 0 ] && COUNT=$(grava search --label pr_created --json 2>/dev/null | jq 'length')
    ;;
  *)             exit 0 ;;
esac

if [ "${COUNT:-0}" -gt 0 ]; then
  echo "$COUNT items waiting at your stage. Pick up the next one." >&2
  exit 2
fi
exit 0
```

## Acceptance Criteria

- Idle teammate with pending work gets redirected
- coding-agent checks both `pr_feedback` and ready queue
- ci-agent checks both `qa_passed` and `pr_created` (monitoring)
- Idle teammate with no pending work goes idle normally
- Unknown teammate names are ignored (exit 0)
- Handles `grava search` failure gracefully (defaults to 0)
