# Generate Grava Issues from Agent Team Plan

> **Purpose:** Feed this document to the `grava-gen-issues` skill (or `/plan` command)
> to create the full issue hierarchy for implementing the agent team pipeline.
>
> **Source plan:** `agent-team-plan-v2.md`

---

## Epic: Agent Team Pipeline

Implement a multi-agent pipeline for the Grava workflow where agents orchestrate
and skills execute. The pipeline: code → review → create PR. Teams run in parallel
terminals, each pulling from the same backlog via atomic claims.

### Required Services

- **Grava CLI** — issue tracker (`grava` binary, `.grava/dolt/` database)
- **Git** — version control, worktrees, feature branches
- **GitHub CLI** (`gh`) — PR creation, merge status checks
- **Claude Code** — agent runtime with Agent tool, skills, hooks

### External APIs

- **GitHub API** (via `gh` CLI) — pull request lifecycle
- **Google Cloud Run** (optional, for future deploy) — `gcloud` CLI

### Third-Party Libraries

- None required for the agent/skill/hook files (all markdown + bash)
- `jq` for JSON parsing in hooks

---

## Stories

### Story 1: Agent Definitions

Create the 4 agent files in `.claude/agents/`. Each is a thin wrapper that
delegates to existing skills.

**Acceptance Criteria:**
- `coder.md` exists with correct frontmatter (`model: sonnet`, `skills: [grava-cli]`, `maxTurns: 100`)
- `reviewer.md` exists with correct frontmatter (`maxTurns: 30`)
- `bug-hunter.md` exists with correct frontmatter (`tools` includes `Agent`)
- `planner.md` exists with correct frontmatter (`maxTurns: 50`)
- Each agent has an `## Input` section documenting what it receives via prompt
- Each agent has a signal contract (final message format) documented in Anti-Patterns
- No agent references `$WORKTREE_PATH` or `$GRAVA_ISSUE_ID` env vars

#### Task 1.1: Coder Agent

Create `.claude/agents/coder.md` per plan Section 3.1.

- Frontmatter: `name: coder`, `model: sonnet`, `tools: Read, Write, Edit, Bash, Glob, Grep`, `skills: [grava-cli]`, `maxTurns: 100`
- Input section: receives `ISSUE_ID` in prompt
- Workflow: Phase A (grava-claim) → Phase B (grava-dev-epic) → Phase C (signal)
- HALT conditions with wisp write + grava stop
- Signal contract: `CODER_DONE: <sha>` or `CODER_HALTED: <reason>`

#### Task 1.2: Reviewer Agent

Create `.claude/agents/reviewer.md` per plan Section 3.2.

- Frontmatter: `name: reviewer`, `model: sonnet`, `tools: Read, Bash, Glob, Grep`, `skills: [grava-cli]`, `maxTurns: 30`
- Input section: receives `ISSUE_ID` in prompt
- Pre-flight: verify `last_commit` exists
- Workflow: delegates to grava-code-review skill
- Signal contract: `REVIEWER_APPROVED` or `REVIEWER_BLOCKED: <findings>`

#### Task 1.3: Bug Hunter Agent

Create `.claude/agents/bug-hunter.md` per plan Section 3.4.

- Frontmatter: `name: bug-hunter`, `model: sonnet`, `tools: Read, Bash, Glob, Grep, Agent`, `skills: [grava-cli]`, `maxTurns: 50`
- Input section: receives `SCOPE` in prompt
- Workflow: delegates to grava-bug-hunt skill
- Signal contract: `BUG_HUNT_COMPLETE` with stats

#### Task 1.4: Planner Agent

Create `.claude/agents/planner.md` per plan Section 3.5.

- Frontmatter: `name: planner`, `model: sonnet`, `tools: Read, Bash, Glob, Grep, Write`, `skills: [grava-cli]`, `maxTurns: 50`
- Input section: receives `DOC_PATH` in prompt
- Setup: `grava doctor` check
- Workflow: delegates to grava-gen-issues skill
- Signal contract: `PLANNER_DONE` with stats

---

### Story 2: Orchestrator Skills

Create 4 orchestrator skills in `.claude/skills/`. These are user-invocable
slash commands that spawn agents via the Agent tool.

