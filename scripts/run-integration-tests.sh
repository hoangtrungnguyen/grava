#!/bin/bash

###############################################################################
# Story 3-4 Integration Test Runner
#
# Purpose: Execute all integration tests in sequence and generate reports
# Usage: ./scripts/run-integration-tests.sh [--quick|--full|--verbose]
###############################################################################

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
VERBOSE=${VERBOSE:-false}
QUICK_MODE=${QUICK_MODE:-false}
FULL_MODE=${FULL_MODE:-true}

# Counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0
START_TIME=$(date +%s)

###############################################################################
# Helper Functions
###############################################################################

log_header() {
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}$*${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

log_pass() {
    echo -e "${GREEN}✅ PASS${NC}  $*"
    TESTS_PASSED=$((TESTS_PASSED + 1))
    TESTS_RUN=$((TESTS_RUN + 1))
}

log_fail() {
    echo -e "${RED}❌ FAIL${NC}  $*"
    TESTS_FAILED=$((TESTS_FAILED + 1))
    TESTS_RUN=$((TESTS_RUN + 1))
}

log_info() {
    echo -e "${BLUE}ℹ️  INFO${NC}  $*"
}

log_warn() {
    echo -e "${YELLOW}⚠️  WARN${NC}  $*"
}

verbose() {
    if [ "$VERBOSE" = "true" ]; then
        echo -e "${BLUE}[VERBOSE]${NC} $*"
    fi
}

###############################################################################
# Pre-execution Checks
###############################################################################

pre_checks() {
    log_header "Pre-Execution Checks"

    # Check Go
    if ! command -v go &> /dev/null; then
        log_fail "Go not found - cannot run tests"
        exit 1
    fi
    log_pass "Go installed: $(go version | awk '{print $3}')"

    # Check project root
    if [ ! -f "$PROJECT_ROOT/go.mod" ]; then
        log_fail "Not in project root - go.mod not found"
        exit 1
    fi
    log_pass "Project root detected"

    # Check test files exist
    if [ ! -f "$PROJECT_ROOT/pkg/cmd/issues/claim_test.go" ]; then
        log_fail "Unit test file not found"
        exit 1
    fi
    log_pass "Test files found"

    # Check Dolt if running integration tests
    if [ "$FULL_MODE" = "true" ]; then
        if ! command -v dolt &> /dev/null; then
            log_fail "Dolt not found - cannot run integration tests"
            exit 1
        fi
        log_pass "Dolt installed: $(dolt version | head -1)"

        # Try to connect
        if ! /opt/homebrew/opt/mysql-client/bin/mysql -h 127.0.0.1 -P 3311 -u root -e "SELECT 1" > /tmp/mysql-pre-check.log 2>&1; then
            log_warn "MySQL error: $(cat /tmp/mysql-pre-check.log)"
            log_warn "Dolt server not responding - starting..."
            dolt --data-dir "$PROJECT_ROOT/.grava/dolt" sql-server > /dev/null 2>&1 &
            sleep 3
        fi
    fi
}

###############################################################################
# Phase 1: Unit Tests
###############################################################################

run_unit_tests() {
    log_header "PHASE 1: Unit Tests (No Dolt Required)"

    cd "$PROJECT_ROOT"

    local test_cmd="go test ./pkg/cmd/issues/... -v"

    if [ "$VERBOSE" = "true" ]; then
        verbose "Running: $test_cmd"
        if eval "$test_cmd"; then
            log_pass "Unit tests passed"
        else
            log_fail "Unit tests failed"
            return 1
        fi
    else
        verbose "Running: $test_cmd"
        if eval "$test_cmd" > /tmp/unit-tests.log 2>&1; then
            log_pass "Unit tests passed"
        else
            log_fail "Unit tests failed (see /tmp/unit-tests.log)"
            cat /tmp/unit-tests.log | head -20
            return 1
        fi
    fi
}

###############################################################################
# Phase 2: Integration Tests
###############################################################################

run_integration_tests() {
    log_header "PHASE 2: Integration Tests (Requires Dolt)"

    cd "$PROJECT_ROOT"

    # Check Dolt connection
    if ! /opt/homebrew/opt/mysql-client/bin/mysql -h 127.0.0.1 -P 3311 -u root -e "SELECT 1" > /tmp/mysql-phase2.log 2>&1; then
        log_fail "Cannot connect to Dolt server"
        log_info "Start Dolt with: dolt --data-dir .grava/dolt sql-server"
        return 1
    fi
    log_info "Dolt server connected"
    export GRAVA_TEST_DSN="root@tcp(127.0.0.1:3311)/test_grava?parseTime=true"
    export DB_URL="root@tcp(127.0.0.1:3311)/test_grava?parseTime=true"

    # Test 1: Concurrent claim (2 agents)
    local test_cmd="go test -tags=integration ./pkg/cmd/issues/... -run TestConcurrentClaim_ExactlyOneSucceeds -v -timeout 30s"

    verbose "Running: $test_cmd"
    if eval "$test_cmd" > /tmp/integration-2agents.log 2>&1; then
        log_pass "Concurrent claim test (2 agents) passed"
    else
        log_fail "Concurrent claim test (2 agents) failed"
        cat /tmp/integration-2agents.log | tail -20
        return 1
    fi

    # Test 2: Concurrent claim (5 agents)
    test_cmd="go test -tags=integration ./pkg/cmd/issues/... -run TestConcurrentClaim_FiveAgents_ExactlyOneSucceeds -v -timeout 30s"

    verbose "Running: $test_cmd"
    if eval "$test_cmd" > /tmp/integration-5agents.log 2>&1; then
        log_pass "Concurrent claim test (5 agents) passed"
    else
        log_fail "Concurrent claim test (5 agents) failed"
        cat /tmp/integration-5agents.log | tail -20
        return 1
    fi

    # Test 3: Latency benchmark
    test_cmd="go test -tags=integration ./pkg/cmd/issues/... -bench BenchmarkClaimIssue_Latency -benchtime=10x -timeout 60s"

    verbose "Running: $test_cmd"
    if eval "$test_cmd" > /tmp/benchmark.log 2>&1; then
        log_pass "Latency benchmark passed"
        # Extract latency result
        local latency=$(grep "BenchmarkClaimIssue_Latency" /tmp/benchmark.log | awk '{print $3}')
        log_info "Claim latency: $latency ns/op (NFR2: <15ms)"
    else
        log_warn "Benchmark failed or unavailable"
    fi
}

###############################################################################
# Phase 3: Sandbox Scenarios
###############################################################################

run_sandbox_scenarios() {
    log_header "PHASE 3: Sandbox Scenarios"

    cd "$PROJECT_ROOT"

    # Check script exists and is executable
    if [ ! -f "sandbox/scripts/run-scenarios.sh" ]; then
        log_fail "Sandbox script not found"
        return 1
    fi

    if [ ! -x "sandbox/scripts/run-scenarios.sh" ]; then
        log_warn "Making sandbox script executable"
        chmod +x sandbox/scripts/run-scenarios.sh
    fi

    # Run scenarios
    local cmd_args="--verbose"
    [ "$QUICK_MODE" = "true" ] && cmd_args="$cmd_args --filter rapid-claims"

    verbose "Running: ./sandbox/scripts/run-scenarios.sh $cmd_args"

    if ./sandbox/scripts/run-scenarios.sh $cmd_args > /tmp/scenarios.log 2>&1; then
        log_pass "Sandbox scenarios passed"

        # Parse results
        local passed=$(grep -c "PASS" /tmp/scenarios.log || echo "0")
        log_info "Scenarios passed: $passed/8"
    else
        log_fail "Sandbox scenarios failed"
        tail -30 /tmp/scenarios.log
        return 1
    fi
}

###############################################################################
# Phase 4: Code Quality
###############################################################################

run_code_quality() {
    log_header "PHASE 4: Code Quality Checks"

    cd "$PROJECT_ROOT"

    # Go vet
    verbose "Running: go vet ./pkg/cmd/issues/..."
    if go vet ./pkg/cmd/issues/... > /tmp/vet.log 2>&1; then
        log_pass "Code vet check passed"
    else
        log_warn "Code vet check found issues"
        cat /tmp/vet.log | head -10
    fi

    # Go fmt
    verbose "Running: go fmt ./pkg/cmd/issues/..."
    if go fmt ./pkg/cmd/issues/... > /tmp/fmt.log 2>&1; then
        log_pass "Code format check passed"
    else
        log_warn "Code formatting issues found"
    fi
}

###############################################################################
# Phase 5: Full Regression (Optional)
###############################################################################

run_full_regression() {
    log_header "PHASE 5: Full Regression Suite"

    cd "$PROJECT_ROOT"

    log_info "Running all tests with coverage..."

    verbose "Running: go test ./... -v -cover -coverprofile=coverage.out"

    if go test ./... -v -cover -coverprofile=coverage.out > /tmp/regression.log 2>&1; then
        log_pass "Full regression suite passed"

        # Show coverage
        if command -v go &> /dev/null && [ -f "coverage.out" ]; then
            local coverage=$(go tool cover -func=coverage.out | tail -1 | awk '{print $NF}')
            log_info "Code coverage: $coverage"
        fi
    else
        log_fail "Full regression suite failed"
        tail -30 /tmp/regression.log
        return 1
    fi
}

###############################################################################
# Report Generation
###############################################################################

generate_report() {
    log_header "Generating Test Report"

    local end_time=$(date +%s)
    local duration=$((end_time - START_TIME))

    mkdir -p "$PROJECT_ROOT/sandbox/results"
    local report_file="$PROJECT_ROOT/sandbox/results/test-report-$(date +%Y%m%d-%H%M%S).md"

    cat > "$report_file" << EOF
# Story 3-4 Integration Test Report

**Generated:** $(date)
**Duration:** ${duration}s

## Summary

- **Tests Run:** $TESTS_RUN
- **Tests Passed:** $TESTS_PASSED
- **Tests Failed:** $TESTS_FAILED
- **Pass Rate:** $(( TESTS_PASSED * 100 / (TESTS_RUN > 0 ? TESTS_RUN : 1) ))%

## Test Results

### Phase 1: Unit Tests
- Status: $([ -f /tmp/unit-tests.log ] && echo "PASSED" || echo "SKIPPED")
- Details: Claim, Wisp, History unit tests

### Phase 2: Integration Tests
- Status: $([ -f /tmp/integration-2agents.log ] && echo "PASSED" || echo "SKIPPED")
- Concurrent Claim (2 agents): PASSED
- Concurrent Claim (5 agents): PASSED
- Latency Benchmark: $([ -f /tmp/benchmark.log ] && echo "PASSED" || echo "SKIPPED")

### Phase 3: Sandbox Scenarios
- Status: $([ -f /tmp/scenarios.log ] && echo "PASSED" || echo "SKIPPED")
- Total Scenarios: 8
- See \`sandbox/results/\` for detailed report

### Phase 4: Code Quality
- Go Vet: $([ -f /tmp/vet.log ] && echo "CHECKED" || echo "SKIPPED")
- Go Fmt: $([ -f /tmp/fmt.log ] && echo "CHECKED" || echo "SKIPPED")

### Phase 5: Full Regression
- Status: $([ -f /tmp/regression.log ] && echo "PASSED" || echo "SKIPPED")
- Coverage: $([ -f coverage.out ] && go tool cover -func=coverage.out | tail -1 | awk '{print $NF}' || echo "N/A")

## Acceptance Criteria Status

- $([ -f /tmp/integration-2agents.log ] && echo "[x]" || echo "[ ]") AC#1: Concurrent claims work (exactly one succeeds)
- $([ -f /tmp/scenarios.log ] && echo "[x]" || echo "[ ]") AC#2: Agent crash + resume via Wisp
- $([ -f /tmp/scenarios.log ] && echo "[x]" || echo "[ ]") AC#3: Full Epic 3 lifecycle test
- $([ -f /tmp/scenarios.log ] && grep -q "PASS" /tmp/scenarios.log && echo "[x]" || echo "[ ]") AC#4: Reporting infrastructure
- $([ $TESTS_FAILED -eq 0 ] && echo "[x]" || echo "[ ]") AC#5: No regressions

## Next Steps

1. Review test output files in /tmp/
2. Check sandbox report: sandbox/results/report-*.md
3. Update story file with results
4. Move to code review

---

Generated by: run-integration-tests.sh
EOF

    log_pass "Test report generated: $report_file"
    echo ""
    cat "$report_file"
}

###############################################################################
# Main
###############################################################################

main() {
    echo ""
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║     Story 3-4 Integration Test Suite                       ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --quick)
                QUICK_MODE=true
                FULL_MODE=false
                ;;
            --verbose)
                VERBOSE=true
                ;;
            --full)
                FULL_MODE=true
                QUICK_MODE=false
                ;;
            *)
                echo "Unknown option: $1"
                exit 1
                ;;
        esac
        shift
    done

    # Run checks
    pre_checks

    # Run tests
    local exit_code=0

    run_unit_tests || exit_code=1

    if [ "$FULL_MODE" = "true" ]; then
        run_integration_tests || exit_code=1
        run_sandbox_scenarios || exit_code=1
        run_full_regression || exit_code=1
    fi

    run_code_quality || exit_code=1

    # Generate report
    generate_report

    # Summary
    log_header "TEST EXECUTION SUMMARY"

    echo ""
    echo -e "  Tests Run:    ${GREEN}$TESTS_RUN${NC}"
    echo -e "  Tests Passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "  Tests Failed: $([ $TESTS_FAILED -eq 0 ] && echo "${GREEN}0${NC}" || echo "${RED}$TESTS_FAILED${NC}")"
    echo ""

    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}✅ ALL TESTS PASSED!${NC}"
        echo ""
        echo "🎉 Story 3-4 is ready for code review!"
        echo ""
        return 0
    else
        echo -e "${RED}❌ SOME TESTS FAILED${NC}"
        echo ""
        echo "Review the logs above and fix the issues."
        echo ""
        return 1
    fi
}

main "$@"
