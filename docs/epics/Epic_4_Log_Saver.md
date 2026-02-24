# Epic 4: Grava Flight Recorder & Ephemeral Store

**Goal:** Implement a persistent, structured logging system and an ephemeral state store. This captures command execution, internal errors, and agent interactions while managing transient "in-flight" state (heartbeats, claims) in an isolated database to prevent Dolt history bloat.

**Implementation Plan:** [Epic 4.1: Ephemeral Store Implementation Plan](./Epic_4.1_Ephemeral_Store_Implementation_Plan.md)

**Success Criteria:**
- `slog` infrastructure initialized with custom handler writing to `.grava/logs/grava.log`
- Log rotation preventing infinite file growth
- Ephemeral SQLite store initialized at `.grava/ephemeral.sqlite3` for transient state
- TTL-based automatic cleanup of ephemeral records (heartbeats, stale claims)
- Global `--debug` flag enabling verbose logging
- Panic recovery middleware capturing stack traces
- `grava debug logs` command implemented for viewing and exporting logs
- Session artifact storage capturing LLM prompts and responses
- `grava show <id> --events` displays full per-issue audit trail
- `grava show <id> --files` displays affected files for the issue
- `grava debug replay <ExecutionID>` replays a session step-by-step for bug reproduction

## User Stories

### 4.1 Structured Logger Initialization
**As a** developer
**I want to** have a structured logging system
**So that** I can debug issues with machine-readable logs

**Acceptance Criteria:**
- `internal/debug/logger.go` created
- `Setup(verbose bool)` function defined
- Logs stored in `.grava/logs/grava.log`
- Format is JSON Lines (`jsonl`) with timestamp, level, message, metadata
- Unique `ExecutionID` (UUID) attached to every log entry

### 4.2 Log Rotation
**As a** user
**I want to** prevent log files from continuously growing
**So that** I don't run out of disk space

**Acceptance Criteria:**
- Check `grava.log` size on startup
- If > 5MB, rename to `grava.log.1` (rolling previous ones)
- Max 5 files kept

### 4.3 Debug Flag
**As a** developer
**I want to** enable verbose logging on demand
**So that** I can see internal state during debugging

**Acceptance Criteria:**
- `--debug` flag added to root command
- `GRAVA_DEBUG` environment variable supported
- Default: Log only `INFO` and `ERROR` to file
- With debug enabled: Log `DEBUG` level and print to `stderr` in real-time

### 4.4 Panic Recovery Middleware
**As a** system maintainer
**I want to** catch crashes and log stack traces
**So that** I can diagnose unexpected failures

**Acceptance Criteria:**
- `defer/recover` block in `main.go`
- Stack trace captured on panic
- Logged with level `FATAL` and `ExecutionID`
- User-friendly message printed to stderr

### 4.5 Command Tracing
**As a** developer
**I want to** trace command execution
**So that** I know exactly what commands were run and with what arguments

**Acceptance Criteria:**
- Log `Command started`, `Args`, `Flags` in `PersistentPreRun`
- Log `Command finished`, `Duration` in `PersistentPostRun`

### 4.6 Session Artifact Storage
**As an** AI agent developer
**I want to** capture LLM prompts and responses
**So that** I can debug agent decision-making ("vibe coding")

**Acceptance Criteria:**
- LLM prompts and responses saved to `.grava/logs/artifacts/<ExecutionID>_prompt.txt`
- Main log entry references the artifact file
- Keeps main logs clean while preserving full context

### 4.7 Debug Logs Command
**As a** user
**I want to** easily view and export logs
**So that** I can troubleshoot issues or report bugs

**Acceptance Criteria:**
- `grava debug logs` prints path to log file
- `grava debug logs --last` prints logs from last execution
- `grava debug logs --export` zips `.grava/logs` into `grava_debug_bundle.zip`

### 4.8 Ephemeral Store (The "Wisp" Layer)
**As an** AI agent
**I want to** store short-lived state like heartbeats and transient claims
**So that** I don't create thousands of junk commits in the main Dolt database