**Acceptance Criteria:**
- Each skill has YAML frontmatter with `name`, `description`, `user-invocable: true`
- `/ship <id>` spawns coder → reviewer (up to 3 rounds) → creates PR
- `/ship-all <team>` loops discovery → /ship pipeline → next issue
- `/plan <path>` spawns planner agent
- `/hunt [scope]` spawns bug-hunter agent
- All use `Agent({ subagent_type, prompt, isolation })` — not Task tool
- Context passed via `prompt` string, not env vars

#### Task 2.1: Ship Skill

Create `.claude/skills/ship/SKILL.md` per plan Section 4.1.

- Phase 1: Spawn coder agent with `isolation: "worktree"`
- Phase 2: Spawn reviewer agent, up to 3 rounds with coder re-spawn on BLOCKED
- Phase 3: Create PR via `gh pr create`, record in grava wisps
- Parse agent return values for signal strings
- Handle PIPELINE_HALTED and PIPELINE_FAILED

**Depends on:** Task 1.1 (coder agent), Task 1.2 (reviewer agent)

#### Task 2.2: Ship-All Skill

Create `.claude/skills/ship-all/SKILL.md` per plan Section 4.2.

- Discovery loop: `grava ready --limit 3 --json` with stale-state checks
- Team identity: accept team name as argument, pass to agent prompts
- Run /ship pipeline inline for each claimable issue
- Stop conditions: backlog empty, 3 consecutive halts, user interrupt
- Final summary with PR URLs

**Depends on:** Task 2.1 (ship skill)

#### Task 2.3: Plan Skill

Create `.claude/skills/plan/SKILL.md` per plan Section 4.3.

- Validate `$ARGUMENTS` path exists
- Spawn planner agent via Agent tool
- Wait for `PLANNER_DONE`
- Suggest `/ship-all` as next step

**Depends on:** Task 1.4 (planner agent)

#### Task 2.4: Hunt Skill

Create `.claude/skills/hunt/SKILL.md` per plan Section 4.4.

- Parse scope from `$ARGUMENTS` (default: since-last-tag)
- Spawn bug-hunter agent via Agent tool
- Wait for `BUG_HUNT_COMPLETE`
- Suggest `/ship-all` as next step

**Depends on:** Task 1.3 (bug-hunter agent)

---

### Story 3: Hooks

Add pipeline hooks to `scripts/hooks/` matching the existing bash convention.

**Acceptance Criteria:**
- `sync-pipeline-status.sh` captures pipeline signals in Bash output and writes grava wisps
- `warn-in-progress.sh` warns on session end about orphaned issues
- Both are executable bash scripts matching existing hook pattern (JSON on stdin, jq parsing)
- `settings.json` updated with PostToolUse and Stop hook entries (merged, not replaced)

#### Task 3.1: Pipeline Status Sync Hook

Create `scripts/hooks/sync-pipeline-status.sh` per plan Section 5.2.

- Read JSON from stdin, extract `tool_name` and `tool_output`
- Only process Bash tool output
- Match signals: CODER_DONE, CODER_HALTED, REVIEWER_APPROVED, REVIEWER_BLOCKED, PR_CREATED, PIPELINE_HALTED, PIPELINE_COMPLETE
- Write matching signal to grava wisp
- Extract issue ID from `grava list --status in_progress`

#### Task 3.2: In-Progress Warning Hook

Create `scripts/hooks/warn-in-progress.sh` per plan Section 5.3.

- List in-progress issues via `grava list --status in_progress --json`
- Print warning to stderr with issue IDs and titles
- Exit 0 always (warning only, never blocks)

#### Task 3.3: Register Hooks in Settings

Update `.claude/settings.json` per plan Section 5.1.

- Add `PostToolUse` entry with `matcher: "Bash"` pointing to `sync-pipeline-status.sh`
- Add `Stop` entry pointing to `warn-in-progress.sh`
- Merge with existing hooks (TaskCompleted, TeammateIdle, TaskCreated) — do NOT replace
- Follow existing nested format: `{ hooks: [{ type: "command", command: "..." }] }`

**Depends on:** Task 3.1, Task 3.2

---

### Story 4: Documentation

Update project documentation to reflect the agent team pipeline.

#### Task 4.1: Update CLAUDE.md

