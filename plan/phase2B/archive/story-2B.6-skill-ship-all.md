# Story 2B.6: /ship-all Skill — Autopilot

Wraps the `grava-next-issue` discover loop around `/ship`'s pipeline. Supports multi-terminal parallel teams via atomic `grava claim`. Includes `gh auth` preflight, autopilot-mode planner invocation, and a team-identity audit trail (`team_history`) so stale-lock takeovers are not silent.

## File

`.claude/skills/ship-all/SKILL.md`

## Frontmatter

```yaml
---
name: ship-all
description: "Autopilot — drain the entire grava backlog through the full pipeline."
user-invocable: true
---
```

## Usage

```
/ship-all [team-name] [--epic <id>] [--label <name>]
```

Examples:
- `/ship-all` — solo team (team name = "solo")
- `/ship-all alpha` — team alpha
- `/ship-all bravo --epic grava-epic-001` — team bravo, scoped to one epic
- `/ship-all charlie --label backend` — team charlie, scoped by label

## Setup — Parse Arguments

```bash
# First positional arg = team name. Remaining = flags.
set -- $ARGUMENTS
TEAM_NAME="${1:-solo}"
shift 2>/dev/null || true

SCOPE_EPIC=""; SCOPE_LABEL=""
while [ $# -gt 0 ]; do
  case "$1" in
    --epic)  SCOPE_EPIC="$2"; shift 2 ;;
    --label) SCOPE_LABEL="$2"; shift 2 ;;
    *) shift ;;
  esac
done

echo "Team: $TEAM_NAME  Epic: ${SCOPE_EPIC:-any}  Label: ${SCOPE_LABEL:-any}"

# preflight before draining backlog
./scripts/preflight-gh.sh || exit 1
```

## Workflow

Read `.claude/skills/grava-next-issue/SKILL.md` for loop semantics.

Repeat until stop condition met:

### 1. Discover

The pipeline only ships **leaf-type** issues (`task`, `bug`) — non-leaf candidates would HALT the coder on scope-check and burn through the consecutive-HALT stop budget. `grava ready` has no `--type` flag, so filter client-side:

```bash
grava ready --limit 10 --json | jq '[.[] | select(.Node.Type == "task" or .Node.Type == "bug")]'
```

Apply scope filters if set:
- `SCOPE_EPIC` → filter candidates to children of that epic (grava tree)
- `SCOPE_LABEL` → filter to issues with that label

Apply the skill's stale-state check on each candidate:
- Skip if `code_review` label present
- Skip if implementation comments exist
- Skip if claim fails due to stale heartbeat lock (other team has it)

If all 3 fail → fetch next batch. If still empty:

```bash
grava list --status open --json
grava list --status in_progress --json
grava stats --json
```

Print the session summary and stop.

### 1b. Planner-derived backlog top-up

If the discover step finds zero claimable issues but a `_global planner_needs_input` wisp is set, surface it in the session summary so the human knows where the planner stalled:

```bash
PLANNER_GAP=$(grava wisp read _global planner_needs_input 2>/dev/null)
if [ -n "$PLANNER_GAP" ]; then
  echo "Note: planner stalled on $PLANNER_GAP — run /plan interactively to unblock."
fi
```

If the scope is a doc that requires planning before issues exist, `/ship-all` invokes the planner agent in autopilot mode:

```
Agent({
  description: "Plan $DOC_PATH (autopilot)",
  subagent_type: "planner",
  prompt: "DOC_PATH: $DOC_PATH. MODE: autopilot. Output PLANNER_DONE or PLANNER_NEEDS_INPUT as last line."
})
```

On `PLANNER_NEEDS_INPUT: ...`: record it, do **not** halt the pipeline, continue to the next claimable issue.

### 2. Ship the chosen issue

Record team ownership with audit trail:

