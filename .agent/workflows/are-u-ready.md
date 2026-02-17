---
description: objective specific Validate that the agent has sufficient context, cleared dependencies, and active environment connections to successfully
---

Protocol: Are You Ready?

**Trigger:** When the user runs the command `are-u-ready` or asks "Are you ready to start [Ticket ID]?"

**Objective:** specific Validate that the agent has sufficient context, cleared dependencies, and active environment connections to successfully complete a ticket *before* writing any code.

---

### Step 1: Ticket Identification
**Logic:**
1.  **Check Input:** Did the user provide a specific Ticket ID?
    * **YES:** Proceed to Step 2 with that ID.
    * **NO:** Scan the `epic/` folder at the project root.
        * Read the issue lists/roadmaps contained in `epic/*.md`.
        * Identify the highest priority 'Todo' or 'Open' item.
        * **Action:** Propose this ticket to the user: *"No ticket specified. Would you like to start on [Ticket ID]: [Brief Description]?"*
        * Wait for user confirmation.

### Step 2: Context & Dependency Analysis
**Source of Truth:**
* **Current Ticket:** Read details from `epic/` or the specific issue file.
* **Past Context:** Read historical data from `tracker/` (Project Root).

**Actions:**
1.  **Read Ticket Description:** Analyze the requirements.
2.  **Scan for Related Issues:**
    * Identify any linked Ticket IDs (e.g., "Blocked by #102", "Relates to #45").
    * Go to `tracker/` and read the markdown files for those specific related tickets.
    * *Critical Check:* Are the blocking tickets marked as `Done` or `Closed` in their status headers?
3.  **Synthesize Context:** specific Summarize the "story so far" based on the `tracker/` logs of related tickets to understand architectural decisions already made.

### Step 3: Environment & Connectivity Check
**Logic:** Based on the technical requirements of the ticket, verify the environment.

1.  **Identify External Dependencies:** Does the ticket mention:
    * Databases (Postgres, Redis, etc.)?
    * 3rd Party APIs (Stripe, OpenAI, AWS)?
    * Internal Microservices?
2.  **Verify Connectivity/Configuration:**
    * **Config Check:** Do valid `.env` or config files exist for these services?
    * **Connection Check:** If possible, perform a non-destructive "ping" or health check (e.g., query the DB version, hit a health check endpoint, or verify the API key format).
    * *Note:* Do not modify data; only verify access.

---

### Step 4: The Verdict (Decision Matrix)

**Output a "Readiness Checklist" to the user:**
- [ ] Ticket Context Loaded
- [ ] Blockers Resolved (Linked Tickets in `tracker/`)
- [ ] Database Connection Verified (if applicable)
- [ ] API Keys/Services Verified (if applicable)

#### Scenario A: NOT READY ❌
**Trigger:** If any blocker is "Open" or a required connection fails.
**Action:** Stop. Do not generate code.
**Output:**
> "I am not ready to start this ticket. Here is the remediation plan:"
1.  **Blockers:** List the specific tickets that must be done first.
2.  **Environment:** List the missing configs or failed connections.
3.  **Recommendation:** "Shall we switch focus to [Blocking Ticket ID] or would you like to debug the connection first?"

#### Scenario B: READY ✅
**Trigger:** All checks pass.
**Action:** Ask for the execution mode.
**Output:**
> "I am ready to start [Ticket ID]. All dependencies and connections look good."
>
> **How would you like to proceed?**
> 1.  **Create a Plan:** I will outline the implementation steps, files to touch, and test strategy.
> 2.  **Start Coding:** I will begin implementing the solution immediately.