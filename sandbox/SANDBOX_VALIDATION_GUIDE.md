# Sandbox Validation Guide — Phase 2 Release Gate

## Purpose

The sandbox is Grava's **internal development validation environment** — not a public user-facing feature, but a **Phase 2 prerequisite** that proves multi-agent multi-branch orchestration is production-ready before launch.

**Gate Requirement**: All 8 scenarios must pass before shipping to 10 target users.

---

## The 8 Scenarios

### Failure Recovery (3 scenarios)

These validate that Grava gracefully recovers from agent crashes and orphaned state without data loss.

1. **[Scenario 03: Agent Crash + Resume](./scenarios/03-agent-crash-and-resume.md)**
   - Agent dies mid-execution
   - Wisps (ephemeral activity log) allows next agent to resume without duplication
   - **Proves**: Work is never lost; crashes don't trigger restart from scratch

2. **[Scenario 04: Worktree Ghost State](./scenarios/04-worktree-ghost-state.md)**
   - DB says `in_progress` but worktree directory is missing (ghost state)
   - `grava doctor` detects and heals the mismatch
   - **Proves**: DB ↔ Filesystem consistency is maintained; corruption is automatically detected

3. **[Scenario 05: Orphaned Branch Cleanup](./scenarios/05-orphaned-branch-cleanup.md)**
   - Branch exists but no active issue claims it (orphaned)
   - `grava doctor --dry-run` shows what would be deleted
   - `grava doctor --fix` safely removes with human safeguards
   - **Proves**: Repo hygiene maintained; no silent data loss

### Edge Cases (3 scenarios)

These validate that Grava prevents common concurrent editing conflicts.

6. **[Scenario 06: Delete vs. Modify Conflict](./scenarios/06-delete-vs-modify-conflict.md)**
   - Agent A deletes file, Agent B modifies it on parallel branch
   - Schema-aware merge driver (`grava merge-slot`) detects the conflict
   - **Proves**: Silent merge corruption is impossible; conflicts are surfaced

7. **[Scenario 07: Large File Concurrent Edits](./scenarios/07-large-file-concurrent-edits.md)**
   - Multiple agents attempt sweeping changes to same large files
   - File reservations prevent concurrent edits
   - Pre-commit hook blocks unauthorized commits
   - **Proves**: Agents can't accidentally step on each other's work

8. **[Scenario 08: Rapid Sequential Claims](./scenarios/08-rapid-sequential-claims.md)**
   - Two agents claim same task milliseconds apart
   - Row-level `SELECT FOR UPDATE` lock ensures at most one succeeds
   - **Proves**: No duplicate work; task claiming is atomic

### Baseline (2 scenarios)

These validate fundamental multi-branch operations.

1. **[Scenario 01: Happy Path](./scenarios/01-happy-path.md)**
   - Two agents claim two different branches simultaneously
   - Both complete work independently
   - Both merge cleanly with no conflicts
   - **Proves**: Parallel multi-branch orchestration works

2. **[Scenario 02: Conflict Detection](./scenarios/02-conflict-detection.md)**
   - Two agents modify code that conflicts
   - Test suite detects breaking change
   - Conflict is surfaced to user (not hidden/silent)
   - **Proves**: "Agents write tests. Tests catch breaking changes. When tests pass but conflicts exist, you decide."

---

## Running the Sandbox

### Local Validation

```bash
# Run all 8 scenarios
./sandbox/scripts/run-scenarios.sh

# Verbose output (helpful for debugging)
./sandbox/scripts/run-scenarios.sh --verbose

# Dry-run (see what would run without executing)
./sandbox/scripts/run-scenarios.sh --dry-run

# Run specific scenario
./sandbox/scripts/run-scenarios.sh --filter happy-path
```

### CI Validation

Runs automatically on every commit to `main` or `release` branches:

```yaml
# .github/workflows/sandbox.yml
- All 8 scenarios validated in parallel
- Report generated and attached to PR
- Release gate: CI passes only if all scenarios pass
```

---

## Validation Results

After running, check:

### Local Results

```bash
cat sandbox/results/report-*.md
```

### CI Results

