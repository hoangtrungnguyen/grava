---
stepsCompleted:
  - step-01-validate-prerequisites
inputDocuments:
  - _bmad-output/planning-artifacts/prd.md
  - _bmad-output/planning-artifacts/architecture.md
  - _bmad-output/planning-artifacts/epics/epic-03-atomic-claim.md
  - _bmad-output/planning-artifacts/implementation-readiness-report-2026-04-05.md
scope: epic-3-all-stories
---

# grava — Epic 3: Atomic Work Claiming & Ephemeral State — Story Creation

## Requirements Inventory (Scoped to Epic 3)

### Functional Requirements

FR5: System Agent can execute an atomic claim on an issue, verifying it is unassigned and immediately locking it to their actor ID (`claim`).

FR18: System Agent / Human Developer can explicitly write to and read from an issue's ephemeral state (Wisp) via dedicated commands to manage working artifacts and execution history.

FR19: System Agent / Human Developer can retrieve the historical progression log of an issue to understand what a previous agent did before handoff.

### Non-Functional Requirements

NFR2 (Write Throughput): `create`, `update`, `claim` must commit in <15ms per operation; >70 inserts/second sustained.

NFR3 (Atomic Execution): Concurrent `grava claim` by multiple agents results in exactly one success and one deterministic rejection — no polluted rows, no deadlock.

NFR5 (Machine Readability): Strict JSON schema adherence for all `--json` outputs; schema changes trigger major version bump.

NFR6 (Zero-Dependency Footprint): Single statically-linked binary; zero external runtime dependencies beyond system shell and Git.

### Additional Requirements

From Architecture:
- `grava claim` uses `SELECT FOR UPDATE` + single transaction: verify `assignee IS NULL`, then `UPDATE assignee = actor_id`
- Concurrent claim race: losing agent receives structured error `{"code": "CLAIM_CONFLICT", "message": "Issue already claimed by agent-X"}`
- Wisp storage: ephemeral key-value store scoped to issue; survives agent crash (stored in DB, not memory)
- Wisp heartbeat: `in_progress` issues track last Wisp write timestamp; used by `grava doctor` for stale agent detection
- Issue history log: ordered ledger of all state transitions with actor, timestamp, and command reference
- ADR-FM5: Wisp lifecycle — next agent reads Wisp to resume from last checkpoint
- ADR-FM4: Claim lock lifecycle — row lock held only for duration of claim transaction
- ADR-003: `claim`, `import`, `ready` logic in named functions with `context.Context`
- Transaction pattern: `BeginTx` → `defer Rollback` → mutations → audit log → `Commit`
- All error paths return `{"error": {"code": "...", "message": "..."}}`

### FR Coverage Map

| FR | Story | Description |
|----|-------|-------------|
| FR5 | Story 3.1 | Atomic claim — verify unassigned, lock to actor ID |
| FR18 | Story 3.2 | Wisp write/read ephemeral state |
| FR19 | Story 3.3 | Issue progression history |

## Epic List

Epic 3: Atomic Work Claiming & Ephemeral State — 3 stories (3.1, 3.2, 3.3)

<!-- Stories will be populated in next step -->
