#!/usr/bin/env bash
# Tests for pr-merge-watcher.sh PIDFILE handling (grava-24fa).
#
# Covers:
#   1. Skip when an actual pr-merge-watcher process is in PIDFILE (live concurrency).
#   2. Proceed when the PIDFILE references a no-longer-running process.
#   3. Proceed when the PIDFILE references a *recycled, unrelated* PID — this
#      is the bug in scripts/pr-merge-watcher.sh:12-15 where a wrapped PID
#      pointing at e.g. a browser would lock the watcher out indefinitely.
#
# Usage:
#   ./scripts/test/test-watcher-pidfile.sh
#
# Exit 0 on all-pass, non-zero on any failure.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WATCHER="$REPO_ROOT/scripts/pr-merge-watcher.sh"

if [ ! -f "$WATCHER" ]; then
  echo "FATAL: $WATCHER not found" >&2
  exit 2
fi

PASS=0
FAIL=0
FAIL_DETAILS=()

assert_pass() {
  local name="$1"; local cond="$2"; local detail="${3:-}"
  if [ "$cond" = "1" ]; then
    PASS=$((PASS+1))
    echo "  pass: $name"
  else
    FAIL=$((FAIL+1))
    FAIL_DETAILS+=("$name${detail:+ -- $detail}")
    echo "  FAIL: $name${detail:+ -- $detail}"
  fi
}

# ─── Test sandbox ─────────────────────────────────────────────────────────
# Each test runs the watcher inside an isolated sandbox so we never touch the
# real .grava/ directory or hit live grava/gh. Strategy:
#   * SANDBOX dir becomes CWD; watcher uses CLAUDE_PROJECT_DIR=$SANDBOX.
#   * Stub `grava` and `gh` on PATH that exit 0 with empty output, so the
#     watcher's ISSUES list comes back empty and the script exits cleanly
#     after the PIDFILE gate — which is exactly the slice we're testing.
#   * Each test prepares a different PIDFILE state, runs the watcher, and
#     asserts on stderr ("skipping this tick" => SKIPPED, otherwise PROCEEDED).

setup_sandbox() {
  SANDBOX=$(mktemp -d)
  mkdir -p "$SANDBOX/.grava" "$SANDBOX/stubs"

  # Stub grava: always success, empty stdout. Covers `grava list ...` and any
  # other call the watcher might make on its way out.
  cat >"$SANDBOX/stubs/grava" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  # Stub gh: success, empty stdout.
  cat >"$SANDBOX/stubs/gh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  # Stub jq for safety — but real jq is fine if available; only override if
  # we want to control output. The watcher pipes through jq, so let real jq
  # handle empty stdin (it'll output nothing for `.[]?`, which is fine).
  chmod +x "$SANDBOX/stubs/grava" "$SANDBOX/stubs/gh"

  PATH="$SANDBOX/stubs:$PATH"
  export PATH
  export CLAUDE_PROJECT_DIR="$SANDBOX"
}

teardown_sandbox() {
  [ -n "${SANDBOX:-}" ] && [ -d "$SANDBOX" ] && rm -rf "$SANDBOX"
  unset SANDBOX CLAUDE_PROJECT_DIR
}

run_watcher() {
  # Capture stderr so we can grep for the skip message. Run from $SANDBOX
  # so the watcher's `cd "$REPO_ROOT"` lands in the sandbox (REPO_ROOT
  # falls back to $(pwd) when CLAUDE_PROJECT_DIR is set, but we set it
  # explicitly to be safe).
  ( cd "$SANDBOX" && bash "$WATCHER" ) 2>"$SANDBOX/stderr" >/dev/null
  echo $?
}

was_skipped() {
  grep -q "skipping this tick" "$SANDBOX/stderr"
}

# ─── Test 1: skip when a real watcher process is live ─────────────────────
# Spawn a long-running bash process whose argv[0] contains "pr-merge-watcher"
# so that `ps -o command= -p $PID | grep -q pr-merge-watcher` matches.
test_live_watcher_skips() {
  echo "test 1: live pr-merge-watcher process => skip"
  setup_sandbox

  # Use exec -a to set argv[0] to a string that includes "pr-merge-watcher",
  # mimicking how the real watcher would appear in `ps -o command=`.
  bash -c 'exec -a "bash pr-merge-watcher.sh" sleep 30' &
  local fake_pid=$!
  # Give the rename a moment to land in the process table.
  sleep 0.2
  echo "$fake_pid" > "$SANDBOX/.grava/pr-merge-watcher.pid"

  run_watcher >/dev/null
  if was_skipped; then
    assert_pass "live watcher detected and skipped" 1
  else
    assert_pass "live watcher detected and skipped" 0 "stderr: $(cat "$SANDBOX/stderr")"
  fi

  kill "$fake_pid" 2>/dev/null
  wait "$fake_pid" 2>/dev/null
  teardown_sandbox
}

# ─── Test 2: stale PIDFILE (PID belongs to no process) ────────────────────
test_stale_pid_proceeds() {
  echo "test 2: PIDFILE pid no longer alive => proceed"
  setup_sandbox

  # Find a PID that's definitely not in use. Pick a low value, then bump
  # until kill -0 fails. Cap iterations so we don't loop forever.
  local dead_pid=99999
  local i=0
  while kill -0 "$dead_pid" 2>/dev/null && [ $i -lt 50 ]; do
    dead_pid=$((dead_pid - 1))
    i=$((i+1))
  done
  echo "$dead_pid" > "$SANDBOX/.grava/pr-merge-watcher.pid"

  run_watcher >/dev/null
  if ! was_skipped; then
    assert_pass "dead PID treated as stale (proceed)" 1
  else
    assert_pass "dead PID treated as stale (proceed)" 0 "stderr: $(cat "$SANDBOX/stderr")"
  fi

  teardown_sandbox
}

# ─── Test 3: recycled PID — unrelated live process ────────────────────────
# This is the bug. Use the test runner's own PID ($$) as a known-live
# unrelated process. Its `ps -o command=` is "bash test-watcher-pidfile.sh"
# (or similar), which does NOT contain "pr-merge-watcher".
test_recycled_unrelated_pid_proceeds() {
  echo "test 3: PIDFILE pid is unrelated live process => proceed (bug fix)"
  setup_sandbox

  # Confirm $$ is alive and its command does NOT contain pr-merge-watcher.
  if ps -o command= -p "$$" 2>/dev/null | grep -q "pr-merge-watcher"; then
    echo "  skip: test runner's own command unexpectedly matches pr-merge-watcher"
    teardown_sandbox
    return
  fi
  echo "$$" > "$SANDBOX/.grava/pr-merge-watcher.pid"

  run_watcher >/dev/null
  if ! was_skipped; then
    assert_pass "recycled unrelated PID treated as stale (proceed)" 1
  else
    assert_pass "recycled unrelated PID treated as stale (proceed)" 0 \
      "PIDFILE pointed at $$ (unrelated live process) but watcher skipped — bug present"
  fi

  teardown_sandbox
}

# ─── Run ─────────────────────────────────────────────────────────────────
test_live_watcher_skips
test_stale_pid_proceeds
test_recycled_unrelated_pid_proceeds

echo
echo "Results: $PASS passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
  echo "Failures:"
  for d in "${FAIL_DETAILS[@]}"; do echo "  - $d"; done
  exit 1
fi
exit 0
