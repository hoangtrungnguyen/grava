# Grava JSON Output Contracts

All commands support `--json` for structured output. This document defines the exact shapes.

## Issue Commands

### `grava show <id> --json`

```json
{
  "id": "grava-42",
  "title": "Fix auth middleware",
  "description": "The middleware fails on...",
  "type": "bug",
  "priority": 1,
  "priority_level": "high",
  "status": "open",
  "created_at": "2026-02-18T10:00:00Z",
  "updated_at": "2026-02-18T10:30:00Z",
  "created_by": "alice",
  "updated_by": "alice",
  "assignee": "bob",
  "agent_model": "claude-opus-4-6",
  "affected_files": ["pkg/auth/middleware.go"],
  "subtasks": ["grava-42.1", "grava-42.2"],
  "labels": ["critical", "backend"],
  "comments": [
    {
      "id": 1,
      "message": "Root cause identified",
      "actor": "alice",
      "agent_model": "claude-opus-4-6",
      "created_at": "2026-02-18T11:00:00Z"
    }
  ],
  "last_commit": "abc123def456"
}
```

Optional fields (`omitempty`): `assignee`, `agent_model`, `affected_files`, `subtasks`, `labels`, `comments`, `last_commit`

### `grava list --json`

```json
[
  {
    "id": "grava-42",
    "title": "Fix auth middleware",
    "type": "bug",
    "priority": 1,
    "status": "open",
    "created_at": "2026-02-18T10:00:00Z"
  }
]
```

Same shape for `grava search <query> --json`.

### `grava create --json`

```json
{
  "id": "grava-42",
  "title": "Fix auth middleware",
  "status": "open",
  "priority": "medium",
  "ephemeral": false
}
```

Same shape for `grava subtask --json` and `grava quick --json`.

### `grava claim <id> --json`

```json
{
  "id": "grava-42",
  "status": "in_progress",
  "actor": "alice"
}
```

### `grava start <id> --json`

```json
{
  "id": "grava-42",
  "status": "in_progress",
  "started_at": "2026-02-18T10:00:00Z"
}
```

### `grava stop <id> --json`

```json
{
  "id": "grava-42",
  "status": "open",
  "stopped_at": "2026-02-18T10:00:00Z"
}
```

### `grava close <id> --json`

```json
{
  "id": "grava-42",
  "status": "closed"
}
```

### `grava update <id> --json`

```json
{
  "id": "grava-42",
  "status": "open"
}
```

### `grava assign <id> --json`

```json
{
  "id": "grava-42",
  "status": "open",
  "assignee": "bob"
}
```

### `grava label <id> --json`

```json
{
  "id": "grava-42",
  "labels_added": ["backend"],
  "labels_removed": ["frontend"],
  "current_labels": ["backend", "critical"]
}
```

### `grava comment <id> --json`

```json
{
  "id": "grava-42",
  "comment_id": 5,
  "message": "Root cause identified",
  "actor": "alice",
  "created_at": "2026-02-18T11:00:00Z"
}
```

### `grava history <id> --json`

```json
[
  {
    "event_type": "create",
    "actor": "alice",
    "timestamp": "2026-02-18T10:00:00Z",
    "details": {"title": "Fix auth middleware", "type": "bug"}
  }
]
```

`details` is a freeform map — contents vary by event type.

## Wisp Commands

### `grava wisp write <id> <key> <value> --json`

```json
{
  "issue_id": "grava-42",
  "key": "step",
  "value": "tests-written",
  "written_by": "alice"
}
```

### `grava wisp read <id> --json`

```json
[
  {
    "key": "step",
    "value": "tests-written",
    "written_by": "alice",
    "written_at": "2026-02-18T10:00:00Z"
  }
]
```

## Graph & Dependency Commands

### `grava ready --json`

Array of ready tasks, sorted by effective priority then age:

```json
[
  {
    "Node": {
      "ID": "grava-42",
      "Title": "Fix auth middleware",
      "Type": "bug",
      "Status": "open",
      "Priority": 1,
      "CreatedAt": "2026-02-18T10:00:00Z",
      "UpdatedAt": "2026-02-18T10:30:00Z",
      "Ephemeral": false,
      "AwaitType": "",
      "AwaitID": "",
      "Metadata": {}
    },
    "EffectivePriority": 1,
    "Age": 86400000000000,
    "PriorityBoosted": false
  }
]
```

