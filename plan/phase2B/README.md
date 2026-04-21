# Phase 2B: Agent Team Pipeline (Skills-Integrated)

Multi-agent team that takes grava issues through code → review → PR → merge via the existing `.claude/skills/` library. Agents orchestrate; skills execute.

Source: split from `archive/agent-team-plan-v2.md` for manageability. Companion issue-gen doc: `archive/agent-team-gen-issues.md`.

## Architecture

| File | Scope |
|------|-------|
| [agent-team-implementation-plan.md](agent-team-implementation-plan.md) | Overview, topology, signals, parallel teams, worktree mgmt, failure modes, maintenance |

## Stories — Agents (`.claude/agents/`)

| Story | File | Description |
|-------|------|-------------|
| [2B.1](story-2B.1-coder-agent.md) | `.claude/agents/coder.md` | Claim issue + implement via `grava-claim` → `grava-dev-epic` |
| [2B.2](story-2B.2-reviewer-agent.md) | `.claude/agents/reviewer.md` | Review `last_commit` via `grava-code-review` |
| [2B.3](story-2B.3-bug-hunter-agent.md) | `.claude/agents/bug-hunter.md` | Audit codebase via `grava-bug-hunt` |
| [2B.4](story-2B.4-planner-agent.md) | `.claude/agents/planner.md` | Doc → issue hierarchy via `grava-gen-issues` |

## Stories — Orchestrator Skills (`.claude/skills/`)

| Story | File | Description |
|-------|------|-------------|
| [2B.5](story-2B.5-skill-ship.md) | `.claude/skills/ship/SKILL.md` | `/ship <id>` — single-issue pipeline (code → review → PR → merge) |
| [2B.6](story-2B.6-skill-ship-all.md) | `.claude/skills/ship-all/SKILL.md` | `/ship-all [team]` — autopilot drain of backlog |
| [2B.7](story-2B.7-skill-plan.md) | `.claude/skills/plan/SKILL.md` | `/plan <doc>` — invoke planner agent |
| [2B.8](story-2B.8-skill-hunt.md) | `.claude/skills/hunt/SKILL.md` | `/hunt [scope]` — invoke bug-hunter agent |

## Stories — Hooks (`scripts/hooks/`)

| Story | File | Description |
|-------|------|-------------|
| [2B.9](story-2B.9-hook-sync-pipeline-status.md) | `scripts/hooks/sync-pipeline-status.sh` | PostToolUse: parse pipeline signals → grava wisps |
| [2B.10](story-2B.10-hook-warn-in-progress.md) | `scripts/hooks/warn-in-progress.sh` | Stop: warn on in-progress issues (per-team) |
| [2B.11](story-2B.11-settings-and-claude-md.md) | `.claude/settings.json` + `CLAUDE.md` | Register hooks; document agent team |

## Prerequisites

- All 7 existing skills resolve: `grava-cli`, `grava-claim`, `grava-dev-epic`, `grava-code-review`, `grava-next-issue`, `grava-bug-hunt`, `grava-gen-issues`
- `gh` CLI authenticated (`gh auth status`)
- `jq` available (already used by Phase 2 hooks)
- `.worktree/` in `.gitignore`

## Implementation Order

| Step | Action | Validates |
|------|--------|-----------|
| 1 | 2B.1-2B.4 agent files | Agents resolve skills correctly |
| 2 | 2B.5-2B.8 skills | End-to-end pipeline |
| 3 | 2B.9-2B.10 hooks | Wisp state tracking |
| 4 | 2B.11 settings + docs | Hooks fire, CLAUDE.md current |
| 5 | Smoke test: `/ship` on low-risk issue | Full pipeline works |
| 6 | Smoke test: `/plan` on small spec | Planner agent works |
| 7 | Smoke test: `/hunt` on single package | Bug hunter works |
| 8 | Production: `/ship-all` end-to-end | Autopilot works |
| 9 | Stress: 2-3 terminals on same backlog | Claim contention works |

## Key Design Decisions

- **Agents orchestrate, skills execute** — logic stays in `.claude/skills/`, sequencing in agents
- **Grava owns worktrees** — `grava claim` provisions `.worktree/<id>` on branch `grava/<id>`; we don't use Claude Code's `isolation: "worktree"` param
- **Signals are strings in the final agent message** — orchestrator parses them, no structured return
- **PR merge = issue done** — Phase 4 polls until merge, resolves comments up to 3 rounds
- **Parallel teams via atomic `grava claim`** — contention handled at the DB layer; each team in its own worktree
