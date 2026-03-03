# Grava Agent Guide: Distributed Issue Tracking with Dolt

Welcome, Agent! **Grava** is a distributed issue tracker built on top of [Dolt](https://github.com/dolthub/dolt). It allows us to manage tasks, bugs, and epics directly from the terminal using a version-controlled SQL database.

As an AI agent working on this repository, you should use `grava` to track your progress, create new tasks, and update issue statuses.

---

## 🚀 Getting Started

Before running any commands, ensure you are in the project root.

### 1. Initialize (If needed)
If you are setting up a new environment:
```bash
grava init
```

### 2. Identify Yourself
Set your identity so your actions are correctly attributed:
```bash
export GRAVA_ACTOR="your-agent-name"
export GRAVA_AGENT_MODEL="gpt-4o" # or your current model
```

---

## 🛠️ Core Commands

### 📋 Listing Issues
To see what needs to be done:
```bash
# List all open issues
grava list

# Filter by status
grava list --status todo
grava list --status in-progress

# Filter by type (task, bug, epic, story)
grava list --type bug
```

### ➕ Creating Issues
When you identify a new task or bug:
```bash
# Create a simple task
grava create --title "Fix login bug" --desc "Users cannot login with Google" --type bug --priority high

# Create a subtask (dot notation for ID will be generated automatically if parent is provided)
grava create --title "Backend implementation" --parent "TASK-1"
```

### 🔍 Inspecting Issues
To see full details, comments, and hierarchy:
```bash
# Show details of an issue
grava show TASK-1

# Show the task tree (useful for Epics)
grava show EPIC-1 --tree
```

### 🔄 Updating Status
Always update the status when you start or finish a task:
```bash
# Start working
grava update TASK-1 --status in-progress

# Complete a task
grava update TASK-1 --status done --desc "Fixed the issue in pkg/auth/auth.go"
```

### 💬 Adding Comments
Keep a trail of your thoughts or findings:
```bash
grava comment TASK-1 "Identified that the issue is caused by a race condition."
```

---

## 🛡️ Best Practices for Agents

1.  **Concrete Descriptions**: Never create an issue without a description. Be specific about what needs to be changed.
2.  **Status Propagation**: When you finish a subtask, check if the parent task should also be marked as done.
3.  **Use Test Environment**: When testing `grava` commands themselves, use the test environment script:
    ```bash
    scripts/setup_test_env.sh
    ```
4.  **Atomic Commits**: `grava` stores data in a Dolt database. Use `grava commit` if you need to manually checkpoint the issue state.
5.  **Hierarchical IDs**: Use dot notation (e.g., `1.1`, `1.2`) for subtasks to maintain a clear hierarchy.

---

## 🆘 Getting Help
If you're unsure about a command's flags:
```bash
grava --help
grava <command> --help
```