**Acceptance Criteria:**
- `internal/storage/ephemeral.go` creates a sidecar SQLite database at `.grava/ephemeral.sqlite3`
- TTL cleanup: Rows older than 24 hours are deleted on startup
- Stores `agent_heartbeats` (agent_id, last_seen, active_task_id)
- Stores `transient_meta` for high-frequency state updates
- Provides a clean separation between "Permanent Record" (Dolt) and "Execution Pulse" (SQLite)

---

### 4.9 Per-Issue Event Audit Trail in `grava show`
**As an** AI agent debugging a bug
**I want to** see the full history of what happened to an issue — who changed it, when, and what changed —
**So that** I can immediately know what was already tried, when a previous agent died, and what state change triggered the bug

**Motivation (Beads-Inspired):**
Beads' `bd show <id>` surfaces an `events` log for every issue. This means any agent arriving at a task for the first time can see "agent-2 claimed this 3h ago, commented it crashed at dag.go:312, then its heartbeat expired." Grava already has an `events` table (confirmed in `scripts/schema/001_initial_schema.sql`) and already writes to it (confirmed in `audit_integration_test.go`). The missing piece is surfacing this data in the `show` command.

#### Implementation Plan

**Step 1: Add `--events` and `--files` flags to `pkg/cmd/show.go`**

The current `showCmd` in `show.go` has no flags. Add two boolean flags in `init()`:

```go
var showEvents bool
var showFiles  bool

func init() {
    rootCmd.AddCommand(showCmd)
    showCmd.Flags().BoolVar(&showEvents, "events", false, "Show full audit trail for this issue")
    showCmd.Flags().BoolVar(&showFiles,  "files",  false, "Show affected files for this issue")
}
```

**Step 2: Query the `events` table when `--events` is passed**

After the existing issue query in `showCmd.RunE`, add a conditional block:

```go
if showEvents {
    eventsQuery := `
        SELECT event_type, actor, old_value, new_value, timestamp
        FROM events
        WHERE issue_id = ?
        ORDER BY timestamp ASC
    `
    rows, err := Store.Query(eventsQuery, id)
    if err != nil {
        return fmt.Errorf("failed to fetch events for %s: %w", id, err)
    }
    defer rows.Close()
    // print each row with formatEvent()
}
```

**Step 3: Human-readable event formatting (`formatEvent` helper)**

Each `event_type` maps to a readable symbol. Add `formatEvent()` to `util.go`:

| event_type | Icon | Display format |
|---|---|---|
| `create` | ✨ | `created by <actor>` |
| `status_changed` | 🔄 | `status: <old_value> → <new_value> by <actor>` |
| `priority_changed` | ⚡ | `priority: <old_value> → <new_value> by <actor>` |
| `dependency_add` | 🔗 | `dep added: <new_value> by <actor>` |
| `comment` | 💬 | `<actor>: "<new_value>"` |
| `claim` | 🤖 | `claimed by <actor>` |
| `claim_expired` | ⏰ | `claim released (heartbeat timeout)` |

**Step 4: `--files` flag behaviour**

The `affected_files` JSON column is already parsed in `show.go` (lines 64–67). Currently it silently prints if non-empty. The explicit `--files` flag changes behaviour:

- **Without `--files`:** Suppress files section entirely (compact default output).
- **With `--files`:** Always print the files section. If empty, print `No affected files recorded.`

This gives agents a reliable way to ask "which files did this task touch?" for impact analysis.

**Step 5: Extend `IssueDetail` struct for `--json` output**

Add an `Events` field so `grava show <id> --json --events` returns structured data:

```go
type EventEntry struct {
    Type      string    `json:"type"`
    Actor     string    `json:"actor"`
    OldValue  string    `json:"old_value,omitempty"`
    NewValue  string    `json:"new_value,omitempty"`
    Timestamp time.Time `json:"timestamp"`
}

type IssueDetail struct {
    // ... existing fields ...
    Events []EventEntry `json:"events,omitempty"`
}
```

