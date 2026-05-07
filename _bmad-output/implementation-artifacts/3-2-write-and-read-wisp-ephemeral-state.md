# Story 3.2: Write and Read Wisp Ephemeral State

Status: done

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

- [x] Task 1: Create database migration for `wisp_entries` table and `wisp_heartbeat_at` column (AC: #1, #4, #5)
  - [x] 1.1 Create migration `008_create_wisp_entries.sql`:
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
  - [x] 1.2 Add `wisp_heartbeat_at TIMESTAMP NULL` column to `issues` table in the same migration
  - [x] 1.3 Update `SchemaVersion` in `pkg/utils/schema.go`

- [x] Task 2: Implement `wispWrite` named function (AC: #1, #5, #6)
  - [x] 2.1 Define `WispWriteParams` struct: `IssueID`, `Key`, `Value`, `Actor`
  - [x] 2.2 Define `WispWriteResult` struct with json tags: `IssueID`, `Key`, `Value`, `WrittenBy`
  - [x] 2.3 Implement `wispWrite(ctx, store, params)` following named function pattern:
    - Verify issue exists: `SELECT id FROM issues WHERE id = ?`
    - UPSERT into `wisp_entries`: `INSERT INTO wisp_entries (issue_id, key_name, value, written_by) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE value = VALUES(value), written_by = VALUES(written_by), written_at = NOW()`
    - Update `wisp_heartbeat_at` on issue row: `UPDATE issues SET wisp_heartbeat_at = NOW() WHERE id = ?`
    - Wrap in `WithAuditedTx` with `EventWispWrite` audit event
  - [x] 2.4 Return `WispWriteResult`

- [x] Task 3: Implement `wispRead` named function (AC: #2, #3, #7, #8)
  - [x] 3.1 Define `WispEntry` struct: `Key`, `Value`, `WrittenBy`, `WrittenAt`
  - [x] 3.2 Implement `wispRead(ctx, store, issueID, key)`:
    - If `key` provided: `SELECT key_name, value, written_by, written_at FROM wisp_entries WHERE issue_id = ? AND key_name = ?`
    - If no `key`: `SELECT key_name, value, written_by, written_at FROM wisp_entries WHERE issue_id = ? ORDER BY written_at`
    - No transaction needed (read-only)
    - Verify issue exists first (return `ISSUE_NOT_FOUND` if not)
  - [x] 3.3 Return single `WispEntry` or `[]WispEntry`

- [x] Task 4: Build Cobra commands (AC: #1-#8)
  - [x] 4.1 Create `pkg/cmd/issues/wisp.go` with `newWispCmd(d)` parent command and subcommands
  - [x] 4.2 `newWispWriteCmd(d)` — args: issue-id, key, value; flags: `--actor` (from Deps)
  - [x] 4.3 `newWispReadCmd(d)` — args: issue-id; optional arg: key
  - [x] 4.4 Register `wisp` command group in `AddCommands()` in `issues.go`
  - [x] 4.5 JSON output: `json.NewEncoder(cmd.OutOrStdout()).Encode(result)` for `--json`

- [x] Task 5: Add `EventWispWrite` constant to `pkg/dolt/events.go`
  - [x] 5.1 Add `EventWispWrite = "wisp_write"` constant

- [x] Task 6: Unit tests for `wispWrite` (AC: #1, #5, #6)
  - [x] 6.1 Test happy path: write key-value to existing issue
  - [x] 6.2 Test upsert: write same key twice → value updated
  - [x] 6.3 Test error: write to non-existent issue → `ISSUE_NOT_FOUND`
  - [x] 6.4 Test heartbeat: verify `wisp_heartbeat_at` updated on issue row
  - [x] 6.5 Test JSON output structure

- [x] Task 7: Unit tests for `wispRead` (AC: #2, #3, #7, #8)
  - [x] 7.1 Test read single key: returns matching entry
  - [x] 7.2 Test read all: returns array of all entries
  - [x] 7.3 Test read missing key: returns `WISP_NOT_FOUND` error
  - [x] 7.4 Test read empty issue: returns `[]`
  - [x] 7.5 Test read from non-existent issue: returns `ISSUE_NOT_FOUND`

- [x] Task 8: Run full test suite
  - [x] 8.1 `go test ./pkg/cmd/issues/... -run TestWisp` — all pass
  - [x] 8.2 `go test ./...` — no regressions
  - [x] 8.3 `go vet ./...` — no issues
  - [x] 8.4 `go build ./...` — clean build

### Review Follow-ups (AI)

- [ ] [AI-Review][Medium] M1: `wispRead` ignores `ctx` parameter — use `store.QueryRowContext(ctx, ...)` and `store.QueryContext(ctx, ...)` instead of non-context variants [pkg/cmd/issues/wisp.go:104,118,127]
- [ ] [AI-Review][Medium] M2: `wispRead` returns `any` with two concrete types (`*WispEntry` / `[]WispEntry`) — consider a wrapper type or separate functions for compile-time safety [pkg/cmd/issues/wisp.go:100]
- [ ] [AI-Review][Low] L1: No key-length validation before insert — `key_name VARCHAR(255)` but no pre-check; oversized key yields raw MySQL error instead of structured `GravaError` [pkg/cmd/issues/wisp.go:wispWrite]
- [ ] [AI-Review][Low] L2: Non-JSON single-entry output includes issue ID prefix but multi-entry format omits it — minor UX inconsistency [pkg/cmd/issues/wisp.go:196]
- [ ] [AI-Review][Low] L3: No dedicated test for `wispRead` all-entries path with non-existent issue — only single-key path tested for `ISSUE_NOT_FOUND` [pkg/cmd/issues/wisp_test.go]

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

claude-sonnet-4-6

### Debug Log References

None — clean implementation, no blockers.

### Completion Notes List

- Story 3.2 context created — requires new table, new command, new event type (2026-04-05)
- Implemented `wispWrite`: UPSERT into `wisp_entries` + heartbeat update on `issues`, wrapped in `WithAuditedTx` with `EventWispWrite` (2026-04-05)
- Implemented `wispRead`: single-key lookup returning `*WispEntry`, all-entries query returning `[]WispEntry`, issue-existence guard on both paths (2026-04-05)
- Cobra commands `grava wisp write` / `grava wisp read` registered in `AddCommands()` (2026-04-05)
- 10 unit tests: 5 for wispWrite (happy path, upsert, ISSUE_NOT_FOUND, heartbeat, JSON structure), 5 for wispRead (single key, all entries, WISP_NOT_FOUND, empty issue, ISSUE_NOT_FOUND) — all pass (2026-04-05)
- Full regression suite: all packages pass, `go vet` clean, `go build` clean (2026-04-05)
- Grava tracking: skipped — grava CLI not on PATH
- Code Review (2026-04-05): 1 High, 2 Medium, 3 Low. Fixed H1 (`--actor` flag unregistered in newWispWriteCmd). 5 action items created for M1, M2, L1-L3.

### File List

- pkg/migrate/migrations/008_create_wisp_entries.sql (new)
- pkg/utils/schema.go (SchemaVersion 7 → 8)
- pkg/dolt/events.go (EventWispWrite constant added)
- pkg/cmd/issues/wisp.go (new — wispWrite, wispRead, newWispCmd, newWispWriteCmd, newWispReadCmd)
- pkg/cmd/issues/wisp_test.go (new — 10 unit tests)
- pkg/cmd/issues/issues.go (AddCommands: registered newWispCmd)
