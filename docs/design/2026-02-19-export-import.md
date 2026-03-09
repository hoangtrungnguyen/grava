# Implementation Plan - Export/Import (grava-de78.10)

Implement `grava export` and `grava import` for bulk issue management.

## User Review Required

> [!IMPORTANT]
> - `export` will default to `jsonl` format (line-delimited JSON).
> - `import` will use `INSERT IGNORE` by default (skipping existing IDs) unless `--overwrite` is specified (UPSERT).
> - Import should handle dependency ordering if possible, or use `SET FOREIGN_KEY_CHECKS=0` temporarily if supported/safe, or just import in two passes (issues first, then dependencies).
> - **Self-referencing dependencies** (if allowed) complicate import order. The simplest approach for Dolt/MySQL is to disable FK checks during bulk import or ensure referenced IDs exist. Since `grava` manages FKs, we should probably import issues first, then dependencies.

## Proposed Changes

### 1. `pkg/cmd/export.go`
- **Command:** `grava export [flags]`
- **Flags:**
    - `--format`: Output format (default: `jsonl`). Currently only `jsonl` supported.
    - `--file`: Output file (default: stdout).
- **Behavior:**
    - Query all issues (including closed/tombstone? Maybe `--include-tombstones` flag?).
    - Convert each row to JSON object.
    - Write to file/stdout.
    - Also need to export `dependencies`? Yes, crucial for graph structure.
    - Maybe export a single JSON object with `{ "issues": [...], "dependencies": [...] }` or a stream of mixed types?
    - **Decision:** Let's keep it simple. `grava export` exports **issues**. Dependencies might need `grava export --type dependencies` or just be included in the issue JSON if we nest them (but that de-normalizes).
    - **Better Approach:** Standard `grava export` dumps a full backup.
    - JSONL format:
        - `{"type": "issue", "data": {...}}`
        - `{"type": "dependency", "data": {...}}`
    - This allows streaming restore.

### 2. `pkg/cmd/import.go`
- **Command:** `grava import [flags]`
- **Flags:**
    - `--file`: Input file (required).
    - `--skip-existing`: Skip if ID exists (default behavior).
    - `--overwrite`: Update if ID exists.
- **Behavior:**
    - Read JSONL line by line.
    - Start transaction.
    - For type `issue`: Insert/Upsert into `issues`.
    - For type `dependency`: Insert/Upsert into `dependencies`.
    - Commit transaction.
    - Handle errors gracefully (e.g. log errors but continue? or abort transaction?). Transactional all-or-nothing is safer for consistency.

## Schema Details

### JSONL Schema
```json
{"type": "issue", "data": {"id": "grava-1", "title": "...", ...}}
{"type": "issue", "data": {"id": "grava-2", "title": "...", ...}}
{"type": "dependency", "data": {"from_id": "grava-1", "to_id": "grava-2", "type": "blocks"}}
```

## Verification Plan

### Automated Tests
- Create `pkg/cmd/export_test.go` and `pkg/cmd/import_test.go`.
- Test export produces valid JSONL.
- Test import correctly restores data into empty DB.
- Test import respects `--overwrite` vs `--skip-existing`.
- Test import handles dependencies correctly.

### Manual Verification
1. `grava create --title "Export Test"`
2. `grava export > backup.jsonl`
3. `grava drop --force`
4. `grava import --file backup.jsonl`
5. `grava list` -> Verify data is back.
