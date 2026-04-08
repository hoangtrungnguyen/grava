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

## Relationships
- **Parent -> Child**: (via `dependencies` table with type `parent-child`)
- **Blocking**: (via `dependencies` table with type `blocks`)
- **Issue -> Events**: One-to-Many audit history.
