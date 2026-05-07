# Module: `scripts/hooks`

**Package role:** Claude Code agent team quality gate hooks. Shell scripts that enforce rules at task boundaries during pipelined issue processing.

> _Updated 2026-04-19 (Phase 2 implementation)._

---

## Files

| File | Hook Event | Purpose |
|:---|:---|:---|
| `validate-task-complete.sh` | `TaskCompleted` | Blocks task completion if `go test ./...` fails |
| `check-teammate-idle.sh` | `TeammateIdle` | Redirects idle teammates to pending work at their pipeline stage |
| `review-loop-guard.sh` | `TaskCreated` | Caps review rounds at 3 per issue, escalates to human |

## Registration

All hooks are registered in `.claude/settings.json` under the `hooks` key. Each hook event maps to one script:

```json
{
  "hooks": {
    "TaskCompleted": [{ "hooks": [{ "type": "command", "command": "./scripts/hooks/validate-task-complete.sh" }] }],
    "TeammateIdle":  [{ "hooks": [{ "type": "command", "command": "./scripts/hooks/check-teammate-idle.sh" }] }],
    "TaskCreated":   [{ "hooks": [{ "type": "command", "command": "./scripts/hooks/review-loop-guard.sh" }] }]
  }
}
```

## Hook Protocol

All hooks follow the same contract:

1. **Input:** JSON on stdin with `hook_event_name`, `teammate_name`, `task_subject`, `cwd`, etc.
2. **Exit 0:** Action proceeds (task completes, teammate idles, task created)
3. **Exit 2:** Action blocked. Stderr is fed back to the agent as guidance.
4. **CWD:** Hooks run from the project root. Each script explicitly `cd`s to the agent's `cwd` from the input JSON to ensure commands run in the correct worktree.

## Script Details

### validate-task-complete.sh

**Event:** `TaskCompleted`

**Scope:** Only validates code-producing agents (`coding-agent`, `fix-agent`, `qa-agent`). Review-agent and ci-agent are not blocked.

**Logic:**
1. Parse `teammate_name` and `cwd` from stdin JSON
2. Skip non-code agents via `case` statement
3. `cd` to the agent's working directory (worktree)
4. Run `go test ./...` — capture output
5. If tests fail: exit 2 with failure details (last 20 lines of test output)
6. If tests pass: exit 0

**Dependencies:** `jq`, `go`

### check-teammate-idle.sh

**Event:** `TeammateIdle`

**Scope:** All 5 pipeline agents. Unknown agent names exit 0 (no-op).

**Logic:**
1. Parse `teammate_name` and `cwd` from stdin JSON
2. Map agent to its pipeline label(s):
   - `coding-agent` -> `pr_feedback` (priority) then `grava ready` (new work)
   - `review-agent` -> `code_review`
   - `fix-agent` -> `changes_requested`
   - `qa-agent` -> `reviewed`
   - `ci-agent` -> `qa_passed` (priority) then `pr_created` (monitoring)
3. Query grava for count of pending work
4. If count > 0: exit 2 with message
5. If count = 0: exit 0 (let idle)

**Dependencies:** `jq`, `grava search --label` (Phase 3 — not yet implemented; defaults to 0 on failure)

### review-loop-guard.sh

**Event:** `TaskCreated`

**Scope:** Any task whose `task_subject` contains a grava issue ID pattern (`grava-[a-f0-9]+`).

**Logic:**
1. Parse `task_subject` and `cwd` from stdin JSON
2. Extract issue ID via regex; exit 0 if none found
3. Count `[REVIEW]` comments on the issue via `grava show --json`
4. If count >= 3:
   - Post `[ESCALATION]` comment
   - Add `needs_human` label
   - Commit and export issue state for cross-machine sync
   - Exit 2 to block task creation
5. If count < 3: exit 0

**Dependencies:** `jq`, `grava` (show, comment, label, commit, export)

## Agent-to-Label Mapping

| Agent | Checks Label | Fallback |
|:---|:---|:---|
| `coding-agent` | `pr_feedback` | `grava ready --limit 1` |
| `review-agent` | `code_review` | — |
| `fix-agent` | `changes_requested` | — |
| `qa-agent` | `reviewed` | — |
| `ci-agent` | `qa_passed` | `pr_created` |

## Known Limitations

1. **Phase 3 dependency:** `check-teammate-idle.sh` uses `grava search --label` which is not yet implemented. Script handles failure gracefully (defaults to count 0, allows idle).
2. **Issue ID extraction:** `review-loop-guard.sh` depends on task subjects containing a grava issue ID. Tasks named without the ID pattern bypass the guard silently.
3. **Test scope:** `validate-task-complete.sh` runs `go test ./...` which tests the entire module. For large codebases, this could be slow. A targeted test run (only changed packages) would be faster.
