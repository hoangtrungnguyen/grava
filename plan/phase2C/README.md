# Phase 2C: Ship Loop — Backlog Drain

Two skills that together form a self-driving backlog drain:
- **`/ship-loop`** — drains all current ready issues by invoking `/ship` per issue until the queue is empty.
- **`/watch-drain`** — periodic poll daemon that watches the database for new ready issues and triggers `/ship-loop` automatically when any appear.

Successor to the archived `ship-all` (phase2B) which was too complex (team management, `grava claim --team`, inlined pipeline phases). Phase 2C stays shallow: each layer delegates to the one below it.

```
/watch-drain  →  /ship-loop  →  /ship  →  coder / reviewer / pr-creator agents
(periodic poll)   (drain loop)   (single issue)
```

## Architecture

| File | Scope |
|------|-------|
| [story-2C.1-skill-ship-loop.md](story-2C.1-skill-ship-loop.md) | `/ship-loop` skill — drain loop, stop conditions, summary |
| [story-2C.2-skill-watch-drain.md](story-2C.2-skill-watch-drain.md) | `/watch-drain` skill + `scripts/issue-drain-watcher.sh` — periodic issue watcher |

## Stories

| Story | File | Description | Status |
|-------|------|-------------|--------|
| [2C.1](story-2C.1-skill-ship-loop.md) | `.claude/skills/ship-loop/SKILL.md` | `/ship-loop` — invoke `/ship` in a loop until ready queue empty | Plan complete |
| [2C.2](story-2C.2-skill-watch-drain.md) | `.claude/skills/watch-drain/SKILL.md` + `scripts/issue-drain-watcher.sh` | `/watch-drain` — poll DB for new issues, trigger `/ship-loop` when found | Plan complete |

## Prerequisites

- Phase 2B fully landed (all `/ship` phases working, watcher running)
- `grava ready --json` returns leaf-type issues in priority order
- `/ship` skill exits after Phase 3 with `PIPELINE_HANDOFF` or a halt/fail signal
- Story 2C.1 (`/ship-loop`) must land before 2C.2 (`/watch-drain`)

## Implementation Order

| Step | Action | Validates |
|------|--------|-----------|
| 1 | Story 2C.1 — `/ship-loop` skill | Skill file resolves, invocation starts loop |
| 2 | Smoke: `/ship-loop` on populated backlog (≥2 ready issues) | Picks and ships issues sequentially, prints running summary |
| 3 | Smoke: `/ship-loop` on empty backlog | Immediate `PIPELINE_INFO`, clean exit |
| 4 | Negative: 3 consecutive halts → autopilot stops, prints halt summary | Stop-condition guard |
| 5 | Story 2C.2 — `/watch-drain` skill + cron script | Skill + script files resolve |
| 6 | Smoke: `/watch-drain` on empty DB → no drain triggered, polling log | Poll-then-skip path |
| 7 | Smoke: `/watch-drain` → add an issue → next poll triggers `/ship-loop` | End-to-end trigger path |
| 8 | Cron script: `issue-drain-watcher.sh` on populated backlog → lock file created, drain spawned | Cron mode |
| 9 | Cron script: concurrent run while lock exists → "already running" log, no duplicate spawn | Lock guard |

## Key Design Decisions

- **Layered delegation** — each skill only adds one concern; no phase inlining across layers
- **Re-discover per iteration** — `/ship-loop` re-queries `grava ready` after each issue; `/watch-drain` re-queries each poll cycle
- **No team management** — atomic `grava claim` inside `/ship` handles parallel-terminal contention
- **Halt budget ≠ failure budget** — consecutive HALTs stop `/ship-loop` (bad spec pattern); consecutive `PIPELINE_FAILED` stop `/watch-drain` (infra problem)
- **Cron for production** — `/watch-drain` skill blocks a Claude Code session; `issue-drain-watcher.sh` + cron is preferred for always-on unattended use. Both are specified in 2C.2.
