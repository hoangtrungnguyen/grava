# Package: migrations

Path: `github.com/hoangtrungnguyen/grava/pkg/migrate/migrations`

## Purpose

Numbered SQL migration files (no Go code) embedded into the Grava binary by
`pkg/migrate`. Each file is a goose migration with `-- +goose Up` (and where
relevant `-- +goose Down`) sections that evolve the Dolt schema.

## Migration Files

- `001_initial_schema.sql` — bootstraps the core schema: `issues`,
  `dependencies`, `events`, `child_counters`, `deletions`, with check
  constraints on priority, status, and issue_type.
- `002_audit_columns.sql` — adds `created_by`, `updated_by`, and `agent_model`
  audit columns to `issues`, `dependencies`, `events`, `child_counters`, and
  `deletions`.
- `003_add_affected_files.sql` — adds the `affected_files` JSON column to
  `issues` for tracking which files an issue impacts.
- `004_add_story_type.sql` — extends the `issue_type` check constraint to
  allow `'story'` alongside the existing types.
- `005_add_work_session_columns.sql` — adds `started_at`/`stopped_at`
  timestamps plus indexes on each, enabling cycle-time queries.
- `006_add_labels_and_comments_tables.sql` — creates normalized
  `issue_labels` and `issue_comments` tables (previously held in the metadata
  JSON blob) with cascading FKs back to `issues`.
- `007_add_archived_status.sql` — extends the `status` check constraint to
  include `'archived'` for soft-delete support.
- `008_create_wisp_entries.sql` — creates the `wisp_entries` ephemeral
  key/value table (Story 3.2) and adds `wisp_heartbeat_at` to `issues`.
- `009_create_file_reservations.sql` — creates `file_reservations` (Story
  8.1) for concurrent-edit safety, with TTL via `expires_ts`/`released_ts`
  and a project-active index.
- `010_create_cmd_audit_log.sql` — creates `cmd_audit_log` (Story 9.1, FR14)
  to record every CLI command invocation with actor, args, and exit code.
- `011_create_conflict_records.sql` — creates `conflict_records` for the
  grava merge-driver pipeline; rows are imported from `.grava/conflicts.json`
  with `pending|resolved|dismissed` status and resolution metadata.

## Dependencies

- Consumed by `pkg/migrate` via `go:embed migrations/*.sql`.
- Run via `pressly/goose` against a Dolt (MySQL-compatible) connection.

## How It Fits

The number of files in this directory must match
`pkg/utils.SchemaVersion`; `grava init` runs `migrate.Run` then writes the
current version to `.grava/schema_version`, and `utils.CheckSchemaVersion`
fails fast with `SCHEMA_MISMATCH` when those drift.
