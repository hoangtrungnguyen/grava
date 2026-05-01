# Phase 2B: Agent Team Pipeline (Skills-Integrated)

Multi-agent team that takes grava issues through code → review → PR → merge via the existing `.claude/skills/` library. Agents orchestrate; skills execute.

Source: split from `archive/agent-team-plan-v2.md` for manageability.

> **Status:** All 20 stories below are **plan complete** (specs ready for implementation). Stories with caveats are flagged in the `Status` column. Implementation has not started — execution follows the order in the [Implementation Order](#implementation-order) table.

## Architecture

| File | Scope |
|------|-------|
| [agent-team-implementation-plan.md](agent-team-implementation-plan.md) | Overview, topology, signals, parallel terminals, worktree mgmt, failure modes, maintenance |

## Stories — CLI & Skill Prerequisites

Four prereq gaps uncovered during the Phase 2B audit. Pipeline stories below reference CLI flags, subcommands, or skill workflow steps that don't exist yet. All must land **before** any agent / skill / hook story is implemented — otherwise the pipeline scripts crash on first call, or the heartbeat-stale alarm fires on every long Phase 1.

| Story | File | Description | Status |
|-------|------|-------------|--------|
| [2B.0a](story-2B.0a-cli-list-label.md) | `pkg/cmd/list.go` | Add `--label` flag to `grava list` (AND semantics, repeatable). Required by watcher (2B.12) for `pr-created` discovery. | Plan complete |
| [2B.0b](story-2B.0b-cli-wisp-delete.md) | `pkg/cmd/wisp.go` | Add `grava wisp delete <id> <key>` subcommand. Required by `/ship` Phase 4 resume (2B.5). The pending-hunt drain uses a file-based queue, not a wisp — wisp namespace is issue-scoped, no sentinel `_global`. | Plan complete |
| [2B.0c](story-2B.0c-cli-description-append.md) | `pkg/cmd/update.go` | Add `--description-append` / `--description-append-from-stdin` flags to `grava update`. Required by watcher (2B.12) CLOSED branch to record PR rejection notes onto the issue description. | Plan complete |
| [2B.0d](story-2B.0d-skill-heartbeat.md) | `.claude/skills/grava-dev-task/workflow.md` | Add `orchestrator_heartbeat` writes at each workflow checkpoint inside the skill. Without this, `grava doctor` flags every Phase 1 task >30 min as stale even when the coder is healthy. | Plan complete |

## Stories — Agents (`.claude/agents/`)

| Story | File | Description | Status |
|-------|------|-------------|--------|
| [2B.1](story-2B.1-coder-agent.md) | `.claude/agents/coder.md` | Implement issue via `grava-dev-task` (skill claims atomically with pre-check) | Plan complete |
| [2B.2](story-2B.2-reviewer-agent.md) | `.claude/agents/reviewer.md` | Review `last_commit` via `grava-code-review` | Plan complete |
| [2B.3](story-2B.3-bug-hunter-agent.md) | `.claude/agents/bug-hunter.md` | Audit codebase via `grava-bug-hunt` | Plan complete |
| [2B.4](story-2B.4-planner-agent.md) | `.claude/agents/planner.md` | Doc → issue hierarchy via `grava-gen-issues` | Plan complete |
| [2B.14](story-2B.14-pr-creator-agent.md) | `.claude/agents/pr-creator.md` | Push branch + `gh pr create` with template | Plan complete |

## Stories — Orchestrator Skills (`.claude/skills/`)

| Story | File | Description | Status |
|-------|------|-------------|--------|
| [2B.5](story-2B.5-skill-ship.md) | `.claude/skills/ship/SKILL.md` | `/ship [id]` — single-issue pipeline (discover when no id → code → review → PR → handoff) | Plan complete — **Note:** needs one-line addendum re: shell-resolvable `Agent` invocation (per 2B.16 line 71) before implementation |
| [2B.7](story-2B.7-skill-plan.md) | `.claude/skills/plan/SKILL.md` | `/plan <doc>` — invoke planner agent | Plan complete |
| [2B.8](story-2B.8-skill-hunt.md) | `.claude/skills/hunt/SKILL.md` | `/hunt [scope]` — invoke bug-hunter agent | Plan complete |

> **Backlog drain:** previously `/ship-all` autopilot. Archived (see `archive/story-2B.6-skill-ship-all.md`). Run `/ship` (no id) repeatedly — Phase 0 discovers the next ready leaf-type issue from the queue. The standalone `grava-next-issue` skill is no longer wired into the pipeline (kept available for ad-hoc terminal use only).

## Stories — Hooks & Async (`scripts/`)

| Story | File | Description | Status |
|-------|------|-------------|--------|
| [2B.9](story-2B.9-hook-sync-pipeline-status.md) | `scripts/hooks/sync-pipeline-status.sh` | PostToolUse: parse pipeline signals (last-line, forward-only) → grava wisps | Plan complete |
| [2B.10](story-2B.10-hook-warn-in-progress.md) | `scripts/hooks/warn-in-progress.sh` | Stop: warn on in-progress issues | Plan complete |
| [2B.11](story-2B.11-settings-and-claude-md.md) | `.claude/settings.json` + `CLAUDE.md` | Register hooks; document agent team & cron setup | Plan complete |
| [2B.12](story-2B.12-pr-merge-watcher.md) | `scripts/pr-merge-watcher.sh` | Async (cron) PR merge tracker — owns Phase 4 outside Claude Code | Plan complete |
| [2B.13](story-2B.13-pre-merge-check.md) | `scripts/pre-merge-check.sh` + `.github/workflows/pre-merge-check.yml` | Cross-branch regression catch (local probe + GH Action) | Plan complete |
| [2B.15](story-2B.15-hunt-scheduling.md) | `.git/hooks/commit-msg` + `scripts/run-pending-hunts.sh` + cron | Bug-hunt triggers (commit token, hourly drain, nightly) | Plan complete |

## Stories — Test Suites (`tests/`)

| Story | File | Description | Status |
|-------|------|-------------|--------|
| [2B.16](story-2B.16-ship-test-suite.md) | `gravav6-sandbox/sandbox/ship/` (pytest) + sandbox-repo CI | Deterministic test suite for `/ship` hosted in the sandbox repo: pytest, stub agents, disposable Dolt, phase + re-entry + contention coverage | Plan complete — module-scoped Dolt fixture relies on unique `grava-test-*` ids to avoid cross-test bleed |

## Stories — Distribution (`plugins/`)

| Story | File | Description | Status |
|-------|------|-------------|--------|
| [2B.17](story-2B.17-grava-plugin-install.md) | `.claude-plugin/marketplace.json` + `plugins/grava/` + `pkg/cmd/bootstrap.go` | Package grava as a Claude Code plugin (skills + agents + hooks bundled); `grava bootstrap` handles binary check, git hooks, cron print | Plan complete — **Note:** confirm marketplace name `grava` unreserved before first publish; fallback to `grava-pipeline` |

## Prerequisites

- All 5 pipeline-active skills resolve: `grava-cli`, `grava-dev-task`, `grava-code-review`, `grava-bug-hunt`, `grava-gen-issues` (claim folded into `grava-dev-task` Step 3; discover folded into `/ship` Phase 0). `grava-next-issue` remains in the library for ad-hoc human use but is not invoked by the pipeline.
- `gh` CLI authenticated (`gh auth status`) — `scripts/preflight-gh.sh` enforces
- `jq` available (already used by Phase 2 hooks)
- `.worktree/` in `.gitignore`
- `grava close --actor <name>` flag — already present (global `--actor` flag); no story needed
- `grava list --label` flag — story 2B.0a; lands before agent stories
- `grava wisp delete` subcommand — story 2B.0b; lands before agent stories
- `grava update --description-append` flag — story 2B.0c; lands before agent stories
- `grava-dev-task` workflow heartbeat writes — story 2B.0d; lands before agent stories
- Branch protection on `main` admin-set (story 2B.13 — manual one-time)
- `make setup` (or equivalent) calls `./scripts/install-hooks.sh` (story 2B.15) and prints cron lines

## Implementation Order

| Step | Action | Validates |
|------|--------|-----------|
| 1a | **Story 2B.0a — `grava list --label` flag** | Watcher discovery (`pr-created` filter) compiles |
| 1b | **Story 2B.0b — `grava wisp delete` subcommand** | `/ship` Phase 4 doesn't crash on stale-comment clear |
| 1c | **Story 2B.0c — `grava update --description-append` flag** | Watcher CLOSED branch records rejection notes to issue description; `/ship --retry` reads them as feedback |
| 1d | **Story 2B.0d — `grava-dev-task` heartbeat writes** | `grava doctor` does not false-flag Phase 1 work as stale; real crashes still surface within 30 min |
| 1e | Confirm prereq audit: 5 pipeline-active skills resolve (`grava-cli`, `grava-dev-task`, `grava-code-review`, `grava-bug-hunt`, `grava-gen-issues`); `gh auth status` clean; `.worktree/` in `.gitignore` | All downstream story dependencies met |
| 2 | 2B.1–2B.4 + 2B.14 agent files | Agents resolve skills correctly |
| 3 | 2B.5 + 2B.7 + 2B.8 skills + `scripts/preflight-gh.sh` | End-to-end single-issue pipeline (with handoff) |
| 4 | 2B.9 + 2B.10 hooks (last-line + forward-only parser) | Wisp state tracking correct under Fix 1 + 8 |
| 5 | 2B.13 pre-merge probe + GH Action | Cross-branch regression catch live |
| 6 | 2B.12 pr-merge-watcher + cron entry | Async merge tracking |
| 7 | 2B.15 commit-msg hook + run-pending-hunts cron | Bug-hunt triggers |
| 8 | 2B.11 settings + CLAUDE.md updates | Hooks fire, docs current |
| 9 | Smoke test: `/ship <id>` on low-risk issue → handoff → manual merge → watcher closes | Full happy path |
| 10 | Smoke test: `/ship` (no id) on populated queue → Phase 0 auto-picks top leaf-type without prompting, skips epic/phase, prints up-to-2 alts | Discover path (auto-pick contract) |
| 11 | Smoke test: `/ship` (no id) on empty queue → exits with `PIPELINE_INFO`, no agents spawned | Discover guard |
| 11a | Smoke test: `/ship` (no id) when top candidate has no description / no AC → exits with `PIPELINE_HALTED: ... failed precondition`, lists `/ship <alt-id>` suggestions + `--force` hint, no agents spawned | Precondition gate (auto-pick path) |
| 11b | Smoke test: `/ship <id>` on a precondition-failing issue → also exits with `PIPELINE_HALTED: ... failed precondition` (gate is always-on); `--force` hint shown | Precondition gate (explicit-id path) |
| 11c | Smoke test: `/ship <id> --force` on the same precondition-failing issue → bypasses gate, emits `PIPELINE_INFO: --force set; bypassing ...`, proceeds to Phase 1; coder may HALT downstream via grava-dev-task spec-check | Force override |
| 12 | Smoke test: `/plan` (interactive) on small spec | Planner agent works |
| 13 | Smoke test: `/hunt` on single package | Bug hunter works |
| 14 | Stress: ≥2 terminals running `/ship` (no id) on same backlog → each picks a different candidate (start with N=2; rerun at N=4 once N=2 is green) | Claim contention via atomic `grava claim` |
| 15 | Negative test: agent emits signal mid-output but unrelated last line → wisp NOT advanced | Fix 1 |
| 16 | Negative test: re-spawn coder emits CODER_DONE after review_blocked → wisp NOT regressed | Fix 8 |
| 17 | **Story 2B.16 — `/ship` test suite (bats + stub agents + disposable Dolt)** lands; `make test-ship` is required-status on PRs touching `/ship` path | Steps 9–16 become regression-gated, not manual-only |

## Key Design Decisions

- **Agents orchestrate, skills execute** — logic stays in `.claude/skills/`, sequencing in agents
- **Phase 4 lives outside Claude Code** — `pr-merge-watcher.sh` owns merge polling; `/ship` exits after Phase 3
- **Last-line-only signals** — orchestrator and hook reject signals that aren't the final non-empty line
- **Forward-only phase advance** — `sync-pipeline-status.sh` rejects regressions
- **Grava owns worktrees** — `grava claim` provisions `.worktree/<id>` on branch `grava/<id>`; we don't use Claude Code's `isolation: "worktree"` param
- **PR creation is an agent** — template/labels/reviewers live in one prompt, not scattered shell
- **PR merge = issue done** — watcher runs `grava close` on merge; `/ship` re-entry only triggers on new comments
- **Parallel terminals via atomic `grava claim`** — contention handled at the DB layer; each terminal works in its own worktree; cross-terminal collisions surface at merge time
- **Planner is interactive only** — blocks for clarification on missing context. (An earlier autopilot mode was retired with `/ship-all`.)
