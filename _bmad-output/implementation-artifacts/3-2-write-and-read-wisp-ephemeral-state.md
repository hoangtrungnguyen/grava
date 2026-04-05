# Story 3.2: Write and Read Wisp Ephemeral State

Status: ready-for-dev

## Story

As an agent,
I want to write and read key-value pairs to an issue's Wisp (ephemeral state store),
So that my working artifacts and execution checkpoints survive a crash and can be resumed by the next agent.

## Acceptance Criteria

1. **AC#1 — Write Wisp Entry**
   Given issue `abc123def456` is claimed by `agent-01` (status=in_progress),
   When I run `grava wisp write abc123def456 checkpoint "step-3-complete" --actor agent-01`,
   Then the key-value pair is stored in the `wisp_entries` table with `issue_id`, `key`, `value`, `written_at=NOW()`, `written_by=agent-01`,
   And `grava wisp write --json` returns `{"issue_id": "abc123def456", "key": "checkpoint", "value": "step-3-complete", "written_by": "agent-01"}`.

2. **AC#2 — Read Single Wisp Entry**
   Given issue `abc123def456` has a Wisp entry with key `checkpoint`,
   When I run `grava wisp read abc123def456 checkpoint --json`,
   Then it returns `{"key": "checkpoint", "value": "step-3-complete", "written_at": "...", "written_by": "agent-01"}`.

3. **AC#3 — Read All Wisp Entries**
   Given issue `abc123def456` has multiple Wisp entries,
   When I run `grava wisp read abc123def456 --json` (no key),
   Then it returns a JSON array of all Wisp entries for the issue: `[{"key": "checkpoint", "value": "step-3-complete", ...}, ...]`.

4. **AC#4 — Persistence Across Restarts**
   Wisp entries persist across process restarts — stored in DB, not in memory.

5. **AC#5 — Wisp Heartbeat Update**
   When `grava wisp write` succeeds, it also updates `wisp_heartbeat_at` on the issue row to `NOW()` (used by `grava doctor` stale-agent detection).

6. **AC#6 — Non-Existent Issue**
   Given no issue with ID `nonexistent` exists,
   When I run `grava wisp write nonexistent key "value" --actor agent-01`,
   Then it returns `{"error": {"code": "ISSUE_NOT_FOUND", "message": "issue nonexistent not found"}}`.

7. **AC#7 — Read Non-Existent Key**
   Given issue `abc123def456` exists but has no Wisp entry with key `missing-key`,
   When I run `grava wisp read abc123def456 missing-key --json`,
   Then it returns `{"error": {"code": "WISP_NOT_FOUND", "message": "no wisp entry found for key \"missing-key\" on issue abc123def456"}}`.

8. **AC#8 — Empty Wisp Read**
   Given issue `abc123def456` exists but has no Wisp entries,
   When I run `grava wisp read abc123def456 --json` (no key),
   Then it returns an empty array `[]`, not an error.

## Tasks / Subtasks

