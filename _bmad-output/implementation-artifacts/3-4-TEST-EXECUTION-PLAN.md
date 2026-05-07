# Story 3-4: Integration Test Execution Plan

**Objective:** Properly execute and validate all integration tests for Epic 3 Sandbox Integration Tests

**Status:** Ready for execution in proper environment

---

## Prerequisites Checklist

- [ ] **Go 1.21+** installed (`go version`)
- [ ] **Dolt** installed and available (`dolt version`)
- [ ] **MySQL driver** available (imported in tests)
- [ ] **Git** repository initialized
- [ ] **Test dependencies** installed (`go mod tidy`)

---

## Test Suite Overview

### Unit Tests (No Dolt Required)
**Location:** `pkg/cmd/issues/claim_test.go`
- Tests claim logic with sqlmock (database mocks)
- No real Dolt connection needed
- Fast execution (~1-2 seconds)

### Integration Tests (Requires Dolt)
**Location:** `pkg/cmd/issues/claim_concurrent_test.go`
- Real Dolt database connection required
- Tests concurrent claim atomicity
- Validates SELECT FOR UPDATE locking
- Execution time: ~5-10 seconds per test

### Sandbox Scenario Tests (Requires Dolt)
**Location:** `sandbox/scripts/run-scenarios.sh`
- 8 multi-agent orchestration scenarios
- Tests Epic 3 features working together
- Gracefully skips if Dolt unavailable
- Execution time: ~30-60 seconds

---

## Test Execution Plan

### Phase 1: Unit Tests (No Environment Setup Needed)

```bash
# Run all unit tests in issues package
cd /Users/trungnguyenhoang/IdeaProjects/grava
go test ./pkg/cmd/issues/... -v

# Run specific test file
go test ./pkg/cmd/issues/ -run TestClaimIssue -v
```

**Expected Output:** All unit tests PASS ✅

---

### Phase 2: Integration Tests (Requires Dolt Running)

```bash
# Start Dolt in the grava database (if not already running)
dolt --data-dir .grava/dolt sql

# In another terminal, run integration tests
cd /Users/trungnguyenhoang/IdeaProjects/grava

# Run concurrent claim tests with integration build tag
go test -tags=integration ./pkg/cmd/issues/... -run TestConcurrentClaim -v -timeout 30s

# Run all integration tests
go test -tags=integration ./pkg/cmd/issues/... -v -timeout 60s
```

**Environment Variable:**
```bash
# Optional: Override default Dolt connection
export GRAVA_TEST_DSN="root@tcp(127.0.0.1:3311)/grava?parseTime=true"
```

**Expected Tests:**
1. `TestConcurrentClaim_ExactlyOneSucceeds` — 2 agents claim same issue
2. `TestConcurrentClaim_FiveAgents_ExactlyOneSucceeds` — 5 agents concurrent claim
3. `BenchmarkClaimIssue_Latency` — Verify <15ms claim latency (NFR2)

**Expected Output:**
```
=== RUN   TestConcurrentClaim_ExactlyOneSucceeds
--- PASS: TestConcurrentClaim_ExactlyOneSucceeds (2.5s)
=== RUN   TestConcurrentClaim_FiveAgents_ExactlyOneSucceeds
--- PASS: TestConcurrentClaim_FiveAgents_ExactlyOneSucceeds (3.1s)
=== RUN   BenchmarkClaimIssue_Latency
--- BENCH: BenchmarkClaimIssue_Latency-8  X  YYYY ns/op  [should be <15ms]
```

---

### Phase 3: Sandbox Scenario Tests (Requires Dolt)

```bash
# Ensure Dolt is running
dolt --data-dir .grava/dolt sql

# In another terminal, run sandbox scenarios
cd /Users/trungnguyenhoang/IdeaProjects/grava
./sandbox/scripts/run-scenarios.sh --verbose

# Or run specific scenarios
./sandbox/scripts/run-scenarios.sh --filter "03-agent-crash" --verbose
./sandbox/scripts/run-scenarios.sh --filter "08-rapid-claims" --verbose
```

**Expected Scenarios:**
1. ✅ **01-happy-path** — Parallel agent execution
2. ✅ **02-conflict-detection** — Merge conflict detection
3. ✅ **03-agent-crash-and-resume** — Wisp recovery after crash
4. ✅ **04-worktree-ghost-state** — grava doctor state healing
5. ✅ **05-orphaned-branch-cleanup** — Safe orphan branch removal
6. ✅ **06-delete-vs-modify-conflict** — Schema-aware merge conflict
7. ✅ **07-large-file-concurrent-edits** — File reservation locking
8. ✅ **08-rapid-sequential-claims** — SELECT FOR UPDATE concurrency

