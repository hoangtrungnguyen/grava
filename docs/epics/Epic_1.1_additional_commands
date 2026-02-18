# Epic 1.1: Additional Commands, Audit Columns & Improvements

**Goal:** Extend the Grava CLI with destructive data-management commands, add provenance/audit columns across all tables, and apply cross-cutting improvements.

---

## 1. New Commands

### 1.1 `grava drop` — Nuclear Reset

Deletes **all data** from every table. Intended for development resets or clean-slate scenarios. Must be guarded with a confirmation prompt to prevent accidental data loss.

**Usage:**
```bash
grava drop [flags]
```

**Flags:**
- `--force`: Skip the interactive confirmation prompt. Required for non-interactive / CI use.

**Behaviour:**
1. Without `--force`, prompt the user:
   ```
   ⚠️  This will DELETE ALL DATA from the Grava database.
   Type "yes" to confirm:
   ```
2. Truncate tables in FK-safe order:
   - `DELETE FROM dependencies`
   - `DELETE FROM events`
   - `DELETE FROM deletions`
   - `DELETE FROM child_counters`
   - `DELETE FROM issues`
3. Print summary:
   ```
   💣 All Grava data has been dropped.
   ```

**Exit Codes:**
- `0` — success
- `1` — user cancelled or DB error

**Acceptance Criteria:**
- [ ] `grava drop` prompts for confirmation and aborts on non-"yes" input
- [ ] `grava drop --force` skips prompt and deletes all data
- [ ] All 5 tables are emptied in FK-safe order
- [ ] Unit tests with mock store verify correct SQL execution order
- [ ] CLI reference updated

---

### 1.2 `grava clear` — Date-Range Purge

Deletes issues (and their related dependencies/events) created within a specified date range. Records deletions in the `deletions` table for tombstone tracking.

**Usage:**
```bash
grava clear --from <date> --to <date> [flags]
```

**Flags:**
- `--from string`: Start date (inclusive), format `YYYY-MM-DD` (**required**).
- `--to string`: End date (inclusive), format `YYYY-MM-DD` (**required**).
- `--force`: Skip interactive confirmation.
- `--include-wisps`: Also delete ephemeral Wisp issues in the range (by default, only non-ephemeral issues are cleared).

**Behaviour:**
1. Parse `--from` and `--to` into `time.Time`. Fail fast on invalid format.
2. Validate `from <= to`.
3. Without `--force`, show preview:
   ```
   ⚠️  Found 12 issue(s) created between 2026-01-01 and 2026-01-31.
   Type "yes" to delete them:
   ```
4. For each matching issue:
   a. Record tombstone in `deletions` table (reason=`clear`, actor=`grava-clear`).
   b. Delete from `issues` (cascading FK handles `dependencies` and `events`).
5. Print summary:
   ```
   🗑️  Cleared 12 issue(s) from 2026-01-01 to 2026-01-31. Tombstones recorded.
   ```

**Acceptance Criteria:**
- [ ] `--from` and `--to` are required; error if missing
- [ ] Invalid date formats produce a clear error message
- [ ] `from > to` is rejected
- [ ] Default behaviour excludes ephemeral issues; `--include-wisps` includes them
- [ ] Tombstones written to `deletions` table before DELETE
- [ ] Unit tests cover: normal range, empty range, invalid dates, force mode
- [ ] CLI reference updated

---

## 2. Audit Columns: `created_by`, `updated_by`, `agent_model`

Add three new columns to **every** table to track who (human or agent) created/modified each row, and which AI model was involved.

### 2.1 Schema Migration (`002_audit_columns.sql`)