- **Pull Request**: Comment with sandbox status
- **Artifacts**: Full report in CI job artifacts
- **Gate**: PR blocked from merge if sandbox fails

---

## Validation Checklist

**Before Phase 2 Launch**:

- [ ] All 8 scenarios documented ✅
- [ ] CI workflow configured ✅
- [ ] Local script working ✅
- [ ] All scenarios passing in CI
- [ ] Test data fixtures in place
- [ ] Dolt database schema ready
- [ ] Agent message system (mcp_agent_mail) integrated
- [ ] File reservation system active
- [ ] `grava doctor` command implemented
- [ ] Wisps table created
- [ ] Schema-aware merge driver (`grava merge-slot`) integrated
- [ ] Row-level locking (`SELECT FOR UPDATE`) working

---

## Key Design Principles

### 1. Failure Recovery is Automatic
- TTL leases auto-expire on crash
- Wisps preserve checkpoints
- `grava doctor` auto-detects issues
- Prevents silent data loss

### 2. Conflicts are Transparent
- Merge conflicts are never hidden
- Tests catch breaking changes
- User maintains decision authority
- Schema-aware merge prevents corruption

### 3. Safeguards Prevent Accidents
- `SELECT FOR UPDATE` ensures at most one claim per task
- File reservations prevent concurrent edits
- Uncommitted changes block destructive operations
- `--dry-run` always available before `--fix`

### 4. No Agent Boundaries are Crossed
- Agents can't modify tests they don't own
- File reservations are exclusive per agent
- Wisps are task-specific (not shared)
- Each agent gets isolated worktree

---

## Troubleshooting

### Scenario Failures

1. Check scenario documentation:
   ```bash
   cat sandbox/scenarios/{failing-scenario}.md
   ```

2. Run with verbose output:
   ```bash
   ./sandbox/scripts/run-scenarios.sh --verbose
   ```

3. Inspect database state:
   ```bash
   dolt sql "SELECT * FROM issues"
   dolt sql "SELECT * FROM wisps"
   dolt sql "SELECT * FROM file_reservations"
   ```

4. Check git branches:
   ```bash
   git branch -a | grep grava/
   ```

5. Review worktree state:
   ```bash
   ls -la .worktrees/
   ```

### Database Issues

```bash
# Reset test database
./scripts/ci_setup_dolt.sh
./scripts/setup_test_env.sh

# Or manually
dolt --data-dir .grava/dolt sql "DROP DATABASE test_grava; CREATE DATABASE test_grava;"
```

### Timing Issues

Scenarios are timing-sensitive. If failures are intermittent:
- Increase TTL timeouts in database
- Add retry logic with exponential backoff
- Check system resources (CPU, disk)

---

## Integration with Phase 2 Development

### Workflow

1. **Development** → Feature branch
2. **Commit** → CI runs sandbox validation
3. **CI Passes** → Feature can be merged
4. **Pre-Launch** → All scenarios pass on `release` branch
5. **Launch** → Deploy to 10 target users with confidence

### Adding New Scenarios

If a new edge case emerges:

1. Create new scenario file: `sandbox/scenarios/09-new-case.md`
2. Add test assertions to run-scenarios.sh
3. Update CI workflow
4. Validate: `./sandbox/scripts/run-scenarios.sh`

---

## Metrics

Track sandbox validation health:

- **Pass Rate**: % of scenario runs that pass
- **Flakiness**: Scenarios that fail intermittently
- **Coverage**: Are all documented strategies tested?
- **Duration**: Total time to run all 8 scenarios

---

## References

- [Failure Recovery Strategy](../../_bmad-output/planning-artifacts/failure-recovery-strategy.md)
- [Edge Case Resolution Strategy](../../_bmad-output/planning-artifacts/edge-case-resolution-strategy.md)
- [ADR-004: Concurrent Agent Hand-off](../../docs/adr/ADR-004.md)
- [Liveness Subsystem Implementation](../../_bmad-output/planning-artifacts/liveness-subsystem-implementation.md)

---

**Status**: Phase 2 Sandbox Validation Framework — Complete ✅

All 8 scenarios documented and ready for implementation and CI integration.
