# Epic 10: Advanced Merge Driver — Schema-Aware 3-Way Merge

**Status:** Planned
**Matrix Score:** 4.25
**FRs covered:** FR22

## Goal

The system automatically executes a schema-aware 3-way cell-level merge of issue state during Git updates. When merge conflicts cannot be resolved automatically (delete-vs-modify, field-level ambiguity), the unresolvable conflict data is safely isolated to a `conflict_records` table for human intervention, and the Notifier alerts the developer. `grava-merge` is registered as a custom Git merge driver via `.gitattributes`.

## ⚠️ Mandatory Spike Story

**E10-Story-1 is a spike.** The 3-way merge driver involves non-trivial Git merge driver integration, Dolt internals, and LWW resolution policy. Spike must complete and pass a proof-of-concept test before any remaining E10 stories are sprint-planned.

**Spike exit criteria:** `grava-merge %O %A %B` executes successfully on a synthetic `issues.jsonl` conflict and produces deterministic output (either merged file or conflict record entry).

## Commands / Behaviors Delivered

| Command / Behavior | FR | Description |
|--------------------|----|-------------|
| `grava-merge %O %A %B` | FR22 | Git merge driver binary — invoked by Git on `issues.jsonl` conflicts |
| `.gitattributes` registration | FR22 | `grava init` writes `issues.jsonl merge=grava-merge` to `.gitattributes` |
| LWW resolution | FR22 | Last-Write-Wins per field using `updated_at` from Dolt `NOW()` |
| `conflict_records` table | FR22 | Stores unresolvable conflicts: `{id, base_version, our_version, their_version, resolved_status}` |
| Notifier alert | FR22 | Emits structured alert when conflict is written to `conflict_records` |

## Merge Driver Architecture (ADR-H1, Beads-Inspired)

### 3-Way Parse
- Git passes three file paths: `%O` (ancestor/base), `%A` (ours), `%B` (theirs)
- Driver reads all three, parses JSONL keyed by `issue_id`
- Per-issue: compute field-level diff (ancestor → ours, ancestor → theirs)

### Resolution Policies
- **LWW (Last-Write-Wins):** Compare `updated_at` timestamps (Dolt `NOW()` — never client clock). Higher `updated_at` wins.
- **delete-wins:** If one side deletes an issue and the other modifies it → deletion wins; modification is recorded in `conflict_records` for human review
- **conflict:** Field changed differently on both sides with equal `updated_at` → write to `conflict_records`, preserve file as-is (no corruption)

### `conflict_records` Table Schema

```sql
CREATE TABLE conflict_records (
  id           VARCHAR(12) PRIMARY KEY,
  issue_id     VARCHAR(12) NOT NULL,
  field_name   VARCHAR(255),
  base_version TEXT,
  our_version  TEXT,
  their_version TEXT,
  resolved_status ENUM('pending', 'resolved', 'dismissed') DEFAULT 'pending',
  detected_at  DATETIME NOT NULL DEFAULT (NOW()),
  resolved_at  DATETIME
);
```

### Notifier Alert
- On any write to `conflict_records`: `notifier.Alert("MERGE_CONFLICT", {issue_id, field_name, detected_at})`
- Console output: `{"code": "MERGE_CONFLICT", "message": "Unresolvable conflict on issue <id> field <field>. Review with: grava conflicts list"}`

## Dependencies

- Epic 1 complete (GravaError, Notifier interface, JSON Error Envelope)
- Epic 7 complete (export/import pipeline — hook infrastructure; `issues.jsonl` format established)
- Epic 11 (Sandbox Validation) gates on Epic 10 for TS-07 (conflict detection scenario)

## Parallel Track

- Can proceed in parallel with Epic 8 and Epic 9 once Epic 7 is complete
- E10-Story-1 (spike) must pass before remaining E10 stories are sprint-planned

## NFR Ownership

| NFR | Role |
|-----|------|
| NFR4 (zero-loss handoff) | *Extended* — conflict_records ensures no data is silently discarded during merge |

## Key Architecture References

- ADR-H1: Dolt NOW() for timestamps — LWW relies on server-side time, never client clock
- ADR-H2: Hook idempotency (`.gitattributes` registration)
- FR22: 3-way schema-aware merge driver requirement
- Edge Case Resolution Strategy: `delete-wins` and `conflict` policies

## Stories

### Story 10.1: ⚠️ SPIKE — Validate `grava-merge` Git Driver Invocation

As a developer,
I want to validate that a custom Git merge driver can access Dolt SQL state during invocation,
So that we have proof-of-concept evidence before committing the remaining merge driver stories to sprint.