**Example Output:**
```
$ grava show grava-a1b2 --events --files

ID:          grava-a1b2
Title:       Fix nil pointer in ReadyEngine
Type:        bug | Priority: critical (0) | Status: in_progress
Created:     2026-02-24T09:00:00Z by agent-1
Updated:     2026-02-24T12:20:00Z by agent-3

Affected Files:
  pkg/graph/ready_engine.go
  pkg/graph/dag.go

Audit Trail:
  09:00  ✨  created by agent-1
  09:05  🔗  dep added: grava-a1b2 → grava-c3d4 (blocks) by agent-1
  11:30  🤖  claimed by agent-2
  11:45  🔄  status: open → in_progress by agent-2
  12:10  💬  agent-2: "Tried approach A. Nil panic at dag.go:312."
  12:15  ⏰  claim released (heartbeat timeout after 45min)
  12:20  🤖  claimed by agent-3
```

**Acceptance Criteria:**
- `grava show <id> --events` prints the full event timeline from the `events` table, sorted ascending by timestamp
- `grava show <id> --files` explicitly shows the `affected_files` list, printing "No affected files recorded" if empty
- `grava show <id> --json --events` includes `"events": [...]` in the JSON payload
- `formatEvent()` helper covers all current `event_type` values in the schema
- Zero new database tables — uses the existing `events` table

---

### 4.10 Step Journal with Dolt Correlation (Bug Replay Trail)
**As an** AI agent that needs to reproduce a crash
**I want to** replay the exact sequence of CLI steps and see the database state diff for each step
**So that** I can identify which operation caused the bug without guessing

**Motivation (Industry Research):**
Undo.io and Replay.io are pioneering "time-travel debugging" for AI agents — recording every instruction and replaying it to pinpoint failures. Beads uses Dolt's version-controlled SQL commits as its time-travel primitive: every `bd update` is a Dolt commit, and `dolt diff HEAD~5 HEAD` shows cell-level changes. Grava can implement a practical version by combining a **Step Journal** (ordered log of CLI invocations with before/after state diffs) with **Dolt commit tagging** (embedding `ExecutionID` in commit messages). Together, `grava debug replay <ExecutionID>` gives an agent a complete, reproducible record of a failed session.

#### Implementation Plan

**Step 1: Step Journal File Format (`internal/debug/journal.go`)**

Each execution session writes to `.grava/logs/replay/<ExecutionID>.jsonl`. Each line is a `StepEntry`:

```go
type StepEntry struct {
    Step        int                    `json:"step"`
    ExecutionID string                 `json:"exec_id"`
    Timestamp   time.Time              `json:"ts"`
    Command     string                 `json:"cmd"`
    Args        []string               `json:"args"`
    Flags       map[string]string      `json:"flags"`
    Before      map[string]interface{} `json:"before,omitempty"`
    After       map[string]interface{} `json:"after,omitempty"`
    DoltCommit  string                 `json:"dolt_commit,omitempty"`
    Result      string                 `json:"result"` // "ok" | "error" | "panic"
    ErrorMsg    string                 `json:"error,omitempty"`
    StackTrace  string                 `json:"stack,omitempty"`
    DurationMs  int64                  `json:"dur_ms"`
}

type Journal struct {
    execID  string
    step    int
    file    *os.File
    current *StepEntry
}
```

**Step 2: When To Write a Step (3-Level Rule)**

| Level | Trigger | Content Written | Gate |
|---|---|---|---|
| **1 — Mutation** | `create`, `update`, `dep`, `comment`, `close`, `claim` | Before + After state diff, Dolt commit hash | Always |
| **2 — Error/Panic** | Any command returning error or triggering panic | Stack trace + graph state snapshot | Always |
| **3 — Engine Trace** | `ReadyEngine.ComputeReady` skip/accept/boost decisions | Per-node skip reason, priority boost source | `--debug` flag only |

**Step 3: Hook Points in Existing Code**

All necessary hook points already exist in Grava:

