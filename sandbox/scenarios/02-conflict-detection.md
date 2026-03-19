# Scenario 2: Conflict Detection — Merge Conflict is Caught

**Status**: Conflict detection validation
**Source**: Test ownership + merge conflict transparency strategy
**Priority**: CRITICAL (core Phase 2 principle: "agents write tests, tests catch breaking changes")

---

## Overview

Two agents modify code that would cause a merge conflict. The system detects the conflict, test suite catches breaking change, and conflict is surfaced to user (not silent).

## Setup

### Prerequisites
- Test suite exists and passes
- Dolt database initialized
- Both agents can write tests

### Steps

1. **Create shared-code task** requiring test additions:
   - File: `pkg/core/processor.go` (existing shared code)
   - Task A: "Refactor processor validation (requires test update)"
   - Task B: "Add processor caching feature (requires test update)"

2. **Agent 1 starts work**:
   ```bash
   grava claim task-a
   # Branch: grava/agent-1/task-a
   ```

3. **Agent 2 starts work simultaneously**:
   ```bash
   grava claim task-b
   # Branch: grava/agent-2/task-b
   ```

4. **Agent 1 modifies shared code**:
   - Changes `processor.go`: refactors validation function signature
   - Writes own test: `processor_validation_test.go` (owns this test)
   - Commits: "refactor: change processor validation signature"

5. **Agent 2 modifies same shared code**:
   - Changes `processor.go`: adds caching that assumes old signature
   - Writes own test: `processor_cache_test.go` (owns this test)
   - Commits: "feat: add processor caching"

6. **Attempt merge**:
   ```bash
   git merge grava/agent-2/task-b
   # CONFLICT in pkg/core/processor.go (both modified)
   ```

---

## Expected Behavior

✅ Both agents can start simultaneously
✅ Both make independent modifications to same file
✅ Git merge detects conflict (file is modified on both sides)
✅ Merge conflict is NOT silently resolved
✅ Agent 1's test suite catches that Agent 2's code breaks validation
✅ Conflict is surfaced to user (not hidden)
✅ User decides resolution path (merge conflict transparency principle)

---

## Validation

**Success Criteria**:
1. ✅ Merge command returns conflict status → `git merge` exit code = 1
2. ✅ Conflict markers appear in `processor.go` → `<<<<< >>>>>` visible
3. ✅ Test suite fails when conflict is auto-resolved incorrectly:
   ```bash
   git status | grep "both modified: pkg/core/processor.go"
   go test ./pkg/core... # FAILS (conflict not resolved)
   ```
4. ✅ Conflict is NOT hidden:
   ```bash
   git diff --name-only --diff-filter=U # Shows conflicted files
   # Output: pkg/core/processor.go
   ```
5. ✅ User decision required:
   - Option A: Keep Agent 1's signature (Agent 2's cache code breaks, needs rewrite)
   - Option B: Keep Agent 2's caching (Agent 1's refactor incomplete, needs adjustment)
   - Option C: Manual merge (both implementations coexist)

**Test Assertions**:
```bash
# Conflict exists
[ $(git status --porcelain | grep "UU\|AA\|DD" | wc -l) -gt 0 ]

# Conflict markers are visible
grep -q "<<<<<" pkg/core/processor.go

# Tests fail on conflict (auto-merge would be wrong)
go test ./pkg/core/... && exit 1 || true

# Conflict is surfaced (not hidden)
git diff --name-only --diff-filter=U | grep -q processor.go
```

---

## Cleanup

```bash
# Option 1: Abandon Agent 2's work
git merge --abort

# Option 2: Resolve manually (simulated)
# - User edits processor.go to resolve conflict
# - Ensures both test suites pass
git add pkg/core/processor.go
git commit -m "resolve: merge conflict between validation and caching"
```

---

## Critical Insight

This scenario validates **the core Phase 2 principle**:

> "Agents write tests. Tests catch breaking changes. When tests pass but conflicts exist, you decide."

- **Tests DO catch breaking changes** (automatic validation)
- **Conflicts are NOT hidden** (user decision authority maintained)
- **No silent merge corruption** (test suite + explicit conflict markers)

---

## Notes

- **Duration**: ~5-10 seconds
- **Network requirements**: Local only
- **Test ownership**: Agent 1 owns validation test, Agent 2 owns cache test
  - Neither agent can modify the other's test
  - Ensures test suite remains authoritative
- **Success signal**: Conflict surfaced, tests fail, user decides
