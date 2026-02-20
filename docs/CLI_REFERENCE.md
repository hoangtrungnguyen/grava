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
- `--actor string`: User or agent identity (env: `GRAVA_ACTOR`).
- `--agent-model string`: AI model identifier (env: `GRAVA_AGENT_MODEL`).
- `--json`: Output result in machine-readable JSON format.

## Commands

### `init`

Initializes the Grava environment. This command performs the following actions:
1. Verifies the Dolt installation.
2. Creates the `.grava` directory.
3. Initializes a local Dolt repository in `.grava/dolt` (if not already present).
4. Finds an available port (starting from 3306) and starts a background Dolt server.
5. Generates a local `.grava.yaml` configuration file with the correct connection string.

**Usage:**
```bash
grava init
```

---
 
### `start`
 
Starts the Dolt SQL server using the configured port in `.grava.yaml`.
 
**Usage:**
```bash
grava start
```
 
---
 
### `stop`
 
Stops the Dolt SQL server running on the configured port. This uses a non-interactive mode (`-y`) to force-stop the process.
 
**Usage:**
```bash
grava stop
```
 
---
 
### `version`
 
Prints the current version of the Grava CLI.
 
**Usage:**
```bash
grava version
```
 
**Example:**
```bash
grava version
# Output: Grava CLI version v1.2.3
```
 
---
 
### `config`

Displays the current configuration settings being used by Grava, including the database URL, actor identity, and the path to the active configuration file.

**Usage:**
```bash
grava config
```

**Flags:**
- `--json`: Output configuration in JSON format.

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
- `--files strings`: (Optional) Comma-separated list of affected files (e.g., `main.go,pkg/api.go`).

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
- `--files strings`: Update the list of affected files.

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
- `--sort string`: Sort criteria (e.g., `priority:asc,created:desc`). Supported fields: `id`, `title`, `type`, `status`, `priority`, `created`, `updated`, `assignee`. Default: `priority:asc,created:desc`.

**Examples:**
```bash
# List all normal (non-ephemeral) issues
grava list

# Filter by status and type
grava list --status open --type bug

# Sort by creation date (newest first)
grava list --sort created:desc

# Sort by priority (asc) and then updated date (desc)
grava list --sort priority:asc,updated:desc

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

### `stats`

Displays usage statistics for the Grava project, including issue counts by status, priority, author, and assignee, as well as daily activity trends.

**Usage:**
```bash
grava stats [flags]
```

**Flags:**
- `--days int`: Number of days to include in the activity history. Default: `7`.

**Examples:**
```bash
# Show stats for the last 7 days (default)
grava stats

# Show activity for the last 30 days
grava stats --days 30

# Output as JSON for dashboard integration
grava stats --json
```

**Output:**
```
Total Issues:   42
Open Issues:    15
Closed Issues:  27

By Status:
  open:         12
  in_progress:  3
  closed:       27

By Priority:
  P1:   5
  P2:   10
  P3:   27

Top Authors:
  alice:        20
  bob:          15

Top Assignees:
  alice:        10
  bob:          5

Activity (Last 7 Days):
  Date          Created Closed
  2026-02-19    3       1
  2026-02-18    1       0
```

---

### `compact`

**Soft-deletes** old ephemeral **Wisp** issues from the database. Issues are marked with `tombstone` status and recorded in the `deletions` table to prevent resurrection during future imports.

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

### `drop`

**Nuclear reset.** Deletes **ALL data** from every table in the Grava database. This is a destructive, non-reversible operation intended for development resets or clean-slate scenarios.

**Usage:**
```bash
grava drop [flags]
```

**Flags:**
- `--force`: Skip the interactive confirmation prompt. **Required** for non-interactive / CI use.

**Behaviour:**
1. Without `--force`, the command prompts for confirmation:
   ```
   ⚠️  This will DELETE ALL DATA from the Grava database.
   Type "yes" to confirm:
   ```
   Any answer other than `"yes"` aborts the operation.
2. Tables are truncated in FK-safe order:
   1. `dependencies`
   2. `events`
   3. `deletions`
   4. `child_counters`
   5. `issues`

**Examples:**
```bash
# Interactive confirmation
grava drop
# Output: ⚠️  This will DELETE ALL DATA from the Grava database.
#         Type "yes" to confirm: yes
#         💣 All Grava data has been dropped.

