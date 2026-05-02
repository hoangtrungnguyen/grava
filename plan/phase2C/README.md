# Phase 2C: Ship Loop — Backlog Drain

A `/ship-loop` skill that drains the entire grava backlog by invoking `/ship` in a loop until the ready queue is empty. Thin orchestration layer — all pipeline logic remains in `/ship`; this skill adds only the iteration control and stop-condition logic.

Successor to the archived `ship-all` (phase2B) which was too complex (team management, `grava claim --team`, inlined pipeline phases). Phase 2C deliberately stays shallow: discover → delegate to `/ship` → repeat.

## Architecture

| File | Scope |
|------|-------|
| [story-2C.1-skill-ship-loop.md](story-2C.1-skill-ship-loop.md) | `/ship-loop` skill — loop controller, stop conditions, summary |

## Stories

| Story | File | Description | Status |
|-------|------|-------------|--------|
| [2C.1](story-2C.1-skill-ship-loop.md) | `.claude/skills/ship-loop/SKILL.md` | `/ship-loop` — invoke `/ship` in a loop until ready queue empty | Plan complete |

## Prerequisites

- Phase 2B fully landed (all `/ship` phases working, watcher running)
- `grava ready --json` returns leaf-type issues in priority order
- `/ship` skill exits after Phase 3 with `PIPELINE_HANDOFF` or a halt/fail signal

## Implementation Order

| Step | Action | Validates |
|------|--------|-----------|
| 1 | Story 2C.1 — `/ship-loop` skill | Skill file resolves, invocation starts loop |
| 2 | Smoke: `/ship-loop` on populated backlog (≥2 ready issues) | Picks and ships issues sequentially, prints running summary |
| 3 | Smoke: `/ship-loop` on empty backlog | Immediate `PIPELINE_INFO`, clean exit |
| 4 | Negative: 3 consecutive halts → autopilot stops, prints halt summary | Stop-condition guard |

## Key Design Decisions

- **Delegate, don't duplicate** — `/ship-loop` never inlines pipeline phases; it invokes the `/ship` skill for each issue and parses the last-line signal
- **Re-discover per iteration** — queue is re-queried after each issue so newly-unblocked issues (parent merged) are included and already-claimed issues are skipped naturally
- **No team management** — removed from archived `/ship-all`; atomic `grava claim` inside `/ship` already handles parallel-terminal contention
- **Halt budget not failure budget** — consecutive HALTs stop the loop (bad spec pattern); individual FAILUREs are logged and the loop continues (transient infra errors shouldn't drain the whole session)
