# Grava Error Handling

## Error Output Format

All commands with `--json` return errors as:
```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "human-readable description"
  }
}
```

Match on `code`, never on `message` (message text varies).

## Error Code → Recovery Map

### Setup & Initialization

| Code | Cause | Recovery |
|------|-------|----------|
| `NOT_INITIALIZED` | `.grava/` directory missing | Run `grava init` |
| `SCHEMA_MISMATCH` | DB schema version differs from CLI version | Run `grava init` to migrate |
| `REDIRECT_STALE` | `.grava/redirect` points to non-existent directory | Delete `.grava/redirect` or fix the target path |

### Database

| Code | Cause | Recovery |
|------|-------|----------|
| `DB_UNREACHABLE` | Dolt server not running, wrong connection string, query failure | Run `grava doctor` then `grava db-start`. Check `--db-url` flag or `DB_URL` env var |
| `DB_COMMIT_FAILED` | Transaction commit failed (MVCC conflict, serialization error) | Retry the command. If persistent, check for concurrent writes |

### Issue Not Found

| Code | Cause | Recovery |
|------|-------|----------|
| `ISSUE_NOT_FOUND` | Issue ID doesn't exist or was archived | Verify ID with `grava list` or `grava search`. Check if it was dropped |
| `PARENT_NOT_FOUND` | Subtask references non-existent parent | Verify parent ID with `grava show <parent_id>` |
| `NODE_NOT_FOUND` | Graph operation references missing issue | Same as ISSUE_NOT_FOUND — verify both from and to IDs exist |

### Status Conflicts

| Code | Cause | Recovery |
|------|-------|----------|
| `ALREADY_CLAIMED` | Another agent claimed this issue (recent heartbeat) | Pick a different issue from `grava ready`. Or coordinate with the claiming agent |
| `ALREADY_IN_PROGRESS` | Issue already in `in_progress` status | Check assignee with `grava show <id>`. Use `stop` first if reclaiming |
| `INVALID_STATUS_TRANSITION` | Illegal state change (e.g., closed → in_progress) | Check current status with `grava show <id>`. May need to reopen first |
| `NOT_IN_PROGRESS` | Trying to `stop` an issue that isn't in_progress | Check status — may already be open or closed |
| `ISSUE_IN_PROGRESS` | Trying to `drop` an issue that's in_progress | `stop` it first, then `drop` |

### Validation

| Code | Cause | Recovery |
|------|-------|----------|
| `MISSING_REQUIRED_FIELD` | Title, path pattern, or other required field empty | Provide the required field. Check `grava <command> --help` |
| `INVALID_ISSUE_TYPE` | Type not in: task, bug, epic, story, feature, chore | Use one of the valid types |
| `INVALID_PRIORITY` | Priority not in: critical, high, medium, low, backlog | Use one of the valid priorities |
| `INVALID_STATUS` | Status not in: open, in_progress, closed, blocked | Use one of the valid statuses |
| `INVALID_FIELD` | Update targets a non-updatable field (e.g., created_at) | Only update mutable fields: title, desc, type, status, priority, files, last-commit |
| `INVALID_DATE` | Date string unparseable | Use YYYY-MM-DD or RFC3339 format |

### File Reservations

| Code | Cause | Recovery |
|------|-------|----------|
| `FILE_RESERVATION_CONFLICT` | Another agent holds exclusive lease on overlapping path | Wait for TTL expiry, or ask the other agent to release. Check `grava reserve --list` |
| `RESERVATION_NOT_FOUND` | Reservation ID doesn't exist or already released | Check active reservations with `grava reserve --list` |

### Import/Export

| Code | Cause | Recovery |
|------|-------|----------|
| `IMPORT_ROLLED_BACK` | Import failed (bad JSON, constraint violation) — entire import rolled back | Fix the JSONL file and retry. Check line-by-line for malformed JSON |
| `IMPORT_CONFLICT` | Duplicate ID without `--skip-existing` or `--overwrite` | Add `--skip-existing` (keep existing) or `--overwrite` (replace) |
| `CORRUPTED_JSON` | Invalid JSON in events table | Database corruption — may need to restore from a Dolt commit or re-import |
| `FILE_NOT_FOUND` | Import/export file path doesn't exist | Check file path. For export, verify parent directory exists |

### Graph

| Code | Cause | Recovery |
|------|-------|----------|
| `CYCLE_DETECTED` | Adding dependency would create a cycle | Run `grava graph cycle` to see existing cycles. Restructure dependencies |

### Worktree

| Code | Cause | Recovery |
|------|-------|----------|
| `WORKTREE_CONFLICT` | Worktree directory or branch already exists | Clean up stale worktrees with `git worktree prune` |
| `WORKTREE_PROVISION_FAILED` | Git worktree creation failed | Check disk space, git state, and permissions |
| `WORKTREE_DIRTY` | Uncommitted changes in worktree | Commit or stash changes, or use `--force` |
| `CLAUDE_WORKTREE_DETECTED` | Running close from inside a Claude worktree | Exit the worktree session first, run close from project root |

### System

| Code | Cause | Recovery |
|------|-------|----------|
| `ATOMIC_FAILURE` | Dual failure: operation failed AND rollback failed | Manual intervention required. Check database state with `grava doctor` |
| `STATUS_UPDATE_FAILED` | Graph layer couldn't write status change | Retry. If persistent, run `grava doctor` |
| `CANCELLED` | Operation cancelled (Ctrl+C, timeout) | Retry the command |
| `WISP_NOT_FOUND` | Wisp key doesn't exist on issue | Check available keys with `grava wisp read <id>` (no key = list all) |
| `CWD_UNREACHABLE` | Working directory gone or no permissions | `cd` to a valid directory |

## Idempotent Commands (Safe to Retry)

These commands produce the same result if run twice:
- `show`, `list`, `search`, `ready`, `blocked` — read-only
- `label --add X` — adding an existing label is a no-op
- `update` — setting same value is harmless
- `wisp write` — overwrites same key
- `export` — overwrites output file
- `comment` — NOT idempotent (creates duplicate comments)
- `claim` — NOT idempotent (fails with ALREADY_CLAIMED on retry)

## Diagnostic Sequence

When something goes wrong:

```bash
# 1. Check database health
grava doctor

# 2. If DB unreachable, restart
grava db-start

# 3. Check for graph problems
grava graph health

# 4. Review recent commands
grava cmd_history --limit 10

# 5. Check issue state
grava show <id> --json
```