- [ ] Task 1: Create database migration for `wisp_entries` table and `wisp_heartbeat_at` column (AC: #1, #4, #5)
  - [ ] 1.1 Create migration `008_create_wisp_entries.sql`:
    ```sql
    CREATE TABLE wisp_entries (
        id INT AUTO_INCREMENT PRIMARY KEY,
        issue_id VARCHAR(32) NOT NULL,
        key_name VARCHAR(255) NOT NULL,
        value TEXT NOT NULL,
        written_by VARCHAR(128) NOT NULL,
        written_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
        UNIQUE KEY uq_wisp_issue_key (issue_id, key_name),
        FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
        INDEX idx_wisp_issue_id (issue_id)
    );
    ```
  - [ ] 1.2 Add `wisp_heartbeat_at TIMESTAMP NULL` column to `issues` table in the same migration
  - [ ] 1.3 Update `SchemaVersion` in `pkg/utils/schema.go`

- [ ] Task 2: Implement `wispWrite` named function (AC: #1, #5, #6)
  - [ ] 2.1 Define `WispWriteParams` struct: `IssueID`, `Key`, `Value`, `Actor`
  - [ ] 2.2 Define `WispWriteResult` struct with json tags: `IssueID`, `Key`, `Value`, `WrittenBy`
  - [ ] 2.3 Implement `wispWrite(ctx, store, params)` following named function pattern:
    - Verify issue exists: `SELECT id FROM issues WHERE id = ?`
    - UPSERT into `wisp_entries`: `INSERT INTO wisp_entries (issue_id, key_name, value, written_by) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE value = VALUES(value), written_by = VALUES(written_by), written_at = NOW()`
    - Update `wisp_heartbeat_at` on issue row: `UPDATE issues SET wisp_heartbeat_at = NOW() WHERE id = ?`
    - Wrap in `WithAuditedTx` with `EventWispWrite` audit event
  - [ ] 2.4 Return `WispWriteResult`

- [ ] Task 3: Implement `wispRead` named function (AC: #2, #3, #7, #8)
  - [ ] 3.1 Define `WispEntry` struct: `Key`, `Value`, `WrittenBy`, `WrittenAt`
  - [ ] 3.2 Implement `wispRead(ctx, store, issueID, key)`:
    - If `key` provided: `SELECT key_name, value, written_by, written_at FROM wisp_entries WHERE issue_id = ? AND key_name = ?`
    - If no `key`: `SELECT key_name, value, written_by, written_at FROM wisp_entries WHERE issue_id = ? ORDER BY written_at`
    - No transaction needed (read-only)
    - Verify issue exists first (return `ISSUE_NOT_FOUND` if not)
  - [ ] 3.3 Return single `WispEntry` or `[]WispEntry`

- [ ] Task 4: Build Cobra commands (AC: #1-#8)
  - [ ] 4.1 Create `pkg/cmd/issues/wisp.go` with `newWispCmd(d)` parent command and subcommands
  - [ ] 4.2 `newWispWriteCmd(d)` — args: issue-id, key, value; flags: `--actor` (from Deps)
  - [ ] 4.3 `newWispReadCmd(d)` — args: issue-id; optional arg: key
  - [ ] 4.4 Register `wisp` command group in `AddCommands()` in `issues.go`
  - [ ] 4.5 JSON output: `json.NewEncoder(cmd.OutOrStdout()).Encode(result)` for `--json`

- [ ] Task 5: Add `EventWispWrite` constant to `pkg/dolt/events.go`
  - [ ] 5.1 Add `EventWispWrite = "wisp_write"` constant

- [ ] Task 6: Unit tests for `wispWrite` (AC: #1, #5, #6)
  - [ ] 6.1 Test happy path: write key-value to existing issue
  - [ ] 6.2 Test upsert: write same key twice → value updated
  - [ ] 6.3 Test error: write to non-existent issue → `ISSUE_NOT_FOUND`
  - [ ] 6.4 Test heartbeat: verify `wisp_heartbeat_at` updated on issue row
  - [ ] 6.5 Test JSON output structure

- [ ] Task 7: Unit tests for `wispRead` (AC: #2, #3, #7, #8)
  - [ ] 7.1 Test read single key: returns matching entry
  - [ ] 7.2 Test read all: returns array of all entries
  - [ ] 7.3 Test read missing key: returns `WISP_NOT_FOUND` error
  - [ ] 7.4 Test read empty issue: returns `[]`
  - [ ] 7.5 Test read from non-existent issue: returns `ISSUE_NOT_FOUND`

- [ ] Task 8: Run full test suite
  - [ ] 8.1 `go test ./pkg/cmd/issues/... -run TestWisp` — all pass
  - [ ] 8.2 `go test ./...` — no regressions
  - [ ] 8.3 `go vet ./...` — no issues
  - [ ] 8.4 `go build ./...` — clean build

## Dev Notes

### Architecture Patterns (MUST FOLLOW)

**Named Function Pattern** (established in Stories 2.1-2.5, 3.1):
```go
func wispWrite(ctx context.Context, store dolt.Store, params WispWriteParams) (WispWriteResult, error)
func wispRead(ctx context.Context, store dolt.Store, issueID, key string) (any, error) // single WispEntry or []WispEntry
```
- All validation upfront before DB access
- Writes inside `WithAuditedTx` (audit + atomicity)
- Reads are read-only — no transaction needed
- Return `*gravaerrors.GravaError` on all user-facing errors

**JSON Output Contract (NFR5):**
- Success: `json.NewEncoder(cmd.OutOrStdout()).Encode(result)` → stdout
- Error: `writeJSONError(cmd, err)` → stderr
- All result structs must have explicit `json:"field"` tags

**UPSERT Pattern for Wisp:**
The `wisp_entries` table has a UNIQUE constraint on `(issue_id, key_name)`. Use `ON DUPLICATE KEY UPDATE` to allow agents to overwrite their own entries without delete+insert.

**ADR-FM5 — Wisp Lifecycle:**
- Next agent reads Wisp to resume from last checkpoint
- Wisp entries survive agent crash (stored in DB)
- `wisp_heartbeat_at` on issue row tracks last activity for stale-agent detection by `grava doctor`

### Command Structure

```
grava wisp write <issue-id> <key> <value> [--actor] [--json]
grava wisp read <issue-id> [key] [--json]
```

The `wisp` command is a parent with `write` and `read` subcommands, following Cobra's subcommand pattern.

### Error Codes

| Scenario | Code | Message |
|---|---|---|
| Issue not found | `ISSUE_NOT_FOUND` | `issue {id} not found` |
| Wisp key not found | `WISP_NOT_FOUND` | `no wisp entry found for key "{key}" on issue {id}` |
| DB error | `DB_UNREACHABLE` | `failed to read/write wisp` |

### Database Schema

```sql
-- New table (migration 008)
CREATE TABLE wisp_entries (
    id INT AUTO_INCREMENT PRIMARY KEY,
    issue_id VARCHAR(32) NOT NULL,
    key_name VARCHAR(255) NOT NULL,
    value TEXT NOT NULL,
    written_by VARCHAR(128) NOT NULL,
    written_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_wisp_issue_key (issue_id, key_name),
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    INDEX idx_wisp_issue_id (issue_id)
);

-- Column addition (migration 008)
ALTER TABLE issues ADD COLUMN wisp_heartbeat_at TIMESTAMP NULL;
```

### Previous Story Learnings (from Stories 2.1-2.6)

- Use `sqlmock.New()` + `dolt.NewClientFromDB(db)` for unit tests
- All mutations inside `WithAuditedTx`
- Avoid package-level mutable state — read flags via `cmd.Flags().GetXxx()` inside RunE
- Audit events must reflect actual changes, not intended changes
- Always check errors from DB operations (never discard `LastInsertId()` errors)
- Cobra subcommands: use `cmd.AddCommand()` pattern from existing command groups

### Sandbox Integration Notes

This story's Wisp feature is required by:
- **Scenario 03 (Agent Crash + Resume):** Wisps store checkpoint data for crash recovery
- **Scenario 08 (Rapid Sequential Claims):** Wisp heartbeat for stale-agent detection

### Project Structure Notes

- New command: `pkg/cmd/issues/wisp.go`
- New tests: `pkg/cmd/issues/wisp_test.go`
- New migration: `pkg/migrate/migrations/008_create_wisp_entries.sql`
- Event constant: `pkg/dolt/events.go` — add `EventWispWrite`
- Schema version: `pkg/utils/schema.go` — bump
- Command registration: `pkg/cmd/issues/issues.go` — `AddCommands()`

### References

- [Source: _bmad-output/planning-artifacts/epics/epic-03-atomic-claim.md#Story 3.2]
- [Source: _bmad-output/planning-artifacts/architecture.md#ADR-FM5 — Wisp lifecycle]
- [Source: pkg/migrate/migrations/001_initial_schema.sql — existing schema for issues table]
- [Source: pkg/cmd/issues/claim.go — named function pattern reference]
- [Source: pkg/cmd/issues/create.go — Cobra command + subcommand pattern]
- [Source: sandbox/scenarios/03-agent-crash-and-resume.md — Wisp usage in sandbox]
- [Source: _bmad-output/implementation-artifacts/2-6-archive-and-purge-issues.md — previous story patterns]

## Dev Agent Record

### Agent Model Used

### Debug Log References

### Completion Notes List

- Story 3.2 context created — requires new table, new command, new event type (2026-04-05)

### File List
