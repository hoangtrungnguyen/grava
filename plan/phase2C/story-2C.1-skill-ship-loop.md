# Story 2C.1: /ship-loop Skill — Backlog Drain Loop

Thin loop controller that invokes `/ship` for each ready leaf-type issue until the queue is empty. Does not inline pipeline phases — all code/review/PR logic lives in `/ship` (story 2B.5).

## File

`.claude/skills/ship-loop/SKILL.md`

## Frontmatter

```yaml
---
name: ship-loop
description: "Drain the grava backlog: invoke /ship in a loop until no open ready issues remain."
user-invocable: true
---
```

## Usage

```
/ship-loop [--max-halts N] [--label <name>] [--epic <id>]
```

Examples:
- `/ship-loop` — drain entire ready queue, stop after 3 consecutive halts
- `/ship-loop --max-halts 1` — stop immediately on first halt (strict mode)
- `/ship-loop --label backend` — scope to issues with `backend` label
- `/ship-loop --epic grava-epic-001` — scope to children of one epic

## Setup

```bash
MAX_HALTS=3
SCOPE_LABEL=""
SCOPE_EPIC=""

for arg in $ARGUMENTS; do
  case "$arg" in
    --max-halts)   shift; MAX_HALTS="${1:-3}" ;;
    --label)       shift; SCOPE_LABEL="${1:-}" ;;
    --epic)        shift; SCOPE_EPIC="${1:-}" ;;
    --*)           echo "PIPELINE_FAILED: unknown flag $arg"; exit 1 ;;
  esac
done

# gh preflight — fail fast before draining the backlog
./scripts/preflight-gh.sh || exit 1

# Counters
SHIPPED=0
HALTED=0
FAILED=0
CONSECUTIVE_HALTS=0

# Per-issue audit trail (for final summary)
SHIPPED_IDS=""
HALTED_IDS=""
FAILED_IDS=""
```

## Discover Helper

Re-run before every iteration so newly-unblocked issues are picked up and already-claimed issues are skipped naturally.

```bash
discover_next() {
  local query_args="--limit 10 --json"

  CANDIDATES=$(grava ready $query_args 2>/dev/null \
    | jq '[.[] | select(.Node.Type == "task" or .Node.Type == "bug")]')

  # Apply scope filters if set
  if [ -n "$SCOPE_EPIC" ]; then
    CANDIDATES=$(echo "$CANDIDATES" \
      | jq --arg epic "$SCOPE_EPIC" '[.[] | select(.Node.ParentID == $epic)]')
  fi
  if [ -n "$SCOPE_LABEL" ]; then
    CANDIDATES=$(echo "$CANDIDATES" \
      | jq --arg lbl "$SCOPE_LABEL" '[.[] | select(.Node.Labels[]? == $lbl)]')
  fi

  echo "$CANDIDATES" | jq -r '.[0].Node.ID // empty'
}
```

## Main Loop

```bash
while true; do
  # --- Discover ---
  ISSUE_ID=$(discover_next)

  if [ -z "$ISSUE_ID" ]; then
    echo "PIPELINE_INFO: ready queue empty — backlog drained."
    STOP_REASON="queue empty"
    break
  fi

  echo "--- Shipping $ISSUE_ID (shipped=$SHIPPED halted=$HALTED failed=$FAILED) ---"
  grava wisp write "$ISSUE_ID" orchestrator_heartbeat "$(date -u +%s)"

  # --- Delegate to /ship ---
  SHIP_RESULT=$(Skill("ship", "$ISSUE_ID"))
  LAST=$(printf '%s' "$SHIP_RESULT" | awk 'NF{l=$0} END{print l}')

  # --- Parse last-line signal ---
  case "$LAST" in
    "PIPELINE_HANDOFF: "*)
      SHIPPED=$((SHIPPED + 1))
      CONSECUTIVE_HALTS=0
      SHIPPED_IDS="$SHIPPED_IDS $ISSUE_ID"
      echo "  ✓ $ISSUE_ID shipped → ${LAST#PIPELINE_HANDOFF: }"
      ;;

    "PIPELINE_COMPLETE: "*)
      # Issue was already done (re-entry on completed issue) — treat as shipped
      SHIPPED=$((SHIPPED + 1))
      CONSECUTIVE_HALTS=0
      SHIPPED_IDS="$SHIPPED_IDS $ISSUE_ID"
      echo "  ✓ $ISSUE_ID already complete"
      ;;

    "PIPELINE_HALTED: "*)
      HALTED=$((HALTED + 1))
      CONSECUTIVE_HALTS=$((CONSECUTIVE_HALTS + 1))
      HALTED_IDS="$HALTED_IDS $ISSUE_ID"
      echo "  ✗ $ISSUE_ID halted — ${LAST#PIPELINE_HALTED: }"
      if [ "$CONSECUTIVE_HALTS" -ge "$MAX_HALTS" ]; then
        STOP_REASON="$CONSECUTIVE_HALTS consecutive halts (needs-human pattern)"
        break
      fi
      ;;

    "PIPELINE_FAILED: "*)
      FAILED=$((FAILED + 1))
      CONSECUTIVE_HALTS=0   # failures are transient; don't count toward halt budget
      FAILED_IDS="$FAILED_IDS $ISSUE_ID"
      echo "  ✗ $ISSUE_ID failed — ${LAST#PIPELINE_FAILED: }"
      # Continue loop: transient infra errors shouldn't stop the drain session
      ;;

    "PIPELINE_INFO: "*)
      # /ship discovered the same empty queue — shouldn't happen since we just
      # discovered ISSUE_ID, but guard it anyway.
      echo "  ~ $ISSUE_ID: pipeline info — ${LAST#PIPELINE_INFO: }"
      STOP_REASON="pipeline info on discovered issue (unexpected empty queue)"
      break
      ;;

    *)
      FAILED=$((FAILED + 1))
      CONSECUTIVE_HALTS=0
      FAILED_IDS="$FAILED_IDS $ISSUE_ID"
      echo "  ✗ $ISSUE_ID signal parse failed — last line: $LAST"
      ;;
  esac
done
```

