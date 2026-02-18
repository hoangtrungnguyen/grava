#!/bin/bash
# =============================================================================
# scripts/test/e2e_test_all_commands.sh
# End-to-end smoke test for ALL grava CLI commands.
#
# Prerequisites:
#   - Dolt SQL Server running on port 3306 (scripts/start_dolt_server.sh)
#   - mysql client available
#   - Go toolchain available
#
# Usage (run from repo root):
#   ./scripts/test/e2e_test_all_commands.sh
#
# Exit code: 0 = all tests passed, 1 = one or more tests failed
# =============================================================================

set -euo pipefail

# ── Config ────────────────────────────────────────────────────────────────────
PORT=3306
HOST="127.0.0.1"
TEST_DB="e2e_grava_$(date +%s)"   # unique name so parallel runs don't collide
DB_URL="root@tcp(${HOST}:${PORT})/${TEST_DB}?parseTime=true"
BINARY="./bin/grava-e2e"
SCHEMA_FILE="scripts/schema/001_initial_schema.sql"

# Resolve mysql client
if [ -f "/opt/homebrew/opt/mysql-client/bin/mysql" ]; then
    MYSQL="/opt/homebrew/opt/mysql-client/bin/mysql"
elif command -v mysql &>/dev/null; then
    MYSQL=$(command -v mysql)
else
    echo "❌ mysql client not found. Install mysql-client and retry."
    exit 1
fi

# ── Counters ──────────────────────────────────────────────────────────────────
PASS=0
FAIL=0
FAILURES=()

# ── Helpers ───────────────────────────────────────────────────────────────────
grava() { "$BINARY" --db-url "$DB_URL" "$@"; }

assert_contains() {
    local test_name="$1"
    local needle="$2"
    local haystack="$3"
    if echo "$haystack" | grep -qF "$needle"; then
        echo "  ✅ PASS: $test_name"
        ((PASS++))
    else
        echo "  ❌ FAIL: $test_name"
        echo "     Expected to contain: $needle"
        echo "     Got: $haystack"
        ((FAIL++))
        FAILURES+=("$test_name")
    fi
}

assert_not_contains() {
    local test_name="$1"
    local needle="$2"
    local haystack="$3"
    if ! echo "$haystack" | grep -qF "$needle"; then
        echo "  ✅ PASS: $test_name"
        ((PASS++))
    else
        echo "  ❌ FAIL: $test_name"
        echo "     Expected NOT to contain: $needle"
        echo "     Got: $haystack"
        ((FAIL++))
        FAILURES+=("$test_name")
    fi
}

assert_exit_nonzero() {
    local test_name="$1"
    local exit_code="$2"
    if [ "$exit_code" -ne 0 ]; then
        echo "  ✅ PASS: $test_name (exit $exit_code)"
        ((PASS++))
    else
        echo "  ❌ FAIL: $test_name (expected non-zero exit, got 0)"
        ((FAIL++))
        FAILURES+=("$test_name")
    fi
}

