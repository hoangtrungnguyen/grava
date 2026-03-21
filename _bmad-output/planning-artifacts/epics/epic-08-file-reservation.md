# Epic 8: File Reservation & Concurrent Edit Safety

**Status:** Planned
**Matrix Score:** 4.05 *(raised from 3.45 after FR-ECS-1a–d split)*
**FRs covered:** FR-ECS-1a, FR-ECS-1b, FR-ECS-1c, FR-ECS-1d

## Goal

Agents can declare advisory file leases before modifying files, preventing concurrent edits to the same paths. The pre-commit Git hook enforces active leases and blocks unauthorized commits. TTL-based auto-expiry releases stale leases on agent crash. Lease artifacts are written to Git for human audit.

## Source

FR-ECS-1a–d are **derived requirements** from `_bmad-output/planning-artifacts/edge-case-resolution-strategy.md`. No direct PRD FR. Split into 4 sub-requirements explicitly to protect from sprint scope cuts and enable independent story decomposition.

## Sub-Requirements

| FR | Phase | Description |
|----|-------|-------------|
| FR-ECS-1a | Phase 1 Required | File reservation declaration — agents declare advisory leases via `grava reserve` before modifying files |
| FR-ECS-1b | Phase 1 Required | Pre-commit hook enforcement — blocks commits to paths held by another agent |
| FR-ECS-1c | Phase 1 Deferral Condition | TTL auto-expiry — stale leases released automatically on agent crash |
| FR-ECS-1d | Phase 1 Deferral Condition | Git artifact audit trail — lease state written to `file_reservations/<sha1>.json` |

## ⚠️ Phase 1 Deferral Condition

FR-ECS-1a (declaration) and FR-ECS-1b (pre-commit enforcement) are **required in Phase 1** for multi-agent safety.

**FR-ECS-1c (TTL auto-expiry) and FR-ECS-1d (Git audit trail) may be deferred to Phase 2** if Phase 1 adoption is primarily single-developer, where TTL race conditions and audit trails are low-value overhead. Document the deferral decision explicitly at sprint planning time with adoption evidence.

## Commands Delivered

| Command | FR | Description |
|---------|----|-------------|
| `grava reserve <path-pattern> [--ttl=Xm] [--exclusive]` | FR-ECS-1a | Declare advisory file lease |
| `grava reserve --release <reservation-id>` | FR-ECS-1a | Explicitly release a lease |
| `grava reserve --list` | FR-ECS-1a | Show active reservations |
| Pre-commit hook enforcement | FR-ECS-1b | Block commit to reserved paths (added to E7 hook stub) |
| TTL background cleanup | FR-ECS-1c | Auto-release expired leases (doctor --fix or background) |
| Audit artifact write | FR-ECS-1d | Write lease state to `file_reservations/<sha1(path)>.json` |

## `file_reservations` Table Schema

```sql
CREATE TABLE file_reservations (
  id          VARCHAR(12) PRIMARY KEY,
  project_id  VARCHAR(12) NOT NULL,
  agent_id    VARCHAR(255) NOT NULL,
  path_pattern VARCHAR(1024) NOT NULL,
  exclusive   BOOLEAN DEFAULT TRUE,
  reason      TEXT,
  created_ts  DATETIME NOT NULL DEFAULT (NOW()),
  expires_ts  DATETIME NOT NULL,
  released_ts DATETIME
);
```

## Conflict Resolution

- Overlapping exclusive lease request → `FILE_RESERVATION_CONFLICT` structured error
- Non-exclusive (shared) leases: multiple readers allowed, blocked by exclusive writer
- Pre-commit violation: `{"code": "FILE_RESERVATION_BLOCK", "message": "Path X is reserved by agent Y until <expires_ts>. Release or wait."}`

## Dependencies

- Epic 7 complete (Git hook registration infrastructure — pre-commit stub must exist before FR-ECS-1b enforcement logic is added)
- Epic 10 (Advanced Merge Driver) is independent — E8 does not need to wait for E10

## Key Architecture References

- ADR-FM5: Wisp lifecycle — issue stays `in_progress` after lease expiry; next agent resumes from Wisp checkpoint
- Failure Recovery Strategy: TTL-based lease auto-release; `grava doctor` extended check #12

## Stories

### Story 8.1: Declare and Release File Leases (FR-ECS-1a)

As an agent,
I want to declare an advisory lease on file paths before modifying them,
So that other agents know which files I intend to modify and can avoid concurrent edits.

**Acceptance Criteria:**

**Given** agent `agent-01` is about to modify `src/cmd/issues/*.go`
**When** I run `grava reserve src/cmd/issues/*.go --ttl=30m --exclusive`
**Then** a row is inserted in `file_reservations`: `{id, project_id, agent_id="agent-01", path_pattern="src/cmd/issues/*.go", exclusive=true, created_ts=NOW(), expires_ts=NOW()+30m}`
**And** `grava reserve --list` returns all active (non-expired, non-released) reservations: `{id, agent_id, path_pattern, exclusive, expires_ts}`
**And** `grava reserve --release <reservation-id>` sets `released_ts=NOW()` on the reservation row; subsequent `--list` excludes it
**And** `grava reserve src/cmd/issues/*.go` (non-exclusive, shared read) inserts with `exclusive=false`; multiple non-exclusive leases on the same path are allowed
**And** `grava reserve --json` conforms to NFR5 schema

---

### Story 8.2: Pre-Commit Hook Enforcement (FR-ECS-1b)

As an agent,
I want the pre-commit Git hook to block commits to paths held by another agent's exclusive lease,
So that concurrent file modifications are caught at commit time before they reach the repository.

**Acceptance Criteria:**

**Given** `agent-02` holds an exclusive reservation on `src/cmd/issues/*.go` and `agent-01` attempts to commit changes to `src/cmd/issues/create.go`
**When** `agent-01` runs `git commit`
**Then** the pre-commit hook (`grava hook pre-commit`) checks all staged file paths against active exclusive leases in `file_reservations`
**And** the commit is blocked with exit code 1 and structured output: `{"code": "FILE_RESERVATION_BLOCK", "message": "Path src/cmd/issues/create.go is reserved by agent-02 until <expires_ts>. Release or wait."}`
**And** if no staged paths overlap with active exclusive leases, the commit proceeds normally (exit code 0)
**And** a non-exclusive (shared) lease does NOT block commits from other agents
**And** an expired lease (past `expires_ts`) does NOT block commits — treated as released

---

> **Stories 8.3 (TTL Auto-Expiry) and 8.4 (Git Audit Trail) are deferred to Phase 2.**
> See [phase-2-deferred/epic-08-stories-phase2.md](phase-2-deferred/epic-08-stories-phase2.md)
