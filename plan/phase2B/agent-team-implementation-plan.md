# Agent Team Implementation Plan (Phase 2B)

Multi-agent team for the Grava workflow where each agent **delegates to existing skills** instead of inlining logic. Skills are the reusable instruction library; agents are the orchestration layer.

Individual deliverables live in sibling story files — this document is the architectural overview.

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
┌──────────────────────────────────────────────────────────────┐
│                       ORCHESTRATOR                           │
│              /ship <id>  or  /ship-all (autopilot)           │
└──┬──────────┬──────────┬──────────┬──────────┬──────────────┘
   │          │          │          │          │
   ▼          ▼          ▼          ▼          ▼
[planner] [bug-hunter] [coder] [reviewer]
   │          │          │          │
   ▼          ▼          ▼          ▼
gen-issues bug-hunt   claim+      code-       → PR
                     dev-epic    review      (gh pr create)

           ┌────────────────────────┐
           │   Grava DB (shared)    │
           │  via .grava/redirect   │
           └────────────────────────┘
```

### Skill ↔ Agent Mapping

| Agent | Primary Skill(s) | Triggered By |
|-------|------------------|--------------|
| `orchestrator` | `grava-next-issue` (loop) | `/ship`, `/ship-all` |
| `planner` | `grava-gen-issues` | User uploads PRD/spec |
| `coder` | `grava-claim` → `grava-dev-epic` | Orchestrator Phase 1 |
| `reviewer` | `grava-code-review` | Orchestrator Phase 2 |
| (orchestrator) | `gh pr create` | Phase 3 — PR creation |
| `bug-hunter` | `grava-bug-hunt` | Periodic / manual |

All agents preload `grava-cli` via the `skills: [grava-cli]` frontmatter field. This injects the CLI mental model into each agent's context automatically.

---

## 3. File Layout

```
.claude/
├── agents/
│   ├── coder.md            ← 2B.1 — invokes grava-claim + grava-dev-epic
│   ├── reviewer.md         ← 2B.2 — invokes grava-code-review
│   ├── bug-hunter.md       ← 2B.3 — invokes grava-bug-hunt
│   └── planner.md          ← 2B.4 — invokes grava-gen-issues
├── skills/
│   ├── ship/SKILL.md       ← 2B.5 — single-issue pipeline orchestrator
│   ├── ship-all/SKILL.md   ← 2B.6 — autopilot via grava-next-issue
│   ├── plan/SKILL.md       ← 2B.7 — invoke planner agent
│   ├── hunt/SKILL.md       ← 2B.8 — invoke bug-hunter agent
│   └── (existing skills)   ← DO NOT MODIFY
├── hooks/
│   └── worktree.sh         ← OPTIONAL: from claude-code-custom-worktree.md
└── settings.json           ← 2B.11 — merge PostToolUse + Stop hooks

scripts/hooks/
├── sync-pipeline-status.sh ← 2B.9 — PostToolUse signal → wisp sync
├── warn-in-progress.sh     ← 2B.10 — Stop hook for orphan warning
├── validate-task-complete.sh  ← existing (Phase 2)
├── check-teammate-idle.sh     ← existing (Phase 2)
└── review-loop-guard.sh       ← existing (Phase 2)

.worktree/                  ← in .gitignore
├── grava-<id>/             ← grava claim (pipeline)
└── <scratch-name>/         ← optional: claude -w <scratch-name>

CLAUDE.md                   ← 2B.11 — Agent Team + Skill Map sections appended
```

---

## 4. Pipeline Signals (agent ↔ orchestrator contract)

Each agent emits exactly ONE signal as a literal string in its final message. The orchestrator parses that string to decide the next phase.

| Signal | Emitter | Meaning |
|--------|---------|---------|
| `CODER_DONE: <sha>` | coder | grava-dev-epic completed, code_review label set |
| `CODER_HALTED: <reason>` | coder | TDD or context loading hit blocker |
| `REVIEWER_APPROVED` | reviewer | grava-code-review verdict APPROVED |
| `REVIEWER_BLOCKED: <findings>` | reviewer | grava-code-review verdict CHANGES_REQUESTED |
| `PR_CREATED: <url>` | orchestrator | PR opened, polling for merge begins |
| `PR_COMMENTS_RESOLVED: <round>` | orchestrator | Coder fixed PR feedback, pushed to branch |
| `PR_MERGED` | orchestrator | PR merged upstream, closing issue now |
| `PIPELINE_COMPLETE: <id>` | orchestrator | PR merged + `grava close` done; team advances |
| `PIPELINE_HALTED: <reason>` | orchestrator | Human intervention needed |
| `PIPELINE_FAILED: <reason>` | orchestrator | PR creation failed or PR closed without merge |
| `PLANNER_DONE` | planner | grava-gen-issues created N issues |
| `BUG_HUNT_COMPLETE` | bug-hunter | grava-bug-hunt filed N bug issues |

### Context Passing

Claude Code agents do NOT inherit environment variables from the parent. All context is passed via the Agent tool's `prompt` parameter.

| Context | How It's Passed | Example |
|---------|-----------------|---------|
| Issue ID | In `prompt` string | `"Implement issue grava-abc123..."` |
| Commit SHA | In `prompt` string (from prior agent result) | `"Last commit: a1b2c3d..."` |
| Review findings | Appended to `prompt` on re-spawn | `"Fix these findings:\n..."` |
| Team name | In `prompt` string | `"You are on team alpha. ..."` |
| Worktree | grava-provisioned at `.worktree/<id>` | Agent `cd .worktree/$ISSUE_ID` after claim |

Agents read shared state from the grava DB via CLI (`grava show`, `grava wisp read`). This is the crash-recovery mechanism — wisps persist across sessions.

---

## 5. End-to-End Pipeline

```
code → review (max 3 rounds) → create PR → resolve PR comments (max 3 rounds) → wait for merge → close issue → next
```

### Phase breakdown (single issue)

| Phase | Actor | Signal out |
|-------|-------|-----------|
| 1. Code | coder agent | `CODER_DONE: <sha>` or `CODER_HALTED` |
| 2. Review | reviewer agent (looped with coder on BLOCKED) | `REVIEWER_APPROVED` or PIPELINE_HALTED after 3 rounds |
| 3. Create PR | orchestrator (no agent) | `PR_CREATED: <url>` |
| 4. Wait for merge (resolve comments) | orchestrator polls + re-spawns coder | `PIPELINE_COMPLETE` on merge |

### Example flows

```
# Spec → backlog → ship
/plan docs/feature-spec.md     # planner → PLANNER_DONE (49 issues)
/ship-all                      # coder → reviewer → PR → merge → next (loop)

