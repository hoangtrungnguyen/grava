# Story 2B.9: Sync Pipeline Status Hook

PostToolUse hook that captures pipeline signals in Bash tool output and syncs them to grava wisps for crash recovery. Fires after every `Bash` tool call.

## File

`scripts/hooks/sync-pipeline-status.sh`

## Hook Event

`PostToolUse` — matcher `Bash`. Runs after the Bash tool returns.

### Input (stdin JSON)

```json
{
  "hook_event_name": "PostToolUse",
  "tool_name": "Bash",
  "tool_input": { "command": "grava claim grava-abc123" },
  "tool_output": "<stdout from the command>",
  "cwd": "/path/to/repo/.worktree/grava-abc123",
  "session_id": "..."
}
```

### Exit Codes

| Code | Behavior |
|------|----------|
| 0 | Hook always exits 0 (non-blocking) |

## Logic

1. Read `tool_name`, `tool_output`, `cwd`, `tool_input.command` from stdin
2. Only process `Bash` tool events with non-empty output
3. Resolve `ISSUE_ID` with three fallbacks:
   - (a) Extract from `cwd` if path matches `.worktree/<id>/` (primary — works for parallel teams)
   - (b) Extract from explicit `grava <cmd> <id>` in the Bash command
   - (c) Scan output for `grava-xxxx` pattern
4. If no ID resolvable, exit silently
5. Match pipeline signal strings in output → write corresponding wisp

## Why cwd-based (not `grava list --status in_progress`)

With 3 parallel teams there are 3 in-progress issues. Using `jq '.[0].id'` on `grava list` would attribute signals to a random team's issue. Reading the cwd (which is inside the agent's worktree) gives the correct per-team issue.

## Script

```bash
#!/bin/bash
# PostToolUse hook — captures pipeline signals in Bash output and
# syncs them to grava wisps for crash recovery.

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
OUTPUT=$(echo "$INPUT" | jq -r '.tool_output // empty')
CWD=$(echo "$INPUT" | jq -r '.cwd // empty')
TOOL_CMD=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# Only process Bash tool output
[ "$TOOL_NAME" = "Bash" ] || exit 0
[ -n "$OUTPUT" ] || exit 0

# Resolve ISSUE_ID from cwd if we're inside a grava worktree.
ISSUE_ID=""
if [[ "$CWD" =~ /\.worktree/([a-zA-Z0-9_-]+)(/|$) ]]; then
  ISSUE_ID="${BASH_REMATCH[1]}"
fi

# Fallback 1: extract from explicit `grava <cmd> <id>` in the bash command
if [ -z "$ISSUE_ID" ] && [ -n "$TOOL_CMD" ]; then
  ISSUE_ID=$(echo "$TOOL_CMD" | grep -oE 'grava[[:space:]]+(claim|show|comment|label|wisp|stop|close|commit)[[:space:]]+[a-zA-Z0-9_-]+' | head -1 | awk '{print $3}')
fi

# Fallback 2: scan output for embedded grava-id
if [ -z "$ISSUE_ID" ]; then
  ISSUE_ID=$(echo "$OUTPUT" | grep -oE 'grava-[a-zA-Z0-9]{4,}' | head -1)
fi

# Nothing to sync — exit silently
[ -n "$ISSUE_ID" ] || exit 0

# Match pipeline signals → grava wisp state
case "$OUTPUT" in
  *CODER_DONE*)           grava wisp write "$ISSUE_ID" pipeline_phase coding_complete 2>/dev/null ;;
  *CODER_HALTED*)         grava wisp write "$ISSUE_ID" pipeline_phase coding_halted 2>/dev/null ;;
  *REVIEWER_APPROVED*)    grava wisp write "$ISSUE_ID" pipeline_phase review_approved 2>/dev/null ;;
  *REVIEWER_BLOCKED*)     grava wisp write "$ISSUE_ID" pipeline_phase review_blocked 2>/dev/null ;;
  *PR_CREATED*)           grava wisp write "$ISSUE_ID" pipeline_phase pr_created 2>/dev/null ;;
  *PR_COMMENTS_RESOLVED*) grava wisp write "$ISSUE_ID" pipeline_phase pr_comments_resolved 2>/dev/null ;;
  *PR_MERGED*)            grava wisp write "$ISSUE_ID" pipeline_phase pr_merged 2>/dev/null ;;
  *PIPELINE_HALTED*)      grava wisp write "$ISSUE_ID" pipeline_phase halted_human_needed 2>/dev/null ;;
  *PIPELINE_FAILED*)      grava wisp write "$ISSUE_ID" pipeline_phase failed 2>/dev/null ;;
  *PIPELINE_COMPLETE*)    grava wisp write "$ISSUE_ID" pipeline_phase complete 2>/dev/null ;;
esac

exit 0
```

## Acceptance Criteria

- Hook is non-blocking: always exits 0, never breaks a Bash call
- Issue ID resolves correctly when cwd is under `.worktree/<id>/`
- Issue ID resolves from explicit `grava` command when cwd isn't a worktree
- Multiple parallel teams each have their own wisps correctly updated (no cross-team mixing)
- `pipeline_phase` wisp reflects the latest signal seen
- Unknown/no signals → no wisp write, exits silently

## Dependencies

- `jq` installed (already required by Phase 2 hooks)
- `grava` CLI on PATH
- Story 2B.11 registers this hook in `.claude/settings.json`

## Test Plan

- Run `grava claim <id>` → wisp `pipeline_phase` should NOT change (not a signal)
- Run a command inside `.worktree/<id>` that echoes `CODER_DONE: abc123` → wisp should update to `coding_complete`
- Run same command outside any worktree with explicit `grava show <id>` present → wisp should still update (fallback 1)
- 2 terminals, each in its own worktree, emitting different signals → each issue gets the right wisp
