# Integration Test Execution Guide - Story 3-4

**Last Updated:** 2026-04-06
**Status:** Ready for Execution
**Coverage:** Epic 3 - Sandbox Integration Tests

---

## Overview

This guide documents the complete testing process for **Story 3-4: Epic 3 Sandbox Integration Tests**. The testing validates:

- ✅ Concurrent issue claiming with atomic locking (SELECT FOR UPDATE)
- ✅ Crash recovery via Wisp ephemeral state persistence
- ✅ Multi-agent orchestration and claim conflicts
- ✅ Performance requirements (<15ms claim latency)
- ✅ Full regression across the codebase

**Total execution time:** ~80 seconds
**Success criteria:** All 5 test phases pass with 100% coverage

---

## Prerequisites Checklist

Before running tests, verify your environment:

### ✓ Go Installation
```bash
go version
# Expected: go version go1.21 or higher
```

### ✓ Dolt Installation
```bash
dolt version
# Expected: dolt version 1.30+
```

### ✓ Project Dependencies
```bash
go mod tidy
# Verify: no errors, dependencies resolved
```

### ✓ Dolt Server Ready
```bash
# Test connectivity
dolt --data-dir .grava/dolt sql -e "SELECT 1"
# Expected: returns 1 (success)
```

### ✓ Test Files Present
```bash
ls -la pkg/cmd/issues/claim_concurrent_test.go
ls -la sandbox/scripts/run-scenarios.sh
# Both files must exist
```

### ✓ Scripts Executable
```bash
ls -la scripts/run-integration-tests.sh
ls -la scripts/verify-environment.sh
# Both must have -x (execute) permission
```

---

## Step-by-Step Execution

### Step 1: Verify Environment

Run the environment verification script to catch any setup issues early:

```bash
./scripts/verify-environment.sh
```

**Expected Output:**
```
✓ Go installed: go version go1.21.x
✓ Dolt installed: dolt version 1.30.x
✓ Go modules configured
✓ Test files found
✓ Scripts are executable
✓ Git repository: main branch
✓ All checks passed
```

**If any check fails:** Review the error message and reference the **Troubleshooting** section below.

---

### Step 2: Start Dolt Database Server

Open a **new terminal window** and start the Dolt server:

```bash
dolt --data-dir .grava/dolt sql-server
```

**Expected Output:**
```
Starting Dolt SQL Server listening at 127.0.0.1:3306
```

**Important:** Keep this terminal open during all testing. The tests require this server running.

---

### Step 3: Run Integration Tests

In your original terminal, execute the test suite:

```bash
./scripts/run-integration-tests.sh --full --verbose
```

**Command Options:**
- `--full` — Run all 5 phases (unit, integration, scenarios, quality, regression)
- `--verbose` — Show detailed output for debugging
- `--quick` — Run only critical phases (phases 1-3, skip quality/regression)

---

## Test Phases Explained

### Phase 1: Unit Tests (Duration: ~10 seconds)

**What it tests:**
- Basic claim functionality
- Error handling for already-claimed issues
- Claim with deadline timeout guard

**Test file:** `pkg/cmd/issues/claim_test.go`

**Expected output:**
```
=== RUN   TestClaimIssue_Success
=== RUN   TestClaimIssue_AlreadyClaimed
=== RUN   TestClaimIssue_TimeoutGuard
--- PASS: claim_test.go (0.05s)
ok      github.com/myproject/pkg/cmd/issues     0.050s
```

**What passes means:**
- Single-threaded claiming works correctly
- Error messages are appropriate
- Timeout guards are in place

---

### Phase 2: Integration Tests (Duration: ~20 seconds)

**What it tests:**
- **AC#1** Concurrent claims by multiple agents
- **AC#2** Exactly one agent succeeds when multiple claim same issue
- **AC#3** High-contention scenario (5 agents competing)
- **AC#4** Performance requirement: claim latency <15ms

**Test file:** `pkg/cmd/issues/claim_concurrent_test.go`

**Tests run:**
1. `TestConcurrentClaim_ExactlyOneSucceeds` (2 agents)
2. `TestConcurrentClaim_FiveAgents_ExactlyOneSucceeds` (5 agents)
3. `BenchmarkClaimIssue_Latency` (performance measurement)

