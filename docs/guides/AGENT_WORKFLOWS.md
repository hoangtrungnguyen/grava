# Agent Workflows

This document describes the core **single-agent** workflows for context gathering and session handoff (`/are-u-ready-grava`, `/landing-the-plane`). One agent, one session, human-in-the-loop.

> **Looking for the multi-agent pipeline?** See [`AGENT_TEAM.md`](./AGENT_TEAM.md) for the `/ship`, `/plan`, `/hunt` orchestration — 5 agents (coder, reviewer, bug-hunter, planner, pr-creator) chained through Claude Code skills, with async PR-merge tracking via cron watchers.

These protocols ensure that AI agents and contributors maintain consistent quality, track progress accurately via the `grava` CLI, and avoid prematurely writing code without proper context.

---

## 1. Are You Ready (Grava Edition)

**Trigger:** `/are-u-ready-grava` (or prompt: *"Are you ready to start [Issue ID]?"*)

**Objective:** To validate that the agent has sufficient context, cleared all blocking dependencies, and verified environment connections *before* writing any code. 

### How it works:

1. **Issue Identification:**
   - Evaluates if a specific issue ID was provided. 
   - If no issue is specified, it automatically queries Grava for the highest priority open issue (`./grava quick --priority 1 --limit 5`) and asks for the user's confirmation.

2. **Context & Dependency Analysis:**
   - **Details:** Runs `./grava show <issue_id>` for requirements.
   - **Epic Context:** If the issue has a parent Epic, it reads the corresponding file from `docs/epics/` to understand broader goals and acceptance criteria.
   - **Blockers:** Scans for blocking issues or dependencies and verifies that any dependent issues are `closed`.

3. **Environment & Connectivity Check:**
   - Runs `./grava doctor` to verify database health and table integrity.
   - Validates `.env` and missing configurations, performing non-destructive ping checks on required databases or APIs.

4. **The Verdict:**
   - **Scenario A (Not Ready):** If blockers are open or environment checks fail, the workflow stops and produces a remediation plan.
   - **Scenario B (Ready):** If all checks pass, it presents a readiness checklist to the user, transitions the issue to `in_progress` (`./grava update <issue_id> --status in_progress`), and begins work.

---

## 2. Landing The Plane (Grava Edition)

**Trigger:** `/landing-the-plane` (or prompt: *"Let's land the plane"*)

**Objective:** To cleanly finalize the current session, verify that code changes are sound, and sync implementation progress with the Grava issue tracker. 

### How it works:

1. **Verification & Cleanup:**
   - Runs unit tests (`go test ./...`).
   - Verifies the build (`go build ./...`).
   - Runs the linter (`golangci-lint run ./...`).
   - Instructs the agent to clean up temporary debug files and console logs. *Note: If a command is updated or created, the documentation in the `docs` folder must be updated.*

2. **Git Hygiene & Persistence:**
   - Stages and commits changes using conventional commits (e.g., `feat(cli): [Description] ([Issue-ID])`).
   - Checks `git status` to ensure a clean working tree.

3. **Issue Tracking (via Grava):**
   - Automatically leaves a session summary comment on the issue (`./grava comment`).
   - Updates the issue's affected files and status to `closed` or `in_progress` along with the latest Git commit hash (`./grava update`).
   - For major milestones, leaves an update on the parent Epic.

4. **Continuity (The Handoff):**
   - Identifies the next logical task (`./grava quick --priority 1 --limit 5`).
   - Drafts a contextual prompt for the next agent session detailing what was completed and what comes next.
   - *Requirement: Do not proceed to the next issue without user permission.*

5. **Final Verdict:**
   The workflow generates a completion summary markdown block confirming that tests passed, code was committed, and the Grava tracker was updated appropriately.
