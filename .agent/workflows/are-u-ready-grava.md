---
description: Validate that the agent has sufficient context, cleared dependencies, and active environment connections to successfully complete a grava issue before writing any code.
---

Protocol: Are You Ready? (Grava Edition)

**Trigger:** When the user runs the command `are-u-ready-grava` or asks "Are you ready to start [Issue ID]?"

**Objective:** Validate that the agent has sufficient context, cleared dependencies, and active environment connections to successfully complete an issue *before* writing any code. All issue tracking is done through the `grava` CLI.

---

### Step 1: Issue Identification (via Grava)
**Logic:**
1.  **Check Input:** Did the user provide a specific Issue ID (e.g. `grava-863c.1`)?
    * **YES:** Verify the issue exists by running:
      ```bash
      ./grava show <issue_id>
      ```
      * If the command succeeds, proceed to Step 2.
      * If the command fails (issue not found), inform the user and stop.
    * **NO:** Find the highest priority open issue by running:
      ```bash
      ./grava quick --priority 1 --limit 5
      ```
      * **Action:** Propose the top issue to the user: *"No issue specified. Would you like to start on [Issue ID]: [Title]?"*
      * Wait for user confirmation.

### Step 2: Context & Dependency Analysis (via Grava + Epics)
**Source of Truth:**
* **Issue Details:** Query via `grava show <issue_id>` to get full description, status, priority, labels, assignee.
* **Epic Context:** Read the relevant epic file from `docs/epics/` to understand the broader goal and acceptance criteria.
* **Past Context:** Read historical data from `tracker/` (Project Root) for session logs and architectural decisions.

**Actions:**
1.  **Read Issue Details:** Run `./grava show <issue_id>` and analyze the requirements from the description.
2.  **Locate Parent Epic:** If the issue ID has a parent (e.g. `grava-863c.1` → parent is `grava-863c`):
    * Run `./grava show <parent_id>` to understand the epic scope.
    * Search `docs/epics/` for the matching epic document and read it for full acceptance criteria.
3.  **Scan for Dependencies:**
    * Check the issue description for mentions of blocking issues or related IDs.
    * Search related issues: `./grava search "<related_keyword>"`
    * *Critical Check:* For any blocking issue found, run `./grava show <blocking_id>` and verify its status is `closed`. If the blocker is still `open` or `in_progress`, flag it.
4.  **Synthesize Context:** Summarize the "story so far" based on:
    * The epic document from `docs/epics/`
    * The `tracker/` session logs for related work
    * The issue descriptions and dependency chain from grava

### Step 3: Environment & Connectivity Check
**Logic:** Based on the technical requirements of the issue, verify the environment.

1.  **Identify External Dependencies:** Does the issue mention:
    * Databases (Dolt, MySQL, etc.)?
    * 3rd Party APIs?
    * Internal services?
2.  **Verify Connectivity/Configuration:**
    * **Grava Health:** Run `./grava doctor` to verify database connectivity and table integrity.
    * **Config Check:** Do valid `.env` or config files exist for required services?
    * **Connection Check:** If possible, perform a non-destructive "ping" or health check (e.g., query the DB version, hit a health check endpoint, or verify the API key format).
    * *Note:* Do not modify data; only verify access.

---

### Step 4: The Verdict (Decision Matrix)

**Output a "Readiness Checklist" to the user:**
- [ ] Issue Context Loaded (`grava show` succeeded)
- [ ] Epic Context Loaded (read from `docs/epics/`)
- [ ] Blockers Resolved (all blocking issues are `closed`)
- [ ] Grava Doctor Passed (DB + tables healthy)
- [ ] Environment Verified (configs, connections)

#### Scenario A: NOT READY ❌
**Trigger:** If any blocker is "open" or a required connection fails.
**Action:** Stop. Do not generate code.
**Output:**
> "I am not ready to start this issue. Here is the remediation plan:"
1.  **Blockers:** List the specific issue IDs (from `grava show`) that must be closed first.
2.  **Environment:** List the missing configs or failed connections (from `grava doctor`).
3.  **Recommendation:** "Shall we switch focus to [Blocking Issue ID] or would you like to debug the connection first?"

#### Scenario B: READY ✅
**Trigger:** All checks pass.
**Action:** Ask for the execution mode.
**Output:**
> "I am ready to start [Issue ID]: [Title]. All dependencies and connections look good."

**Then update the issue status:**
```bash
./grava update <issue_id> --status in_progress
```
