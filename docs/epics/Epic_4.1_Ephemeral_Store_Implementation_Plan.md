# Epic 4.1: Ephemeral Store (Wisp Layer) Implementation Plan

**Created:** 2026-02-24
**Epic:** [Epic 4: Grava Flight Recorder & Ephemeral Store](./Epic_4_Log_Saver.md)
**Status:** Planned
**Target Package:** `internal/storage`, `pkg/graph`

---

## 1. Technical Design

The Ephemeral Store provides a low-latency, "invisible" storage layer for transient agent state. It uses SQLite specifically to avoid the transaction overhead and history bloat of Dolt for data that has no long-term value.

### 1.1 Storage Architecture
- **Primary Backend:** SQLite 3
- **File Location:** `.grava/ephemeral.sqlite3`
- **Concurrency:** WAL (Write-Ahead Logging) mode enabled for multi-agent access.

### 1.2 Schema Design

#### Table: `agent_heartbeats`
| Column | Type | Description |
|---|---|---|
| `agent_id` | TEXT (PK) | Unique ID for the agent session |
| `last_seen` | DATETIME | Last heartbeat timestamp |
| `active_task_id` | TEXT | Task ID the agent is currently working on |
| `metadata` | JSON | Extended agent status (OS, model, etc.) |

#### Table: `transient_claims`
| Column | Type | Description |
|---|---|---|
| `issue_id` | TEXT (PK) | ID of the claimed issue |
| `agent_id` | TEXT | ID of the owning agent |
| `claimed_at` | DATETIME | Timestamp of claim |
| `expires_at` | DATETIME | Hard expiration for the claim |

---

## 2. Implementation Phases

### Phase 1: SQLite Infrastructure (`internal/storage/ephemeral`)
- [ ] Initialize SQLite driver (using `modernc.org/sqlite` for CGO-free portability).
- [ ] Implement `EphemeralStore` struct with connection pooling.
- [ ] Implement auto-migration for `agent_heartbeats` and `transient_claims`.
- [ ] Enable WAL mode and busy timeouts.

### Phase 2: Claim & Heartbeat API
- [ ] `ClaimIssue(issueID, agentID, duration)`: Atomically upsert a claim.
- [ ] `ReleaseIssue(issueID, agentID)`: Remove a claim.
- [ ] `Heartbeat(agentID, taskID)`: Update agent liveness.
- [ ] `GetActiveClaims()`: Retrieve all current valid claims.

### Phase 3: TTL & Maintenance
- [ ] `CleanupExpired()`: Delete rows from `transient_claims` where `now > expires_at`.
- [ ] `PurgeStaleAgents()`: Delete agents who haven't sent a heartbeat in > 24 hours.
- [ ] Hook cleanup into `grava` startup (Post-Init).

### Phase 4: Graph Integration (`pkg/graph/ready_engine.go`)
- [ ] Update `ReadyEngine` to accept an `EphemeralStore` interface.
- [ ] Modify `ComputeReady` to join with the active claims list.
- [ ] Filter out tasks already claimed by other active agents.

---

## 3. Verification Plan

### Unit Tests
- `internal/storage/ephemeral_test.go`: Test atomic claims, expiry logic, and WAL concurrency.
- `pkg/graph/ready_engine_multi_test.go`: Mock ephemeral store to verify that `ComputeReady` hides claimed tasks.

### Integration Tests
- Run two `grava` instances in parallel (simulated).
- Verify instance A cannot see tasks claimed by instance B.
- Verify that killing instance A (stale heartbeat) allows instance B to work on the task after TTL.
