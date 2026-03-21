# Epic 3: Atomic Work Claiming & Ephemeral State

**Status:** Planned
**Matrix Score:** 4.60 *(highest score — highest confidence)*
**FRs covered:** FR5, FR18, FR19

## Goal

Agents can atomically claim an issue (guaranteeing exactly one claimant under concurrent conditions), track their working state through the Wisp system for crash-safe handoffs, and retrieve the full progression log of any issue before picking it up.

## Commands Delivered

| Command | FR | Description |
|---------|----|-------------|
| `grava claim <id>` | FR5 | Atomic claim — verify unassigned, lock to actor ID |
| `grava wisp write <id> <key> <value>` | FR18 | Write to issue's ephemeral state |
| `grava wisp read <id> [key]` | FR18 | Read from issue's ephemeral state |
| `grava history <id>` | FR19 | Retrieve full progression log of an issue |

## NFR Ownership

| NFR | Role |
|-----|------|
| NFR3 (atomic execution) | *Owned* — concurrent `grava claim` by two agents → exactly one success, one deterministic rejection |

## Key Implementation Notes

- `grava claim` uses `SELECT FOR UPDATE` + single transaction: verify `assignee IS NULL`, then `UPDATE assignee = actor_id`
- Concurrent claim race: losing agent receives structured error `{"code": "CLAIM_CONFLICT", "message": "Issue already claimed by agent-X"}`
- Wisp storage: ephemeral key-value store scoped to issue; survives agent crash (stored in DB, not memory)
- Wisp heartbeat: `in_progress` issues track last Wisp write timestamp; used by `grava doctor` for stale agent detection
- Issue history log: ordered ledger of all state transitions with actor, timestamp, and command reference
- ADR-FM5: Wisp lifecycle — next agent reads Wisp to resume from last checkpoint

## Dependencies

- Epic 1 Story 0a complete (WithAuditedTx required for claim atomicity)
- Epic 1 Story 0b complete (WithDeadlockRetry required for concurrent claim retry)

## Parallel Track

- Can begin after Epic 1 Story 0a is merged
- Can proceed in parallel with Epic 2
- Epic 9 (Worktree Orchestration) extends this epic — never replaces it

## Stories

### Story 3.1: Atomic Issue Claim

As an agent,
I want to atomically claim an unassigned issue under concurrent conditions,
So that exactly one agent owns the issue and no two agents work on the same task simultaneously.

**Acceptance Criteria:**

**Given** issue `abc123def456` exists with `assignee=NULL` and `status=open`
**When** I run `grava claim abc123def456 --actor agent-01`
**Then** a single DB transaction issues `SELECT FOR UPDATE` on the issues row, verifies `assignee IS NULL`, then sets `assignee=agent-01` and `status=in_progress`
**And** `grava claim --json` returns `{"id": "abc123def456", "status": "in_progress", "assignee": "agent-01"}`
**And** a second concurrent `grava claim abc123def456 --actor agent-02` executed simultaneously returns `{"error": {"code": "CLAIM_CONFLICT", "message": "Issue already claimed by agent-01"}}` — no polluted row, no deadlock
**And** `grava claim` on an already-claimed issue (non-concurrent) returns the same `CLAIM_CONFLICT` error immediately
**And** the claim operation completes in <15ms (NFR2)

---

### Story 3.2: Write and Read Wisp Ephemeral State

As an agent,
I want to write and read key-value pairs to an issue's Wisp (ephemeral state store),
So that my working artifacts and execution checkpoints survive a crash and can be resumed by the next agent.

**Acceptance Criteria:**

**Given** issue `abc123def456` is claimed by `agent-01`
**When** I run `grava wisp write abc123def456 checkpoint "step-3-complete"`
**Then** the key-value pair is stored in the `wisp_entries` table with `issue_id`, `key`, `value`, `written_at=NOW()`, `written_by=agent-01`
**And** `grava wisp read abc123def456 checkpoint` returns `{"key": "checkpoint", "value": "step-3-complete", "written_at": "..."}`
**And** `grava wisp read abc123def456` (no key) returns all Wisp entries for the issue as a JSON array
**And** Wisp entries persist across process restarts — stored in DB, not in memory
**And** `grava wisp write` updates `wisp_heartbeat_at` on the issue row to `NOW()` (used by `grava doctor` stale-agent detection)
**And** writing to a non-existent issue returns `{"error": {"code": "ISSUE_NOT_FOUND", ...}}`

---

### Story 3.3: Retrieve Issue Progression History

As a developer or agent,
I want to retrieve the full ordered progression log of an issue,
So that I can understand what previous agents did before picking up the work.

**Acceptance Criteria:**

**Given** issue `abc123def456` has had multiple state transitions and Wisp writes
**When** I run `grava history abc123def456`
**Then** an ordered log is returned showing each event: `{event_type, actor, timestamp, details}` — covering status changes, claim/release, Wisp writes, comments, and label changes
**And** `grava history abc123def456 --json` returns a JSON array of events conforming to NFR5 schema
**And** `grava history abc123def456 --since "2026-03-01"` filters events to after the given date
**And** a new agent running `grava history abc123def456` before claiming can see the full prior context, enabling crash-safe handoff
**And** the history for an issue with no events returns an empty array `[]`, not an error
