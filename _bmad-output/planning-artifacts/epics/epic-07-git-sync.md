# Epic 7: Git Sync — Export, Import & Hook Pipeline

**Status:** Planned
**Grava ID:** grava-089b
**Matrix Score:** 4.20
**FRs covered:** FR20, FR21, FR24, FR25
*(FR22 — 3-way merge driver — moved to Epic 10: Advanced Merge Driver)*

## Goal

The system automatically synchronizes issue state with Git operations — exporting to `issues.jsonl`, importing with Dual-Safety Check (all-or-nothing transaction), and triggering sync automatically via Git hook stubs on `git pull`/merge. The merge driver implementation is a dedicated epic (Epic 10) to give it appropriate risk isolation.

## Commands / Behaviors Delivered

| Command / Behavior | FR | Description |
|--------------------|----|-------------|
| `grava export` | FR20 | Export DB state to `issues.jsonl` |
| `grava import <file>` | FR21 | Hydrate DB from `issues.jsonl` (with Dual-Safety Check) |
| `grava hook post-merge` | FR25 | Triggered by Git post-merge hook; runs import pipeline |
| `grava hook pre-commit` | FR25 | Stub only in E7; enforcement logic added in E8 (file reservation) |
| Dual-Safety Check | FR24 | JSONL hash vs Dolt state before any import |

## Import Pipeline

- All-or-nothing transaction: full `issues.jsonl` import wrapped in single DB transaction
- On mid-import crash/connection loss: full rollback with message: `"Import rolled back — database connection lost. Your data is unchanged. Safe to retry."`
- No partial state possible (CM-1)

## Dual-Safety Check (FR24)

Before any import:
1. Compute JSONL hash
2. Compare against current Dolt commit state
3. Abort if uncommitted local changes would be overwritten
4. Error: `{"code": "IMPORT_CONFLICT", "message": "Uncommitted local changes detected. Commit or export first."}`

## Git Hook Registration (FR25, ADR-H2)

- `grava init` writes one-liner shell stubs to `.git/hooks/post-merge` and `.git/hooks/pre-commit`
- Hook registration is **idempotent**: never overwrites existing hooks silently
- If hook already exists: append `grava hook <event>` call to existing hook file (with comment marker)

## NFR Ownership

| NFR | Role |
|-----|------|
| NFR4 (zero-loss handoff) | *Owned* — export/import must preserve 100% of dependency links and core fields |

## Dependencies

- Epic 1 complete (GravaError, JSON Error Envelope, hook subcommand infrastructure)
- Epic 3 complete (Wisp state must be included in export for full handoff)

## Parallel Track

- E6 (Onboarding) can proceed in parallel with E7 — no story-level dependencies shared
- E8 (File Reservation) depends on E7 for pre-commit hook stub existence
- E10 (Advanced Merge Driver) depends on E7 for hook infrastructure

## Key Architecture References

- ADR-H1: Dolt NOW() for timestamps
- ADR-H2: Hook idempotency
- ADR-001: Git hook binary
- CM-1: Import rollback

## Stories

### Story 7.1: Export DB State to JSONL *(grava-7ce0)*

As a developer or agent,
I want to export the full Grava database state to a portable JSONL file,
So that the current issue tracking state can be committed to Git and shared across clones.

**Acceptance Criteria:**

**Given** a Grava workspace with issues, dependencies, labels, comments, and Wisp entries
**When** I run `grava export`
**Then** a file `issues.jsonl` is written to the `.grava/` directory with one JSON object per line, covering all non-archived issues with their full field set (id, title, status, priority, assignee, labels, comments, dependencies, wisp entries)
**And** `grava export --output <path>` writes the file to the specified path instead
**And** `grava export --json` returns `{"exported_path": ".grava/issues.jsonl", "issue_count": 42, "exported_at": "..."}`
**And** 100% of dependency links and core fields are preserved in the exported file (NFR4 baseline)
**And** the export operation completes without requiring a DB lock — it is read-only

---

### Story 7.2: Import with Dual-Safety Check (All-or-Nothing) *(grava-2d87)*

As a developer or agent,
I want to import a Grava JSONL export into the local database with a safety check against data loss,
So that workspace state is fully restored after a `git pull` or clone without risk of overwriting uncommitted local changes.

**Acceptance Criteria:**

**Given** a valid `issues.jsonl` file exists
**When** I run `grava import issues.jsonl`
**Then** a Dual-Safety Check executes first: compute JSONL hash and compare against current Dolt commit state
**And** if uncommitted local changes would be overwritten, the import is aborted with `{"error": {"code": "IMPORT_CONFLICT", "message": "Uncommitted local changes detected. Commit or export first."}}`
**And** if the safety check passes, the full import executes in a single DB transaction (all-or-nothing)
**And** if the import transaction is interrupted mid-execution (connection loss, crash), a full rollback occurs with message: `"Import rolled back — database connection lost. Your data is unchanged. Safe to retry."`
**And** a successful import returns `{"imported": 42, "updated": 5, "skipped": 0}` with exit code 0
**And** `grava import` on a non-existent file returns `{"error": {"code": "FILE_NOT_FOUND", ...}}`

---

### Story 7.3: Register Git Hook Stubs (Idempotent) *(grava-5827)*

As a developer,
I want `grava init` to register Git hook stubs for `post-merge` and `pre-commit`,
So that the sync pipeline runs automatically on every `git pull`/merge without manual setup.

**Acceptance Criteria:**

**Given** `grava init` is run in a Git repository
**When** `.git/hooks/post-merge` and `.git/hooks/pre-commit` do not yet exist
**Then** `grava init` writes one-liner shell stubs: `#!/bin/sh\ngrava hook post-merge` and `#!/bin/sh\ngrava hook pre-commit` respectively, with executable bit set
**And** if a hook file already exists, `grava init` appends `# grava-hook-start\ngrava hook <event>\n# grava-hook-end` after the existing content — never overwrites silently
**And** re-running `grava init` (idempotency): if the `# grava-hook-start` marker is already present, no duplicate append occurs
**And** `grava init` reports: `{"hooks_registered": ["post-merge", "pre-commit"], "hooks_appended": [], "hooks_skipped": []}` distinguishing new registrations from appends

---

### Story 7.4: Automatic Sync on `git pull` via Post-Merge Hook *(grava-1259)*

As a developer or agent,
I want the `issues.jsonl` to be automatically imported after every `git pull`/merge,
So that the local Grava database stays synchronized with the repository state without manual intervention.

**Acceptance Criteria:**

**Given** the `post-merge` Git hook stub is registered
**When** `git pull` or `git merge` completes and triggers the post-merge hook
**Then** `grava hook post-merge` executes automatically and detects whether `issues.jsonl` changed in the merge
**And** if `issues.jsonl` changed: the Dual-Safety Check runs; if it passes, the import pipeline executes; the hook exits 0 on success
**And** if `issues.jsonl` did not change: the hook is a no-op; exits 0 immediately
**And** if the import fails (conflict or crash): the hook exits non-zero and prints the structured error — the Git merge is NOT rolled back (hook failure is advisory, not blocking)
**And** `grava hook post-merge --dry-run` shows what would be imported without executing
