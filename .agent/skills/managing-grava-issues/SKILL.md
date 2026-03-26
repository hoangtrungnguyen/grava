---
name: managing-grava-issues
description: Use when creating, updating, or querying epics, stories, or tasks in the Grava issue tracker. Applies when setting up a sprint, syncing docs with grava, bulk-updating issue types or statuses, or wiring parent-child relationships between epics and stories.
---

# Managing Grava Issues

## Overview

Grava is a local CLI issue tracker backed by Dolt. The hierarchy is:
**epic → story → task/subtask**. All issue management goes through `./grava` (never edit the DB directly).

## Issue Types

| Type | When to use |
|------|------------|
| `epic` | Large body of work spanning multiple stories |
| `story` | User-facing slice of an epic (agile story) |
| `task` | Implementation unit inside a story |
| `bug` | Defect report |
| `feature` | Standalone feature not tied to an epic |
| `chore` | Non-feature work (migrations, cleanup) |

## Quick Reference — Common Commands

```bash
# Create an epic
./grava create --type epic --title "E1: Foundation & Scaffold" --priority high

# Create a story (link to parent epic)
./grava create --type story --title "1.1: Core Error Types" --parent grava-05c6 --priority high

# Create a task
./grava create --type task --title "Write unit tests for GravaError" --parent grava-480f

# Create a subtask (numbered child of a parent)
./grava subtask <parent-id> --title "Add test for Cause field"

# Update type/status/priority
./grava update <id> --type story
./grava update <id> --status in_progress
./grava update <id> --priority critical

# List, filter, inspect
./grava list
./grava list --status open
./grava show <id>
./grava search "<keyword>"
./grava ready          # shows unblocked open issues
./grava quick          # shows high/critical priority issues
```

## Creating Epics and Stories from Docs

When syncing markdown epic files to grava:

1. **Create the epic first**, note its ID
2. **Create each story** with `--parent <epic-id>` and `--type story`
3. **Add the grava ID** back to the markdown file — format: `**Grava ID:** grava-XXXX` under `**Status:**`, and `*(grava-XXXX)*` appended to each `### Story X.Y:` heading

```bash
# Example sync for one epic
EPIC_ID=$(./grava create --type epic --title "E2: Issue Lifecycle" --priority high --json | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
./grava create --type story --title "2.1: Create Issues" --parent "$EPIC_ID" --priority high
```

## Bulk Type Updates

When updating many issues at once, the DB constraint `check_issue_type` must include the target type. If it doesn't, add a migration first:

```sql
-- pkg/migrate/migrations/00N_add_<type>.sql
ALTER TABLE issues
  DROP CONSTRAINT check_issue_type,
  ADD CONSTRAINT check_issue_type CHECK (issue_type IN ('bug', 'feature', 'task', 'epic', 'chore', 'message', 'story'));
```

Apply directly to the running DB:
```bash
dolt --data-dir .grava/dolt sql -q "<ALTER TABLE statement>"
```

Then bulk update:
```bash
for id in grava-xxx grava-yyy; do ./grava update "$id" --type story; done
```

## Status Lifecycle

```
open → in_progress → closed
           ↓
        blocked → in_progress
           ↓
        deferred
```

Always mark `in_progress` when starting work; `closed` when done:
```bash
./grava update <id> --status in_progress
./grava update <id> --status closed
```

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Creating stories without `--parent` | Always link stories to their epic via `--parent <epic-id>` |
| Using `task` type for stories | Stories are user-facing slices — use `--type story` |
| Forgetting to add grava ID back to markdown docs | After creating an issue, annotate the source doc immediately |
| Bulk update hits constraint violation | Check `check_issue_type` constraint includes target type; add migration if needed |
| Running `grava update` with multiple IDs in one call | Pass one ID per call; loop in shell |
