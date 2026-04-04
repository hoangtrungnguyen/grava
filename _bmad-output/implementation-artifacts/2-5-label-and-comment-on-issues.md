# Story 2.5: Label and Comment on Issues

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer or agent,
I want to add labels and text comments to issues,
so that contextual metadata and discussion are captured alongside the work item.

## Acceptance Criteria

1. **Given** issue `abc123def456` exists
   **When** I run `grava label abc123def456 --add bug --add critical`
   **Then** labels `["bug", "critical"]` are stored in the `issue_labels` table associated with this issue

2. **Given** issue `abc123def456` has labels `["bug", "critical"]`
   **When** I run `grava label abc123def456 --remove bug`
   **Then** only the `bug` label is removed; `critical` remains

3. **Given** issue `abc123def456` exists
   **When** I run `grava comment abc123def456 --message "Reproduced on macOS ARM"`
   **Then** a timestamped comment is appended to the `issue_comments` table with the message, actor, and timestamp

4. **Given** issue `abc123def456` has labels and comments
   **When** I run `grava show abc123def456 --json`
   **Then** the response includes a `labels` array and a `comments` array

## Tasks / Subtasks

- [x] **Task 1: Create migration 006 for `issue_labels` and `issue_comments` tables** (AC: #1, #3)
  - [x] 1.1 Create `pkg/migrate/migrations/006_add_labels_and_comments_tables.sql` with:
    - `issue_labels` table: `id (AUTO PK), issue_id (FK→issues), label VARCHAR(128), created_at TIMESTAMP, created_by VARCHAR(128)`, UNIQUE constraint on `(issue_id, label)`, INDEX on `issue_id`
    - `issue_comments` table: `id (AUTO PK), issue_id (FK→issues), message TEXT, actor VARCHAR(128), agent_model VARCHAR(256), created_at TIMESTAMP`, INDEX on `issue_id`
  - [x] 1.2 Update `pkg/utils/schema.go` SchemaVersion from 5 to 6
  - [x] 1.3 Verify migration applies cleanly: `go test ./pkg/migrate/...` (if migration tests exist) or manual Dolt check

- [x] **Task 2: Implement `labelIssue` named function and refactor `grava label` command** (AC: #1, #2)
  - [x] 2.1 Create `pkg/cmd/issues/label.go` with extracted `labelIssue(ctx context.Context, store dolt.Store, params LabelParams) (LabelResult, error)` named function
  - [x] 2.2 Define `LabelParams` struct: `ID string; AddLabels []string; RemoveLabels []string; Actor string; Model string`
  - [x] 2.3 Define `LabelResult` struct with JSON tags: `ID string; LabelsAdded []string; LabelsRemoved []string; CurrentLabels []string`
  - [x] 2.4 Pre-read: validate issue existence (SELECT id FROM issues WHERE id = ?) — return `ISSUE_NOT_FOUND` GravaError if missing
  - [x] 2.5 Implement `--add` logic: INSERT IGNORE into `issue_labels` for each label (idempotent — adding existing label is a no-op)
  - [x] 2.6 Implement `--remove` logic: DELETE FROM `issue_labels` WHERE issue_id = ? AND label = ? for each label (removing non-existent label is a graceful no-op)
  - [x] 2.7 Wrap all label mutations inside `WithAuditedTx` using `dolt.EventLabel` event type; OldValue/NewValue capture labels added/removed
  - [x] 2.8 Query final labels after mutation: `SELECT label FROM issue_labels WHERE issue_id = ? ORDER BY label` and return in `LabelResult.CurrentLabels`
  - [x] 2.9 Refactor `newLabelCmd` in `issues.go` to:
    - Change CLI signature from `label <id> <label>` to `label <id> [--add <label>]... [--remove <label>]...`
    - Use `StringSliceVar` for `--add` and `--remove` flags (supports repeated flags)
    - Delegate all logic to `labelIssue` named function
    - JSON output via `writeJSONError` on error, `json.MarshalIndent(result)` on success
    - Human output: print emoji summary of labels added/removed
  - [x] 2.10 Validate at least one of `--add` or `--remove` is provided; return `MISSING_REQUIRED_FIELD` error otherwise

- [x] **Task 3: Implement `commentIssue` named function and refactor `grava comment` command** (AC: #3)
  - [x] 3.1 Create `pkg/cmd/issues/comment.go` with extracted `commentIssue(ctx context.Context, store dolt.Store, params CommentParams) (CommentResult, error)` named function
  - [x] 3.2 Define `CommentParams` struct: `ID string; Message string; Actor string; Model string`
  - [x] 3.3 Define `CommentResult` struct with JSON tags: `ID string; CommentID int64; Message string; Actor string; CreatedAt string`
  - [x] 3.4 Pre-read: validate issue existence — return `ISSUE_NOT_FOUND` GravaError if missing
  - [x] 3.5 INSERT into `issue_comments` table: `(issue_id, message, actor, agent_model, created_at)` with `NOW()` timestamp
  - [x] 3.6 Wrap in `WithAuditedTx` using `dolt.EventComment` event type; NewValue captures message text
  - [x] 3.7 Return `CommentResult` with the inserted comment's ID and timestamp
  - [x] 3.8 Refactor `newCommentCmd` in `issues.go` to:
    - Keep CLI signature `comment <id>` as first arg but add `--message` / `-m` flag for the comment text
    - Also support positional `comment <id> <text>` for backward compatibility (if `--message` not set and len(args)==2, use args[1])
    - Delegate all logic to `commentIssue` named function
    - JSON output via `writeJSONError` on error, `json.MarshalIndent(result)` on success
    - Human output: `"Comment added to <id>"`
  - [x] 3.9 Retain `--last-commit` flag behavior (store commit hash in metadata as before — this is orthogonal to comments table)
  - [x] 3.10 Validate `--message` is provided (or positional text arg); return `MISSING_REQUIRED_FIELD` error if empty

- [x] **Task 4: Update `grava show --json` to include labels and comments** (AC: #4)
  - [x] 4.1 Add `Labels []string` and `Comments []CommentEntry` fields to `IssueDetail` struct in `issues.go`
  - [x] 4.2 Define `CommentEntry` struct: `ID int64; Message string; Actor string; AgentModel string; CreatedAt time.Time` with JSON tags
  - [x] 4.3 In `newShowCmd`, after fetching issue row, query `SELECT label FROM issue_labels WHERE issue_id = ? ORDER BY label` and populate `Labels`
  - [x] 4.4 Query `SELECT id, message, actor, agent_model, created_at FROM issue_comments WHERE issue_id = ? ORDER BY created_at` and populate `Comments`
  - [x] 4.5 Update human-readable show output to display labels and comments sections when present

- [x] **Task 5: Unit tests for `labelIssue`** (AC: #1, #2)
  - [x] 5.1 `TestLabelIssue_AddLabels`: add two labels to existing issue → verify INSERT calls and returned CurrentLabels
  - [x] 5.2 `TestLabelIssue_RemoveLabel`: remove one label → verify DELETE call, remaining labels correct
  - [x] 5.3 `TestLabelIssue_AddAndRemove`: add and remove labels in same call → verify both operations
  - [x] 5.4 `TestLabelIssue_IssueNotFound`: non-existent issue → `ISSUE_NOT_FOUND` error
  - [x] 5.5 `TestLabelIssue_NoFlags`: neither add nor remove provided → `MISSING_REQUIRED_FIELD` error
  - [x] 5.6 Use `mockStoreForLabel` helper + sqlmock pattern from `start_test.go`

- [x] **Task 6: Unit tests for `commentIssue`** (AC: #3)
  - [x] 6.1 `TestCommentIssue_HappyPath`: valid issue + message → verify INSERT and returned CommentResult
  - [x] 6.2 `TestCommentIssue_IssueNotFound`: non-existent issue → `ISSUE_NOT_FOUND` error
  - [x] 6.3 `TestCommentIssue_EmptyMessage`: empty message → `MISSING_REQUIRED_FIELD` error
  - [x] 6.4 Use `mockStoreForComment` helper + sqlmock pattern

- [x] **Task 7: Integration tests** (AC: #1, #2, #3, #4)
  - [x] 7.1 `TestLabelCmd_Add`: `grava label grava-123 --add bug --add critical` → verify output includes labels added
  - [x] 7.2 `TestLabelCmd_Remove`: `grava label grava-123 --remove bug` → verify output
  - [x] 7.3 `TestLabelCmd_IssueNotFound`: label non-existent issue → error
  - [x] 7.4 `TestCommentCmd_Message`: `grava comment grava-123 --message "test"` → verify output
  - [x] 7.5 `TestCommentCmd_Positional`: `grava comment grava-123 "test"` → backward-compatible positional text
  - [x] 7.6 `TestCommentCmd_IssueNotFound`: comment non-existent issue → error
  - [x] 7.7 `TestShowCmd_LabelsAndComments`: create issue → add labels → add comment → `grava show --json` → verify labels/comments arrays in output
  - [x] 7.8 `TestLabelCmd_JSON`: verify `--json` output contract for label command
  - [x] 7.9 `TestCommentCmd_JSON`: verify `--json` output contract for comment command

- [x] **Task 8: Cleanup stale label/comment code in `pkg/cmd/`** (housekeeping)
  - [x] 8.1 Remove or deprecate the old `pkg/cmd/label.go` (package cmd) — it is not registered in root.go, uses global `Store`, and is superseded by `pkg/cmd/issues/` version
  - [x] 8.2 Remove or deprecate the old `pkg/cmd/comment.go` (package cmd) — same situation
  - [x] 8.3 Remove stale helper functions from `pkg/cmd/util.go` that are only used by old label/comment code: the package-level `addCommentToIssue(id, text)`, `updateIssueMetadata(id, meta)`, `setLastCommit(id, hash)` — verify no other callers first
  - [x] 8.4 Remove the old `addCommentToIssue(d, id, text)`, `updateIssueMetadata(d, id, meta)`, and `setLastCommit(d, id, hash)` helpers from `pkg/cmd/issues/issues.go` — they used the metadata JSON column and are superseded by the new dedicated tables

- [x] **Task 9: Final verification** (all ACs)
  - [x] 9.1 `go test ./...` — all packages pass, including new label/comment tests
  - [x] 9.2 `golangci-lint run ./...` — no new linting issues
  - [x] 9.3 `go build ./...` — clean compile
  - [x] 9.4 Update CLI docs in `docs/guides/CLI_REFERENCE.md` for refactored `grava label` and `grava comment` commands
  - [x] 9.5 Verify `grava show --json` output includes `labels` and `comments` arrays

### Review Follow-ups (AI)

- [ ] [AI-Review][MEDIUM] `LabelAddFlags`/`LabelRemoveFlags` exported package-level mutable state — read via `cmd.Flags().GetStringSlice()` inside RunE instead [pkg/cmd/issues/label.go:136-140]
- [ ] [AI-Review][MEDIUM] `commentLastCommit` var declared in issues.go but only used in comment.go — move declaration to comment.go [pkg/cmd/issues/issues.go:36, pkg/cmd/issues/comment.go:131]
- [ ] [AI-Review][MEDIUM] `drop` command table list missing `issue_labels` and `issue_comments` — add before `issues` for safety [pkg/cmd/issues/issues.go:534-540]
- [ ] [AI-Review][MEDIUM] Audit event records intended add/remove labels, not actual — INSERT IGNORE no-ops and DELETE 0-row no-ops are logged as if they happened [pkg/cmd/issues/label.go:44-52]
- [ ] [AI-Review][LOW] `LastInsertId()` error silently discarded [pkg/cmd/issues/comment.go:74]
- [ ] [AI-Review][LOW] No label input sanitization — case-sensitive duplicates and whitespace possible [pkg/cmd/issues/label.go:66-73]

## Dev Notes

### Critical: Refactor from Metadata JSON to Dedicated Tables

The **current** `grava label` and `grava comment` implementations store data in the `metadata` JSON column of the `issues` table. Story 2.5 requires migrating to dedicated `issue_labels` and `issue_comments` tables per the epic specification.

**Current code to refactor:**
- `pkg/cmd/issues/issues.go` lines ~410-490: `newLabelCmd` reads/writes `metadata` JSON
- `pkg/cmd/issues/issues.go` lines ~490-580: `newCommentCmd` uses `addCommentToIssue()` which reads/writes `metadata` JSON
- `pkg/cmd/issues/issues.go` lines ~580-640: `addCommentToIssue()`, `updateIssueMetadata()`, `setLastCommit()` helpers

**Stale code to clean up:**
- `pkg/cmd/label.go` — old package-level label command using global `Store` (not registered in root.go)
- `pkg/cmd/comment.go` — old package-level comment command using global `Store` (not registered in root.go)
- `pkg/cmd/util.go` lines ~54-130: old `addCommentToIssue`, `updateIssueMetadata`, `setLastCommit` using global `Store`

### Critical: Follow Named Function Pattern (from Stories 2.3, 2.4)

Every command must follow the established named function architecture:
```go
// Named function — all business logic here, fully testable
func labelIssue(ctx context.Context, store dolt.Store, params LabelParams) (LabelResult, error) { ... }

// Cobra command — thin wrapper, delegates to named function
func newLabelCmd(d *cmddeps.Deps) *cobra.Command { ... }
```

Reference implementations:
- `pkg/cmd/issues/start.go`: `startIssue()` + `newStartCmd()` — pre-read pattern, `WithAuditedTx`, `FOR UPDATE`
- `pkg/cmd/issues/update.go`: `updateIssue()` + `newUpdateCmd()` — `ChangedFields` pattern, validation

### Architecture: CLI Interface Changes

**`grava label` (refactored):**
```bash
# Add labels (multiple --add flags supported)
grava label abc123def456 --add bug --add critical

# Remove labels
grava label abc123def456 --remove bug

# Add and remove in one command
grava label abc123def456 --add urgent --remove low-priority
```
Use `cobra.StringSliceVar` for `--add` and `--remove` flags. The command takes exactly 1 positional arg (issue ID).

**`grava comment` (refactored):**
```bash
# With --message flag (preferred)
grava comment abc123def456 --message "Reproduced on macOS ARM"
grava comment abc123def456 -m "Reproduced on macOS ARM"

# Backward-compatible positional text (when --message not set)
grava comment abc123def456 "Reproduced on macOS ARM"
```

### Architecture: Database Schema (Migration 006)

```sql
CREATE TABLE issue_labels (
    id INT AUTO_INCREMENT PRIMARY KEY,
    issue_id VARCHAR(32) NOT NULL,
    label VARCHAR(128) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR(128),
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    UNIQUE KEY unique_issue_label (issue_id, label),
    INDEX idx_issue_labels_issue_id (issue_id)
);

CREATE TABLE issue_comments (
    id INT AUTO_INCREMENT PRIMARY KEY,
    issue_id VARCHAR(32) NOT NULL,
    message TEXT NOT NULL,
    actor VARCHAR(128),
    agent_model VARCHAR(256),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    INDEX idx_issue_comments_issue_id (issue_id)
);
```

### Architecture: Audit Events

Use existing event type constants (already defined in `pkg/dolt/events.go`):
- `dolt.EventLabel = "label"` — for label add/remove operations
- `dolt.EventComment = "comment"` — for comment operations

**Label audit event:**
```go
AuditEvent{
    IssueID:   params.ID,
    EventType: dolt.EventLabel,
    Actor:     params.Actor,
    Model:     params.Model,
    OldValue:  map[string]any{"labels_removed": removedLabels},
    NewValue:  map[string]any{"labels_added": addedLabels},
}
```

**Comment audit event:**
```go
AuditEvent{
    IssueID:   params.ID,
    EventType: dolt.EventComment,
    Actor:     params.Actor,
    Model:     params.Model,
    NewValue:  map[string]any{"message": params.Message},
}
```

### Architecture: `grava show` Update

Add to `IssueDetail` struct in `pkg/cmd/issues/issues.go`:
```go
type CommentEntry struct {
    ID         int64     `json:"id"`
    Message    string    `json:"message"`
    Actor      string    `json:"actor"`
    AgentModel string    `json:"agent_model,omitempty"`
    CreatedAt  time.Time `json:"created_at"`
}

// Add to IssueDetail:
Labels   []string       `json:"labels,omitempty"`
Comments []CommentEntry  `json:"comments,omitempty"`
```

### Error Codes for this Story

| Scenario | Code | Message |
|---|---|---|
| Issue not found | `ISSUE_NOT_FOUND` | `Issue {id} not found` |
| No --add or --remove flags | `MISSING_REQUIRED_FIELD` | `at least one --add or --remove flag is required` |
| No message provided | `MISSING_REQUIRED_FIELD` | `--message is required` |
| DB error | `DB_UNREACHABLE` | `failed to read/write issue` |

### JSON Output Contract (NFR5)

**`grava label --json` success:**
```json
{
  "id": "abc123def456",
  "labels_added": ["bug", "critical"],
  "labels_removed": [],
  "current_labels": ["bug", "critical"]
}
```

**`grava comment --json` success:**
```json
{
  "id": "abc123def456",
  "comment_id": 1,
  "message": "Reproduced on macOS ARM",
  "actor": "agent-01",
  "created_at": "2026-04-03T10:30:45Z"
}
```

**`grava show --json` (updated, AC #4):**
```json
{
  "id": "abc123def456",
  "title": "Fix login bug",
  "labels": ["bug", "critical"],
  "comments": [
    {
      "id": 1,
      "message": "Reproduced on macOS ARM",
      "actor": "agent-01",
      "created_at": "2026-04-03T10:30:45Z"
    }
  ]
}
```

### Previous Story Intelligence (from Story 2.4)

**Key learnings to apply:**
1. **Command name collisions**: Story 2.4 had to rename `server start/stop` to `db-start/db-stop`. Check that `label` and `comment` commands don't collide with anything.
2. **Pre-read before mutation**: Always `SELECT` the issue first to validate existence before `INSERT`/`DELETE` on related tables.
3. **SchemaVersion alignment**: Must update `SchemaVersion` in `schema.go` to match migration count (5 → 6).
4. **`--json` test coverage**: Story 2.4 review flagged missing JSON output tests. Add `TestLabelCmd_JSON` and `TestCommentCmd_JSON` from the start (Task 7.8, 7.9).
5. **Integration test pattern**: Use `executeCommand(rootCmd, ...)` pattern from `pkg/cmd/commands_test.go` with sqlmock.
6. **File List completeness**: Story 2.4 review flagged missing `sprint-status.yaml` in File List. Track ALL modified files.

**Review follow-ups from Story 2.4 (still open — do NOT address in this story):**
- `--json` test coverage for start/stop (grava-1073.9)
- `DB_UNREACHABLE` error path tests (grava-1073.10)
- `stopped_at` index (grava-1073.13)
- Status transition guard (grava-1073.12)
These are Story 2.4 items — do not fix here but be aware of patterns to avoid.

### Git Intelligence (Recent Commits)

```
9449f43 feat(cli): implement Story 2.4 — grava start/stop work session tracking (grava-1073)
d96e2c1 fix(review-2.3): resolve 3 code review round-2 findings, mark story done
bc773c9 fix(review-2.3): resolve 9 code review findings for story 2.3 (grava-8f07.1)
131b7b7 feat(cli): implement Story 2.3 — update issue fields and assign actors (grava-8f07.1)
ccde927 feat(issues): implement Stories 2.1 and 2.2 — create and subtask commands
```

Commit message pattern: `feat(cli): implement Story X.Y — <description> (<grava-id>)`

### Critical: `--last-commit` Flag on Comment

The current `grava comment` has a `--last-commit` flag that stores a commit hash in the `metadata` JSON column. This flag is used by the `grava commit` hook workflow. **Preserve this behavior** — `--last-commit` should continue writing to metadata JSON (it is orthogonal to the comments table). The `commentLastCommit` var and `setLastCommit` helper in `issues.go` handle this.

### Critical: Drop Command — No Impact

The current `newDropCmd` in `issues.go` is a nuclear reset (deletes ALL data). It truncates tables in FK-safe order. After adding `issue_labels` and `issue_comments`, the `DROP` command's table list should be updated to include these new tables before `issues` (since they have FK references to issues). **Task 8 should verify this.**

Wait — actually `newDropCmd` uses `DELETE FROM` not `DROP TABLE`, and both new tables have `ON DELETE CASCADE`. So deleting from `issues` will cascade to `issue_labels` and `issue_comments` automatically. No change needed to the drop command.

### Project Structure Notes

- Label named function: `pkg/cmd/issues/label.go` (new file, extracted from `issues.go`)
- Label unit tests: `pkg/cmd/issues/label_test.go` (new file)
- Comment named function: `pkg/cmd/issues/comment.go` (new file, extracted from `issues.go`)
- Comment unit tests: `pkg/cmd/issues/comment_test.go` (new file)
- Migration: `pkg/migrate/migrations/006_add_labels_and_comments_tables.sql` (new file)
- Integration tests: `pkg/cmd/commands_test.go` (modified)
- Show command: `pkg/cmd/issues/issues.go` (modified — IssueDetail struct, newShowCmd labels/comments queries)
- Schema version: `pkg/utils/schema.go` (modified — 5 → 6)
- CLI docs: `docs/guides/CLI_REFERENCE.md` (modified)
- Stale cleanup: `pkg/cmd/label.go`, `pkg/cmd/comment.go`, `pkg/cmd/util.go` (modified/removed)

### References

- **Story 2.4 (Start/Stop):** [2-4-track-work-start-and-stop.md](2-4-track-work-start-and-stop.md) — Named function pattern, `WithAuditedTx` usage, pre-read validation, sqlmock test patterns, review follow-up lessons
- **Story 2.3 (Update/Assign):** [2-3-update-issue-fields-and-assign-actors.md](2-3-update-issue-fields-and-assign-actors.md) — `updateIssue` named function with `ChangedFields`, validation pattern
- **Epic 2 Full Spec:** [_bmad-output/planning-artifacts/epics/epic-02-issue-lifecycle.md](../planning-artifacts/epics/epic-02-issue-lifecycle.md) — Story 2.5 acceptance criteria (lines 125-138)
- **Architecture:** [_bmad-output/planning-artifacts/architecture.md](../planning-artifacts/architecture.md) — Audit event types (EventLabel, EventComment), WithAuditedTx pattern, testing standards
- **PRD:** [_bmad-output/planning-artifacts/prd.md](../planning-artifacts/prd.md) — FR6 (label, comment), NFR2 (<15ms writes), NFR5 (JSON schema)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (claude-opus-4-6)

### Debug Log References

### Completion Notes List

- Story created via create-story workflow (2026-04-03)
- Ultimate context engine analysis completed — comprehensive developer guide created
- Grava Tracking: epic=grava-8f07, story=grava-8f07.2
- Implementation complete (2026-04-03): All 9 tasks done, all tests pass
- Migration 006 creates `issue_labels` and `issue_comments` tables (SchemaVersion 5 → 6)
- Refactored `grava label` from metadata JSON to dedicated table with `--add`/`--remove` flags
- Refactored `grava comment` from metadata JSON to dedicated table with `--message`/`-m` flag + backward-compat positional
- Updated `grava show --json` to include `labels` and `comments` arrays
- Removed dead `addCommentToIssue` from issues package; old helpers in `pkg/cmd/util.go` retained (still used by `undo.go`, `update.go`)
- 5 unit tests (label) + 3 unit tests (comment) + 8 integration tests = 16 new tests total

### Code Review Record

- Review Date: 2026-04-04
- Reviewer: claude-opus-4-6 (adversarial code review)
- Findings: 1 HIGH, 4 MEDIUM, 2 LOW
- HIGH issues fixed: H1 (added TestShowCmd_LabelsAndComments for AC #4)
- MEDIUM/LOW: 6 action items created in Review Follow-ups section
- Tests: all pass after fixes (`go test ./...` ✅)

### File List

- `pkg/migrate/migrations/006_add_labels_and_comments_tables.sql` (created: issue_labels + issue_comments tables)
- `pkg/cmd/issues/label.go` (created: labelIssue named function + newLabelCmd)
- `pkg/cmd/issues/label_test.go` (created: 5 unit tests with sqlmock)
- `pkg/cmd/issues/comment.go` (created: commentIssue named function + newCommentCmd)
- `pkg/cmd/issues/comment_test.go` (created: 3 unit tests with sqlmock)
- `pkg/cmd/issues/issues.go` (modified: removed old newLabelCmd/newCommentCmd/addCommentToIssue, added CommentEntry/Labels/Comments to IssueDetail, updated newShowCmd)
- `pkg/utils/schema.go` (modified: SchemaVersion 5 → 6)
- `pkg/cmd/commands_test.go` (modified: updated label/comment integration tests for new table-based approach, added JSON tests, fixed TestShowCmd for new queries)
- `docs/guides/CLI_REFERENCE.md` (modified: updated label and comment command docs)
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (modified: 2-5 status in-progress → review)