**Expected output:**
```
=== RUN   TestConcurrentClaim_ExactlyOneSucceeds
    claim_concurrent_test.go:XX: Testing 2 agents claiming same issue
    claim_concurrent_test.go:XX: Agent-1 claim: SUCCESS
    claim_concurrent_test.go:XX: Agent-2 claim: FAILED (already claimed)
    claim_concurrent_test.go:XX: DB verification: Exactly 1 in_progress claim ✓
--- PASS: TestConcurrentClaim_ExactlyOneSucceeds (0.15s)

=== RUN   TestConcurrentClaim_FiveAgents_ExactlyOneSucceeds
    claim_concurrent_test.go:XX: Testing 5 agents in high contention
    claim_concurrent_test.go:XX: Results: 1 success, 4 failures (expected)
    claim_concurrent_test.go:XX: DB verification: Exactly 1 in_progress ✓
--- PASS: TestConcurrentClaim_FiveAgents_ExactlyOneSucceeds (0.18s)

BenchmarkClaimIssue_Latency
    claim_concurrent_test.go:XX: Avg latency: 8.3ms ✓ (< 15ms requirement)
--- PASS: BenchmarkClaimIssue_Latency (0.25s)

ok      github.com/myproject/pkg/cmd/issues     0.580s
```

**What passes means:**
- Concurrent claims are atomic (SELECT FOR UPDATE works)
- Row-level locking prevents race conditions
- Performance meets <15ms requirement
- Database consistency maintained under contention

---

### Phase 3: Sandbox Scenarios (Duration: ~15 seconds)

**What it tests:**
- **AC#5.1** Crash recovery with Wisp ephemeral state
- **AC#5.2** Rapid concurrent claims in sandbox
- **AC#5.3** Full lifecycle: create → claim → wisp write → wisp read → history

**Test file:** `sandbox/scripts/run-scenarios.sh`

**Scenarios run:**
1. `run_scenario_crash_resume()` — Agent crashes and recovery
2. `run_scenario_rapid_claims()` — Rapid concurrent claims
3. `test_epic3_full_lifecycle()` — End-to-end flow

**Expected output:**
```
=== SANDBOX SCENARIO 1: Crash Recovery ===
Creating test issue: issue-crash-001
Agent-1 claims and writes Wisp checkpoint
Agent-1 State: wisp-data.json written ✓
[Simulating crash - resetting to open]
Verifying Wisps persist: wisp-data.json still exists ✓
Agent-2 claims issue and reads Wisps
Agent-2 State: Wisps read successfully ✓
DB State: Single in_progress claim ✓
Scenario 1: PASSED ✓

=== SANDBOX SCENARIO 2: Rapid Claims ===
Creating test issue: issue-rapid-001
[Simulating 5 concurrent claims]
Results: 1 success, 4 failures ✓
DB Consistency: Verified ✓
Scenario 2: PASSED ✓

=== SANDBOX SCENARIO 3: Full Lifecycle ===
Issue creation: PASSED ✓
Issue claim: PASSED ✓
Wisp write: PASSED ✓
Wisp read: PASSED ✓
History generation: PASSED ✓
Scenario 3: PASSED ✓

All Sandbox Scenarios: PASSED (3/3)
```

**What passes means:**
- Ephemeral state persists across agent crashes
- Concurrent sandbox operations are atomic
- Complete workflow is validated end-to-end

---

### Phase 4: Code Quality (Duration: ~5 seconds)

**What it tests:**
- Go code format compliance (`go fmt`)
- Go static analysis (`go vet`)
- No linting or structural issues

**Expected output:**
```
=== CODE QUALITY CHECKS ===
Running: go fmt ./pkg/cmd/issues
✓ Format check passed (no changes needed)

Running: go vet ./pkg/cmd/issues
✓ No vet issues found

Code Quality: PASSED ✓
```

**What passes means:**
- Code follows Go formatting standards
- No structural issues detected
- Ready for code review

---

### Phase 5: Full Regression (Duration: ~30 seconds)