# Skip confirmation (for CI/scripts)
grava drop --force
# Output: 💣 All Grava data has been dropped.
```

**Exit Codes:**
- `0` — success, all data deleted
- `1` — user cancelled or DB error

---

### `clear`

**Soft-delete** issues (and related data) created within a specified date range. Issues are marked with `tombstone` status and recorded in the `deletions` table.

**Usage:**
```bash
grava clear --from <date> --to <date> [flags]
```

**Flags:**
- `--from string`: Start date (inclusive), format `YYYY-MM-DD` (**required**).
- `--to string`: End date (inclusive), format `YYYY-MM-DD` (**required**).
- `--force`: Skip interactive confirmation.
- `--include-wisps`: Also delete ephemeral Wisp issues in the range.

**Example:**
```bash
grava clear --from 2026-01-01 --to 2026-01-31
```

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

### `doctor`

Runs a series of **read-only** diagnostic checks against the Grava database and prints a health report. Useful for verifying a fresh install or debugging a broken environment.

**Usage:**
```bash
grava doctor
```

**Checks performed:**

| # | Check | Failure mode |
|---|---|---|
| 1 | DB connectivity | `FAIL` if the server is unreachable |
| 2 | Required tables present (`issues`, `dependencies`, `deletions`, `child_counters`) | `FAIL` per missing table |
| 3 | Orphaned dependency edges | `WARN` if edges reference deleted issues |
| 4 | Issues missing a title | `WARN` if any untitled rows exist |
| 5 | Wisp count | `WARN` if > 100 Wisps (suggests running `grava compact`) |

**Exit codes:**
- `0` — all critical checks passed (warnings are OK)
- `1` — one or more `FAIL` checks detected

**Example output:**
```
🩺 Grava Doctor Report
──────────────────────────────────────────────────
✅  DB connectivity                connected (server 8.0.31)
✅  Table: issues                  exists
✅  Table: dependencies            exists
✅  Table: deletions               exists
✅  Table: child_counters          exists
✅  Orphaned dependencies          none found
✅  Untitled issues                none found
⚠️   Wisp count                    150 Wisp(s) in database — consider running `grava compact`
──────────────────────────────────────────────────
✅ All critical checks passed.
```

---

### `search`


Searches for issues whose **title**, **description**, or **metadata** contain the given text. The match is case-insensitive and uses SQL `LIKE` pattern matching. Ephemeral Wisp issues are excluded by default.

**Usage:**
```bash
grava search <query> [flags]
```

**Arguments:**
- `<query>`: The text to search for (required). Quote multi-word queries.

**Flags:**
- `--wisp`: Include ephemeral Wisp issues in results (default: `false`).

**Examples:**
```bash
# Find all issues mentioning "login"
grava search "login"

# Search within Wisp scratchpad notes too
grava search "auth" --wisp
```

**Output:**
```
ID          Title                  Type   Priority  Status  Created
grava-1     Fix login bug          bug    1         open    2026-02-18

🔍 1 result(s) for "login"
```

> **Note:** When no results are found, the command exits `0` and prints `No issues found matching "<query>"`.

---

### `quick`

Lists **open** issues at or above a given priority threshold. Useful for a fast daily triage view. Ephemeral Wisp issues are always excluded.

**Usage:**
```bash
grava quick [flags]
```

**Flags:**
- `--priority int`: Show issues at or above this priority level. Default: `1` (high). Scale: `0`=critical, `1`=high, `2`=medium, `3`=low, `4`=backlog.
- `--limit int`: Maximum number of results to return. Default: `20`.

**Examples:**
```bash
# Show critical + high priority open issues (default)
grava quick

# Include medium priority issues as well
grava quick --priority 2