# Bug hunt → fix → ship
/hunt                          # bug-hunter → 8 bugs filed
/ship-all                      # drain in priority order

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
| Pipeline | `grava claim <id>` | coder Phase A | `grava/<id>` | grava (`grava close` removes) |
| Ad-hoc | Claude worktree hook | `claude -w <name>` | `worktree-<name>` | Claude hook (`WorktreeRemove`) |

### Why not merge the two paths?

`grava claim` does more than create a worktree:
- Atomic claim (prevents parallel teams racing)
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

## 7. Parallel Teams (Multi-Terminal)

Run N agent teams in N terminals, all pulling from the same grava backlog. Each team works in its own worktree on a feature branch.

```
Terminal 1                Terminal 2                Terminal 3
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│ /ship-all alpha │      │ /ship-all bravo │      │ /ship-all charlie│
│ discover→claim  │      │ discover→claim  │      │ discover→claim  │
│ coder (worktree)│      │ coder (worktree)│      │ coder (worktree)│
│ reviewer        │      │ reviewer        │      │ reviewer        │
│ gh pr create    │      │ gh pr create    │      │ gh pr create    │
│ wait for merge  │      │ wait for merge  │      │ wait for merge  │
│ close + next    │      │ close + next    │      │ close + next    │
└───────┬─────────┘      └───────┬─────────┘      └───────┬─────────┘
        └────────────────────────┼────────────────────────┘
                                 ▼
                    ┌────────────────────────┐
                    │    Grava DB (shared)   │
                    │ atomic claim contention│
                    └────────────────────────┘
```

### How contention is resolved

| Scenario | What Happens |
|----------|-------------|
| Two teams claim same issue | `grava claim` is atomic — loser skips to next candidate |
| Two teams modify same file | Each in its own worktree — no conflict during work. Resolved at merge time. |
| Team finishes but branch can't merge cleanly | Expected. Human resolves at merge time. |

### Team identity

Recorded in wisps per issue:

```bash
grava wisp write $ISSUE_ID team "$TEAM_NAME"
```

Surfaced in:
- Agent prompts: `"You are on team alpha. ..."`
- PR titles: `[alpha] grava-abc123: <title>`
- Stop hook warnings: `[alpha] grava-abc123: <title>`

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
| Stale claim (heartbeat lock) | grava-next-issue stale check | Skipped automatically, next candidate tried |
| Already-implemented in queue | grava-next-issue stale check | Skipped (has `code_review` label) |
| Reviewer loop exhausted | 3 BLOCKED rounds | Issue labeled `needs-human`, pipeline halts |
| PR creation fails | `PIPELINE_FAILED: pr creation` | Check `gh auth`, push branch manually, re-run |
| PR comments unresolvable | `PIPELINE_HALTED: ...PR comments...` | Coder couldn't fix feedback after 3 rounds — `needs-human` |
| PR closed without merge | `PIPELINE_FAILED: PR closed without merge` | Re-open PR or re-run `/ship` from scratch |
| Planner gap (missing API) | grava-gen-issues validation | Skill blocks, asks user for clarification |
| Bug hunter empty | grava-bug-hunt finds nothing | Reports "0 bugs found" — codebase is clean |
| Claim contention (parallel teams) | `grava claim` returns "already claimed" | Team skips to next candidate automatically |
| Merge conflict (at merge time) | `git merge` fails | Human resolves — branches preserved, no work lost |
| Cross-team regression (at merge time) | `go test` fails on merged main | Human fixes before deploy |

---

## 9. Maintenance

### When to update skills vs agents

| Change Type | Update Skill | Update Agent |
|-------------|--------------|--------------|
| Add a TDD step | `grava-dev-epic/workflow.md` | No change |
| Add a review check | `grava-code-review/SKILL.md` | No change |
| Add new pipeline phase | New skill OR new agent | Both — new agent invokes new skill |
| Change PR template | (no skill) | `ship/SKILL.md` Phase 3 |
| Change signal vocabulary | All affected agents + CLAUDE.md | Yes |

The rule: **logic lives in skills, sequencing lives in agents**. If the change is *what* an agent does, it's a skill update. If the change is *when* it does it, it's an agent or orchestrator update.

---

## See Also

- Story files (2B.1–2B.11) for each deliverable's implementation details
- `archive/agent-team-gen-issues.md` — companion issue-gen doc (stale; feed this folder to `/plan` instead)
- `claude-code-custom-worktree.md` (repo root) — optional worktree hook for ad-hoc use
- Phase 2 (`plan/phase2/`) — prerequisite quality-gate hooks already in place