```
root.go  PersistentPreRunE  → journal.BeginStep(cmd, args, captureBeforeState())
root.go  PersistentPostRunE → journal.FinalizeStep("ok", captureAfterState(), doltCommit)
main.go  defer/recover      → journal.FinalizeStep("panic", nil, panicInfo); journal.Flush()
store.go DOLT_COMMIT call   → embed [exec:<ID> step:<N>] in commit message
ready_engine.go             → journal.TraceReady(skips, accepts, boosts) if debug mode
```

**Step 4: Dolt Commit Tagging**

Extend every mutation's commit message in `store.go` to embed the
`ExecutionID` and step number:

```sql
-- Before:
CALL DOLT_COMMIT('-m', 'grava: update grava-a1b2 status=closed');

-- After:
CALL DOLT_COMMIT('-m', 'grava: update grava-a1b2 status=closed [exec:uuid-1234 step:7]');
```

This makes Dolt history directly queryable by session:
```sql
SELECT commit_hash, message FROM dolt_log WHERE message LIKE '%exec:uuid-1234%';
```

**Step 5: `grava debug replay <ExecutionID>` Command**

New subcommand under `grava debug`. Reads `.grava/logs/replay/<ExecutionID>.jsonl` and prints an annotated timeline:

```
$ grava debug replay exec-uuid-1234

Replaying session exec-uuid-1234  (34 steps, 2026-02-24 12:00–12:15)
───────────────────────────────────────────────────────────────────────
▶  1  [2ms]    grava create --title "Fix auth bug" --type bug   → OK
               After:  { id: "grava-a1b2", status: "open" }
               Dolt:   abc12345

▶  2  [5ms]    grava dep add grava-a1b2 grava-c3d4 blocks       → OK
               After:  { edges: 12 → 13 }
               Dolt:   def45678
...
❌ 34  [0ms]   grava ready                                       → PANIC
               Error:  nil pointer dereference in dag.go:312
               Stack:  GetNode → ComputeReady → cmd/ready.go:45
               State:  { nodes: 47, edges: 83, cache_valid: false }

───────────────────────────────────────────────────────────────────────
Bug window: Steps 2–34 (12 mutations between first dep add and panic).
Next step:  grava debug diff exec-uuid-1234
```

**Step 6: `grava debug diff <ExecutionID>` Command**

Queries Dolt commit history for the session's tagged commits and runs a SQL diff:

```go
// 1. Find first and last commits tagged with exec ID
firstCommit, lastCommit := findSessionCommits(execID)

// 2. Query Dolt diff view
query := `
    SELECT from_status, to_status, from_priority, to_priority, id
    FROM dolt_diff_issues
    WHERE from_commit = ? AND to_commit = ?
`
// 3. Print field-level diff per issue row
```

Output:
```
$ grava debug diff exec-uuid-1234

Dolt diff for session exec-uuid-1234 (commits abc12345..xyz99999)
─────────────────────────────────────────────────────────────────
Issue grava-a1b2 (Fix nil pointer):
  status:   open → in_progress
  assignee: "" → "agent-2"

Issue grava-c3d4 (Implement caching):
  (no changes — this issue was only read, never written)
```

**Step 7: Pruning Old Journals**

On startup, after log rotation (Story 4.2), prune step journals older than 30 days:
```go
// internal/debug/journal.go
func PruneOldJournals(replayDir string, maxAge time.Duration) error {
    // Walk .grava/logs/replay/, delete files older than maxAge
}
```

**Acceptance Criteria:**
- Every mutation command writes a `StepEntry` with `Before`/`After` diff to the session's `.jsonl` file
- Every panic writes a `StepEntry` with `result:"panic"`, full stack trace, and graph state snapshot
- Dolt commit messages include `[exec:<id> step:<n>]` tag for every mutation
- `grava debug replay <ExecutionID>` prints the numbered timeline with per-step diffs
- `grava debug diff <ExecutionID>` shows field-level Dolt SQL diff for all session mutations
- Level-3 `ReadyEngine` trace entries are only written when `--debug` is active
- Journal files older than 30 days are pruned on startup
