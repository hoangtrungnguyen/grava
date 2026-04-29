# Agent Team Implementation Plan (Phase 2B)

Multi-agent team for the Grava workflow where each agent **delegates to existing skills** instead of inlining logic. Skills are the reusable instruction library; agents are the orchestration layer.

Individual deliverables live in sibling story files — this document is the architectural overview.

> **Architecture invariants:**
> - Phase 4 (merge poll) is **not** owned by `/ship` — it's an async cron job (`pr-merge-watcher.sh`, story 2B.12). `/ship` exits after PR creation with `PIPELINE_HANDOFF`.
> - PR creation lives in a dedicated `pr-creator` agent (story 2B.14), not inline bash.
> - Signal parser is last-line-only and forward-only; body prose can no longer trigger phase advance.
> - Planner is interactive only — blocks for human clarification on missing context. (An earlier autopilot mode was retired with `/ship-all`; see `archive/story-2B.6-skill-ship-all.md`.)
> - Backlog drain: `/ship` (no id) discovers the next ready leaf-type issue inline (Phase 0) and ships it. Rerun per issue. The standalone `grava-next-issue` skill is no longer wired into the pipeline; the `/ship-all` autopilot was archived.

---

## 1. Design Principle

> **Agents orchestrate, skills execute.**

Every skill in `.claude/skills/` already encodes domain expertise (TDD workflow, code review checklist, severity classification, etc.). Agents are thin wrappers that:
1. Receive context via their `prompt` parameter (no env vars — Claude Code doesn't pass them)
2. Read and follow the right skill at the right phase
3. Translate skill outputs into pipeline signals (`CODER_DONE`, `REVIEWER_APPROVED`, etc.)

> **API Note:** Agents are spawned via the **Agent tool** with `subagent_type` matching the agent's `name` field. Context flows through the `prompt` string and the grava DB (wisps for crash recovery).
>
> **Worktree ownership:** `grava claim <id>` provisions `.worktree/<id>` with branch `grava/<id>`. We do **NOT** use Claude Code's `isolation: "worktree"` param — grava owns worktrees. Agents `cd .worktree/<id>` after claim and work there. This gives persistent, predictable worktrees that survive across agent invocations (required for review-round-2 re-spawn and PR comment fix loops).
>
> **Relation to `claude-code-custom-worktree.md`:** That guide adds `WorktreeCreate` / `WorktreeRemove` hooks so `claude -w <name>` creates worktrees under `.worktree/`. The pipeline deliberately bypasses this — agents call `grava claim` directly for full lifecycle control. The custom worktree hook remains useful for ad-hoc exploration outside the pipeline (see Section 6).

---

## 2. Team Topology

```
┌────────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│  /ship <id>        │  │  /plan <doc>    │  │  /hunt [scope]  │
│  (single-issue;    │  │  (interactive)  │  │  (audit)        │
│   loop manually)   │  └────────┬────────┘  └────────┬────────┘
└──┬───────┬──────┬──┘           │                    │
   │       │      │              │                    │
   ▼       ▼      ▼              ▼                    ▼
[coder][reviewer][pr-creator] [planner]          [bug-hunter]
   │       │      │              │                    │
   ▼       ▼      ▼              ▼                    ▼
dev-task  code-  gh pr        gen-issues           bug-hunt
          review  create
                 + template

                  ↓ HANDOFF (no Claude Code, /ship only)
          ┌───────────────────────┐
          │  pr-merge-watcher.sh  │  cron / launchd
          │  (story 2B.12)        │  every 5 min
          └───────────────────────┘
                  │
                  ▼
        on merge → grava close → done
        on comments → wisp → /ship re-entry

           ┌────────────────────────┐
           │   Grava DB (shared)    │
           │  via .grava/redirect   │
           └────────────────────────┘
```

`/plan` and `/hunt` are siblings of `/ship` — not children. They run independently to populate the backlog (`/plan` from a doc, `/hunt` by auditing). The operator then drains the backlog one issue at a time via `/ship` (no id) — Phase 0 inside `/ship` discovers the next ready leaf-type issue, and the same skill ships it. The standalone `grava-next-issue` skill is no longer wired into the pipeline (kept available for ad-hoc terminal use).

### Skill ↔ Agent Mapping

| Agent | Primary Skill(s) | Triggered By |
|-------|------------------|--------------|
| `orchestrator` (`/ship`) | discover-then-ship inline (Phase 0 reads `grava ready --json`, filters to leaf types) | User runs `/ship` (no id) or `/ship <id>` |
| `planner` | `grava-gen-issues` | User uploads PRD/spec via `/plan` (interactive only) |
| `coder` | `grava-dev-task` (skill spec-checks then claims atomically) | Orchestrator Phase 1 |
| `reviewer` | `grava-code-review` | Orchestrator Phase 2 |
| `pr-creator` | (no skill — gh + template) | Orchestrator Phase 3 |
| `bug-hunter` | `grava-bug-hunt` | Manual `/hunt`, commit token, nightly cron (story 2B.15) |
| `pr-merge-watcher` (script, not agent) | (cron) | Async post-Phase-3 |

All agents preload `grava-cli` via the `skills: [grava-cli]` frontmatter field. This injects the CLI mental model into each agent's context automatically.

**Model policy: agents inherit from the orchestrator.** No agent frontmatter pins a `model` field. Whatever model the user runs `/ship` (or `/plan`, `/hunt`) under is the model every spawned sub-agent uses. Rationale: keeps the cost/quality knob in one place — flip the orchestrator's model and the whole pipeline follows. Per-agent pins drift over time and make it impossible to compare cohorts cleanly when evaluating a model upgrade.

---

## 3. File Layout

```
.claude/
├── agents/
│   ├── coder.md            ← 2B.1 — invokes grava-dev-task (claim folded into Step 3)
│   ├── reviewer.md         ← 2B.2 — invokes grava-code-review
│   ├── bug-hunter.md       ← 2B.3 — invokes grava-bug-hunt
│   ├── planner.md          ← 2B.4 — invokes grava-gen-issues
│   └── pr-creator.md       ← 2B.14 — pushes branch, opens PR
├── skills/
│   ├── ship/SKILL.md       ← 2B.5 — single-issue pipeline orchestrator
│   ├── plan/SKILL.md       ← 2B.7 — invoke planner agent
│   ├── hunt/SKILL.md       ← 2B.8 — invoke bug-hunter agent
│   └── (existing skills)   ← DO NOT MODIFY
├── hooks/
│   └── worktree.sh         ← OPTIONAL: from claude-code-custom-worktree.md
└── settings.json           ← 2B.11 — merge PostToolUse + Stop hooks

scripts/
├── preflight-gh.sh             ← 2B.5 — gh auth precheck
├── pr-merge-watcher.sh         ← 2B.12 — async merge tracker (cron)
├── pre-merge-check.sh          ← 2B.13 — local merge probe
├── run-pending-hunts.sh        ← 2B.15 — drains pending_hunt wisp
├── install-hooks.sh            ← 2B.15 — installs git hooks from scripts/git-hooks/
├── git-hooks/
│   └── commit-msg              ← 2B.15 — bug-hunt: <scope> token enqueue
└── hooks/
    ├── sync-pipeline-status.sh ← 2B.9 (Fixes 1, 8) — PostToolUse signal → wisp
    ├── warn-in-progress.sh     ← 2B.10 — Stop hook for orphan warning
    ├── validate-task-complete.sh  ← existing (Phase 2)
    ├── check-teammate-idle.sh     ← existing (Phase 2)
    └── review-loop-guard.sh       ← existing (Phase 2)

.github/workflows/
└── pre-merge-check.yml     ← 2B.13 — merged-with-main test on every grava/** push

.worktree/                  ← in .gitignore
├── grava-<id>/             ← grava claim (pipeline)
└── <scratch-name>/         ← optional: claude -w <scratch-name>

CLAUDE.md                   ← 2B.11 — Agent Team + Skill Map sections appended
```

---

## 4. Pipeline Signals (agent ↔ orchestrator contract)

Each agent emits exactly ONE signal as the **last non-empty line** of its final message. Body prose with signal-shaped substrings is rejected. Phase advance is forward-only — `sync-pipeline-status.sh` (story 2B.9) refuses regressions.

**Signal protocol version:** v1. Future renames bump to v2 with a `SIGNAL_PROTO: v2` preamble check.

| Signal | Emitter | Meaning |
|--------|---------|---------|
| `CODER_DONE: <sha>` | coder | grava-dev-task completed, code_review label set |
| `CODER_HALTED: <reason>` | coder | TDD or context loading hit blocker |
| `REVIEWER_APPROVED` | reviewer | grava-code-review verdict APPROVED |
| `REVIEWER_BLOCKED: <findings>` | reviewer | grava-code-review verdict CHANGES_REQUESTED |
| `PR_CREATED: <url>` | pr-creator | PR opened, ownership hands off to watcher |
| `PR_FAILED: <reason>` | pr-creator | Push or `gh pr create` failed |
| `PR_COMMENTS_RESOLVED: <round>` | orchestrator (re-entry) | Coder fixed PR feedback, CI passed |
| `PR_MERGED` | pr-merge-watcher | PR merged on GitHub; watcher closed the grava issue |
| `PIPELINE_HANDOFF: <id> ...` | orchestrator | `/ship` exiting; pr-merge-watcher owns from here |
| `PIPELINE_COMPLETE: <id>` | orchestrator (re-entry) | Wisp shows watcher already merged + closed |
| `PIPELINE_HALTED: <reason>` | orchestrator | Human intervention needed |
| `PIPELINE_FAILED: <reason>` | orchestrator | Signal parse failure or PR closed without merge |
| `PIPELINE_INFO: <reason>` | orchestrator | Re-entry no-op (e.g. still awaiting merge, no new comments) |
| `PLANNER_DONE` | planner | grava-gen-issues created N issues |
| `PLANNER_NEEDS_INPUT: <summary>` | planner | Spec missing required context — planner halts and asks human |
| `BUG_HUNT_COMPLETE` | bug-hunter | grava-bug-hunt filed N bug issues |

### Context Passing

Claude Code agents do NOT inherit environment variables from the parent. All context is passed via the Agent tool's `prompt` parameter.

| Context | How It's Passed | Example |
|---------|-----------------|---------|
| Issue ID | In `prompt` string | `"Implement issue grava-abc123..."` |
| Commit SHA | In `prompt` string (from prior agent result) | `"Last commit: a1b2c3d..."` |
| Review findings | Appended to `prompt` on re-spawn | `"Fix these findings:\n..."` |
| Worktree | grava-provisioned at `.worktree/<id>` | Agent `cd .worktree/$ISSUE_ID` after claim |

Agents read shared state from the grava DB via CLI (`grava show`, `grava wisp read`). This is the crash-recovery mechanism — wisps persist across sessions.

---

## 5. End-to-End Pipeline

```
code → review (max 3 rounds) → create PR → HANDOFF
                                              ↓
                                     pr-merge-watcher (cron)
                                              ↓
                                  on comments → /ship re-entry → fix loop (max 3) → ↑
                                  on merge → grava close → COMPLETE
```

### Phase breakdown (single issue)

| Phase | Actor | Signal out |
|-------|-------|-----------|
| 1. Code | coder agent | `CODER_DONE: <sha>` or `CODER_HALTED` |
| 2. Review | reviewer agent (looped with coder on BLOCKED) | `REVIEWER_APPROVED` or `PIPELINE_HALTED` after 3 rounds |
| 3. Create PR | pr-creator agent | `PR_CREATED: <url>` or `PR_FAILED` |
| 3.5. Handoff | orchestrator | `PIPELINE_HANDOFF: <id>` — `/ship` exits |
| 4. Async merge wait | `pr-merge-watcher.sh` (cron, story 2B.12) | (writes wisps; no signal) |
| 4a. Comment fix | `/ship` re-entry on `pr_new_comments` wisp | `PR_COMMENTS_RESOLVED: <round>` → re-handoff, OR `PIPELINE_HALTED` |
| 5. Close | watcher (`grava close` on merge) | `pipeline_phase=complete` wisp |

### Example flows

```
# Spec → backlog → ship (manual loop)
/plan docs/feature-spec.md     # planner → PLANNER_DONE (49 issues)
/ship                          # Phase 0 discovers grava-abc123 → coder → reviewer → PR → handoff
/ship                          # Phase 0 discovers grava-abc124 → repeat
/ship grava-abc127             # explicit id when you want to override the auto-pick

# Bug hunt → fix → ship
/hunt                          # bug-hunter → 8 bugs filed
/ship                          # Phase 0 picks highest-priority bug, ships it

# Single issue with review loop
/ship grava-abc123
# Round 1: coder, reviewer finds 2 CRITICAL → BLOCKED
# Round 2: coder fixes, reviewer finds 1 HIGH → BLOCKED
# Round 3: coder fixes, reviewer APPROVED → PR created
```

### Recovery after crash

```bash
grava doctor
# Shows grava-abc123 in_progress, last wisp pipeline_phase=coding_complete

/ship grava-abc123
# Coder sees code_review label + last_commit → skips Phase A → reviewer picks up
```

---

## 6. Worktree Management

Both grava and Claude Code can produce git worktrees — both land under `.worktree/`. The pipeline uses grava; Claude's custom hook (`claude-code-custom-worktree.md`) is available for ad-hoc work outside the pipeline.

| Path | Creator | Trigger | Branch | Lifecycle owner |
|------|---------|---------|--------|-----------------|
| Pipeline | `grava claim <id>` | `grava-dev-task` Step 3 (called by coder) | `grava/<id>` | grava (`grava close` removes) |
| Ad-hoc | Claude worktree hook | `claude -w <name>` | `worktree-<name>` | Claude hook (`WorktreeRemove`) |

### Why not merge the two paths?

`grava claim` does more than create a worktree:
- Atomic claim (prevents parallel terminals racing)
- Status transition to `in_progress`
- Prerequisite / dependency check
- Wisp heartbeat for crash recovery
- Branch naming convention (`grava/<id>`) — enables `grava close` cleanup

The Claude hook is a thin git-worktree wrapper. Keep them independent.

### Gotchas

1. **Don't run `claude -w grava-<id>`** for an issue in the pipeline — the hook would create branch `worktree-grava-<id>`, conflicting with `grava/grava-<id>`. Use `/ship <id>` or manual `grava claim <id>` instead.
2. **`.worktree/` must be in `.gitignore`**.
3. **Ad-hoc worktrees don't sync with grava** — no status updates, no wisps. Fine for exploration only.

---

## 7. Parallel Terminals

Multiple terminals can run `/ship` against the same backlog concurrently. Each terminal's Phase 0 discover reads the same `grava ready` queue, but the atomic `grava claim` inside `grava-dev-task` Step 3 guarantees only one terminal wins each issue — losers fall through and the next `/ship` invocation picks a different candidate. Each winner works in the worktree `grava claim` provisions at `.worktree/<id>`.

```
Terminal 1                Terminal 2                Terminal 3
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│ /ship grava-abc1 │     │ /ship grava-abc2 │     │ /ship grava-abc3 │
│ coder (worktree) │     │ coder (worktree) │     │ coder (worktree) │
│ reviewer         │     │ reviewer         │     │ reviewer         │
│ pr-creator → PR  │     │ pr-creator → PR  │     │ pr-creator → PR  │
│ HANDOFF, exit    │     │ HANDOFF, exit    │     │ HANDOFF, exit    │
└────────┬─────────┘     └────────┬─────────┘     └────────┬─────────┘
         └─────────────────────────┼─────────────────────────┘
                                   ▼
                      ┌────────────────────────┐
                      │    Grava DB (shared)   │
                      │ atomic claim contention│
                      └────────────────────────┘
```

### How contention is resolved

| Scenario | What Happens |
|----------|-------------|
| Two terminals claim same issue | `grava claim` is atomic — loser receives error, picks a different candidate |
| Two terminals modify same file | Each in its own worktree — no conflict during work. Resolved at merge time. |
| Branch can't merge cleanly | Expected. Human resolves at merge time. |

### Merge & Deploy (human-driven, outside pipeline)

PRs are the pipeline's output. Merge and deploy are separate:

```bash
grava list --label pr-created --json
gh pr list --state open
gh pr merge <pr-number> --merge
# Deploy when ready
go test -race ./...
gcloud run deploy grava --source . --region asia-southeast1 --quiet
```

---

## 8. Failure Modes & Recovery

| Failure | Detected By | Recovery |
|---------|-------------|----------|
| Coder skill HALT | `CODER_HALTED` signal | Issue returned to `open`, human notified |
| Coder crashed mid-impl | `grava doctor` orphan check | Re-run `/ship <id>` — coder reads wisp + commit, resumes |
| Stale claim (heartbeat lock) | `grava ready` open-only filter (status `in_progress` excludes them) | Phase 0 skips automatically. Recovery: `grava doctor` heals stale heartbeats → status returns to `open`, then re-discoverable. |
| Already-implemented in queue (awaiting review) | `grava ready` open-only filter (status `in_progress` after claim) | Phase 0 skips — claimed issues never appear, regardless of `code_review` label |
| Auto-picked issue lacks spec (no description / no AC) | `/ship` Phase 0 precondition gate | Halts with `PIPELINE_HALTED: ... failed precondition`, suggests `/ship <alt-id>`. Operator fixes spec or picks alternate. No silent fall-through. |
| Reviewer loop exhausted | 3 BLOCKED rounds | Issue labeled `needs-human`, pipeline halts |
| PR creation fails | `PIPELINE_FAILED: pr creation` | Check `gh auth`, push branch manually, re-run |
| PR comments unresolvable | `PIPELINE_HALTED: ...PR comments...` | Coder couldn't fix feedback after 3 rounds — `needs-human` |
| PR closed without merge | `PIPELINE_FAILED: PR closed without merge` | Re-open PR or re-run `/ship` from scratch |
| Planner gap (missing API) | grava-gen-issues validation | Skill blocks, asks user for clarification |
| Bug hunter empty | grava-bug-hunt finds nothing | Reports "0 bugs found" — codebase is clean |
| Claim contention (parallel terminals) | `grava claim` returns "already claimed" | Operator picks a different candidate |
| Merge conflict (at merge time) | `git merge` fails | Human resolves — branches preserved, no work lost |
| Cross-branch regression (at merge time) | `go test` fails on merged main | Human fixes before deploy |

---

## 9. Maintenance

### When to update skills vs agents

| Change Type | Update Skill | Update Agent |
|-------------|--------------|--------------|
| Add a TDD step | `grava-dev-task/workflow.md` | No change |
| Add a review check | `grava-code-review/SKILL.md` | No change |
| Add new pipeline phase | New skill OR new agent | Both — new agent invokes new skill |
| Change PR template | (no skill) | `ship/SKILL.md` Phase 3 |
| Change signal vocabulary | All affected agents + CLAUDE.md | Yes |

The rule: **logic lives in skills, sequencing lives in agents**. If the change is *what* an agent does, it's a skill update. If the change is *when* it does it, it's an agent or orchestrator update.

---

## See Also

- Story files (2B.1–2B.15) for each deliverable's implementation details
- `claude-code-custom-worktree.md` (repo root) — optional worktree hook for ad-hoc use
- Phase 2 (`plan/phase2/`) — prerequisite quality-gate hooks already in place