# Cap output at 5 results
grava quick --limit 5
```

**Output:**
```
ID          Title                    Type   Priority  Status  Created
grava-1     Critical crash fix       bug    0         open    2026-02-18
grava-2     High priority refactor   task   1         open    2026-02-18

⚡ 2 high-priority issue(s) need attention.
```

> **Note:** When no matching issues exist, the command prints `🎉 No high-priority open issues. You're all caught up!` and exits `0`.

---

---

### `export`

Exports issues and dependencies to a file (default: stdout) in **JSONL** format (line-delimited JSON). Useful for backups, migrations, or analysis.

**Usage:**
```bash
grava export [flags]
```

**Flags:**
- `-f, --file string`: Output file path. Defaults to stdout.
- `--format string`: Output format. Defaults to `jsonl`. (Currently only `jsonl` is supported).
- `--include-wisps`: Include ephemeral Wisp issues in the export. Default: `false`.
- `--skip-tombstones`: Exclude soft-deleted (tombstone) issues. Default: `false` (includes them for full backup).

**Examples:**
```bash
# Export all issues to a file
grava export --file backup.jsonl

# Export only active issues (no tombstones) including wisps
grava export --include-wisps --skip-tombstones > active_backup.jsonl

# Pipe to another tool
grava export | jq .
```

---

### `import`

Imports issues and dependencies from a JSONL file. Supports upsert behavior to update existing records.

**Usage:**
```bash
grava import --file <path> [flags]
```

**Flags:**
- `-f, --file string`: Input file path (**required**).
- `--overwrite`: Update records if the ID already exists (Upsert).
- `--skip-existing`: Skip records if the ID already exists (Ignore duplicates).

**Examples:**
```bash
# Restore from backup (fails on duplicate IDs)
grava import --file backup.jsonl

# Update existing issues from an export
grava import --file changes.jsonl --overwrite

# Import new issues only, ignoring existing ones
grava import --file legacy.jsonl --skip-existing
```

---

### `history`

Displays the modification history of a specific issue using Dolt's version control capabilities. It shows commit hashes, authors, dates, and status changes.

**Usage:**
```bash
grava history <id>
```

**Example:**
```bash
grava history grava-123
```

**Output:**
```
History for Issue grava-123:

COMMIT     AUTHOR               DATE                      STATUS          TITLE
------------------------------------------------------------------------------------------------
a1b2c3d4   alice                2026-02-19T14:47:24+07:00 open            Fix bug
e5f6g7h8   bob                  2026-02-18T10:00:00+07:00 backlog         Init task
```

---

### `undo`

Reverts the last change to an issue.
- If the issue has **uncommitted changes**, it reverts to the last committed state (HEAD).
- If the issue is **clean** (matches HEAD), it reverts to the previous commit (HEAD~1).

**Usage:**
```bash
grava undo <id>
```

**Example:**
```bash
# Undo accidental changes
grava undo grava-123
# Output:
# Discarding uncommitted changes (reverting to HEAD)...
# ✅ Reverted issue grava-123.
```

---

### `commit`

Commits staged changes in the issue tracker to the Dolt version history. This is required for changes to appear in `grava history`.

**Usage:**
```bash
grava commit -m <message>
```

**Flags:**
- `-m, --message string`: Commit message (**required**).

**Example:**
```bash
grava commit -m "Finish work on login feature"
# Output: ✅ Committed changes. Hash: a1b2c3d4...
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

## Database Migrations

Grava uses an automated migration system (powered by [goose](https://github.com/pressly/goose)) to manage its database schema.

- **Automatic Updates**: Migrations are bundled with the Grava binary and run automatically whenever you execute a command (except `init` and `help`).
- **Internal State**: The migration version is tracked in a internal `goose_db_version` table.
- **Development**: New schema changes are added as `.sql` files in `pkg/migrate/migrations/` and require a rebuild of the CLI.

## Environment Variables

- `DB_URL`: Sets the database connection string if `--db-url` flag is not provided.
- `GRAVA_ACTOR`: Sets the default actor name.
- `GRAVA_AGENT_MODEL`: Sets the default agent model name.
