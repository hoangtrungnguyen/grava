# Data Models: Grava

## Schema Overview
Grava uses **Dolt** for version-controlled persistence. The schema is optimized for auditability and hierarchical issue tracking. Authoritative source: `pkg/migrate/migrations/001_initial_schema.sql` … `011_create_conflict_records.sql`.

> Run `dolt --data-dir .grava/dolt sql -q "DESCRIBE <table>"` to confirm the live schema in any deployment.

## Core Tables

### `issues`
The primary ledger for all project objectives.
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | VARCHAR(32) PK | Hierarchical ID (e.g. `grava-a1b2.1`) |
| `title` | VARCHAR(255) | Concise summary |
| `description` | LONGTEXT | Long-form body |
| `status` | VARCHAR(32) | `open`, `in_progress`, `blocked`, `closed`, `archived`, `tombstone`, … (default `open`) |
| `priority` | INT | `0` (Critical) to `4` (Backlog), default `4` |
| `issue_type` | VARCHAR(32) | `bug`, `feature`, `task`, `epic`, `story`, `chore`, `message`, … (default `task`) |
| `assignee` | VARCHAR(128) | Agent or user identity (nullable) |
| `metadata` | JSON | Extensible payload (nullable) |
| `created_at` / `updated_at` | TIMESTAMP | Audit timestamps (default `CURRENT_TIMESTAMP`) |
| `ephemeral` | TINYINT(1) | `1` = Wisp issue, excluded from exports |
| `await_type` | VARCHAR(32) | What this issue is awaiting (e.g. `human_review`) |
| `await_id` | VARCHAR(128) | The awaited entity ID |
| `created_by` / `updated_by` | VARCHAR(128) | Audit actors (default `'unknown'`) |
| `agent_model` | VARCHAR(128) | AI model identifier (nullable) |
| `affected_files` | JSON | List of files an agent modified during the work session |
| `started_at` / `stopped_at` | TIMESTAMP | Work session bounds (indexed) |
| `wisp_heartbeat_at` | TIMESTAMP | Last heartbeat — drives stale-claim detection (TTL 1h) |

### `dependencies`
Directed edges of the project knowledge graph.
| Column | Type | Description |
| :--- | :--- | :--- |
| `from_id` | VARCHAR(32) PK | Dependent issue |
| `to_id` | VARCHAR(32) PK | Dependency issue |
| `type` | VARCHAR(32) PK | `blocks`, `parent-child`, … |
| `metadata` | JSON | Optional payload |
| `created_by` / `updated_by` | VARCHAR(128) | Audit actors |
| `agent_model` | VARCHAR(128) | AI model identifier |

### `events`
Append-only ledger capturing atomic mutations for forensic observability.
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | INT PK auto-increment | |
| `issue_id` | VARCHAR(32) | Affected issue |
| `event_type` | VARCHAR(64) | `create`, `update`, `claim`, `release`, `drop`, `clear`, … (see `pkg/dolt/events.go`) |
| `actor` | VARCHAR(128) | Who performed the change |
| `old_value` / `new_value` | JSON | State before / after mutation |
| `timestamp` | TIMESTAMP | When it happened (default `CURRENT_TIMESTAMP`) |
| `created_by` / `updated_by` | VARCHAR(128) | Audit actors (default `'unknown'`) |
| `agent_model` | VARCHAR(128) | AI model identifier |

### `child_counters`
Atomic suffix allocator for hierarchical IDs (`grava-xxxx.<n>` subtasks).
| Column | Type | Description |
| :--- | :--- | :--- |
| `parent_id` | VARCHAR(32) PK | Parent issue ID |
| `next_child` | INT | Next subtask suffix (default `1`) |
| `created_by` / `updated_by` | VARCHAR(128) | Audit actors |
| `agent_model` | VARCHAR(128) | AI model identifier |

### `deletions`
Tombstone manifest for tracking deleted IDs to prevent resurrection.
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | VARCHAR(32) PK | Deleted issue ID |
| `deleted_at` | TIMESTAMP | When deleted (default `CURRENT_TIMESTAMP`) |
| `reason` | TEXT | `clear`, `compact`, manual rationale, … |
| `actor` | VARCHAR(128) | Who initiated the deletion |
| `created_by` / `updated_by` | VARCHAR(128) | Audit actors |
| `agent_model` | VARCHAR(128) | AI model identifier |