**Expected Output:**
```
🎯 Summary
===========
Total:  8
Passed: 8
Failed: 0
Skipped: 0
✅ All scenarios passed! Phase 2 sandbox validated.
```

**Report Generated:** `sandbox/results/report-YYYYMMDD-HHMMSS.md`

---

## Phase 4: Code Quality Checks

```bash
# Go vet (static analysis)
go vet ./...

# Go fmt (code formatting check)
go fmt ./... && echo "✅ All files properly formatted"

# Golint (if available)
golint ./pkg/cmd/issues/...
```

---

## Phase 5: Full Regression Suite

```bash
# Run ALL tests (unit + integration)
go test ./... -v -timeout 120s

# With coverage report
go test ./... -v -cover -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

---

## Acceptance Criteria Validation Checklist

### AC#1: Concurrent Claims Test ✅
- [ ] `TestConcurrentClaim_ExactlyOneSucceeds` passes
- [ ] Exactly 1 success, 1 ALREADY_CLAIMED error
- [ ] DB state verified: single assignee, status=in_progress
- [ ] No deadlock observed
- [ ] Execution time < 5 seconds

### AC#2: Crash-Resume Scenario ✅
- [ ] `run_scenario_crash_resume()` completes successfully
- [ ] Wisp entries persist after "crash"
- [ ] Second agent reads prior agent's Wisp entries
- [ ] No work duplication
- [ ] Cleanup successful

### AC#3: Full Epic 3 Lifecycle ✅
- [ ] `test_epic3_full_lifecycle()` completes successfully
- [ ] Flow: create → claim → wisp write → wisp read → history
- [ ] Event ordering verified
- [ ] All operations atomic

### AC#4: Reporting Infrastructure ✅
- [ ] Report generated at `sandbox/results/report-{{timestamp}}.md`
- [ ] Summary statistics present
- [ ] All scenarios listed with status
- [ ] Pass rate calculated

### AC#5: No Regressions ✅
- [ ] `go test ./...` all pass
- [ ] `go test -tags=integration ./...` all pass
- [ ] `./sandbox/scripts/run-scenarios.sh` all pass
- [ ] `go vet ./...` no issues
- [ ] Coverage maintained or improved

---

## Dolt Setup Guide

If Dolt is not running:

```bash
# Start Dolt server
dolt --data-dir .grava/dolt sql-server

# In another terminal, verify connection
dolt --data-dir .grava/dolt sql -q "SELECT 1;"

# Check grava database exists
dolt --data-dir .grava/dolt sql -q "SHOW DATABASES;"

# View schema
dolt --data-dir .grava/dolt sql -q "DESCRIBE issues;"
```

---

## Success Criteria

✅ **All tests pass**
```
Unit Tests:        PASS (0 failures)
Integration Tests: PASS (0 failures)
Sandbox Scenarios: PASS (8/8)
Code Quality:      PASS (go vet clean)
Coverage:          Maintained or improved
```

✅ **Test Reports Generated**
- Unit test output
- Integration test output
- Sandbox validation report
- Coverage report (optional)

✅ **AC#5 Complete**
- All 4 subtasks verified
- Evidence documented
- Story ready for code review

---

## Expected Execution Time

| Phase | Time | Notes |
|-------|------|-------|
| Unit Tests | ~2 sec | No Dolt needed |
| Integration Tests | ~30 sec | 3 tests × ~10s each |
| Sandbox Scenarios | ~60 sec | 8 scenarios × ~7.5s avg |
| Code Quality | ~5 sec | vet + fmt |
| **Total** | **~2 minutes** | With Dolt running |

---

## Troubleshooting

### "Dolt not available" Error
```bash
# Check if dolt is installed
which dolt

# If missing, install via Homebrew (macOS)
brew install dolt

# Or download from https://github.com/dolthub/dolt/releases
```

### Connection Refused Error
```bash
# Verify Dolt server is running
dolt sql-server --data-dir .grava/dolt &

# Check connection
mysql -h 127.0.0.1 -P 3306 -u root -e "SELECT 1;"
```

### Tests Still Failing
- Check `.grava/dolt/` directory exists
- Verify database schema is initialized
- Review test DSN: `root@tcp(127.0.0.1:3311)/grava?parseTime=true`
- Check test database user has permissions

---

## Next Steps After Testing

1. ✅ All tests pass → Mark AC#5 complete
2. ✅ Generate evidence report
3. ✅ Transition to code review
4. ✅ Schedule review with different LLM
5. ✅ Close Epic 3
6. ✅ Start Epic 4 planning

---

**Document Generated:** 2026-04-06
**Story:** 3-4 Epic 3 Sandbox Integration Tests
**Status:** Ready for execution
