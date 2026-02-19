---
name: getting-highest-priority-issue
description: "Use this skill when you need to find the most important task to work on next. It queries the Grava issue tracker for the highest priority open issue."
---

# Getting Highest Priority Issue

## Overview

This skill helps you identify the next piece of work by querying the Grava issue tracker for the highest priority open issue. It prioritizes critical and high-priority items first, falling back to the general backlog if necessary.

## Procedure

1.  **Check for High/Critical Priority Issues**
    Run the `./grava quick` command to find immediate priorities. This command automatically filters for high-priority open issues.
    ```bash
    ./grava quick --limit 1
    ```

2.  **Analyze Output**
    *   **Case A: Issue Found**
        If the command returns an issue (e.g., `grava-123`), skip to Step 4 with that ID.
    *   **Case B: No High Priority Issues**
        If the command returns "No high-priority open issues" (or similar empty state), check the general backlog for any open work.
        Run:
        ```bash
        ./grava list --status open
        ```
        Pick the first issue from this list. If the list is empty, inform the user there are no open issues.

3.  **Retrieve Issue Details**
    Once you have identified the target Issue ID (e.g., `grava-123`), retrieve its full details to understand the scope.
    ```bash
    ./grava show <ISSUE_ID>
    ```

4.  **Present to User**
    Present the issue details (ID, Title, Type, Priority) to the user and ask for confirmation to proceed with this task.
    *   Example: *"The highest priority issue appears to be **[ID] Title** (Priority: High). Should we start working on this?"*

## Example Usage

**User:** "What's next?"

**Agent Action:**
1.  Run `./grava quick --limit 1` -> Output: `grava-5 Critical Bug`.
2.  Run `./grava show grava-5`.
3.  Response: *"The highest priority issue is **grava-5: Critical Bug**. It involves fixing the login crash. Shall we start?"*
