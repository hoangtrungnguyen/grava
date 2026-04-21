# Story 2B.6: /ship-all Skill — Autopilot

Wraps the `grava-next-issue` discover loop around `/ship`'s pipeline. Supports multi-terminal parallel teams via atomic `grava claim`.

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
```

## Workflow

Read `.claude/skills/grava-next-issue/SKILL.md` for loop semantics.

Repeat until stop condition met:

### 1. Discover

```bash
grava ready --limit 3 --json
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

### 2. Ship the chosen issue

Record team ownership:

```bash
grava wisp write $ISSUE_ID team "$TEAM_NAME"
grava commit -m "team $TEAM_NAME took $ISSUE_ID"
```

Then run the full /ship pipeline inline (Phases 1-4 from Story 2B.5):
- Spawn `coder` — prompt prefix: `"You are on team $TEAM_NAME. ..."`
- Spawn `reviewer` (up to 3 rounds)
- Create PR via `gh pr create` from `grava/$ISSUE_ID` branch, title prefix `[$TEAM_NAME]`
- Wait for PR merge with comment-fix loop (max 3 rounds)

### 3. Loop

After /ship completes (success, halt, or fail):
- Print: `--- Issue <id> done. Checking for next... ---`
- Loop back to Discover

## Stop Conditions

- Backlog drained (no claimable issues)
- 3 consecutive `PIPELINE_HALTED` (avoid infinite halt loop)
- User interrupt

## Final Summary

```
--- /ship-all [$TEAM_NAME] Complete ---
Issues with PR created: <count>
  - <id>: <title> → PR: <pr-url>
Issues halted: <count>
  - <id>: <reason>
Issues skipped (claimed by other team): <count>
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

- `/ship-all` with no args runs solo mode (TEAM_NAME=solo)
- `/ship-all alpha` records `team: alpha` wisp on every claimed issue
- `--epic` and `--label` scope filters are applied to discover step
- Two terminals running `/ship-all X` and `/ship-all Y` on same backlog do not duplicate work (atomic claim)
- After 3 consecutive PIPELINE_HALTED, autopilot stops to avoid infinite halt loop
- Final summary lists PR counts, halts, skips with reasons
- PR title includes `[team-name]` prefix when team is non-solo

## Dependencies

- Story 2B.5 (/ship skill — inline pipeline)
- `.claude/skills/grava-next-issue/` (exists)
- Stories 2B.1-2B.2 (coder, reviewer agents)

## Signals Emitted

- Aggregates from 2B.5 per-issue; final summary is the skill's exit output.
