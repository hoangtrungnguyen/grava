# Grava Mental Model

## Entity Hierarchy

```
Epic
 └── Story
      └── Task / Bug / Feature / Chore
           └── Subtask (ID: parent.N, e.g. grava-42.1)
```

- **Epic** — large body of work, container for stories
- **Story** — user-visible deliverable, should have a parent epic
- **Task/Bug/Feature/Chore** — atomic work items
- **Subtask** — child of any issue, auto-numbered (`grava-42.1`, `grava-42.2`)

Issue types: `task`, `bug`, `epic`, `story`, `feature`, `chore`

## Issue Lifecycle (State Machine)

```
         ┌──────────┐
         │   open   │ ← default on create
         └────┬─────┘
              │ claim / start
              ▼
       ┌─────────────┐
       │ in_progress  │
       └──┬───┬───┬──┘
          │   │   │
    stop  │   │   │ close
    ──────┘   │   └──────► closed
              │
        block │
              ▼
        ┌─────────┐
        │ blocked  │──── unblock ──► open
        └─────────┘

    drop ──► archived ──► tombstone (via clear)
```

Valid statuses: `open`, `in_progress`, `closed`, `blocked`, `deferred`, `pinned`, `archived`, `tombstone`

Key transitions:
- `claim` = assign + set `in_progress` atomically
- `start` = set `in_progress` (without reassigning)
- `stop` = revert to `open`
- `close` = set `closed`
- `drop` = soft-delete to `archived`
- `clear` = purge archived → `tombstone`

## Priority System

| Name | Value | Meaning |
|------|-------|---------|
| critical | 0 | Drop everything |
| high | 1 | Do next |
| medium | 2 | Default |
| low | 3 | When convenient |
| backlog | 4 | Someday |

Lower numeric value = higher urgency. The `ready` command sorts by effective priority (may be boosted by inheritance from blockers).

## Dependency Graph (DAG)

Dependencies form a directed acyclic graph. Types:

**Blocking (hard):**
- `blocks` — from_id blocks to_id
- `blocked-by` — inverse

**Soft:**
- `waits-for`, `depends-on`

**Hierarchical:**
- `parent-child`, `child-of`, `subtask-of`, `has-subtask`

**Semantic:**
- `duplicates` / `duplicated-by`
- `relates-to`
- `supersedes` / `superseded-by`
- `fixes` / `fixed-by`
- `caused-by` / `causes`

**Ordering:**
- `follows` / `precedes`

Only `blocks` and `blocked-by` types prevent a task from appearing in `grava ready` output. Cycles are illegal — `grava graph cycle` detects them.

## Gate System

Issues can have await conditions beyond dependencies:

| Gate Type | AwaitID Format | Meaning |
|-----------|---------------|---------|
| `timer` | RFC3339 timestamp | Blocked until timestamp |
| `gh:pr` | `owner/repo/pulls/123` | Blocked until PR merges |
| `human` | (any) | Blocked until manual approval |

Gates are checked by the ready engine alongside dependency status.

## Wisps (Ephemeral State)

Wisps are key-value pairs attached to issues. Use for:
- Crash recovery checkpoints (`grava wisp write <id> step "phase-3"`)
- Temporary agent-to-agent communication
- Scratchpad data that shouldn't pollute comments

Properties:
- Hidden from `grava list` by default (use `--wisp` to include)
- Survive process crashes (stored in DB, not memory)
- Compacted by `grava compact --days N`
- Each entry has: key, value, written_by, written_at

## Identity Model

- `--actor` flag or `GRAVA_ACTOR` env var identifies who performs an action
- Stored as `created_by`, `updated_by`, `assignee` on issues
- `--agent-model` or `GRAVA_AGENT_MODEL` tracks which AI model performed the action
- Default actor: `"unknown"` — always set explicitly

## Audit Trail

Every write command is logged in `cmd_audit_log` with:
- Command name, arguments, actor, timestamp, issue ID affected

View with `grava cmd_history --limit N --since YYYY-MM-DD --actor <name>`

Read-only commands (list, show, ready, etc.) are NOT logged.

## Database Layer (Dolt)

- Dolt = MySQL-compatible database with git-like versioning
- `grava commit -m "message"` = snapshot current DB state (like git commit)
- `grava export` / `grava import` = JSONL-based data portability
- Local data dir: `.grava/dolt/`

**Port allocation:** `grava init` dynamically assigns a port starting at 3306. If 3306 is busy (another grava project, existing MySQL), it scans upward (3307, 3308, ...) up to port 4305. The assigned port is tracked globally in `~/.grava/ports.json` to prevent conflicts across projects on the same machine.

**Connection URL resolution** (highest priority wins):

| Source | Example |
|--------|---------|
| `--db-url` flag | `grava --db-url "root@tcp(127.0.0.1:3311)/dolt?parseTime=true" list` |
| `.grava.yaml` config | `db_url: root@tcp(127.0.0.1:3311)/dolt?parseTime=true` |
| `DB_URL` env var | `export DB_URL="root@tcp(127.0.0.1:3311)/dolt?parseTime=true"` |
| Hardcoded default | `root@tcp(127.0.0.1:3306)/dolt?parseTime=true` |

**To find the port for the current project:** read `db_url` from `.grava.yaml` in the project root, or run `grava config --json`.

Commands that skip DB: `help`, `version`, `init`, `db-start`, `db-stop`, `merge-slot`, `install`

## File Reservations

Advisory file leases for multi-agent coordination:
- `grava reserve "src/**/*.go" --exclusive --ttl 30 --reason "refactoring auth"`
- Prevents other agents from claiming overlapping exclusive leases
- Auto-expires after TTL (default: 30 minutes)
- Not enforced at filesystem level — advisory only

## Git Integration

- Custom merge driver for `issues.jsonl` (configured via `grava install`)
- Hooks: pre-commit, post-merge, post-checkout, prepare-commit-msg
- `.gitattributes` routes conflict resolution to grava's 3-way merge
