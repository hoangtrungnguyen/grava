# Module: `pkg/migrate`

**Package role:** Goose-based database schema migration runner. All SQL files are embedded in the binary via go:embed.

> _Updated 2026-04-17 (comprehensive doc review)._

---

## Files

| File | Lines | Exported Symbols |
|:---|:---|:---|
| `migrate.go` | 30 | Run |

## Migrations

| File | Description |
|:---|:---|
| `001_initial_schema.sql` | Core tables: issues, dependencies, events, child_counters, deletions |
| `002_audit_columns.sql` | Audit columns (created_by, updated_by, agent_model) |
| `003_add_affected_files.sql` | affected_files column on issues |
| `004_add_story_type.sql` | Add 'story' to allowed issue types |
| `005_add_work_session_columns.sql` | Work session tracking columns |
| `006_add_labels_and_comments_tables.sql` | issue_labels and issue_comments tables |
| `007_add_archived_status.sql` | 'archived' status for soft-delete |
| `008_create_wisp_entries.sql` | Wisp ephemeral state store (Epic 3) |
| `009_create_file_reservations.sql` | File reservation leases for concurrent edit safety (Epic 8) |
| `010_create_cmd_audit_log.sql` | Command history ledger (Epic 9) |
| `011_create_conflict_records.sql` | Merge conflict persistence (Epic 6) |