## Final Summary

```bash
echo ""
echo "--- /ship-loop Complete ---"
echo "Stopped because: ${STOP_REASON:-user interrupt}"
echo "Shipped (PR created/handed off): $SHIPPED"
[ -n "$SHIPPED_IDS" ] && echo "  IDs:$SHIPPED_IDS"
echo "Halted (needs human):            $HALTED"
[ -n "$HALTED_IDS" ] && echo "  IDs:$HALTED_IDS"
echo "Failed (signal/infra):           $FAILED"
[ -n "$FAILED_IDS" ] && echo "  IDs:$FAILED_IDS"
```

## Stop Conditions

| Condition | Behaviour |
|-----------|-----------|
| Ready queue empty | Clean exit — "backlog drained" |
| N consecutive `PIPELINE_HALTED` | Stop loop — bad spec pattern; operator needs to fix issues |
| Individual `PIPELINE_FAILED` | Log, **continue** — transient infra error; don't drain the session |
| User interrupt (Ctrl-C) | Print partial summary |

Why halt budget is consecutive, not total: a cluster of halts at the top of the queue suggests a systemic spec problem (all issues in this batch lack AC, or the precondition gate keeps firing). Interleaved halts (HALTED → SHIPPED → HALTED) are normal variance and should not stop a long drain session.

Why failures continue: `PIPELINE_FAILED` signals are parse errors or `gh pr create` transients. They don't indicate a bad backlog — retrying the next issue is the right call.

## Acceptance Criteria

- `/ship-loop` (no args) drains queue sequentially; each issue delegates to `/ship <id>` via Skill tool
- `/ship-loop` on empty backlog → immediate `PIPELINE_INFO: ready queue empty`, clean exit with summary (0/0/0)
- After `MAX_HALTS` consecutive halts → prints summary with stop reason "N consecutive halts", exit 0
- Individual `PIPELINE_FAILED` does NOT increment consecutive-halt counter and does NOT stop the loop
- `--label <name>` and `--epic <id>` scope filters applied in discover step
- `--max-halts 1` stops on first halt
- Final summary always printed: shipped/halted/failed counts + IDs
- `gh` auth missing → exits before first iteration (preflight)
- Re-discovery runs after every iteration — newly-unblocked issues (parent shipped in this session) are picked up without restart
- Two terminals running `/ship-loop` on the same backlog do not duplicate work (contention handled by atomic `grava claim` inside `/ship`)

## Dependencies

- Story 2B.5 (`/ship` skill — all phases implemented and landing `PIPELINE_HANDOFF` as last-line signal)
- `scripts/preflight-gh.sh` (from 2B.5)
- `grava ready --json` with leaf-type filtering
- `jq` on PATH

## Signals Emitted

- `PIPELINE_INFO: ready queue empty` — successful drain
- `PIPELINE_INFO: pipeline info on discovered issue` — unexpected guard (defensive)
- No new signals added to the protocol; `/ship-loop` is a consumer, not a new emitter
