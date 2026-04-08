#!/bin/bash

###############################################################################
# Environment Verification Script for Story 3-4 Integration Tests
#
# Purpose: Verify all prerequisites are met before running tests
# Usage: ./scripts/verify-environment.sh
###############################################################################

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Counters
CHECKS_PASSED=0
CHECKS_FAILED=0
CHECKS_WARNING=0

###############################################################################
# Helper Functions
###############################################################################

log_pass() {
    echo -e "${GREEN}✅ PASS${NC}  $*"
    ((CHECKS_PASSED++))
}

log_fail() {
    echo -e "${RED}❌ FAIL${NC}  $*"
    ((CHECKS_FAILED++))
}

log_warn() {
    echo -e "${YELLOW}⚠️  WARN${NC}  $*"
    ((CHECKS_WARNING++))
}

log_info() {
    echo -e "${BLUE}ℹ️  INFO${NC}  $*"
}

log_header() {
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}$*${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

###############################################################################
# Checks
###############################################################################

check_go() {
    log_header "1. Checking Go Installation"

    if command -v go &> /dev/null; then
        local go_version=$(go version | awk '{print $3}')
        log_pass "Go is installed: $go_version"

        # Check version is 1.21+
        local major=$(echo $go_version | cut -d. -f1)
        local minor=$(echo $go_version | cut -d. -f2)

        if [ "$major" = "go1" ] && [ "$minor" -ge 21 ]; then
            log_pass "Go version is 1.21+ (required)"
        else
            log_warn "Go version is $go_version (1.21+ recommended)"
        fi
    else
        log_fail "Go is NOT installed"
        echo ""
        echo "  Install Go from: https://golang.org/dl/"
        echo "  Or use: brew install go"
        return 1
    fi
}

check_dolt() {
    log_header "2. Checking Dolt Installation"

    if command -v dolt &> /dev/null; then
        local dolt_version=$(dolt version | head -1)
        log_pass "Dolt is installed: $dolt_version"
    else
        log_fail "Dolt is NOT installed"
        echo ""
        echo "  Install Dolt from: https://github.com/dolthub/dolt/releases"
        echo "  Or use: brew install dolt"
        return 1
    fi
}

check_project_root() {
    log_header "3. Checking Project Root"

    if [ -f "go.mod" ] && [ -d ".grava" ]; then
        log_pass "Project root detected (go.mod and .grava exist)"
        log_info "Working directory: $(pwd)"
    else
        log_fail "Not in project root (missing go.mod or .grava)"
        echo ""
        echo "  Expected: /Users/trungnguyenhoang/IdeaProjects/grava"
        echo "  Actual: $(pwd)"
        return 1
    fi
}

check_go_modules() {
    log_header "4. Checking Go Dependencies"

    if [ ! -f "go.mod" ]; then
        log_fail "go.mod not found"
        return 1
    fi

    log_info "Verifying modules..."
    if go mod verify > /dev/null 2>&1; then
        log_pass "Go modules are valid"
    else
        log_warn "Go modules need tidying"
        echo ""
        echo "  Run: go mod tidy"
    fi

    # Check key dependencies
    if grep -q "testify" go.mod; then
        log_pass "testify dependency found"
    else
        log_warn "testify dependency not found (needed for tests)"
    fi

    if grep -q "go-sqlmock" go.mod; then
        log_pass "go-sqlmock dependency found"
    else
        log_warn "go-sqlmock dependency not found (needed for unit tests)"
    fi

    if grep -q "mysql" go.mod; then
        log_pass "MySQL driver dependency found"
    else
        log_warn "MySQL driver not found (needed for integration tests)"
    fi
}

check_dolt_database() {
    log_header "5. Checking Dolt Database"

    if [ ! -d ".grava/dolt" ]; then
        log_fail ".grava/dolt directory not found"
        return 1
    fi

    log_pass ".grava/dolt directory exists"

    # Check if Dolt server is running
    if timeout 3 dolt sql "SELECT 1;" > /dev/null 2>&1; then
        log_pass "Dolt server is running and responsive"

        # Check grava database
        if dolt sql "USE grava;" > /dev/null 2>&1; then
            log_pass "grava database exists and is accessible"
        else
            log_warn "grava database not accessible (may need to be initialized)"
        fi
    else
        log_warn "Dolt server is NOT running"
        echo ""
        echo "  Start Dolt with: dolt --data-dir .grava/dolt sql-server"
        echo "  Or run in another terminal: dolt --data-dir .grava/dolt sql-server &"
    fi
}

check_test_files() {
    log_header "6. Checking Test Files"

    local test_files=(
        "pkg/cmd/issues/claim_test.go"
        "pkg/cmd/issues/claim_concurrent_test.go"
        "sandbox/scripts/run-scenarios.sh"
    )

    for file in "${test_files[@]}"; do
        if [ -f "$file" ]; then
            log_pass "Test file found: $file"
        else
            log_fail "Test file NOT found: $file"
        fi
    done
}

check_script_permissions() {
    log_header "7. Checking Script Permissions"

    if [ -f "sandbox/scripts/run-scenarios.sh" ]; then
        if [ -x "sandbox/scripts/run-scenarios.sh" ]; then
            log_pass "run-scenarios.sh is executable"
        else
            log_warn "run-scenarios.sh is not executable"
            echo ""
            echo "  Fix with: chmod +x sandbox/scripts/run-scenarios.sh"
        fi
    fi
}

check_git_repo() {
    log_header "8. Checking Git Repository"

    if git rev-parse --git-dir > /dev/null 2>&1; then
        log_pass "Git repository found"

        # Check git status
        if [ -z "$(git status --porcelain)" ]; then
            log_pass "Working tree is clean (no uncommitted changes)"
        else
            log_warn "Working tree has uncommitted changes"
        fi
    else
        log_fail "Not a git repository"
    fi
}

###############################################################################
# Main
###############################################################################

main() {
    echo ""
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║  Environment Verification for Story 3-4 Integration Tests  ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    # Run all checks
    check_go || true
    check_dolt || true
    check_project_root || true
    check_go_modules || true
    check_dolt_database || true
    check_test_files || true
    check_script_permissions || true
    check_git_repo || true

    # Summary
    log_header "VERIFICATION SUMMARY"

    echo ""
    echo -e "  ${GREEN}Passed:${NC}  $CHECKS_PASSED"
    echo -e "  ${YELLOW}Warnings:${NC}  $CHECKS_WARNING"
    echo -e "  ${RED}Failed:${NC}  $CHECKS_FAILED"
    echo ""

    if [ $CHECKS_FAILED -eq 0 ]; then
        echo -e "${GREEN}✅ All critical checks passed!${NC}"
        echo ""
        echo "You're ready to run tests:"
        echo ""
        echo "  1. Unit tests (no Dolt needed):"
        echo "     go test ./pkg/cmd/issues/... -v"
        echo ""
        echo "  2. Integration tests (requires Dolt running):"
        echo "     go test -tags=integration ./pkg/cmd/issues/... -v"
        echo ""
        echo "  3. Sandbox scenarios (requires Dolt running):"
        echo "     ./sandbox/scripts/run-scenarios.sh --verbose"
        echo ""
        echo "  4. Run all tests at once:"
        echo "     ./scripts/run-integration-tests.sh"
        echo ""
        return 0
    else
        echo -e "${RED}❌ Some critical checks failed. Please fix them before running tests.${NC}"
        echo ""
        echo "See warnings and errors above for details."
        echo ""
        return 1
    fi
}

main "$@"
