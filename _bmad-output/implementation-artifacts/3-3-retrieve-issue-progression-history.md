# Story 3.3: Retrieve Issue Progression History

Status: ready-for-dev

## Story

As a developer or agent,
I want to retrieve the full ordered progression log of an issue,
So that I can understand what previous agents did before picking up the work.

## Acceptance Criteria

1. **AC#1 — Full History**
   Given issue `abc123def456` has had multiple state transitions and Wisp writes,
   When I run `grava history abc123def456`,
   Then an ordered log is returned showing each event: `{event_type, actor, timestamp, details}` — covering status changes, claim/release, Wisp writes, comments, and label changes.

2. **AC#2 — JSON Output**
   When I run `grava history abc123def456 --json`,
   Then a JSON array of events is returned conforming to NFR5 schema: `[{"event_type": "create", "actor": "agent-01", "timestamp": "...", "details": {...}}, ...]`.

3. **AC#3 — Date Filter**
   When I run `grava history abc123def456 --since "2026-03-01"`,
   Then events are filtered to after the given date.

4. **AC#4 — Pre-Claim Context**
   A new agent running `grava history abc123def456` before claiming can see the full prior context, enabling crash-safe handoff.

5. **AC#5 — Empty History**
   Given issue `abc123def456` exists but has no events (e.g., just created),
   When I run `grava history abc123def456 --json`,
   Then it returns an empty array `[]`, not an error.

6. **AC#6 — Non-Existent Issue**
   Given no issue with ID `nonexistent` exists,
   When I run `grava history nonexistent --json`,
   Then it returns `{"error": {"code": "ISSUE_NOT_FOUND", "message": "issue nonexistent not found"}}`.

## Tasks / Subtasks

