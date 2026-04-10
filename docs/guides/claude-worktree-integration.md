# Guide: Using Grava with Claude Worktrees

This guide explains how to use **Grava** in conjunction with **Claude Code's** native worktree support to safely run multiple agents or parallel tasks in isolated environments.

## Prerequisite: Claude CLI
Grava's worktree orchestration is designed to integrate seamlessly with the **Claude CLI**. Ensure you have `claude` installed and authenticated.

---

## 1. The Workflow

Instead of manual branching, follow this three-step cycle:

### Step 1: Claim the Issue
Run `grava claim` from your main project directory. This locks the issue in the database so no other agent can take it.

```bash
grava claim <issue-id>
```

**Note:** Grava will output a "Recommended Command" to start your isolated session.

### Step 2: Start an Isolated Session
Use Claude's `--worktree` flag to create a temporary Git worktree and a dedicated branch for the issue.

```bash
claude --worktree <issue-id>
```

Claude will:
1. Create a directory at `.claude/worktrees/<issue-id>/`.
2. Checkout a new branch named `worktree-<issue-id>`.
3. Open a new Claude session inside that directory.

### Step 3: Complete or Pause Work
Once the task is done (or if you need to pause), transition the issue state in Grava:

**To Complete:**
```bash
grava close <issue-id>
```

**To Pause:**
```bash
grava stop <issue-id>
```

After updating the state, type `exit` in the Claude session. Claude will ask if you want to delete the worktree. Choose **Yes** if the work is committed/pushed (for `close`) or if you want to clear the local directory (for `stop`).

---

## 2. Advanced: Automatic Isolation with Subagents

You can create a specialized Grava subagent that **always** runs in a worktree. Create a file at `.claude/agents/grava.md`:

```markdown
---
name: grava-dev
description: Specialized developer for Grava issues. Always uses worktrees.
isolation: worktree
---

You are a senior developer working on Grava.
Always start by running `grava claim <id>` to lock the issue.
Work within this isolated worktree.
When finished, run `grava close <id>` before ending the session.
```

Now you can invoke it:
```bash
claude "use the grava-dev agent to fix issue abc-123"
```

---

## 3. Important Notes
- **Database Persistence:** Grava is smart enough to find the main database even when running inside a sub-worktree. All notes, audit logs, and status updates are saved to the project's root.
- **Git Hooks:** If you have established Git hooks (e.g., via `grava init`), they will correctly trigger in the worktrees during commits.