**Acceptance Criteria:**

**Given** a test Git repository with `issues.jsonl` tracked and `*.jsonl merge=grava-merge` in `.gitattributes`
**When** a synthetic conflict on `issues.jsonl` is created between two branches and `git merge` is run
**Then** Git invokes `grava-merge %O %A %B` with the three file paths (ancestor, ours, theirs)
**And** `grava-merge` can successfully open and read all three JSONL files
**And** `grava-merge` can execute a Dolt SQL query (`SELECT NOW()`) to confirm DB connectivity during merge hook lifecycle
**And** the spike produces a deterministic output: either a merged `issues.jsonl` written to `%A` (success) or a structured exit code indicating unresolvable conflict
**And** a spike report is written to `.grava/spike-reports/merge-driver-poc.md` documenting: invocation confirmed (Y/N), DB accessible during merge (Y/N), blockers (if any)
**And** the spike registers a runnable CI scenario: `grava sandbox run --scenario=spike-merge-driver` exits 0 — this is the **hard gate**, not the markdown report alone
**And** ⛔ **if spike fails** (either the sandbox scenario exits non-zero OR DB connectivity is unconfirmed): remaining Epic 10 stories are blocked — scope renegotiation required before sprint planning proceeds

---

### Story 10.2: Register `grava-merge` Driver and Parse 3-Way Input

As a developer,
I want `grava init` to register the `grava-merge` driver via `.gitattributes`,
So that Git automatically invokes the driver on `issues.jsonl` conflicts without manual configuration.

**Acceptance Criteria:**

**Given** `grava init` is run in a repository (spike 10.1 passed)
**When** initialization completes
**Then** `.gitattributes` contains `issues.jsonl merge=grava-merge` (added idempotently — no duplicate lines on re-init)
**And** `.git/config` (or global Git config) contains `[merge "grava-merge"] name=Grava Schema-Aware Merge Driver` and `driver=grava merge-driver %O %A %B`
**And** `grava-merge %O %A %B` successfully parses all three JSONL files keyed by `issue_id`, producing an in-memory map of `{issue_id: {base, ours, theirs}}`
**And** for issues that exist only in one side (clean add/delete with no conflict): they are merged into the output without entering conflict resolution
**And** `grava merge-driver --dry-run %O %A %B` outputs the merge plan without writing to `%A`

---

### Story 10.3: LWW Resolution and Conflict Isolation

As an agent,
I want the merge driver to automatically resolve field-level conflicts using last-write-wins,
So that the majority of concurrent edits are merged without human intervention.

**Acceptance Criteria:**

**Given** `grava-merge` has parsed ancestor, ours, and theirs versions for issue `abc123def456`
**When** a field was changed on both sides (e.g., `status` changed to `in_progress` on ours, `paused` on theirs)
**Then** the field with the higher `updated_at` timestamp wins (LWW); `updated_at` is read from Dolt `NOW()` — never client clock (ADR-H1)
**And** `delete-wins` policy: if one side deletes the issue and the other modifies it → deletion wins; the modification is recorded in `conflict_records` as `resolved_status=pending` for human review
**And** equal `updated_at` on conflicting fields → written to `conflict_records` as `resolved_status=pending`; the `%A` output preserves the field as-is (no corruption, no data loss)
**And** after resolution, the merged `issues.jsonl` is written back to `%A` with exit code 0 (Git proceeds with merge)
**And** if any records were written to `conflict_records`, the Notifier emits: `{"code": "MERGE_CONFLICT", "message": "Unresolvable conflict on issue <id>. Review with: grava conflicts list"}`

---

### Story 10.4: View and Resolve Conflicts

As a developer,
I want to view and dismiss unresolvable merge conflicts,
So that I can inspect what was isolated and manually resolve or accept the outcome.

**Acceptance Criteria:**

**Given** `conflict_records` contains pending conflicts from a merge
**When** I run `grava conflicts list`
**Then** all `resolved_status=pending` records are shown: `{id, issue_id, field_name, base_version, our_version, their_version, detected_at}`
**And** `grava conflicts list --json` returns a JSON array conforming to NFR5 schema
**And** `grava conflicts resolve <conflict-id> --accept=ours|theirs` sets the chosen version on the issue field and marks `resolved_status=resolved`, `resolved_at=NOW()`
**And** `grava conflicts dismiss <conflict-id>` marks `resolved_status=dismissed` without applying any change (human has reviewed and accepted the auto-merged state)
**And** `grava conflicts list` on a workspace with no pending conflicts returns `[]` with exit code 0
