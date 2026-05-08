#!/usr/bin/env bash
# Unit tests for scripts/ship/dep-check.sh.
#
# We mock the `grava` CLI by prepending a temp bin directory to PATH that
# contains a shell stub. The stub reads the desired exit code and stdout
# from environment variables we set per test case, so we can deterministically
# exercise every branch of dep-check.sh without touching Dolt.
#
# Exit codes contract for dep-check.sh:
#   0 — clean (no open blockers)
#   1 — fail-safe (grava errored, or returned malformed JSON)
#   2 — open blocker(s) found (summary on stderr)
#
# Usage:
#   ./scripts/test/test-dep-check.sh
#
# Exit 0 on all-pass, non-zero on any failure.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DEP_CHECK="$REPO_ROOT/scripts/ship/dep-check.sh"

if [ ! -x "$DEP_CHECK" ]; then
  echo "FAIL: $DEP_CHECK is not executable (or missing)" >&2
  exit 1
fi

# ─── Mock-grava harness ────────────────────────────────────────────────
TMP_BIN=$(mktemp -d)
trap 'rm -rf "$TMP_BIN"' EXIT

cat > "$TMP_BIN/grava" <<'STUB'
#!/usr/bin/env bash
# Test stub. Honors:
#   MOCK_GRAVA_EXIT   — exit code (default 0)
#   MOCK_GRAVA_STDOUT — stdout payload
#   MOCK_GRAVA_STDERR — stderr payload
[ -n "${MOCK_GRAVA_STDERR:-}" ] && printf '%s' "$MOCK_GRAVA_STDERR" >&2
[ -n "${MOCK_GRAVA_STDOUT:-}" ] && printf '%s' "$MOCK_GRAVA_STDOUT"
exit "${MOCK_GRAVA_EXIT:-0}"
STUB
chmod +x "$TMP_BIN/grava"
export PATH="$TMP_BIN:$PATH"

# ─── Test runner ───────────────────────────────────────────────────────
PASS=0
FAIL=0

run_case() {
  local name="$1"; local expected_rc="$2"
  local actual_rc

  set +e
  ACTUAL_STDERR=$("$DEP_CHECK" grava-test 2>&1 >/dev/null)
  actual_rc=$?
  set -e

  if [ "$actual_rc" = "$expected_rc" ]; then
    echo "  PASS: $name (rc=$actual_rc)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $name — expected rc=$expected_rc, got rc=$actual_rc"
    echo "       stderr: $ACTUAL_STDERR"
    FAIL=$((FAIL + 1))
  fi
}

reset_mock() {
  unset MOCK_GRAVA_EXIT MOCK_GRAVA_STDOUT MOCK_GRAVA_STDERR
}

# ─── Cases ─────────────────────────────────────────────────────────────

echo "Test: happy path (no blockers)"
reset_mock
export MOCK_GRAVA_EXIT=0
export MOCK_GRAVA_STDOUT='[]'
run_case "empty array → exit 0" 0

echo "Test: closed-only blockers are filtered (preserves grava-cd50 fix)"
reset_mock
export MOCK_GRAVA_EXIT=0
export MOCK_GRAVA_STDOUT='[{"id":"grava-x1","status":"closed"},{"id":"grava-x2","status":"tombstone"}]'
run_case "closed/tombstone filtered → exit 0" 0

echo "Test: open blocker → exit 2"
reset_mock
export MOCK_GRAVA_EXIT=0
export MOCK_GRAVA_STDOUT='[{"id":"grava-blk1","status":"open","title":"upstream"}]'
run_case "open blocker → exit 2" 2

echo "Test: mixed (one open, one closed) → exit 2"
reset_mock
export MOCK_GRAVA_EXIT=0
export MOCK_GRAVA_STDOUT='[{"id":"grava-blk1","status":"open"},{"id":"grava-x1","status":"closed"}]'
run_case "mixed open+closed → exit 2" 2

echo "Test: grava errored (DB down) → fail-safe exit 1"
reset_mock
export MOCK_GRAVA_EXIT=1
export MOCK_GRAVA_STDERR='dial tcp 127.0.0.1:3306: connect: connection refused'
export MOCK_GRAVA_STDOUT=''
run_case "grava exit 1 → dep-check exit 1" 1

echo "Test: grava errored with exit 2 (other failure) → fail-safe exit 1"
reset_mock
export MOCK_GRAVA_EXIT=2
export MOCK_GRAVA_STDERR='schema migration in flight'
run_case "grava exit 2 → dep-check exit 1" 1

echo "Test: malformed JSON (jq parse fails) → fail-safe exit 1"
reset_mock
export MOCK_GRAVA_EXIT=0
export MOCK_GRAVA_STDOUT='this is not json {'
run_case "malformed JSON → exit 1" 1

echo "Test: non-array JSON (object instead) → fail-safe exit 1"
reset_mock
export MOCK_GRAVA_EXIT=0
export MOCK_GRAVA_STDOUT='{"error":"unexpected"}'
run_case "non-array JSON → exit 1" 1

echo "Test: empty stdout from grava → fail-safe exit 1"
reset_mock
export MOCK_GRAVA_EXIT=0
export MOCK_GRAVA_STDOUT=''
run_case "empty stdout → exit 1" 1

# ─── Stderr summary check on blocker case ──────────────────────────────
echo "Test: open-blocker case includes blocker id in stderr summary"
reset_mock
export MOCK_GRAVA_EXIT=0
export MOCK_GRAVA_STDOUT='[{"id":"grava-foo123","status":"open"}]'
set +e
SUMMARY=$("$DEP_CHECK" grava-test 2>&1 >/dev/null)
set -e
if echo "$SUMMARY" | grep -q "grava-foo123"; then
  echo "  PASS: blocker id present in stderr summary"
  PASS=$((PASS + 1))
else
  echo "  FAIL: stderr summary missing blocker id; got: $SUMMARY"
  FAIL=$((FAIL + 1))
fi

# ─── Argv guard ────────────────────────────────────────────────────────
echo "Test: missing issue id arg → exit 1"
reset_mock
export MOCK_GRAVA_EXIT=0
export MOCK_GRAVA_STDOUT='[]'
set +e
"$DEP_CHECK" >/dev/null 2>&1
NOARG_RC=$?
set -e
if [ "$NOARG_RC" = "1" ]; then
  echo "  PASS: missing arg → rc=1"
  PASS=$((PASS + 1))
else
  echo "  FAIL: missing arg expected rc=1, got rc=$NOARG_RC"
  FAIL=$((FAIL + 1))
fi

# ─── Summary ───────────────────────────────────────────────────────────
echo ""
echo "─────────────────────────────────────"
echo "Results: $PASS passed, $FAIL failed"
echo "─────────────────────────────────────"

[ "$FAIL" -eq 0 ]