- [ ] Task 1: Implement `issueHistory` named function (AC: #1, #3, #5, #6)
  - [ ] 1.1 Define `HistoryEntry` struct: `EventType` (json:"event_type"), `Actor` (json:"actor"), `Timestamp` (json:"timestamp"), `Details` (json:"details")
  - [ ] 1.2 Implement `issueHistory(ctx, store, issueID, since)`:
    - Verify issue exists: `SELECT id FROM issues WHERE id = ?` → `ISSUE_NOT_FOUND` if not
    - Query events: `SELECT event_type, actor, old_value, new_value, timestamp FROM events WHERE issue_id = ? [AND timestamp >= ?] ORDER BY timestamp ASC, id ASC`
    - Map each row to `HistoryEntry`:
      - `EventType` = `event_type`
      - `Actor` = `actor`
      - `Timestamp` = `timestamp`
      - `Details` = merge of `old_value` and `new_value` (or `new_value` if old is null)
    - Return `[]HistoryEntry`
  - [ ] 1.3 No transaction needed (read-only)

- [ ] Task 2: Build Cobra command (AC: #1-#6)
  - [ ] 2.1 Create `pkg/cmd/issues/history.go` with `newHistoryCmd(d)`
  - [ ] 2.2 Args: `history <issue-id>` — exactly 1 arg
  - [ ] 2.3 Flag: `--since` (string, RFC3339 date) — optional date filter
  - [ ] 2.4 JSON output: `json.NewEncoder(cmd.OutOrStdout()).Encode(entries)` for `--json`
  - [ ] 2.5 Human-readable output: formatted table or list (non-JSON mode)
  - [ ] 2.6 Register in `AddCommands()` in `issues.go`

- [ ] Task 3: Unit tests for `issueHistory` (AC: #1, #3, #5, #6)
  - [ ] 3.1 Test happy path: issue with multiple events returns ordered array
  - [ ] 3.2 Test `--since` filter: only events after the date are returned
  - [ ] 3.3 Test empty history: returns `[]`
  - [ ] 3.4 Test non-existent issue: returns `ISSUE_NOT_FOUND`
  - [ ] 3.5 Test JSON output structure matches `HistoryEntry` schema
  - [ ] 3.6 Test event types coverage: verify create, claim, update, wisp_write, comment, label events all appear

- [ ] Task 4: Integration with Wisp events (AC: #1, #4)
  - [ ] 4.1 Verify that `EventWispWrite` events (from Story 3.2) appear in history when `grava wisp write` is used
  - [ ] 4.2 Verify `EventClaim` events (from Story 3.1) appear in history when `grava claim` is used

- [ ] Task 5: Run full test suite
  - [ ] 5.1 `go test ./pkg/cmd/issues/... -run TestHistory` — all pass
  - [ ] 5.2 `go test ./...` — no regressions
  - [ ] 5.3 `go vet ./...` — no issues
  - [ ] 5.4 `go build ./...` — clean build

## Dev Notes

### Architecture Patterns (MUST FOLLOW)

**Named Function Pattern:**
```go
func issueHistory(ctx context.Context, store dolt.Store, issueID, since string) ([]HistoryEntry, error)
```
- Read-only — no transaction needed
- Verify issue exists first
- Return `*gravaerrors.GravaError` on all user-facing errors

**JSON Output Contract (NFR5):**
- Success: `json.NewEncoder(cmd.OutOrStdout()).Encode(entries)` → stdout
- Error: `writeJSONError(cmd, err)` → stderr

### Event Types Already in the System

The `events` table already captures all mutations from Stories 2.1-2.6 and 3.1:

| Event Constant | event_type value | Written By |
|---|---|---|
| `dolt.EventCreate` | `"create"` | Story 2.1 |
| `dolt.EventSubtask` | `"subtask"` | Story 2.2 |
| `dolt.EventUpdate` | `"update"` | Story 2.3 |
| `dolt.EventStart` | `"start"` | Story 2.4 |
| `dolt.EventStop` | `"stop"` | Story 2.4 |
| `dolt.EventLabel` | `"label"` | Story 2.5 |
| `dolt.EventComment` | `"comment"` | Story 2.5 |
| `dolt.EventDrop` | `"drop"` | Story 2.6 |
| `dolt.EventClear` | `"clear"` | Story 2.6 |
| `dolt.EventClaim` | `"claim"` | Story 3.1 |
| `dolt.EventWispWrite` | `"wisp_write"` | Story 3.2 (add if not yet present) |

### Existing Database Schema

```sql
-- events table (migration 001)
CREATE TABLE events (
    id INT AUTO_INCREMENT PRIMARY KEY,
    issue_id VARCHAR(32) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    actor VARCHAR(128) NOT NULL,
    old_value JSON,
    new_value JSON,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    INDEX idx_issue_timestamp (issue_id, timestamp)
);
```

The `idx_issue_timestamp` index provides efficient querying for history lookups.

### Command Structure

```
grava history <issue-id> [--since DATE] [--json]
```

### Error Codes

| Scenario | Code | Message |
|---|---|---|
| Issue not found | `ISSUE_NOT_FOUND` | `issue {id} not found` |
| DB error | `DB_UNREACHABLE` | `failed to read history` |

### Date Filtering

The `--since` flag accepts RFC3339 or date-only format (`"2026-03-01"`). Parse with `time.Parse` trying multiple layouts:
1. `"2006-01-02T15:04:05Z07:00"` (RFC3339)
2. `"2006-01-02"` (date only, treat as midnight UTC)

### Previous Story Learnings (from Stories 2.1-2.6)

- Read-only queries don't need `WithAuditedTx`
- Use `store.QueryContext()` for SELECT queries (not ExecContext)
- Always close `rows.Close()` after iteration
- Use `sql.NullString` / `sql.NullTime` for nullable columns
- `json.Unmarshal` for JSON columns (old_value, new_value)

### Project Structure Notes

- New command: `pkg/cmd/issues/history.go`
- New tests: `pkg/cmd/issues/history_test.go`
- No new migration needed — uses existing `events` table
- Command registration: `pkg/cmd/issues/issues.go` — `AddCommands()`

### References

- [Source: _bmad-output/planning-artifacts/epics/epic-03-atomic-claim.md#Story 3.3]
- [Source: _bmad-output/planning-artifacts/architecture.md#audit trail, events table]
- [Source: pkg/migrate/migrations/001_initial_schema.sql — events table schema]
- [Source: pkg/dolt/events.go — event constants]
- [Source: pkg/dolt/tx.go — WithAuditedTx pattern reference]
- [Source: _bmad-output/implementation-artifacts/3-2-write-and-read-wisp-ephemeral-state.md — EventWispWrite from Story 3.2]

## Dev Agent Record

### Agent Model Used

### Debug Log References

### Completion Notes List

- Story 3.3 context created — uses existing events table, no migration needed (2026-04-05)

### File List