section() { echo ""; echo "━━━ $1 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; }

# ── Preflight ─────────────────────────────────────────────────────────────────
section "Preflight"

echo "🔍 Checking Dolt server on port $PORT..."
if ! lsof -i :"$PORT" >/dev/null 2>&1; then
    echo "❌ Dolt server not running on port $PORT."
    echo "   Start it with: ./scripts/start_dolt_server.sh"
    exit 1
fi
echo "  ✅ Dolt server detected."

echo "🔨 Building grava binary..."
go build -o "$BINARY" ./cmd/grava/
echo "  ✅ Binary built at $BINARY"

echo "📦 Creating test database '$TEST_DB'..."
"$MYSQL" -h "$HOST" -P "$PORT" -u root \
    -e "CREATE DATABASE ${TEST_DB};"
"$MYSQL" -h "$HOST" -P "$PORT" -u root -D "$TEST_DB" < "$SCHEMA_FILE"
echo "  ✅ Schema applied."

# ── Cleanup trap ──────────────────────────────────────────────────────────────
cleanup() {
    echo ""
    echo "🧹 Cleaning up..."
    "$MYSQL" -h "$HOST" -P "$PORT" -u root \
        -e "DROP DATABASE IF EXISTS ${TEST_DB};" 2>/dev/null || true
    rm -f "$BINARY"
    echo "  ✅ Dropped database '$TEST_DB' and removed binary."
}
trap cleanup EXIT

# =============================================================================
# TESTS
# =============================================================================

# ── create ────────────────────────────────────────────────────────────────────
section "grava create"

OUT=$(grava create --title "Fix login bug" --type bug --priority high 2>&1)
assert_contains "create: normal issue" "Created issue:" "$OUT"
ISSUE_ID=$(echo "$OUT" | grep -oE 'grava-[a-z0-9]+' | head -1)
echo "     → Created issue ID: $ISSUE_ID"

OUT=$(grava create --title "Scratch note" --ephemeral 2>&1)
assert_contains "create: ephemeral wisp" "Wisp" "$OUT"
WISP_ID=$(echo "$OUT" | grep -oE 'grava-[a-z0-9]+' | head -1)
echo "     → Created wisp ID: $WISP_ID"

OUT=$(grava create --title "Second issue" --type task --priority low 2>&1)
assert_contains "create: second normal issue" "Created issue:" "$OUT"
ISSUE2_ID=$(echo "$OUT" | grep -oE 'grava-[a-z0-9]+' | head -1)
echo "     → Created issue2 ID: $ISSUE2_ID"

# ── show ──────────────────────────────────────────────────────────────────────
section "grava show"

OUT=$(grava show "$ISSUE_ID" 2>&1)
assert_contains "show: title present"    "Fix login bug" "$OUT"
assert_contains "show: type present"     "bug"           "$OUT"
assert_contains "show: priority present" "high"          "$OUT"
assert_contains "show: status present"   "open"          "$OUT"

OUT=$(grava show "grava-doesnotexist" 2>&1) && EC=0 || EC=$?
assert_exit_nonzero "show: non-existent issue returns error" "$EC"

# ── list ──────────────────────────────────────────────────────────────────────
section "grava list"

OUT=$(grava list 2>&1)
assert_contains     "list: normal issue visible"   "$ISSUE_ID"  "$OUT"
assert_not_contains "list: wisp hidden by default" "$WISP_ID"   "$OUT"

OUT=$(grava list --wisp 2>&1)
assert_contains     "list --wisp: wisp visible"       "$WISP_ID"  "$OUT"
assert_not_contains "list --wisp: normal issue hidden" "$ISSUE_ID" "$OUT"

# ── update ────────────────────────────────────────────────────────────────────
section "grava update"

OUT=$(grava update "$ISSUE_ID" --status in_progress 2>&1)
assert_contains "update: status changed" "Updated issue $ISSUE_ID" "$OUT"

OUT=$(grava show "$ISSUE_ID" 2>&1)
assert_contains "update: show reflects new status" "in_progress" "$OUT"

OUT=$(grava update "$ISSUE_ID" --title "Fix login bug (revised)" 2>&1)
assert_contains "update: title changed" "Updated issue $ISSUE_ID" "$OUT"

OUT=$(grava update "grava-doesnotexist" --status closed 2>&1) && EC=0 || EC=$?
assert_exit_nonzero "update: non-existent issue returns error" "$EC"

# ── subtask ───────────────────────────────────────────────────────────────────
section "grava subtask"

OUT=$(grava subtask "$ISSUE_ID" --title "Subtask Alpha" --desc "First subtask" 2>&1)
assert_contains "subtask: created" "Created subtask:" "$OUT"
SUBTASK_ID=$(echo "$OUT" | grep -oE "${ISSUE_ID//./\\.}\.[0-9]+" | head -1)
echo "     → Created subtask ID: $SUBTASK_ID"

OUT=$(grava show "$SUBTASK_ID" 2>&1)
assert_contains "subtask: show works" "Subtask Alpha" "$OUT"

# ── comment ───────────────────────────────────────────────────────────────────
section "grava comment"

OUT=$(grava comment "$ISSUE_ID" "Investigated root cause, see PR #42" 2>&1)
assert_contains "comment: added" "💬 Comment added to $ISSUE_ID" "$OUT"

# Add a second comment to verify append behaviour
OUT=$(grava comment "$ISSUE_ID" "Follow-up: confirmed fix works" 2>&1)
assert_contains "comment: second comment added" "💬 Comment added to $ISSUE_ID" "$OUT"

# Verify comments are stored in metadata via mysql
META=$("$MYSQL" -h "$HOST" -P "$PORT" -u root -D "$TEST_DB" -sNe \
    "SELECT metadata FROM issues WHERE id='${ISSUE_ID}';")
assert_contains "comment: metadata contains first comment text"  "Investigated root cause" "$META"
assert_contains "comment: metadata contains second comment text" "Follow-up"               "$META"

OUT=$(grava comment "grava-doesnotexist" "text" 2>&1) && EC=0 || EC=$?
assert_exit_nonzero "comment: non-existent issue returns error" "$EC"

# ── dep ───────────────────────────────────────────────────────────────────────
section "grava dep"

OUT=$(grava dep "$ISSUE_ID" "$ISSUE2_ID" 2>&1)
assert_contains "dep: default blocks type" "🔗 Dependency created: $ISSUE_ID -[blocks]-> $ISSUE2_ID" "$OUT"

OUT=$(grava dep "$ISSUE_ID" "$SUBTASK_ID" --type "relates-to" 2>&1)
assert_contains "dep: custom type" "-[relates-to]->" "$OUT"

# Verify row in dependencies table
DEP_COUNT=$("$MYSQL" -h "$HOST" -P "$PORT" -u root -D "$TEST_DB" -sNe \
    "SELECT COUNT(*) FROM dependencies WHERE from_id='${ISSUE_ID}';")
assert_contains "dep: rows in dependencies table" "2" "$DEP_COUNT"

OUT=$(grava dep "$ISSUE_ID" "$ISSUE_ID" 2>&1) && EC=0 || EC=$?
assert_exit_nonzero "dep: self-loop returns error" "$EC"

# ── label ─────────────────────────────────────────────────────────────────────
section "grava label"

OUT=$(grava label "$ISSUE_ID" "needs-review" 2>&1)
assert_contains "label: added" 'Label "needs-review" added to' "$OUT"

# Idempotency — adding same label again
OUT=$(grava label "$ISSUE_ID" "needs-review" 2>&1)
assert_contains "label: idempotent (already present)" "already present" "$OUT"

# Add a second distinct label
OUT=$(grava label "$ISSUE_ID" "priority:high" 2>&1)
assert_contains "label: second label added" 'Label "priority:high" added to' "$OUT"

# Verify both labels are in metadata
META=$("$MYSQL" -h "$HOST" -P "$PORT" -u root -D "$TEST_DB" -sNe \
    "SELECT metadata FROM issues WHERE id='${ISSUE_ID}';")
assert_contains "label: metadata contains needs-review" "needs-review"  "$META"
assert_contains "label: metadata contains priority:high" "priority:high" "$META"

OUT=$(grava label "grava-doesnotexist" "tag" 2>&1) && EC=0 || EC=$?
assert_exit_nonzero "label: non-existent issue returns error" "$EC"

# ── assign ────────────────────────────────────────────────────────────────────
section "grava assign"

OUT=$(grava assign "$ISSUE_ID" "alice" 2>&1)
assert_contains "assign: assigned to alice" "Assigned $ISSUE_ID to alice" "$OUT"

# Verify in DB
ASSIGNEE=$("$MYSQL" -h "$HOST" -P "$PORT" -u root -D "$TEST_DB" -sNe \
    "SELECT assignee FROM issues WHERE id='${ISSUE_ID}';")
assert_contains "assign: DB reflects alice" "alice" "$ASSIGNEE"

# Reassign to agent identity
OUT=$(grava assign "$ISSUE_ID" "agent:planner-v2" 2>&1)
assert_contains "assign: agent identity" "Assigned $ISSUE_ID to agent:planner-v2" "$OUT"

# Clear assignee
OUT=$(grava assign "$ISSUE_ID" "" 2>&1)
assert_contains "assign: cleared" "Assignee cleared on $ISSUE_ID" "$OUT"

ASSIGNEE=$("$MYSQL" -h "$HOST" -P "$PORT" -u root -D "$TEST_DB" -sNe \
    "SELECT COALESCE(assignee,'NULL') FROM issues WHERE id='${ISSUE_ID}';")
assert_contains "assign: DB assignee is NULL after clear" "NULL" "$ASSIGNEE"

OUT=$(grava assign "grava-doesnotexist" "bob" 2>&1) && EC=0 || EC=$?
assert_exit_nonzero "assign: non-existent issue returns error" "$EC"

# ── compact ───────────────────────────────────────────────────────────────────
section "grava compact"

# Create a fresh wisp for compaction
OUT=$(grava create --title "Old wisp to compact" --ephemeral 2>&1)
OLD_WISP_ID=$(echo "$OUT" | grep -oE 'grava-[a-z0-9]+' | head -1)

# Backdate it so --days 0 picks it up
"$MYSQL" -h "$HOST" -P "$PORT" -u root -D "$TEST_DB" \
    -e "UPDATE issues SET created_at = DATE_SUB(NOW(), INTERVAL 10 DAY) WHERE id='${OLD_WISP_ID}';"

OUT=$(grava compact --days 7 2>&1)
assert_contains "compact: purged old wisp" "Compacted 1 Wisp(s)" "$OUT"

# Verify tombstone in deletions table
DEL_COUNT=$("$MYSQL" -h "$HOST" -P "$PORT" -u root -D "$TEST_DB" -sNe \
    "SELECT COUNT(*) FROM deletions WHERE id='${OLD_WISP_ID}';")
assert_contains "compact: tombstone in deletions table" "1" "$DEL_COUNT"

# Verify wisp is gone from issues
ISSUE_COUNT=$("$MYSQL" -h "$HOST" -P "$PORT" -u root -D "$TEST_DB" -sNe \
    "SELECT COUNT(*) FROM issues WHERE id='${OLD_WISP_ID}';")
assert_contains "compact: wisp removed from issues" "0" "$ISSUE_COUNT"

OUT=$(grava compact --days 7 2>&1)
assert_contains "compact: nothing to compact" "Nothing to compact" "$OUT"

# =============================================================================
# Summary
# =============================================================================
section "Results"
TOTAL=$((PASS + FAIL))
echo ""
echo "  Tests run:    $TOTAL"
echo "  ✅ Passed:    $PASS"
echo "  ❌ Failed:    $FAIL"

if [ ${#FAILURES[@]} -gt 0 ]; then
    echo ""
    echo "  Failed tests:"
    for f in "${FAILURES[@]}"; do
        echo "    • $f"
    done
fi

echo ""
if [ "$FAIL" -eq 0 ]; then
    echo "🎉 All $TOTAL E2E tests passed!"
    exit 0
else
    echo "💥 $FAIL/$TOTAL tests FAILED."
    exit 1
fi