```sql
-- Migration: Add audit/provenance columns to all tables

-- issues
ALTER TABLE issues
  ADD COLUMN created_by VARCHAR(128) DEFAULT 'unknown' COMMENT 'User or agent who created this row',
  ADD COLUMN updated_by VARCHAR(128) DEFAULT 'unknown' COMMENT 'User or agent who last modified this row',
  ADD COLUMN agent_model VARCHAR(128) COMMENT 'AI model identifier (e.g. gemini-2.5-pro, claude-4)';

-- dependencies
ALTER TABLE dependencies
  ADD COLUMN created_by VARCHAR(128) DEFAULT 'unknown',
  ADD COLUMN updated_by VARCHAR(128) DEFAULT 'unknown',
  ADD COLUMN agent_model VARCHAR(128);

-- events
ALTER TABLE events
  ADD COLUMN created_by VARCHAR(128) DEFAULT 'unknown',
  ADD COLUMN updated_by VARCHAR(128) DEFAULT 'unknown',
  ADD COLUMN agent_model VARCHAR(128);

-- child_counters
ALTER TABLE child_counters
  ADD COLUMN created_by VARCHAR(128) DEFAULT 'unknown',
  ADD COLUMN updated_by VARCHAR(128) DEFAULT 'unknown',
  ADD COLUMN agent_model VARCHAR(128);

-- deletions
ALTER TABLE deletions
  ADD COLUMN created_by VARCHAR(128) DEFAULT 'unknown',
  ADD COLUMN updated_by VARCHAR(128) DEFAULT 'unknown',
  ADD COLUMN agent_model VARCHAR(128);
```

### 2.2 CLI Integration

| Source | How to populate |
|--------|----------------|
| `created_by` | New `--actor` global flag, or `$GRAVA_ACTOR` env var. Default: `"unknown"`. |
| `updated_by` | Same as `created_by`, set on every write operation. |
| `agent_model` | New `--agent-model` global flag, or `$GRAVA_AGENT_MODEL` env var. Default: `NULL`. |

**Implementation:**
1. Add `--actor` and `--agent-model` as **persistent flags** on `rootCmd`.
2. Bind to env vars `GRAVA_ACTOR` and `GRAVA_AGENT_MODEL` via viper.
3. Update **every** `INSERT` and `UPDATE` query in all commands to include the three new columns.
4. Display `created_by` and `agent_model` in `grava show` output.

**Affected Commands:**
- `create` / `subtask` → set `created_by`, `updated_by`, `agent_model` on INSERT
- `update` → set `updated_by`, `agent_model` on UPDATE
- `comment` / `label` → set `updated_by`, `agent_model` on metadata UPDATE
- `assign` → set `updated_by`, `agent_model` on UPDATE
- `dep` → set `created_by`, `updated_by`, `agent_model` on INSERT
- `compact` → set `created_by`, `agent_model` on deletions INSERT
- `clear` (new) → set `created_by`, `agent_model` on deletions INSERT
- `show` → display `created_by`, `updated_by`, `agent_model`

**Acceptance Criteria:**
- [ ] Migration script `002_audit_columns.sql` created and tested
- [ ] All INSERT/UPDATE queries include audit columns
- [ ] `--actor` and `--agent-model` global flags work
- [ ] Env vars `GRAVA_ACTOR` and `GRAVA_AGENT_MODEL` are respected
- [ ] `grava show` displays provenance info
- [ ] Existing tests updated to account for new columns
- [ ] CLI reference updated with new global flags

---

## 3. Suggested Improvements

After analysing the full codebase, here are cross-cutting improvements ranked by impact:

### 🔴 High Priority

#### 3.1 Transaction Safety
**Problem:** Commands like `compact` and the new `clear`/`drop` perform multiple sequential SQL statements without a transaction. If the process crashes mid-way, the DB is left in an inconsistent state (e.g. tombstone recorded but issue not deleted).

**Fix:** Add `BeginTx() / Commit() / Rollback()` to the `Store` interface and wrap multi-statement operations in transactions.

```go
// Add to Store interface:
BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
```

