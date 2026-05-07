---
name: managing-grava-issues
description: Use when creating, updating, querying, or managing issues in the Grava issue tracker. Applies whenever the user mentions issues, tasks, bugs, epics, stories, sprints, dependencies, blockers, labels, comments, assignments, or any project tracking activity. Also use when syncing docs with grava, bulk-updating issues, checking what's ready to work on, or managing the issue lifecycle.
---

# Managing Grava Issues

## Overview

Grava is a local CLI issue tracker backed by Dolt (version-controlled database). The hierarchy is:
**epic → story → task/subtask**. All issue management goes through `grava` CLI — never edit the DB directly.

Use `--json` on any command for machine-readable output. Use `--actor <name>` to set identity (defaults to `GRAVA_ACTOR` env var).

## Issue Types & Priority

| Type | When to use |
|------|------------|
| `epic` | Large body of work spanning multiple stories |
| `story` | User-facing slice of an epic |
| `task` | Implementation unit inside a story |
| `bug` | Defect report |

Priority values: `low`, `medium` (default), `high`, `critical`.

Status lifecycle:
```
open → in_progress → closed
         ↓
      blocked → in_progress
         ↓
      deferred
```

## Setup — Creating & Importing Issues

```bash
# Full create
grava create --type epic --title "E1: Foundation" --priority high --desc "Description here"

# Quick create (defaults: type=task, priority=medium)
grava quick "Fix login bug"

# Story linked to parent epic
grava create --type story --title "1.1: Core Error Types" --parent grava-05c6 --priority high

# Subtask (gets hierarchical ID like parent.1)
grava subtask <parent-id> --title "Add test for Cause field"

# Ephemeral issue (excluded from normal queries, used for agent scratch state)
grava create --title "temp analysis" --ephemeral

# Import/export
grava export > backup.json
grava import < backup.json
```

When syncing markdown epic files to grava:
1. Create the epic first, capture its ID from `--json` output
2. Create each story with `--parent <epic-id>` and `--type story`
3. Add the grava ID back to the markdown file

```bash
EPIC_ID=$(grava create --type epic --title "E2: Issue Lifecycle" --priority high --json | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
grava create --type story --title "2.1: Create Issues" --parent "$EPIC_ID" --priority high
```

## Daily Work — Claim, Start, Stop, Comment

```bash
# Pick up work (atomically sets in_progress + assigns to you)
grava claim <id>

# Track cycle time
grava start <id>             # Mark work started (records timestamp)
grava stop <id>              # Mark work stopped, return to open (ready queue)

# Close when done
grava update <id> --status closed

# Add comments
grava comment <id> -m "Investigated root cause, see PR #42"
grava comment <id> "Positional text also works"

# Wisp: key-value checkpoints for agent crash recovery
grava wisp write <id> --key "step" --value "parsing"
grava wisp read <id>
```

Prefer `claim` when picking up work (combines assignment + status change). Use `start`/`stop` for cycle time tracking.

## Query — List, Search, Ready, Blocked

```bash
# Browse issues
grava show <id>              # Full details
grava show <id> --tree       # Hierarchical tree view
grava list                   # All issues
grava list --status open     # Filter by status
grava list --type story      # Filter by type
grava list --sort priority:asc,created:desc
grava list --include-archived
grava list --wisp            # Show only ephemeral issues
grava search "keyword"       # Full-text search

# Work discovery
grava ready                  # Unblocked tasks sorted by priority + age
grava ready --limit 5
grava ready --priority 1     # Only critical
grava ready --show-inherited # Show if priority was inherited/boosted
grava blocked                # Currently blocked tasks
grava blocked --depth 3      # Show transitive blockers up to 3 levels

# Audit trail
grava history <id>           # Status changes, comments, labels
grava history <id> --since 2026-01-01
```

## Manage — Update, Label, Assign, Dependencies

```bash
# Update fields (only pass what you want to change)
grava update <id> --title "New title"
grava update <id> --status in_progress
grava update <id> --priority critical
grava update <id> --type story
grava update <id> --desc "Updated description"
grava update <id> --files "cmd/main.go,pkg/db.go"

# Labels
grava label <id> --add bug --add critical
grava label <id> --remove low-priority

# Assignment
grava assign <id> --actor alice
grava assign <id> --actor "agent:planner-v2"
grava assign <id> --unassign

# Dependencies
grava dep <from> <to>                    # Create "blocks" dependency (from blocks to)
grava dep <from> <to> --type relates-to  # Types: blocks, relates-to, duplicates, parent-child, subtask-of
grava dep tree <id>                      # Show dependency tree (ancestry)
grava dep impact <id>                    # Show downstream impact (successors)
grava dep path <from> <to>              # Show blocking path between two issues
grava dep clear <id>                     # Remove all dependencies for an issue
grava dep batch <file.json>              # Batch create from JSON
```

One ID per `update` call; loop for bulk updates.

## Cleanup — Drop, Clear, Undo, Commit

```bash
# Archive / delete
grava drop <id>              # Soft-delete (archive) a done/open issue
grava drop <id> --force      # Archive even if in_progress
grava drop --all --force     # Nuclear reset: delete ALL data (no undo!)
grava clear                  # Purge archived issues
grava compact                # Purge old ephemeral wisp issues

# Undo
grava undo <id>              # Revert to previous state (uncommitted → HEAD, clean → HEAD~1)

# Version control
grava commit -m "Sprint 4 setup: created epic and stories"
```

Always commit after significant batches of changes.

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Creating stories without `--parent` | Always link stories to their epic via `--parent <epic-id>` |
| Using `task` type for stories | Stories are user-facing slices — use `--type story` |
| Forgetting to add grava ID back to docs | After creating, annotate the source doc immediately |
| Running `grava update` with multiple IDs | One ID per call; loop in shell |
| Using `update --status in_progress` instead of `claim` | `claim` is atomic (status + assign); prefer it |
| Not committing after bulk changes | Run `grava commit -m "..."` after batch operations |
