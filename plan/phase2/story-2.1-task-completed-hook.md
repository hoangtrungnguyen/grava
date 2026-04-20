# Story 2.1: TaskCompleted Hook

Block task completion if tests are failing.

## File

`scripts/hooks/validate-task-complete.sh`

## Hook Event

`TaskCompleted` — fires when a teammate marks a task as complete.

### Input (stdin JSON)

```json
{
  "hook_event_name": "TaskCompleted",
  "task_id": "task-001",
  "task_subject": "Implement user authentication",
  "task_description": "Add login and signup endpoints",
  "teammate_name": "coding-agent",
  "team_name": "my-team",
  "completion_note": "All tests passing",
  "session_id": "abc123",
  "cwd": "/path/to/repo"
}
```

### Exit Codes

| Code | Behavior |
|------|----------|
| 0 | Task completion proceeds |
| 2 | Blocks completion. Stderr fed back to the agent |

## Logic

1. Read `task_subject` and `teammate_name` from stdin
2. Only validate for agents that write code: `coding-agent`, `fix-agent`, `qa-agent`
3. Run `go test ./...`
4. If tests fail, exit 2 — agent must fix before completing
5. Otherwise exit 0

## Script

```bash
#!/bin/bash
INPUT=$(cat)
TEAMMATE=$(echo "$INPUT" | jq -r '.teammate_name // empty')

# Only validate code-producing agents
case "$TEAMMATE" in
  coding-agent|fix-agent|qa-agent) ;;
  *) exit 0 ;;
esac

if ! go test ./... > /dev/null 2>&1; then
  echo "Tests failing. Fix before marking complete." >&2
  exit 2
fi
exit 0
```

## Acceptance Criteria

- coding-agent, fix-agent, qa-agent tasks with failing tests cannot be marked complete
- review-agent and ci-agent tasks are not blocked by this hook
- Agent receives feedback message and continues working
