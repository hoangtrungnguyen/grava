# Grava CLI Reference

This document provides a comprehensive reference for the Grava Command Line Interface (CLI).

## Overview

Grava uses a cobra-based CLI structure. All commands follow the pattern:
```bash
grava [command] [flags]
```

## Global Flags

The following flags are available for all commands:

- `--config string`: Path to the config file (default is `$HOME/.grava.yaml`).
- `--db-url string`: Dolt database connection string (e.g., `user:pass@tcp(host:port)/dbname`).

## Commands

### `init`

Initializes the Grava environment. This command creates the default configuration and verifies the Dolt installation. It is idempotent and safe to run multiple times.

**Usage:**
```bash
grava init
```

---

### `create`

Creates a new issue in the tracker.

**Usage:**
```bash
grava create [flags]
```

**Flags:**
- `-t, --title string`: Issue title (**required**).
- `-d, --desc string`: Detailed description of the issue.
- `--type string`: Issue type. Allowed values: `task`, `bug`, `epic`, `story`, `feature`. Default: `task`.
- `-p, --priority string`: Priority level. Allowed values: `low`, `medium`, `high`, `critical`. Default: `medium`.
- `--parent string`: (Optional) Parent Issue ID if creating a direct child manually (prefer `subtask` command for hierarchy).
- `--ephemeral`: Mark the issue as ephemeral (a **Wisp**). Wisps are excluded from normal `list` output and are intended as temporary scratchpad notes for AI agents or short-lived work items. Default: `false`.

**Examples:**
```bash
# Create a normal issue
grava create --title "Fix login bug" --type bug --priority high

# Create an ephemeral Wisp (temporary scratchpad)
grava create --title "Investigate flaky test" --ephemeral
# Output: 👻 Created ephemeral issue (Wisp): grava-abc
```

---

### `subtask`

Creates a hierarchical subtask for an existing parent issue. The subtask ID will be generated in the format `parent_id.sequence` (e.g., `grava-123.1`).

**Usage:**
```bash
grava subtask <parent_id> [flags]
```

**Flags:**
- `-t, --title string`: Subtask title (**required**).
- `-d, --desc string`: Subtask description.
- `--type string`: Subtask type. Default: `task`.
- `-p, --priority string`: Priority level. Default: `medium`.

**Example:**
```bash
# Creates a subtask under issue 'grava-abc'
grava subtask grava-abc --title "Implement backend logic" --priority high
```

---

### `show`

Displays detailed information about a specific issue.

**Usage:**
```bash
grava show <id>
```

**Example:**
```bash
grava show grava-123.1
```

**Output:**
```
ID:          grava-123.1
Title:       Implement backend logic
Type:        task
Priority:    high (1)
Status:      open
Created:     2026-02-18T10:00:00Z
Updated:     2026-02-18T10:00:00Z

Description:
Details here...
```

---

### `update`

Updates specific fields of an existing issue. Only provided flags will update fields; others remain unchanged.

**Usage:**
```bash
grava update <id> [flags]
```

**Flags:**
- `-t, --title string`: New title.
- `-d, --desc string`: New description.
- `--status string`: New status (e.g., `open`, `in_progress`, `closed`, `blocked`).
- `-p, --priority string`: New priority.

**Example:**
```bash
grava update grava-123.1 --status closed --desc "Completed successfully"
```

---

### `list`

Lists issues in the tracker, optionally filtered by status or type. **Ephemeral Wisp issues are excluded by default.** Use `--wisp` to view them instead.

**Usage:**
```bash
grava list [flags]
```

**Flags:**
- `-s, --status string`: Filter by status (e.g., `open`, `closed`).
- `-t, --type string`: Filter by issue type.
- `--wisp`: Show only ephemeral Wisp issues (inverts the default ephemeral filter).

**Examples:**
```bash
# List all normal (non-ephemeral) issues
grava list

# Filter by status and type
grava list --status open --type bug

# List only ephemeral Wisps
grava list --wisp
```

**Output:**
```
ID          Title                  Type     Priority  Status  Created
grava-123   Fix login              bug      1         open    2026-02-18
grava-124   Add feature            task     2         open    2026-02-18
```

---

### `compact`

Purges old ephemeral **Wisp** issues from the database and records each deletion in the `deletions` table to prevent resurrection during future imports.

**Usage:**
```bash
grava compact [flags]
```