Note: `ready` output uses Go default marshaling (PascalCase keys), not snake_case. `Age` is in nanoseconds.

### `grava dep <from> <to> --json`

```json
{
  "status": "created",
  "from_id": "grava-10",
  "to_id": "grava-42",
  "type": "blocks"
}
```

On removal (`--remove`): `"status": "removed"`.

### `grava blocked --json`

```json
[
  {
    "id": "grava-42",
    "title": "Fix auth middleware",
    "status": "open",
    "assignee": "bob"
  }
]
```

### `grava graph visualize --format json`

```json
{
  "nodes": [
    {"id": "grava-42", "title": "Fix auth", "type": "bug", "status": "open", "priority": 1}
  ],
  "edges": [
    {"from": "grava-10", "to": "grava-42", "type": "blocks"}
  ]
}
```

### `grava stats --json`

```json
{
  "total_issues": 50,
  "open_issues": 20,
  "closed_issues": 25,
  "blocked_count": 3,
  "stale_in_progress_count": 2,
  "avg_cycle_time_minutes": 1440.5,
  "by_status": {"open": 20, "closed": 25, "in_progress": 5},
  "by_priority": {"0": 2, "1": 10, "2": 30, "3": 8},
  "by_author": {"alice": 30, "bob": 20},
  "by_assignee": {"alice": 15, "bob": 10},
  "created_by_date": {"2026-02-18": 5, "2026-02-19": 3},
  "closed_by_date": {"2026-02-18": 2}
}
```

Note: `by_priority` keys are numeric strings. `avg_cycle_time_minutes` is null if no closed issues.

## Reservation Commands

### `grava reserve <path> --json`

```json
{
  "reservation": {
    "id": "res-a1b2c3",
    "project_id": "grava",
    "agent_id": "alice",
    "path_pattern": "pkg/auth/**/*.go",
    "exclusive": true,
    "reason": "refactoring auth",
    "created_ts": "2026-02-18T10:00:00Z",
    "expires_ts": "2026-02-18T10:30:00Z",
    "remaining_seconds": 1800
  }
}
```

### `grava reserve --list --json`

```json
{
  "reservations": [
    {
      "id": "res-a1b2c3",
      "project_id": "grava",
      "agent_id": "alice",
      "path_pattern": "pkg/auth/**/*.go",
      "exclusive": true,
      "reason": "refactoring auth",
      "created_ts": "2026-02-18T10:00:00Z",
      "expires_ts": "2026-02-18T10:30:00Z",
      "remaining_seconds": 1200
    }
  ]
}
```

### `grava reserve --release <id> --json`

```json
{
  "status": "released",
  "id": "res-a1b2c3"
}
```

## Sync Commands

### `grava import --json`

```json
{
  "imported": 10,
  "updated": 3,
  "skipped": 2
}
```

### `grava export --json` (metadata)

```json
{
  "exported_path": "backup.jsonl",
  "issue_count": 50,
  "exported_at": "2026-02-18T10:00:00Z"
}
```

The actual JSONL file contains one record per line (see JSONL Record Format below).

### JSONL Record Format (export/import)

Each line in the JSONL file is a self-contained issue record:

```json
{
  "id": "grava-42",
  "title": "Fix auth middleware",
  "description": "...",
  "type": "bug",
  "priority": 1,
  "status": "open",
  "created_at": "2026-02-18T10:00:00Z",
  "updated_at": "2026-02-18T10:30:00Z",
  "created_by": "alice",
  "updated_by": "alice",
  "agent_model": "claude-opus-4-6",
  "metadata": {},
  "affected_files": ["pkg/auth/middleware.go"],
  "ephemeral": false,
  "labels": ["critical"],
  "comments": [
    {"id": 1, "message": "...", "actor": "alice", "created_at": "2026-02-18T11:00:00Z"}
  ],
  "dependencies": [
    {"from_id": "grava-10", "to_id": "grava-42", "type": "blocks", "created_by": "alice"}
  ],
  "wisp_entries": [
    {"key": "step", "value": "done", "written_by": "alice", "written_at": "2026-02-18T12:00:00Z"}
  ]
}
```

## Error Response (All Commands)

```json
{
  "error": {
    "code": "ISSUE_NOT_FOUND",
    "message": "issue grava-999 not found"
  }
}
```

See [error-handling.md](error-handling.md) for the full error code reference.
