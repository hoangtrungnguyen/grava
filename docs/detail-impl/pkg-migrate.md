# Module: `pkg/migrate`

**Package role:** Goose-based database schema migration runner. All SQL files are embedded in the binary via go:embed.

> _Auto-generated on 2026-04-08 (commit `baefe84`). Edit `scripts/update-docs.sh` to change the template._

---

## Files

| File | Lines | Exported Symbols |
|:---|:---|:---|
| `migrate.go` | 30 | Run |

## Migrations

| File | Description |
|:---|:---|
| `001_initial_schema.sql` | Based on EPIC-1 and TASK-1-2 |
| `002_audit_columns.sql` | Based on Epic 1.1, Task 2.1 |
| `003_add_affected_files.sql` | Based on User Request to track files relevant to an issue |
| `004_add_story_type.sql` | Add 'story' to allowed issue types |
| `005_add_work_session_columns.sql` | Add work session tracking columns to issues table |
| `006_add_labels_and_comments_tables.sql` | Add dedicated tables for issue labels and comments |
| `007_add_archived_status.sql` | Add 'archived' to allowed issue statuses for soft-delete support |
| `008_create_wisp_entries.sql` | Story 3.2: Wisp ephemeral state store |