Append Agent Team section to `CLAUDE.md` per plan Section 6.

- Agent Team command table (ship, ship-all, plan, hunt)
- Skill ↔ Agent Map table
- Pipeline Signals table (agent ↔ orchestrator contract)
- Context Passing table (how agents receive state)

---

### Story 5: PR-Merge Issue Closure

Automate closing grava issues when their PRs merge.

#### Task 5.1: GitHub Actions Workflow

Create `.github/workflows/close-grava-issue.yml` per plan Section 8.

- Trigger on `pull_request.closed` where `merged == true`
- Extract grava issue ID from PR title (`grava-XXXX` pattern)
- Run `grava close` + `grava comment` with merge URL
- Run `grava commit` to persist state

---

### Story 6: Parallel Team Support

Enable multiple terminals to run `/ship-all` concurrently on the same backlog.

**Acceptance Criteria:**
- `/ship-all <team-name>` passes team identity to agent prompts and wisps
- `grava claim` atomicity prevents double-claiming (already built into grava)
- Each coder agent uses `isolation: "worktree"` for git isolation
- Final summary includes "Issues skipped (claimed by other team)" count

#### Task 6.1: Team Identity in Ship-All

Update `/ship-all` skill to:
- Parse team name from `$ARGUMENTS`
- Include team name in Agent tool prompts: `"You are on team $TEAM_NAME..."`
- Record team in wisps: `grava wisp write $ISSUE_ID team "$TEAM_NAME"`
- Track skipped-due-to-contention count in summary

**Depends on:** Task 2.2 (ship-all skill)

---

### Story 7: Smoke Tests

Validate each pipeline component works end-to-end.

#### Task 7.1: Smoke Test — Single Issue Ship

- Pick a low-risk open issue
- Run `/ship <id>`
- Verify: coder claims, implements, reviewer approves, PR created
- Verify: grava wisps track each phase

#### Task 7.2: Smoke Test — Plan Command

- Create a small test spec document
- Run `/plan <test-spec.md>`
- Verify: planner creates issues with correct hierarchy and dependencies

#### Task 7.3: Smoke Test — Hunt Command

- Run `/hunt pkg/cmd/` (single package scope)
- Verify: bug-hunter reviews files, classifies findings, creates issues

#### Task 7.4: Smoke Test — Parallel Teams

- Open 2 terminals with 3+ ready issues
- Run `/ship-all alpha` and `/ship-all bravo`
- Verify: no double-claims, both teams produce PRs, wisps show team identity

**Depends on:** Stories 1-6

---

## Dependency Graph

```
Story 1 (agents)
  ├── Task 1.1 coder
  ├── Task 1.2 reviewer
  ├── Task 1.3 bug-hunter
  └── Task 1.4 planner
         │
         ▼
Story 2 (orchestrator skills)
  ├── Task 2.1 ship      ← depends on 1.1, 1.2
  ├── Task 2.2 ship-all  ← depends on 2.1
  ├── Task 2.3 plan      ← depends on 1.4
  └── Task 2.4 hunt      ← depends on 1.3
         │
         ▼
Story 3 (hooks)           ← independent of Story 2
  ├── Task 3.1 sync hook
  ├── Task 3.2 warning hook
  └── Task 3.3 settings  ← depends on 3.1, 3.2
         │
         ▼
Story 4 (docs)            ← after Stories 1-3
  └── Task 4.1 CLAUDE.md
         │
         ▼
Story 5 (PR closure)      ← independent
  └── Task 5.1 GH Actions
         │
         ▼
Story 6 (parallel teams)  ← depends on 2.2
  └── Task 6.1 team identity
         │
         ▼
Story 7 (smoke tests)     ← depends on all above
  ├── Task 7.1 /ship
  ├── Task 7.2 /plan
  ├── Task 7.3 /hunt
  └── Task 7.4 parallel
```

## Priority

| Priority | Stories | Rationale |
|----------|---------|-----------|
| P0 (critical path) | 1, 2 | Agents + skills = the pipeline |
| P1 (enables production) | 3, 4, 5 | Hooks, docs, PR closure |
| P2 (enables scale) | 6 | Parallel teams |
| P3 (validation) | 7 | Smoke tests confirm everything works |
