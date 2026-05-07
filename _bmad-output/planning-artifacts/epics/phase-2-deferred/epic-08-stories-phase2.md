# Epic 8 — Phase 2 Deferred Stories

**Deferred from:** [epic-08-file-reservation.md](../epic-08-file-reservation.md)
**Deferral decision:** FR-ECS-1c and FR-ECS-1d are low-value overhead for single-developer Phase 1 adoption. Document adoption evidence at sprint planning time before promoting to active sprint.
**Promotion trigger:** Phase 1 has ≥2 concurrent agents regularly operating on the same workspace.

---

### Story 8.3: TTL Auto-Expiry — Stale Lease Cleanup (FR-ECS-1c)

As a developer,
I want stale file leases to be automatically released when their TTL expires,
So that a crashed agent does not permanently block other agents from committing to reserved paths.

**Acceptance Criteria:**

**Given** agent `agent-01` crashed and its reservation on `src/cmd/issues/*.go` has `expires_ts` in the past
**When** `grava doctor` (check #12) or `grava doctor --fix` runs
**Then** check #12 identifies all reservations where `expires_ts < NOW()` and `released_ts IS NULL`
**And** `grava doctor --fix` executes TTL auto-release: sets `released_ts=NOW()` for each expired reservation
**And** the issue previously claimed by `agent-01` remains `in_progress` — preserved for next agent resume via Wisp (ADR-FM5)
**And** `grava doctor --fix` emits: `{"check": "expired_file_reservations", "status": "fixed", "action_taken": "released 2 expired leases", "snapshot_path": "..."}`
**And** `grava doctor --dry-run` shows expired leases that would be released without executing

---

### Story 8.4: Git Artifact Audit Trail (FR-ECS-1d)

As a developer,
I want active lease state written to the repository as a Git-tracked artifact,
So that there is a human-auditable record of which files were reserved by which agents at any point in time.

**Acceptance Criteria:**

**Given** an active file reservation exists for `agent-01` on `src/cmd/issues/*.go`
**When** `grava reserve` creates or releases the reservation
**Then** the lease state is written to `.grava/file_reservations/<sha1(path_pattern)>.json` with content `{id, agent_id, path_pattern, exclusive, created_ts, expires_ts, released_ts}`
**And** the file is committed to Git as part of the reservation operation (included in the next `grava export` cycle)
**And** `grava reserve --list` reads from the DB (authoritative) — the Git artifact is audit trail only, not the source of truth
**And** on reservation release, the corresponding `.json` file is updated with `released_ts` set
**And** `.grava/file_reservations/` is tracked in Git (not excluded via `.git/info/exclude`)