```bash
# grava claim --team is the authoritative ownership write — it rejects collision
# with another team's active heartbeat. Stale-lock release is allowed but writes
# the prior team into team_history first.
grava claim "$ISSUE_ID" --team "$TEAM_NAME" || {
  echo "Skip: $ISSUE_ID claim contention"; continue
}

OLD_TEAM=$(grava wisp read "$ISSUE_ID" team 2>/dev/null)
if [ -n "$OLD_TEAM" ] && [ "$OLD_TEAM" != "$TEAM_NAME" ]; then
  PRIOR_HISTORY=$(grava wisp read "$ISSUE_ID" team_history 2>/dev/null)
  # Cap audit chain at last 5 transitions
  NEW_HISTORY="${OLD_TEAM}@$(date -u +%s); ${PRIOR_HISTORY}"
  NEW_HISTORY=$(printf '%s' "$NEW_HISTORY" | awk -F';' '{for(i=1;i<=5&&i<=NF;i++)printf "%s%s",$i,(i<5&&i<NF?";":"")}')
  grava wisp write "$ISSUE_ID" team_history "$NEW_HISTORY"
fi
grava wisp write "$ISSUE_ID" team "$TEAM_NAME"
grava commit -m "team $TEAM_NAME took $ISSUE_ID"
```

> **Dependency:** `grava claim --team <name>` flag is required by Fix 12. If the CLI does not yet support it, the `/ship-all` story is blocked on a CLI change ticket. Document this in the README prereqs.

Then run the full /ship pipeline inline (Phases 1-4 from Story 2B.5):
- Spawn `coder` — prompt prefix: `"You are on team $TEAM_NAME. ..."`
- Spawn `reviewer` (up to 3 rounds)
- Create PR via the `pr-creator` agent (Story 2B.14) — title prefix `[$TEAM_NAME]`
- Phase 4 = handoff to `pr-merge-watcher.sh` (Story 2B.12); `/ship-all` does NOT poll here

### 3. Loop

After /ship completes (success, halt, or fail):
- Print: `--- Issue <id> done. Checking for next... ---`
- Loop back to Discover

## Stop Conditions

- Backlog drained (no claimable issues)
- 3 consecutive `PIPELINE_HALTED` (avoid infinite halt loop)
- 3 consecutive `PLANNER_NEEDS_INPUT` (planner repeatedly stalled — surface to human)
- User interrupt

## Aggregating per-issue results

Each `/ship` invocation now exits after PR creation (`PIPELINE_HANDOFF`). `/ship-all` treats `PIPELINE_HANDOFF` and `PIPELINE_COMPLETE` both as "this issue is no longer my problem" and advances. Final summary distinguishes them.

## Final Summary

```
--- /ship-all [$TEAM_NAME] Complete ---
Issues with PR handed off to watcher: <count>
  - <id>: <title> → PR: <pr-url>
Issues halted: <count>
  - <id>: <reason>
Issues skipped (claimed by other team): <count>
Planner gaps recorded: <count>
  - <doc-path>: <missing>
Stopped because: <reason>
```

## Parallel Team Launch

```bash
# Terminal 1
claude          # then: /ship-all alpha

# Terminal 2
claude          # then: /ship-all bravo

# Terminal 3
claude          # then: /ship-all charlie
```

Claim contention is resolved atomically by `grava claim`. Each team's worktree is at `.worktree/<its-claimed-id>`, no cross-team conflicts during work. Merge conflicts surface at merge time (human-resolved).

## Acceptance Criteria

- `gh` auth missing → exits before discover
- `/ship-all` with no args runs solo mode (TEAM_NAME=solo)
- `/ship-all alpha` claims via `grava claim --team alpha` and records `team: alpha` wisp
- Stale-lock release writes prior team into `team_history` (capped 5 entries) before overwriting `team`
- `--epic` and `--label` scope filters are applied to discover step
- Two terminals running `/ship-all X` and `/ship-all Y` on same backlog do not duplicate work (atomic claim)
- After 3 consecutive PIPELINE_HALTED **or** 3 consecutive PLANNER_NEEDS_INPUT, autopilot stops
- Planner invoked under `MODE=autopilot` — `PLANNER_NEEDS_INPUT` does NOT crash the pipeline
- Final summary lists PR-handoff counts, halts, skips, planner gaps with reasons
- PR title includes `[team-name]` prefix when team is non-solo

## Dependencies

- Story 2B.5 (/ship skill — inline pipeline; phases 1–3 + handoff)
- Story 2B.12 (pr-merge-watcher — owns post-handoff merge tracking)
- Story 2B.14 (pr-creator agent — Phase 3)
- `.claude/skills/grava-next-issue/` (exists)
- Stories 2B.1-2B.4 (coder, reviewer, bug-hunter, planner agents)
- `scripts/preflight-gh.sh`
- CLI dependency: `grava claim --team <name>` flag — separate CLI ticket if not present

## Signals Emitted

- Aggregates from 2B.5 per-issue; final summary is the skill's exit output.
