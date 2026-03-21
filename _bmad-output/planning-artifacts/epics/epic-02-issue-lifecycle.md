# Epic 2: Issue Lifecycle — Create, Track & Manage

**Status:** Planned
**Matrix Score:** 4.40
**FRs covered:** FR1, FR2, FR3, FR4, FR6, FR7

## Goal

Agents and developers can create issues and macro-epics, break them into subtasks, update status and fields, tag and comment on issues, assign them to actors, and safely archive or remove them — all operations fully audited.

## Commands Delivered

| Command | FR | Description |
|---------|----|-------------|
| `grava create` | FR1 | Create discrete issues or macro-epics |
| `grava quick` | FR1 | Rapid issue creation with minimal input |
| `grava subtask` | FR2 | Break issue into hierarchical subtasks |
| `grava update` | FR3 | Update status, priority, custom fields |
| `grava assign` | FR3 | Assign/unassign actors to issues |
| `grava start` | FR4 | Mark work as started; record timestamp |
| `grava stop` | FR4 | Mark work as stopped; record timestamp |
| `grava label` | FR6 | Add/remove labels to issues |
| `grava comment` | FR6 | Append text notes to issues |
| `grava drop` | FR7 | Soft-delete (archive) an issue |
| `grava clear` | FR7 | Purge archived issues from active space |

## Dependencies

- Epic 1 Story 0a complete (GravaError, zerolog, WithAuditedTx)

## Parallel Track

- Can begin after Epic 1 Story 0a is merged
- Can proceed in parallel with Epic 3

## Key Implementation Notes

- Subtask ID generation uses `SELECT FOR UPDATE` on `child_counters` for atomic ID allocation (ADR-H3)
- 12-char hex base ID (birthday collision ~65K issues at 1%)
- All write operations wrapped in `WithAuditedTx`
- All `--json` outputs conform to NFR5 JSON schema versioning contract

## NFR Validation Points

| NFR | Validation |
|-----|------------|
| NFR2 (<15ms writes) | `create` and `update` must commit in <15ms; benchmark in Story 2-1 |
| NFR5 (JSON schema) | All `--json` outputs validated against schema in unit tests |

## Stories

### Story 2.1: Create Issues and Macro-Epics

As a developer or agent,
I want to create a new issue or macro-epic with a title, description, and priority,
So that work items are tracked in the Grava database from the moment they are identified.

**Acceptance Criteria:**

**Given** `grava init` has been run and `.grava/` exists
**When** I run `grava create --title "Fix login bug" --priority high`
**Then** a new issue record is inserted in the `issues` table with a unique 12-char hex ID, `status=open`, `created_at=NOW()`, and the provided fields
**And** `grava create --json` returns `{"id": "abc123def456", "title": "Fix login bug", "status": "open", "priority": "high"}` conforming to NFR5 schema
**And** `grava quick "Fix login bug"` creates an issue with defaults (priority=medium, no description) in one command
**And** running `grava create` without `--title` returns `{"error": {"code": "MISSING_REQUIRED_FIELD", "message": "title is required"}}`
**And** the operation completes in <15ms (NFR2 baseline)

---

### Story 2.2: Break Issues into Subtasks

As a developer or agent,
I want to decompose an existing issue into numbered subtasks,
So that large issues can be tracked at a granular level with parent-child relationships.

**Acceptance Criteria:**

**Given** issue `abc123def456` exists with `status=open`
**When** I run `grava subtask abc123def456 --title "Write unit tests"`
**Then** a new subtask is created with ID format `abc123def456.1` (parent.sequence) using `SELECT FOR UPDATE` on `child_counters` for atomic ID generation
**And** the subtask appears in `grava show abc123def456` output under a `subtasks` array
**And** two concurrent `grava subtask abc123def456` calls produce subtasks `.1` and `.2` with no ID collision (NFR3 at subtask level)
**And** `grava subtask` on a non-existent parent returns `{"error": {"code": "ISSUE_NOT_FOUND", "message": "Issue abc123def456 not found"}}`

---

### Story 2.3: Update Issue Fields and Assign Actors

As a developer or agent,
I want to update an issue's status, priority, and assignee,
So that the current state of work is always accurately reflected in the tracker.

**Acceptance Criteria:**

**Given** issue `abc123def456` exists
**When** I run `grava update abc123def456 --status in_progress --priority low`
**Then** the `issues` table row is updated atomically via `WithAuditedTx`; `updated_at` is set to `NOW()`
**And** `grava assign abc123def456 --actor agent-01` sets `assignee=agent-01` on the issue
**And** `grava assign abc123def456 --unassign` clears the assignee field
**And** updating a non-existent issue returns `{"error": {"code": "ISSUE_NOT_FOUND", ...}}`
**And** updating to an invalid status returns `{"error": {"code": "INVALID_STATUS", "message": "Valid statuses: open, in_progress, paused, done, archived"}}`
**And** `grava update --json` returns the full updated issue record conforming to NFR5 schema

---

### Story 2.4: Track Work Start and Stop

As a developer or agent,
I want to explicitly record when I start and stop working on an issue,
So that cycle time and work-in-progress can be measured accurately.

**Acceptance Criteria:**

**Given** issue `abc123def456` exists with `status=open`
**When** I run `grava start abc123def456`
**Then** `status` transitions to `in_progress`, `started_at=NOW()`, `actor=current_actor_id` recorded in the `work_sessions` table
**And** running `grava stop abc123def456` transitions status back to `paused`, records `stopped_at=NOW()` in `work_sessions`
**And** `grava start` on an already `in_progress` issue returns `{"error": {"code": "ALREADY_IN_PROGRESS", "message": "Issue is already being worked on by agent-01"}}`
**And** `grava stop` on an issue not `in_progress` returns `{"error": {"code": "NOT_IN_PROGRESS", ...}}`
**And** both operations complete within <15ms (NFR2)

---

### Story 2.5: Label and Comment on Issues

As a developer or agent,
I want to add labels and text comments to issues,
So that contextual metadata and discussion are captured alongside the work item.

**Acceptance Criteria:**

**Given** issue `abc123def456` exists
**When** I run `grava label abc123def456 --add bug --add critical`
**Then** labels `["bug", "critical"]` are stored in the `issue_labels` table associated with this issue
**And** `grava label abc123def456 --remove bug` removes only the `bug` label; `critical` remains
**And** `grava comment abc123def456 --message "Reproduced on macOS ARM"` appends a timestamped comment to the `issue_comments` table
**And** `grava show abc123def456 --json` includes `labels` array and `comments` array in the response

---

### Story 2.6: Archive and Purge Issues

As a developer or agent,
I want to safely remove or archive issues from the active tracking space,
So that completed or cancelled work does not pollute the active queue.

**Acceptance Criteria:**

**Given** issue `abc123def456` exists with `status=done`
**When** I run `grava drop abc123def456`
**Then** `status` transitions to `archived`; the issue is excluded from `grava list` and `grava ready` by default but retrievable via `grava list --include-archived`
**And** `grava clear` purges all `archived` issues from the database permanently (hard delete) and returns a count: `{"purged": 3}`
**And** `grava drop` on an `in_progress` issue requires `--force` flag; without it returns `{"error": {"code": "ISSUE_IN_PROGRESS", "message": "Cannot drop an active issue. Use --force to override."}}`
**And** all drop/clear operations are wrapped in `WithAuditedTx`
