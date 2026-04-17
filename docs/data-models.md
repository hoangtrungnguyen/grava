# Data Models: Grava

## Schema Overview
Grava uses **Dolt** for version-controlled persistence. The schema is optimized for auditability and hierarchical issue tracking.

## Core Tables

### `issues`
The primary ledger for all project objectives.
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | VARCHAR(32) | Hierarchical ID (e.g. grava-a1b2.1) |
| `title` | VARCHAR(255) | Concise summary |
| `status` | VARCHAR(32) | open, in_progress, blocked, closed, etc. |
| `priority` | INT | 0 (Critical) to 4 (Backlog) |
| `issue_type` | VARCHAR(32) | bug, feature, task, epic, chore, message |
| `assignee` | VARCHAR(128) | Agent or user identity |
| `metadata` | JSON | Extensible payload |
| `ephemeral` | BOOLEAN | If true, excluded from exports (Wisp) |

### `dependencies`
Defines the directed edges of the project knowledge graph.
| Column | Type | Description |
| :--- | :--- | :--- |
| `from_id` | VARCHAR(32) | Dependent issue |
| `to_id` | VARCHAR(32) | Dependency issue |
| `type` | VARCHAR(32) | blocks, parent-child, etc. |

### `events`
Append-only ledger capturing atomic mutations for forensic observability.
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | INT | Auto-increment primary key |
| `issue_id` | VARCHAR(32) | Affected issue |
| `event_type` | VARCHAR(64) | create, update, delete, transition, etc. |
| `actor` | VARCHAR(128) | Who performed the change |
| `old_value` | JSON | State before mutation |
| `new_value` | JSON | State after mutation |
| `timestamp` | TIMESTAMP | When it happened |

### `child_counters`
Tracks the next available suffix for hierarchical IDs to ensure atomicity.

### `deletions`
Tombstone manifest for tracking deleted IDs to prevent resurrection.

### `issue_labels`
Labels attached to issues for categorization and filtering.
| Column | Type | Description |
| :--- | :--- | :--- |
| `issue_id` | VARCHAR(32) | Parent issue |
| `label` | VARCHAR(128) | Label string (e.g. "bug", "code_review") |

### `issue_comments`
Threaded comments on issues.
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | INT | Auto-increment primary key |
| `issue_id` | VARCHAR(32) | Parent issue |
| `message` | TEXT | Comment body |
| `actor` | VARCHAR(128) | Author |
| `agent_model` | VARCHAR(128) | AI model (nullable) |
| `created_at` | TIMESTAMP | When posted |

### `wisp_entries`
Ephemeral key-value state for crash recovery and heartbeats.
| Column | Type | Description |
| :--- | :--- | :--- |
| `issue_id` | VARCHAR(32) | Parent issue |
| `key` | VARCHAR(128) | State key (e.g. "status", "step") |
| `value` | TEXT | State value |
| `written_by` | VARCHAR(128) | Actor |
| `written_at` | TIMESTAMP | When written |

### `file_reservations`
Advisory file-path leases for concurrent edit safety (Epic 8).
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | VARCHAR(12) | Reservation ID (e.g. res-a1b2c3) |
| `project_id` | VARCHAR(12) | Project scope |
| `agent_id` | VARCHAR(255) | Lease holder |
| `path_pattern` | VARCHAR(1024) | Glob pattern (e.g. src/cmd/*.go) |
| `exclusive` | BOOLEAN | TRUE = write lock, FALSE = shared read |
| `reason` | TEXT | Human-readable reason |
| `created_ts` | DATETIME | When declared |
| `expires_ts` | DATETIME | TTL expiry |
| `released_ts` | DATETIME | When explicitly released (nullable) |

### `conflict_records`
Merge conflict persistence for the LWW merge driver (Epic 6).
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | VARCHAR(16) | sha1(issue_id + field)[:8] |
| `issue_id` | VARCHAR(32) | Conflicting issue |
| `field` | VARCHAR(128) | Field name (empty = whole-issue) |
| `local_val` | TEXT | Value on current branch |
| `remote_val` | TEXT | Value on other branch |
| `status` | VARCHAR(16) | pending, resolved, dismissed |
| `detected_at` | TIMESTAMP | When detected |
| `resolved_at` | TIMESTAMP | When resolved (nullable) |
| `resolution` | VARCHAR(16) | ours, theirs, dismissed (nullable) |

### `cmd_audit_log`
Command history ledger for auditing all write operations (Epic 9).
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | INT | Auto-increment primary key |
| `command` | VARCHAR(128) | Command name |
| `args` | TEXT | Serialized arguments |
| `actor` | VARCHAR(128) | Who ran it |
| `agent_model` | VARCHAR(128) | AI model (nullable) |
| `exit_code` | INT | Result code |
| `created_at` | TIMESTAMP | When executed |

## Relationships
- **Parent -> Child**: (via `dependencies` table with type `parent-child`)
- **Blocking**: (via `dependencies` table with type `blocks`)
- **Issue -> Events**: One-to-Many audit history.
- **Issue -> Labels**: One-to-Many via `issue_labels`.
- **Issue -> Comments**: One-to-Many via `issue_comments`.
- **Issue -> Wisp Entries**: One-to-Many ephemeral state.
- **File Reservations -> Agent**: Each lease is owned by one agent.