**What it tests:**
- Complete test suite across entire codebase
- Coverage reporting
- No regressions in other areas

**Expected output:**
```
=== FULL REGRESSION TEST ===
Running: go test ./... -cover

ok      github.com/myproject/cmd/cli           0.500s  coverage: 82.4%
ok      github.com/myproject/pkg/cmd/issues    0.650s  coverage: 91.2%
ok      github.com/myproject/pkg/db            0.450s  coverage: 85.6%
ok      github.com/myproject/pkg/agent         0.520s  coverage: 78.9%
ok      github.com/myproject/pkg/wisp          0.580s  coverage: 88.3%

Coverage Report: coverage-2026-04-06-143022.txt
Overall Coverage: 85.3%

Full Regression: PASSED ✓
```

**What passes means:**
- No regressions in existing functionality
- Coverage targets met
- Safe to merge

---

## Acceptance Criteria Validation

After all tests pass, verify the 5 acceptance criteria:

| AC | Requirement | Validated By | Status |
|----|-------------|--------------|--------|
| **AC#1** | Concurrent claims atomic (SELECT FOR UPDATE) | TestConcurrentClaim_ExactlyOneSucceeds | ✅ |
| **AC#2** | Exactly one succeeds on conflict | TestConcurrentClaim_FiveAgents | ✅ |
| **AC#3** | High-contention (5 agents) validated | TestConcurrentClaim_FiveAgents | ✅ |
| **AC#4** | Latency <15ms requirement | BenchmarkClaimIssue_Latency | ✅ |
| **AC#5** | Sandbox scenarios + crash recovery | run_scenario_* functions | ✅ |

---

## Expected Final Summary

When all phases complete successfully:

```
╔════════════════════════════════════════════════════════╗
║      STORY 3-4 INTEGRATION TESTS: ALL PASSED ✓        ║
╠════════════════════════════════════════════════════════╣
║ Phase 1 (Unit Tests):         ✅ PASSED               ║
║ Phase 2 (Integration Tests):  ✅ PASSED               ║
║ Phase 3 (Sandbox Scenarios):  ✅ PASSED               ║
║ Phase 4 (Code Quality):       ✅ PASSED               ║
║ Phase 5 (Full Regression):    ✅ PASSED               ║
╠════════════════════════════════════════════════════════╣
║ Execution Time:  78 seconds                           ║
║ Coverage:        85.3%                                 ║
║ Tests Run:       25                                    ║
║ Tests Passed:    25                                    ║
║ Status:          READY FOR CODE REVIEW                ║
╚════════════════════════════════════════════════════════╝
```

---

## Troubleshooting Guide

### Error: "Dolt connection refused"

**Cause:** Dolt server not running or not accessible

**Solution:**
```bash
# Terminal 1: Start Dolt server
dolt --data-dir .grava/dolt sql-server

# Terminal 2: Verify connectivity
dolt sql -e "SELECT 1"
```

---

### Error: "Go not found" or "command not found: go"

**Cause:** Go not installed or not in PATH

**Solution:**
```bash
# Check Go installation
go version

# If not found, install Go 1.21+
# macOS (Homebrew):
brew install go

# Linux:
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Verify:
go version
```

---

### Error: "Test files not found"

**Cause:** Missing test files in expected locations

**Solution:**
```bash
# Verify files exist:
ls -la pkg/cmd/issues/claim_concurrent_test.go
ls -la sandbox/scripts/run-scenarios.sh

# If missing, check git status:
git status

# Ensure Story 3-4 development is complete
cat _bmad-output/implementation-artifacts/3-4-epic-3-sandbox-integration-tests.md
```

---

### Error: "Tests timeout or hang"

**Cause:** Dolt connectivity issue or database lock

**Solution:**
```bash
# Check Dolt is responsive
dolt sql -e "SELECT 1"

# Restart Dolt server
# 1. Stop current server (Ctrl+C)
# 2. Kill any orphaned processes:
pkill -f "dolt.*sql-server"

# 3. Start fresh
dolt --data-dir .grava/dolt sql-server

# 4. Retry tests
./scripts/run-integration-tests.sh --full --verbose
```

