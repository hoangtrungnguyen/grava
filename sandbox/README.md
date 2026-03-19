# Grava Multi-Branch Orchestration Sandbox

This directory contains the validated test scenarios for Grava Phase 2: ensuring multi-agent orchestration works reliably across multiple branches before launch to users.

## Overview

The sandbox tests **8 core scenarios** derived from documented recovery and edge case strategies:
- Failure recovery strategies (agent crashes, worktree cleanup, orphaned branches)
- Edge case handling (delete vs. modify conflicts, large file changes, rapid claims)
- Happy path validation (parallel execution without conflicts)

## Sandbox Pass Criteria

**Phase 2 launch gate**: All 8 scenarios must pass in CI before shipping to 10 target users.

## Scenarios

1. **Happy Path** — 2 agents claim 2 branches, both complete work, no conflicts
2. **Conflict Detection** — Merge would conflict, test catches it, HumanOverseer alert triggered
3. **Agent Crash + Resume** — Agent dies mid-execution, Wisps recovery allows next agent to resume
4. **Worktree Ghost State** — `grava doctor` detects and heals DB/filesystem mismatches
5. **Orphaned Branch Cleanup** — `grava doctor` safely removes branches with human safeguards
6. **Delete vs. Modify Conflict** — Schema-aware merge driver detects deletion on one branch, modification on another
7. **Large File Concurrent Edits** — File reservations prevent concurrent edits to same large files
8. **Rapid Sequential Claims** — `SELECT FOR UPDATE` locks ensure only one agent claims task

## Running Scenarios

### Local Execution
```bash
./sandbox/scripts/run-scenarios.sh
```

### CI Validation
Runs automatically on every commit (`.github/workflows/sandbox.yml`)

## Documentation Structure

Each scenario has:
- **Setup** — How to reproduce the scenario
- **Expected Behavior** — What should happen (from documented strategy)
- **Validation** — How we know it worked
- **Cleanup** — Restore state for next scenario

## Files

- `scenarios/` — Individual scenario documentation (8 files)
- `scripts/` — Scenario execution and validation scripts
- `fixtures/` — Test data and branch configurations

---

**Status**: Phase 2 sandbox development in progress