#### 3.2 Input Validation Layer
**Problem:** There's no centralized validation. Each command manually checks inputs. Invalid `issue_type` or `status` values only fail at the DB constraint level, producing cryptic MySQL errors instead of user-friendly messages.

**Fix:** Create a `pkg/validation` package with reusable validators:
```go
func ValidateIssueType(t string) error { ... }
func ValidateStatus(s string) error { ... }
func ValidatePriority(p string) (int, error) { ... }
func ValidateDateRange(from, to string) (time.Time, time.Time, error) { ... }
```

#### 3.3 Consistent Error Handling & Logging
**Problem:** Error messages are inconsistent. Some use `fmt.Errorf`, some use `cmd.Printf`. No structured logging for debugging agent workflows.

**Fix:** Add a lightweight structured logger (e.g. `log/slog`) with `--verbose` / `--json` flags:
```bash
grava create --title "Fix bug" --verbose   # debug output
grava list --json                          # machine-readable JSON output
```

### 🟡 Medium Priority

#### 3.4 `--json` Output Format
**Problem:** Output is human-readable tabwriter only. AI agents parsing Grava output need machine-readable JSON.

**Fix:** Add `--json` / `--output json` global flag. When set, all commands emit JSON to stdout. This is critical for the MCP integration (Epic 6).

#### 3.5 `grava export` / `grava import` Commands
**Problem:** No way to bulk export/import issues. Useful for backups, migrations between Dolt remotes, and seeding test data.

**Fix:**
```bash
grava export --format jsonl > issues.jsonl
grava import --file issues.jsonl --skip-deleted
```

#### 3.6 `grava undo` / `grava history` Commands
**Problem:** Grava sits on Dolt (a version-controlled DB), but there's no CLI exposure of `dolt diff`, `dolt log`, or `dolt reset`. This is a missed opportunity.

**Fix:**
```bash
grava history                  # shows dolt log
grava history <id>             # shows change history for a single issue via events table
grava undo                     # runs dolt reset --hard HEAD~1
```

#### 3.7 Priority Mapping Duplication
**Problem:** The priority string→int mapping (`critical=0, high=1, medium=2, low=3`) is duplicated in `create.go`, `subtask.go`, and potentially `quick.go`. This violates DRY and is error-prone.

**Fix:** Extract to `pkg/core/priority.go`:
```go
func ParsePriority(s string) (int, error) { ... }
func FormatPriority(i int) string { ... }
```

### 🟢 Nice to Have

#### 3.8 Config-Driven Default Actor
Allow `.grava/config.yaml` to set defaults:
```yaml
db_url: "root@tcp(127.0.0.1:3306)/dolt?parseTime=true"
actor: "alice"
agent_model: "gemini-2.5-pro"
```

#### 3.9 `grava stats` Command
Quick overview of the database:
```
📊 Grava Stats
  Total issues:     142
  Open:              87
  Closed:            55
  Wisps:             12
  Dependencies:      34
  Events:           412
```

#### 3.10 Soft Delete Instead of Hard Delete
Instead of `DELETE FROM issues`, set `status = 'tombstone'` and record in `deletions`. This preserves the full audit trail in the `issues` table itself while still honoring the deletion semantics for queries.

---

## Implementation Order

| Phase | Items | Effort |
|-------|-------|--------|
| **Phase 1** | 3.7 (Priority extraction), 2.x (Audit columns migration + CLI flags) | 1 day |
| **Phase 2** | 1.1 (`drop`), 1.2 (`clear`), 3.1 (Transactions) | 1 day |
| **Phase 3** | 3.2 (Validation), 3.3 (Logging), 3.4 (JSON output) | 1 day |
| **Phase 4** | 3.5 (Export/Import), 3.6 (History/Undo), 3.8-3.10 | 2 days |

---

## Dependencies

- **Requires:** Epic 1 (Storage Substrate) — all tables must exist
- **Blocks:** Epic 6 (MCP Integration) — JSON output mode is prerequisite for agent consumption
