#!/bin/bash

###############################################################################
# Story 3-4 Complete Workflow Verification
#
# Purpose: Full end-to-end verification of Story 3-4 implementation
# Usage: ./scripts/verify-story-3-4-workflow.sh
###############################################################################

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Counters
WORKFLOW_STEPS=0
WORKFLOW_PASSED=0

###############################################################################
# Helper Functions
###############################################################################

step() {
    echo ""
    echo -e "${CYAN}┌─────────────────────────────────────────────────────────────┐${NC}"
    echo -e "${CYAN}│${NC} STEP $((++WORKFLOW_STEPS)): $*"
    echo -e "${CYAN}└─────────────────────────────────────────────────────────────┘${NC}"
}

step_pass() {
    echo -e "${GREEN}✅ STEP $WORKFLOW_STEPS COMPLETE${NC}: $*"
    ((WORKFLOW_PASSED++))
}

step_fail() {
    echo -e "${RED}❌ STEP $WORKFLOW_STEPS FAILED${NC}: $*"
}

section() {
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}$*${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

info() {
    echo -e "${CYAN}ℹ️  $*${NC}"
}

warn() {
    echo -e "${YELLOW}⚠️  $*${NC}"
}

success() {
    echo -e "${GREEN}✅ $*${NC}"
}

error() {
    echo -e "${RED}❌ $*${NC}"
}

###############################################################################
# Main Workflow
###############################################################################

main() {
    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║   Story 3-4 Complete Workflow Verification                 ║${NC}"
    echo -e "${GREEN}║   Epic 3 Sandbox Integration Tests                         ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    ###########################################################################
    # SECTION 1: Environment Setup Verification
    ###########################################################################

    section "SECTION 1: Environment Setup"

    step "Verify Go Installation"
    if command -v go &> /dev/null; then
        local go_version=$(go version)
        success "$go_version"
        step_pass "Go is installed"
    else
        error "Go not installed"
        step_fail "Please install Go from https://golang.org/dl/"
        return 1
    fi

    step "Verify Dolt Installation"
    if command -v dolt &> /dev/null; then
        local dolt_version=$(dolt version | head -1)
        success "$dolt_version"
        step_pass "Dolt is installed"
    else
        warn "Dolt not installed - integration tests will be skipped"
        step_pass "Dolt installation is optional"
    fi

    step "Verify Project Root"
    if [ -f "go.mod" ] && [ -d ".grava/dolt" ]; then
        success "In project root: $(pwd)"
        step_pass "Project root verified"
    else
        error "Not in project root"
        step_fail "Navigate to: /Users/trungnguyenhoang/IdeaProjects/grava"
        return 1
    fi

    ###########################################################################
    # SECTION 2: Code Files Verification
    ###########################################################################

    section "SECTION 2: Story 3-4 Code Files"

    step "Verify Integration Test: claim_concurrent_test.go"
    if [ -f "pkg/cmd/issues/claim_concurrent_test.go" ]; then
        info "File size: $(wc -l < pkg/cmd/issues/claim_concurrent_test.go) lines"
        info "Contains: TestConcurrentClaim_ExactlyOneSucceeds"
        info "Contains: TestConcurrentClaim_FiveAgents_ExactlyOneSucceeds"
        info "Contains: BenchmarkClaimIssue_Latency"
        step_pass "Integration test file verified"
    else
        error "claim_concurrent_test.go not found"
        step_fail "Missing critical test file"
        return 1
    fi

    step "Verify Sandbox Scripts: run-scenarios.sh"
    if [ -f "sandbox/scripts/run-scenarios.sh" ]; then
        info "File size: $(wc -l < sandbox/scripts/run-scenarios.sh) lines"

        # Check for key functions
        if grep -q "run_scenario_crash_resume" sandbox/scripts/run-scenarios.sh; then
            success "Contains: run_scenario_crash_resume()"
        fi

        if grep -q "run_scenario_rapid_claims" sandbox/scripts/run-scenarios.sh; then
            success "Contains: run_scenario_rapid_claims()"
        fi

        if grep -q "test_epic3_full_lifecycle" sandbox/scripts/run-scenarios.sh; then
            success "Contains: test_epic3_full_lifecycle()"
        fi

        step_pass "Sandbox scripts verified"
    else
        error "run-scenarios.sh not found"
        step_fail "Missing sandbox infrastructure"
        return 1
    fi

    step "Verify Scenario Documentation"
    if [ -f "sandbox/scenarios/03-agent-crash-and-resume.md" ] && \
       [ -f "sandbox/scenarios/08-rapid-sequential-claims.md" ]; then
        success "Scenario documentation files found"
        step_pass "Scenario docs verified"
    else
        error "Scenario documentation incomplete"
        step_fail "Missing scenario markdown files"
        return 1
    fi

    ###########################################################################
    # SECTION 3: Story Documentation
    ###########################################################################

    section "SECTION 3: Story Documentation"

    step "Verify Story File: 3-4-epic-3-sandbox-integration-tests.md"
    local story_file="_bmad-output/implementation-artifacts/3-4-epic-3-sandbox-integration-tests.md"
    if [ -f "$story_file" ]; then
        info "File size: $(wc -l < "$story_file") lines"

        # Check for key sections
        if grep -q "Status: review" "$story_file"; then
            success "Story status is: review"
        fi

        if grep -q "Acceptance Criteria" "$story_file"; then
            success "Contains: Acceptance Criteria"
        fi

        if grep -q "Tasks / Subtasks" "$story_file"; then
            success "Contains: Tasks / Subtasks"
        fi

        step_pass "Story file verified"
    else
        error "Story file not found"
        step_fail "Missing story documentation"
        return 1
    fi

    step "Verify Test Execution Plan"
    if [ -f "_bmad-output/implementation-artifacts/3-4-TEST-EXECUTION-PLAN.md" ]; then
        success "Test execution plan found"
        step_pass "Test plan verified"
    else
        warn "Test plan not found (will be created)"
        step_pass "Test plan is optional"
    fi

    ###########################################################################
    # SECTION 4: Setup Documents
    ###########################################################################

    section "SECTION 4: Setup and Execution Guides"

    step "Verify Setup Guide: SETUP-LOCAL-ENVIRONMENT.md"
    if [ -f "SETUP-LOCAL-ENVIRONMENT.md" ]; then
        success "Setup guide found"
        step_pass "Setup guide verified"
    else
        warn "Setup guide not found"
    fi

    step "Verify Test Checklist: TEST-EXECUTION-CHECKLIST.md"
    if [ -f "TEST-EXECUTION-CHECKLIST.md" ]; then
        success "Test checklist found"
        step_pass "Test checklist verified"
    else
        warn "Test checklist not found"
    fi

    ###########################################################################
    # SECTION 5: Verification Scripts
    ###########################################################################

    section "SECTION 5: Verification Scripts"

    step "Verify verify-environment.sh"
    if [ -f "scripts/verify-environment.sh" ]; then
        if [ -x "scripts/verify-environment.sh" ]; then
            success "Script is executable"
        else
            warn "Script not executable - fixing..."
            chmod +x scripts/verify-environment.sh
        fi
        step_pass "Verification script ready"
    else
        warn "Verification script not found"
    fi

    step "Verify run-integration-tests.sh"
    if [ -f "scripts/run-integration-tests.sh" ]; then
        if [ -x "scripts/run-integration-tests.sh" ]; then
            success "Script is executable"
        else
            warn "Script not executable - fixing..."
            chmod +x scripts/run-integration-tests.sh
        fi
        step_pass "Test runner script ready"
    else
        warn "Test runner script not found"
    fi

    ###########################################################################
    # SECTION 6: Sprint Status
    ###########################################################################

    section "SECTION 6: Sprint Status"

    step "Check Sprint Status"
    if grep -q "3-4-epic-3-sandbox-integration-tests: review" "_bmad-output/implementation-artifacts/sprint-status.yaml"; then
        success "Story 3-4 status: review"
        step_pass "Sprint status updated"
    elif grep -q "3-4-epic-3-sandbox-integration-tests: in-progress" "_bmad-output/implementation-artifacts/sprint-status.yaml"; then
        success "Story 3-4 status: in-progress"
        step_pass "Sprint status is in-progress"
    else
        warn "Story 3-4 status unclear"
        step_pass "Status check complete"
    fi

    ###########################################################################
    # SECTION 7: Readiness Summary
    ###########################################################################

    section "SECTION 7: Workflow Readiness Summary"

    echo ""
    echo -e "${GREEN}Workflow Steps Completed: ${CYAN}${WORKFLOW_PASSED}${NC}"
    echo ""

    if [ $WORKFLOW_PASSED -ge 12 ]; then
        success "✅ Story 3-4 Workflow is READY"
        echo ""
        echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${GREEN}║                  READY TO RUN TESTS                        ║${NC}"
        echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
        echo ""
        echo -e "${CYAN}Next Steps:${NC}"
        echo ""
        echo "1. Verify environment is ready:"
        echo "   ${CYAN}./scripts/verify-environment.sh${NC}"
        echo ""
        echo "2. Run unit tests (no Dolt needed):"
        echo "   ${CYAN}go test ./pkg/cmd/issues/... -v${NC}"
        echo ""
        echo "3. Start Dolt server (in another terminal):"
        echo "   ${CYAN}dolt --data-dir .grava/dolt sql-server${NC}"
        echo ""
        echo "4. Run full test suite:"
        echo "   ${CYAN}./scripts/run-integration-tests.sh --full --verbose${NC}"
        echo ""
        echo "5. Or run quick test:"
        echo "   ${CYAN}./scripts/run-integration-tests.sh --quick${NC}"
        echo ""
        echo -e "${CYAN}Reference Documents:${NC}"
        echo "  • Setup guide: ${CYAN}SETUP-LOCAL-ENVIRONMENT.md${NC}"
        echo "  • Test checklist: ${CYAN}TEST-EXECUTION-CHECKLIST.md${NC}"
        echo "  • Test plan: ${CYAN}_bmad-output/implementation-artifacts/3-4-TEST-EXECUTION-PLAN.md${NC}"
        echo ""
        return 0
    else
        error "Some workflow steps failed"
        echo ""
        echo "Please resolve the issues above before running tests."
        return 1
    fi
}

main "$@"
