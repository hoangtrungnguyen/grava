# Story 2B.9: Sync Pipeline Status Hook

PostToolUse hook that captures pipeline signals in Bash tool output and syncs them to grava wisps for crash recovery. Fires after every `Bash` tool call. Two correctness rules govern writes: signals must appear on the **last non-empty line** of the output (so body prose with signal substrings cannot trigger writes), and `pipeline_phase` advances are **forward-only** (so a re-spawned coder emitting `CODER_DONE` after `review_blocked` does not regress the wisp).

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
3. Cheap pre-filter: skip if output lacks any signal-shaped substring
4. Resolve `ISSUE_ID` with three fallbacks:
   - (a) Extract from `cwd` if path matches `.worktree/<id>/` (primary — works for parallel terminals)
   - (b) Extract from explicit `grava <cmd> <id>` in the Bash command
   - (c) Scan output for `grava-xxxx` pattern
5. If no ID resolvable, exit silently
6. **Fix 1 — last-line-only parse:** isolate the final non-empty line; signals must appear there. Body prose (test logs, this plan file's quoted examples, code review excerpts) cannot trigger a write.
7. **Fix 8 — idempotent forward-only write:** compare current `pipeline_phase` wisp before writing. Only advance forward in the canonical phase order. Terminal phases (`failed`, `halted_human_needed`) always overwrite.

## Why cwd-based (not `grava list --status in_progress`)

With multiple parallel terminals there can be many in-progress issues at once. Using `jq '.[0].id'` on `grava list` would attribute signals to a random one. Reading the cwd (which is inside the agent's worktree) gives the correct per-terminal issue.

## Phase Order

```
claimed → coding_complete → review_blocked → review_approved
       → pr_created → pr_awaiting_merge → pr_comments_resolved
       → pr_merged → complete
```

Terminal (overwrite anytime): `failed`, `halted_human_needed`, `coding_halted`, `planner_needs_input`.

`review_blocked` precedes `review_approved` because BLOCKED happens first in any round that needs re-work; APPROVED is the terminal "review passed" state. The `PHASE_ORDER` array in the script (below) is the canonical source — keep this comment in sync.

`review_blocked` is forward of `coding_complete` because the reviewer ran. A re-spawned coder emitting `CODER_DONE` after a BLOCKED round produces `coding_complete` again — that is a regression vs `review_blocked` and is rejected by the guard. Re-entry into review is signalled by the orchestrator (or by reviewer agent emitting APPROVED/BLOCKED again).

## Script

```bash
#!/bin/bash
# PostToolUse hook — captures pipeline signals in Bash output and
# syncs them to grava wisps for crash recovery.
# Last-line-only parse + forward-only phase order (terminal states overwrite).

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
OUTPUT=$(echo "$INPUT" | jq -r '.tool_output // empty')
CWD=$(echo "$INPUT" | jq -r '.cwd // empty')
TOOL_CMD=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# Only process Bash tool output
[ "$TOOL_NAME" = "Bash" ] || exit 0
[ -n "$OUTPUT" ] || exit 0

# Cheap pre-filter — skip output that obviously has no signal
echo "$OUTPUT" | grep -qE '(CODER_(DONE|HALTED)|REVIEWER_(APPROVED|BLOCKED)|PR_(CREATED|COMMENTS_RESOLVED|MERGED|FAILED)|PIPELINE_(COMPLETE|HALTED|FAILED)|PLANNER_NEEDS_INPUT)' || exit 0

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

# Fix 1: last-line-only parse — signals must be the final non-empty line
LAST_LINE=$(printf '%s' "$OUTPUT" | awk 'NF{l=$0} END{print l}')

NEW_PHASE=""
case "$LAST_LINE" in
  "CODER_DONE: "*)            NEW_PHASE="coding_complete" ;;
  "CODER_HALTED: "*)          NEW_PHASE="coding_halted" ;;
  "REVIEWER_APPROVED")        NEW_PHASE="review_approved" ;;
  "REVIEWER_BLOCKED: "*|"REVIEWER_BLOCKED") NEW_PHASE="review_blocked" ;;
  "PR_CREATED: "*)            NEW_PHASE="pr_created" ;;
  "PR_FAILED: "*)             NEW_PHASE="failed" ;;
  "PR_COMMENTS_RESOLVED: "*)  NEW_PHASE="pr_comments_resolved" ;;
  "PR_MERGED")                NEW_PHASE="pr_merged" ;;
  "PIPELINE_COMPLETE: "*)     NEW_PHASE="complete" ;;
  "PIPELINE_HALTED: "*)       NEW_PHASE="halted_human_needed" ;;
  "PIPELINE_FAILED: "*)       NEW_PHASE="failed" ;;
  "PLANNER_NEEDS_INPUT: "*)   NEW_PHASE="planner_needs_input" ;;
  *) exit 0 ;;
esac

# Fix 8: idempotent forward-only write
PHASE_ORDER=(claimed coding_complete review_blocked review_approved pr_created pr_awaiting_merge pr_comments_resolved pr_merged complete)
idx_of() {
  local target="$1" i=0
  for p in "${PHASE_ORDER[@]}"; do
    [ "$p" = "$target" ] && { echo "$i"; return; }
    i=$((i+1))
  done
  echo -1
}

CURRENT=$(grava wisp read "$ISSUE_ID" pipeline_phase 2>/dev/null)
WRITE=0
case "$NEW_PHASE" in
  failed|halted_human_needed|coding_halted|planner_needs_input)
    # Terminal / out-of-band states overwrite anything
    WRITE=1
    ;;
  *)
    NEW_IDX=$(idx_of "$NEW_PHASE")
    CUR_IDX=$(idx_of "$CURRENT")
    # Allow forward moves. Unknown CURRENT (-1) → allow.
    if [ "$NEW_IDX" -ge 0 ] && { [ "$CUR_IDX" -lt 0 ] || [ "$NEW_IDX" -gt "$CUR_IDX" ]; }; then
      WRITE=1
    fi
    ;;
esac

if [ "$WRITE" -eq 1 ]; then
  grava wisp write "$ISSUE_ID" pipeline_phase "$NEW_PHASE" 2>/dev/null
fi

exit 0
```

## Acceptance Criteria

- Hook is non-blocking: always exits 0, never breaks a Bash call
- **Fix 1**: a Bash command whose body prints `CODER_DONE: abc` mid-output but ends on an unrelated line does NOT update the wisp
- **Fix 1**: signals on the last non-empty line trigger a write
- **Fix 8**: re-spawned coder emitting `CODER_DONE` after `review_blocked` is rejected (no regression)
- **Fix 8**: terminal `failed` / `halted_human_needed` always overwrite
- Issue ID resolves correctly when cwd is under `.worktree/<id>/`
- Issue ID resolves from explicit `grava` command when cwd isn't a worktree
- Multiple parallel terminals each update their own issue's wisps correctly (no cross-issue mixing)
- Unknown / no signals → no wisp write, exits silently
- Pre-filter `grep -qE` short-circuits the hook on irrelevant Bash output (cheap path)

## Dependencies

- `jq` installed (already required by Phase 2 hooks)
- `grava` CLI on PATH
- Story 2B.11 registers this hook in `.claude/settings.json`

## Test Plan

- Run `grava claim <id>` → wisp `pipeline_phase` set to `claimed` (assuming claim emits that signal; otherwise no-op)
- Run a command inside `.worktree/<id>` whose final line is `CODER_DONE: abc123` → wisp updates to `coding_complete`
- Run a command whose body contains `CODER_DONE` but final line is `done` → wisp NOT updated
- After wisp is `review_blocked`, run command whose final line is `CODER_DONE: xyz` → wisp NOT regressed
- After wisp is `coding_complete`, emit `PIPELINE_FAILED: ...` → wisp overwritten to `failed` (terminal)
- 2 terminals, each in its own worktree, emitting different signals → each issue gets the right wisp
