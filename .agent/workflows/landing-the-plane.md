---
description: Clean up the session, commit work, and update the issue status using Grava.
---

# Protocol: Landing the Plane (Grava Edition)

**Trigger:** When the user says "Let's land the plane" or runs `/landing-the-plane`.

**Objective:** Finalize the current session, ensure code is verified, and use the `grava` CLI to track progress instead of manual tracker files.

---

### Step 1: Verification & Cleanup

1.  **Run Tests:**
    // turbo
    ```bash
    go test ./...
    ```
2.  **Verify Build:**
    // turbo
    ```bash
    go build ./...
    ```
3.  **Cleanup Artifacts:** Delete any temporary debugging files, print statements, or console logs.

---

### Step 2: Git Hygiene & Persistence

1.  **Commit Changes:**
    *   Stage all relevant files.
    *   Commit with a clear message following the project's convention (e.g., `feat(cli): [Description] ([Issue-ID])`).
2.  **Verify Git State:**
    ```bash
    git status
    ```

---

### Step 3: Issue Tracking (via Grava)

**Action:** Update the database to reflect the session's achievements.

1.  **Add Completion Summary:**
    // turbo
    ```bash
    ./grava comment <issue_id> "Session Summary: [List key changes, decisions, and tests passed]"
    ```
2.  **Update Issue Status:**
    If the task is complete:
    // turbo
    ```bash
    ./grava update <issue_id> --status closed
    ```
    If work is still in progress:
    // turbo
    ```bash
    ./grava update <issue_id> --status in_progress
    ```
3.  **Handoff Note:** Update the parent epic if significant milestones were reached.
    // turbo
    ```bash
    ./grava comment <epic_id> "Milestone reached: [Description]. Subtask [id] is now closed."
    ```

---

### Step 4: Continuity (The Handoff)

1.  **Identify Next Task:**
    // turbo
    ```bash
    ./grava quick --priority 1 --limit 5
    ```
2.  **Draft Next Session Prompt:**
    Draft a context-rich prompt for the next agent, summarizing what was done and what the immediate goal for the next session is (e.g., "Starting on Issue X-123: Sub-task Y").

Note: Do not proceed next issue without user permission

---

### Step 5: Final Verdict

Present a completion summary:
```markdown
## ✅ Session Completed
- **Verified:** Tests Passing & Build Clean
- **Git:** Changes committed ([Hash])
- **Grava:** Issue [ID] updated to [Status]
- **Summary:** [Brief list of changes]
```

---
## Note
- Do not use `grava clear` and `grava drop` commands without user's permission