**Flags:**
- `--days int`: Delete Wisps older than this many days. Default: `7`. Pass `0` to delete **all** Wisps regardless of age.

**Examples:**
```bash
# Purge Wisps older than 7 days (default)
grava compact

# Purge Wisps older than 30 days
grava compact --days 30

# Purge ALL Wisps immediately
grava compact --days 0
```

**Output:**
```
🧹 Compacted 3 Wisp(s) older than 7 day(s). Tombstones recorded in deletions table.
```

> **Note:** Each purged Wisp ID is written to the `deletions` table with `reason='compact'` and `actor='grava-compact'`. This tombstone prevents a deleted Wisp from being re-imported if the database is ever restored from an older snapshot.

---

### `comment`

Appends a comment to an existing issue. Comments are stored as a JSON array in the issue's `metadata` column. Each entry records the text, timestamp, and actor.

**Usage:**
```bash
grava comment <id> <text>
```

**Arguments:**
- `<id>`: The issue ID to comment on.
- `<text>`: The comment text (quote if it contains spaces).

**Example:**
```bash
grava comment grava-abc "Investigated root cause, see PR #42"
# Output: 💬 Comment added to grava-abc
```

---

### `dep`

Creates a directed dependency edge between two issues. The relationship is stored in the `dependencies` table. The default type is `blocks`.

**Usage:**
```bash
grava dep <from_id> <to_id> [flags]
```

**Arguments:**
- `<from_id>`: The source issue (the one that blocks or relates).
- `<to_id>`: The target issue (the one being blocked or related to).

**Flags:**
- `--type string`: Dependency type. Examples: `blocks`, `relates-to`, `duplicates`, `parent-child`. Default: `blocks`.

**Examples:**
```bash
# grava-abc blocks grava-def (default)
grava dep grava-abc grava-def

# Custom relationship type
grava dep grava-abc grava-def --type relates-to
# Output: 🔗 Dependency created: grava-abc -[relates-to]-> grava-def
```

> **Note:** `from_id` and `to_id` must be different issues. The dependency is stored as a directed edge `(from_id, to_id, type)` with a composite primary key, so duplicate edges of the same type are rejected by the database.

---

### `label`

Adds a label to an existing issue. Labels are stored as a JSON array in the issue's `metadata` column. Adding a label that already exists is a **no-op** (idempotent).

**Usage:**
```bash
grava label <id> <label>
```

**Arguments:**
- `<id>`: The issue ID to label.
- `<label>`: The label string to add (e.g., `needs-review`, `priority:high`).

**Examples:**
```bash
grava label grava-abc "needs-review"
# Output: 🏷️  Label "needs-review" added to grava-abc

# Adding an existing label is safe
grava label grava-abc "needs-review"
# Output: 🏷️  Label "needs-review" already present on grava-abc
```

---

### `assign`

Sets the `assignee` field on an existing issue. The assignee can be a human username or an agent identity string. Passing an empty string clears the assignee.

**Usage:**
```bash
grava assign <id> <user>
```

**Arguments:**
- `<id>`: The issue ID to assign.
- `<user>`: The username or agent identity. Pass `""` to unassign.

**Examples:**
```bash
grava assign grava-abc alice
# Output: 👤 Assigned grava-abc to alice

grava assign grava-abc "agent:planner-v2"
# Output: 👤 Assigned grava-abc to agent:planner-v2

grava assign grava-abc ""
# Output: 👤 Assignee cleared on grava-abc
```

---

## Wisps (Ephemeral Issues)

**Wisps** are temporary, ephemeral issues intended for AI agents or developers who need a short-lived scratchpad that doesn't pollute the permanent project history.

| Behaviour | Normal Issue | Wisp (`--ephemeral`) |
|---|---|---|
| Appears in `grava list` | ✅ Yes | ❌ No (hidden by default) |
| Appears in `grava list --wisp` | ❌ No | ✅ Yes |
| Stored in DB | ✅ Yes | ✅ Yes |
| Can be compacted/deleted | ❌ No | ✅ Yes (via `grava compact`) |

Create a Wisp:
```bash
grava create --title "Temp: explore approach X" --ephemeral
```

View all Wisps:
```bash
grava list --wisp
```

---

## Environment Variables

- `DB_URL`: Sets the database connection string if `--db-url` flag is not provided.
