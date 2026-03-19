# Edge Case Resolution Strategy — Multi-Agent Orchestrator

> Sources: Internal architecture docs + [mcp_agent_mail](https://github.com/Dicklesworthstone/mcp_agent_mail) patterns

---

## Edge Case 1: Delete vs. Modify on Parallel Branches

**Scenario:** Agent A deletes a file/record on one branch while Agent B modifies the same file/record on a parallel branch.

### Detection
- Schema-aware merge driver (`grava merge-slot`) intercepts `.jsonl` / structured file merges
- Loads Ancestor (`%O`), Ours (`%A`), Theirs (`%B`) into memory
- Detects conflict: ID present in Ancestor + Branch B but **missing** in Branch A

### Resolution Algorithm
1. Parse all three versions into hash maps keyed by `Issue_ID`
2. Evaluate configured policy:
   - **`delete-wins`** → deletion takes precedence, auto-resolve
   - **`conflict`** → write conflicting states to conflict isolation table, return exit code 1
3. Halt automated merge on `conflict` policy
4. Fire `HumanOverseer` alert to both agents

### Data Structures
```sql
-- In-memory 3-way parse (per merge session)
-- { issue_id -> record_data } for ancestor, ours, theirs

-- Conflict isolation table
conflict_records (
  id             TEXT PRIMARY KEY,
  base_version   JSONB,
  our_version    JSONB,
  their_version  JSONB,
  resolved_status TEXT DEFAULT 'pending'
)
```

### mcp_agent_mail Integration
- `HumanOverseer` identity bypasses all agent contact policies
- Broadcasts `🚨 MESSAGE FROM HUMAN OVERSEER 🚨` to both agents
- Agents pause current work, follow manual resolution steps, then resume
- Once conflict table is cleared, merge re-runs automatically

---

## Edge Case 2: Large File Changes Across Multiple Branches

**Scenario:** Multiple agents attempt sweeping changes to the same large files or directories simultaneously.

### Detection
- Proactive: agents must declare **advisory file leases** before modifying files
- Reactive: Git pre-commit hook blocks commits to paths held by another agent

### Resolution Algorithm
1. **Declare intent:** Agent A calls `file_reservation_paths(path="src/**", exclusive=true, ttl=3600)`
2. **Grant & record:** Lease stored in DB + `.json` artifact written to Git repo
3. **Block concurrent edits:** Agent B receives `FILE_RESERVATION_CONFLICT` if it tries to reserve the same path
4. **Pre-commit enforcement:** Hook `hooks.d/pre-commit/50-agent-mail.py` scans active leases, blocks rogue commits
5. **Release:** Agent A calls `release_file_reservations` on task close; TTL expiry auto-releases on crash

### Data Structures
```sql
file_reservations (
  id           TEXT PRIMARY KEY,
  project_id   TEXT,
  agent_id     TEXT,
  path_pattern TEXT,
  exclusive    BOOLEAN,
  reason       TEXT,        -- maps to task ID, e.g. "bd-123"
  created_ts   TIMESTAMP,
  expires_ts   TIMESTAMP,   -- TTL-based auto-expiry
  released_ts  TIMESTAMP
)
```

```
-- Git artifact (human-auditable)
file_reservations/<sha1(path)>.json
```

### mcp_agent_mail Integration
- Overlapping file leases trigger **auto-allow contact** — Agent B can message Agent A directly without a handshake
- Agent B negotiates lease transfer or asks Agent A to expedite via async message thread
- Thread ID matches task ID for zero-mapping coordination

---

## Edge Case 3: Rapid Sequential Claims

**Scenario:** Agent A and Agent B attempt to claim the same task milliseconds apart.

### Detection
- Row-level `SELECT FOR UPDATE` lock during the claim transaction
- Only one agent can hold the lock; second is queued and reads updated state

### Resolution Algorithm
1. Both agents query backlog (e.g., `bv --robot-priority`)
2. Agent A issues claim → DB executes `SELECT ... FOR UPDATE` on the task row
3. Transaction updates `status = in_progress`, sets `assignee = agent_a_id`, releases lock
4. Agent B's transaction reads `in_progress` state → safely aborts with "already claimed"
5. Agent A writes **Wisps** (ephemeral activity logs) throughout execution
6. If Agent A crashes, Agent C claims the task and reads Wisps to avoid duplicating partial work
7. Wisps are deleted when task is officially closed

### Data Structures
```sql
-- Issues table (must include)
issues (
  id       TEXT PRIMARY KEY,
  status   TEXT,   -- open | in_progress | closed
  assignee TEXT    -- agent_id
)

-- Wisps: ephemeral activity log
wisps (
  id         TEXT PRIMARY KEY,
  issue_id   TEXT REFERENCES issues(id),
  agent_id   TEXT,
  log_entry  TEXT,
  created_ts TIMESTAMP
  -- deleted on task close
)
```

### mcp_agent_mail Integration
- Claiming agent broadcasts to unified thread: `send_message(thread_id="bd-123", subject="Start: <task>", ack_required=true)`
- Thread ID = Task ID — no mapping layer needed
- All agents notified via inbox, reducing redundant DB polling
- `summarize_thread` tool lets later agents catch up without loading full history

---

## Cross-Cutting Patterns (from mcp_agent_mail)

| Pattern | Description |
|---------|-------------|
| **Thread ID = Task ID** | Mail thread ID always matches task ID (e.g., `bd-123`) — zero mapping overhead |
| **Auto-Allow Contact** | Agents sharing overlapping file leases or threads can message freely without handshake |
| **Human Overseer** | Emergency bypass identity that forces all agents to pause and await instructions |
| **Dual Persistence** | Markdown files in Git (audit trail) + SQLite FTS5 (fast agent queries) |
| **Memorable Agent Names** | Human-readable "adjective+noun" IDs (e.g., `GreenCastle`) for easier log debugging |
| **Consent-Lite Messaging** | `auto` policy allows free messaging within shared context; `request_contact` handshake required otherwise |
