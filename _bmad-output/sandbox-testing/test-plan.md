---
project: grava
phase: Phase 1
created: '2026-03-21'
scope: sandbox-testing
relatedDocs:
  - _bmad-output/sandbox-testing/requirements.md
  - _bmad-output/planning-artifacts/architecture.md
  - sandbox/SANDBOX_VALIDATION_GUIDE.md
status: draft
---

# Sandbox Test Plan

**Project:** grava — Phase 1 Core Substrate Validation
**Date:** 2026-03-21

This document defines the test scenarios, acceptance criteria, and execution approach for validating Phase 1 of the Grava system in a sandbox environment.

---

## Test Execution Overview

### Prerequisites

1. Clean machine or dedicated sandbox directory (not the developer's active repo)
2. Grava binary built from current HEAD: `go build -o grava ./cmd/grava`
3. Dolt installed and available on `$PATH`
4. Available port for Dolt server (default `3306`; use `3307` for sandbox to avoid conflicts)
5. `bash` and `git` available on the host

### Setup Script (per run)

```bash
# Create sandbox repo
mkdir /tmp/grava-sandbox && cd /tmp/grava-sandbox
git init
grava init                        # Standard single-agent init
# OR for worktree multi-agent runs:
grava init --enable-worktrees
```

### Teardown Script (per run)

```bash
grava coordinator stop 2>/dev/null || true
rm -rf /tmp/grava-sandbox
```

---

## Test Scenarios

---

### TS-01: Concurrent Claim Race (SBX-FR-1)

**Requirement:** SBX-FR-1
**Priority:** P1 — Must Pass

**Setup:**
1. Create one issue: `grava create --title "Shared Issue" --json`
2. Record the issue ID

**Execution:**
```bash
# Run 10 agents simultaneously claiming the same issue
for i in $(seq 1 10); do
  grava claim <ISSUE_ID> --actor "agent-$i" --json &
done
wait
```

**Acceptance Criteria:**

**Given** 10 agents concurrently execute `grava claim` on the same issue
**When** all 10 commands complete
**Then** exactly 1 command exits with code `0` and JSON `{"status": "in_progress"}`
**And** exactly 9 commands exit with a non-zero code and a deterministic JSON error (machine-readable error code)
**And** no agent receives an ambiguous or partial response
**And** no deadlock or hang occurs (all processes complete within 5 seconds)

---

### TS-02: Ready Queue Under Swarm Load (SBX-FR-2)

**Requirement:** SBX-FR-2
**Priority:** P1 — Must Pass

**Setup:**
1. Seed 500 issues with mixed statuses and dependency chains
2. Ensure at least 50 issues are in `open` state with no blockers

**Execution:**
```bash
# 30 agents query ready simultaneously
for i in $(seq 1 30); do
  grava ready --json --actor "agent-$i" > /tmp/ready-$i.json &
done
wait
```

**Acceptance Criteria:**

**Given** 30 agents query `grava ready` simultaneously
**When** all responses are collected
**Then** each agent receives a valid JSON array (no malformed output)
**And** no issue in `in_progress` state appears in any agent's ready queue
**And** all responses complete within 2 seconds wall-clock time

---

### TS-03: Performance Baseline at 10K Issues (SBX-NFR-1)

**Requirement:** SBX-NFR-1
**Priority:** P1 — Must Pass

**Setup:**
```bash
# Seed 10,000 issues via bulk script
for i in $(seq 1 10000); do
  grava create --title "Issue $i" --priority medium
done
```

**Execution:**
```bash
time grava ready --json > /dev/null
time grava list --json > /dev/null
```

**Acceptance Criteria:**

**Given** the database contains 10,000 active and ephemeral issues
**When** `grava ready --json` is executed
**Then** the command completes and returns structured JSON in under 100ms
**And** `grava list --json` also completes in under 100ms

---

### TS-04: Write Throughput (SBX-NFR-2)

**Requirement:** SBX-NFR-2
**Priority:** P1 — Must Pass

**Execution:**
```bash
# Measure 70+ writes/second across 30 agents
START=$(date +%s%3N)
for i in $(seq 1 210); do
  grava create --title "Throughput Issue $i" --actor "agent-$((i % 30 + 1))" &
done
wait
END=$(date +%s%3N)
echo "210 writes in $((END - START))ms"
```

**Acceptance Criteria:**

**Given** 30 agents performing concurrent issue creation
**When** 210 write operations are executed
**Then** all 210 writes complete within 3 seconds (= 70 writes/sec threshold)
**And** each individual write commits in < 15ms (validate via Dolt query log)
**And** zero rows are corrupt or partial in the database

---

### TS-05: Export / Import Round-Trip Fidelity (SBX-FR-5)

**Requirement:** SBX-FR-5, NFR4
**Priority:** P1 — Must Pass

**Execution:**
```bash
# Create 100 issues with dep chains
grava create --title "Parent" --json | jq -r '.id' > /tmp/parent-id
grava subtask <parent-id> --title "Child A"
grava subtask <parent-id> --title "Child B"
grava dep --add <child-a-id> <child-b-id>

# Export
grava export --output /tmp/grava-export.jsonl

# Import to fresh repo
mkdir /tmp/grava-sandbox-2 && cd /tmp/grava-sandbox-2
git init && grava init
grava import --input /tmp/grava-export.jsonl

# Compare
grava list --json > /tmp/after-import.json
```

**Acceptance Criteria:**

**Given** a populated Grava database with dependency chains
**When** `grava export` and `grava import` are executed on a fresh repo
**Then** issue count matches exactly
**And** all dependency links are identical (`grava dep` output matches before/after)
**And** all status fields, priorities, and actor assignments are preserved
**And** `grava doctor` passes on the imported repo

---

### TS-06: Chaos — Dolt Crash Mid-Import (SBX-CHAOS-1)

**Requirement:** SBX-CHAOS-1, CM-1
**Priority:** P1 — Must Pass

**Execution:**
```bash
# Start import of 1000-issue file in background
grava import --input /tmp/large-export.jsonl &
IMPORT_PID=$!

# Kill Dolt server during import
sleep 0.2
pkill -f "dolt sql-server"

# Wait for import to exit
wait $IMPORT_PID
IMPORT_EXIT=$?
```

**Acceptance Criteria:**

**Given** Dolt is killed mid-import
**When** the CLI detects the connection loss
**Then** the CLI outputs: `"Import rolled back — database connection lost. Your data is unchanged. Safe to retry."`
**And** the process exits non-zero
**And** no partial rows exist in the database (verify via `grava list --json` after Dolt restart)
**And** re-running the import after Dolt restart succeeds cleanly

---

### TS-07: Init Idempotency (SBX-CHAOS-4 / SBX-FR-12)

**Requirement:** SBX-FR-12, CM-4
**Priority:** P2 — Should Pass

**Execution:**
```bash
grava init --enable-worktrees
grava init --enable-worktrees   # Second call — must be no-op
```

**Acceptance Criteria:**

**Given** `grava init --enable-worktrees` has already been run
**When** `grava init --enable-worktrees` is run a second time
**Then** the CLI logs `"grava already initialized, skipping"`
**And** no duplicate Git hooks are registered
**And** no second coordinator process is started (verify via `grava coordinator status`)
**And** config file content is unchanged

---

### TS-08: Worktree Lifecycle — Close with Uncommitted Changes (SBX-FR-9)

**Requirement:** SBX-FR-9, ADR-004 teardown
**Priority:** P2 — Should Pass

**Execution:**
```bash
grava init --enable-worktrees
grava create --title "Feature X" --json | jq -r '.id' > /tmp/issue-id
grava claim $(cat /tmp/issue-id) --actor agent-01

# Make an uncommitted change in the worktree
echo "dirty" >> .worktrees/agent-01/somefile.txt

# Attempt close
grava close $(cat /tmp/issue-id) --actor agent-01
```

**Acceptance Criteria:**

**Given** an agent has uncommitted changes in its worktree
**When** `grava close <id>` is executed
**Then** the command aborts with error: `"error: worktree agent-01 has uncommitted changes. Commit or stash before closing."`
**And** the issue status remains `in_progress` in the database
**And** the worktree directory still exists with all changes intact

---

### TS-09: Doctor Health Checks (SBX-FR-11)

**Requirement:** SBX-FR-11, ADR-FM7
**Priority:** P2 — Should Pass

**Execution:**
```bash
grava doctor
```

**Acceptance Criteria:**

**Given** a properly initialized Grava environment
**When** `grava doctor` is run
**Then** all 7 Phase 1 checks are executed and reported
**And** output contains `✅ pass` / `⚠️ warning` / `❌ fail` per check
**And** exit code is `0` when all checks pass
**And** exit code is non-zero when any `❌ fail` check is present

---

### TS-10: Git Hook Integration (SBX-FR-13)

**Requirement:** SBX-FR-13, ADR-001
**Priority:** P2 — Should Pass

**Execution:**
```bash
# Setup: two clones of same repo
git clone /tmp/grava-sandbox /tmp/grava-sandbox-clone
cd /tmp/grava-sandbox

# Create an issue in the original
grava create --title "Hooked Issue" --json
grava export

git add issues.jsonl && git commit -m "add issue"
git push

# Pull in clone — hook should fire
cd /tmp/grava-sandbox-clone
git pull
```

**Acceptance Criteria:**

**Given** a Grava repo with Git hooks registered via `grava init`
**When** `git pull` is executed and brings in updated `issues.jsonl`
**Then** the `post-merge` hook fires `grava hook post-merge`
**And** the clone's database is synchronized with the imported issue data
**And** `grava list` in the clone shows the newly pulled issue

---

## Test Run Report Template

After each sandbox run, record results in this format:

```
## Sandbox Run Report

Date: YYYY-MM-DD
Binary: git SHA or version
Dolt Version: x.y.z

| Test | Requirement | Result | Notes |
|------|-------------|--------|-------|
| TS-01 | SBX-FR-1 | ✅ PASS / ❌ FAIL / ⚠️ PARTIAL | |
| TS-02 | SBX-FR-2 | | |
| TS-03 | SBX-NFR-1 | | Measured: Xms |
| TS-04 | SBX-NFR-2 | | Measured: X writes/sec |
| TS-05 | SBX-FR-5 | | |
| TS-06 | SBX-CHAOS-1 | | |
| TS-07 | SBX-FR-12 | | |
| TS-08 | SBX-FR-9 | | |
| TS-09 | SBX-FR-11 | | |
| TS-10 | SBX-FR-13 | | |

### Overall Status: PASS / FAIL
### Blockers: [list any blocking failures]
### Observations: [runtime notes]
```