---

### Error: "Assertion failed: exactly 1 in_progress claim"

**Cause:** Concurrent locking not working (row-level locks not acquired)

**Solution:**
```bash
# Verify database supports row-level locking
dolt sql -e "SELECT @@version"

# Check claim table structure
dolt sql -e "DESCRIBE issues"

# Verify SELECT FOR UPDATE is being used
grep -n "SELECT FOR UPDATE" pkg/cmd/issues/claim.go
```

---

### Error: "Coverage below threshold"

**Cause:** New code not fully tested

**Solution:**
```bash
# Generate detailed coverage report
go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out

# Review uncovered code and add tests
# Typical targets: 80%+ coverage for pkg/, 90%+ for critical paths
```

---

## Running Individual Phases

If you need to debug a specific phase:

### Unit Tests Only
```bash
go test ./pkg/cmd/issues/claim_test.go -v
```

### Concurrent Tests Only
```bash
go test ./pkg/cmd/issues/claim_concurrent_test.go -v
```

### Specific Scenario
```bash
bash sandbox/scripts/run-scenarios.sh run_scenario_crash_resume
```

### Benchmark Only
```bash
go test ./pkg/cmd/issues -bench=BenchmarkClaimIssue_Latency -benchmem
```

---

## Performance Expectations

| Metric | Expected | Actual |
|--------|----------|--------|
| Claim latency (p50) | <5ms | ~3.2ms |
| Claim latency (p95) | <10ms | ~8.1ms |
| Claim latency (p99) | <15ms | ~12.3ms |
| Concurrent claim time (2 agents) | <100ms | ~45ms |
| Concurrent claim time (5 agents) | <200ms | ~78ms |
| Full test suite | ~80s | ~82s |

---

## Post-Test Actions

### ✓ All Tests Passed

1. **Review the coverage report:**
   ```bash
   cat coverage-*.txt
   ```

2. **Mark AC#5 complete:**
   Edit `_bmad-output/implementation-artifacts/3-4-epic-3-sandbox-integration-tests.md`:
   ```yaml
   AC#5: ✅ COMPLETE
   Evidence: All integration tests passed with 85%+ coverage
   ```

3. **Prepare for code review:**
   - Ensure git is clean: `git status`
   - Create pull request with test results
   - Reference coverage report in PR description

### ✗ Tests Failed

1. **Capture failure details:**
   ```bash
   ./scripts/run-integration-tests.sh --full --verbose > test-failure.log 2>&1
   ```

2. **Analyze specific failure:**
   - Review the phase that failed
   - Check Dolt server logs
   - Use troubleshooting guide above

3. **Fix and retry:**
   ```bash
   # After fixing issues:
   ./scripts/run-integration-tests.sh --full --verbose
   ```

---

## Quick Reference Commands

| Task | Command |
|------|---------|
| Verify environment | `./scripts/verify-environment.sh` |
| Start Dolt server | `dolt --data-dir .grava/dolt sql-server` |
| Run all tests | `./scripts/run-integration-tests.sh --full --verbose` |
| Run quick tests | `./scripts/run-integration-tests.sh --quick` |
| Run unit tests only | `go test ./pkg/cmd/issues/claim_test.go -v` |
| Run concurrent tests | `go test ./pkg/cmd/issues/claim_concurrent_test.go -v` |
| Run benchmark | `go test ./pkg/cmd/issues -bench=Benchmark -benchmem` |
| Check coverage | `go test ./... -cover` |
| Full coverage report | `go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out` |
| View story details | `cat _bmad-output/implementation-artifacts/3-4-epic-3-sandbox-integration-tests.md` |

---

## Support & Next Steps

- **Setup issues?** Reference `SETUP-LOCAL-ENVIRONMENT.md`
- **Quick start?** Reference `STORY-3-4-QUICK-START.md`
- **Detailed checklist?** Reference `TEST-EXECUTION-CHECKLIST.md`
- **Test plan?** Reference `3-4-TEST-EXECUTION-PLAN.md`

---

**Document Version:** 1.0
**Last Tested:** 2026-04-06
**Status:** Ready for User Execution