### `issue_labels`
Labels attached to issues for categorization and filtering.
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | INT PK auto-increment | |
| `issue_id` | VARCHAR(32) | Parent issue (indexed) |
| `label` | VARCHAR(128) | Label string (e.g. `bug`, `code_review`) |
| `created_at` | TIMESTAMP | When applied |
| `created_by` | VARCHAR(128) | Who applied it |

### `issue_comments`
Threaded comments on issues.
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | INT PK auto-increment | |
| `issue_id` | VARCHAR(32) | Parent issue (indexed) |
| `message` | TEXT | Comment body |
| `actor` | VARCHAR(128) | Author |
| `agent_model` | VARCHAR(256) | AI model identifier |
| `created_at` | TIMESTAMP | When posted |

### `wisp_entries`
Ephemeral key-value state for crash recovery and heartbeats.
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | INT PK auto-increment | |
| `issue_id` | VARCHAR(32) | Parent issue (indexed) |
| `key_name` | VARCHAR(255) | State key (`key` is reserved in MySQL/Dolt, hence `key_name`) |
| `value` | TEXT | State value |
| `written_by` | VARCHAR(128) | Actor that wrote the entry |
| `written_at` | TIMESTAMP | When written |

### `file_reservations`
Advisory file-path leases for concurrent edit safety (Epic 8).
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | VARCHAR(12) PK | Reservation ID (`res-a1b2c3`) |
| `project_id` | VARCHAR(12) | Project scope (indexed) |
| `agent_id` | VARCHAR(255) | Lease holder |
| `path_pattern` | VARCHAR(1024) | Glob pattern (e.g. `src/cmd/*.go`) |
| `exclusive` | TINYINT(1) | `1` = write lock, `0` = shared read |
| `reason` | TEXT | Human-readable reason |
| `created_ts` | DATETIME | When declared |
| `expires_ts` | DATETIME | TTL expiry |
| `released_ts` | DATETIME | When explicitly released (nullable) — `grava doctor --fix` populates this for expired rows |

### `conflict_records`
Merge conflict persistence for the LWW merge driver (Epic 6).
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | VARCHAR(16) PK | `sha1(issue_id + field)[:8]` |
| `issue_id` | VARCHAR(32) | Conflicting issue (indexed) |
| `field` | VARCHAR(128) | Field name (`''` = whole-issue conflict) |
| `local_val` | TEXT | Value on current branch |
| `remote_val` | TEXT | Value on other branch |
| `status` | VARCHAR(16) | `pending`, `resolved`, `dismissed` (indexed; default `pending`) |
| `detected_at` | TIMESTAMP | When detected |
| `resolved_at` | TIMESTAMP | When resolved (nullable) |
| `resolution` | VARCHAR(16) | `ours`, `theirs`, `dismissed` (nullable) |

### `cmd_audit_log`
Command-history ledger for auditing all write operations (Epic 9).
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | VARCHAR(12) PK | Audit entry ID |
| `command` | VARCHAR(255) | Command name |
| `actor` | VARCHAR(255) | Who ran it (indexed; default `'unknown'`) |
| `args_json` | TEXT | Serialized arguments |
| `exit_code` | INT | Result code |
| `created_at` | DATETIME | When executed (indexed; default `CURRENT_TIMESTAMP`) |

### `goose_db_version`
Goose migration tracker (managed automatically). Not user-facing.

## Relationships
- **Parent → Child**: via `dependencies` with `type='parent-child'`.
- **Blocking**: via `dependencies` with `type='blocks'`.
- **Issue → Events**: one-to-many audit history (no FK; `events.issue_id` indexed).
- **Issue → Labels**: one-to-many via `issue_labels`.
- **Issue → Comments**: one-to-many via `issue_comments`.
- **Issue → Wisp Entries**: one-to-many ephemeral state via `wisp_entries`.
- **File Reservation → Agent**: each lease is owned by one `agent_id` and scoped to one `project_id`.
- **Conflict Record → Issue**: many `conflict_records` may reference one `issue_id`, one per conflicting field.
