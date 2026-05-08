#!/usr/bin/env bash
# Tests for pr-merge-watcher.sh COMMENTS_JSON validation (grava-431b).
#
# Bug: when `gh api .../pulls/N/comments` fails or returns a non-array
# (rate-limit error string, malformed JSON, an object, plain text), the
# downstream pipeline crashes:
#   * `jq -c '[.[] | ...]'` errors with "Cannot index string with string".
#   * NEW becomes empty -> `jq 'length'` outputs nothing.
#   * `[ "$NEW_COUNT" -gt 0 ]` errors with "integer expression expected".
# These cascade and corrupt the iteration for every issue under watch.
#
# Fix: gate the comments-diff jq behind `jq -e 'type == "array"'` and
# defensively coerce NEW_COUNT to 0 when non-numeric.
#
# Strategy: PATH-prepend stub `gh` and `grava` binaries that we control per
# test, run the watcher inside a sandbox, capture stderr/stdout, and assert
# on observable outcomes:
#   * "skipping comment check" log line appears for non-array gh output.
#   * No "integer expression expected" from bash `[ ... -gt ]`.
#   * No "Cannot index string with string" from jq.
#   * For valid empty/populated arrays, behaviour is unchanged.
#
# Usage:
#   ./scripts/test/test-watcher-comments.sh
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
# Each test gets a fresh sandbox with stub binaries. The stubs read fixture
# files from $SANDBOX/fixtures/ to decide what to print, keyed by the
# subcommand pattern. That keeps the stubs declarative and per-test data
# trivially overridable.
#
# Stub responsibilities:
#   * grava list --label pr-created --json  -> single-issue array
#   * grava wisp read <id> pr_number        -> "42"
#   * grava wisp read <id> pr_url           -> "http://example/pr/42"
#   * grava wisp read <id> ...              -> empty (forces fallback paths)
#   * grava wisp write ...                  -> success, log to wisps.log
#   * grava label / commit / show / signal  -> success, no-op
#   * gh pr view ... state                  -> "OPEN" (so we reach comments)
#   * gh pr view ... reviewDecision         -> "APPROVED"
#   * gh api .../comments                   -> contents of $SANDBOX/fixtures/gh-comments.out
#                                              with optional stderr from .err and exit code from .rc

setup_sandbox() {
  SANDBOX=$(mktemp -d)
  mkdir -p "$SANDBOX/.grava" "$SANDBOX/stubs" "$SANDBOX/fixtures"

  # Default fixture: empty array (well-formed, no work to do).
  printf '%s\n' '[]' > "$SANDBOX/fixtures/gh-comments.out"
  : > "$SANDBOX/fixtures/gh-comments.err"
  echo 0 > "$SANDBOX/fixtures/gh-comments.rc"

  # ── grava stub ────────────────────────────────────────────────────────
  # Routes by first arg. wisp read returns empty for everything except the
  # two keys we want to seed (pr_number, pr_url) so the watcher takes the
  # default-fallback paths the bug exercises.
  cat >"$SANDBOX/stubs/grava" <<'STUB'
#!/usr/bin/env bash
SANDBOX_ROOT="${SANDBOX_ROOT:-$(pwd)}"
case "$1" in
  list)
    # Emit one in-flight PR-created issue.
    echo '[{"id":"grava-test01"}]'
    ;;
  wisp)
    sub="$2"; id="$3"; key="$4"
    case "$sub" in
      read)
        case "$key" in
          pr_number) echo "42" ;;
          pr_url)    echo "http://example/pr/42" ;;
          *)         : ;; # empty stdout, exit 0 — matches real CLI for missing wisp
        esac
        ;;
      write)
        # Log writes to a file for assertions.
        echo "$id $key=$5" >> "$SANDBOX_ROOT/wisps.log"
        ;;
    esac
    ;;
  show|label|commit|signal|update|comment|close)
    : # success no-op
    ;;
  *)
    : # default success
    ;;
esac
exit 0
STUB

  # ── gh stub ───────────────────────────────────────────────────────────
  # We only care about three call-shapes:
  #   * gh pr view N --json state ...
  #   * gh pr view N --json reviewDecision ...
  #   * gh api repos/.../comments
  # Everything else returns empty success.
  cat >"$SANDBOX/stubs/gh" <<'STUB'
#!/usr/bin/env bash
SANDBOX_ROOT="${SANDBOX_ROOT:-$(pwd)}"

# Walk argv looking for distinguishing tokens.
mode="other"
for a in "$@"; do
  case "$a" in
    api)              mode="api" ;;
    state)            [ "$mode" = "other" ] && mode="state" ;;
    reviewDecision)   mode="reviewDecision" ;;
  esac
done

case "$mode" in
  state)
    # Force OPEN so the watcher continues to the comments block — that's
    # the slice we're testing.
    echo "OPEN"
    ;;
  reviewDecision)
    echo "APPROVED"
    ;;
  api)
    # Fixture-driven response.
    cat "$SANDBOX_ROOT/fixtures/gh-comments.out"
    if [ -s "$SANDBOX_ROOT/fixtures/gh-comments.err" ]; then
      cat "$SANDBOX_ROOT/fixtures/gh-comments.err" >&2
    fi
    exit "$(cat "$SANDBOX_ROOT/fixtures/gh-comments.rc")"
    ;;
  *)
    : # success, empty
    ;;
esac
exit 0
STUB

  chmod +x "$SANDBOX/stubs/grava" "$SANDBOX/stubs/gh"

  PATH="$SANDBOX/stubs:$PATH"
  export PATH
  export CLAUDE_PROJECT_DIR="$SANDBOX"
  export SANDBOX_ROOT="$SANDBOX"
}

teardown_sandbox() {
  [ -n "${SANDBOX:-}" ] && [ -d "$SANDBOX" ] && rm -rf "$SANDBOX"
  unset SANDBOX SANDBOX_ROOT CLAUDE_PROJECT_DIR
}

# Configure the gh-api response for the next watcher run.
set_gh_comments_response() {
  printf '%s' "$1" > "$SANDBOX/fixtures/gh-comments.out"
  printf '%s' "${2:-}" > "$SANDBOX/fixtures/gh-comments.err"
  echo "${3:-0}"          > "$SANDBOX/fixtures/gh-comments.rc"
}

# Run the watcher, capture stdout+stderr separately. We invoke bash with
# `-x` disabled but capture stderr fully so we can grep for the diagnostic
# log line and for any bash arithmetic error noise.
run_watcher() {
  ( cd "$SANDBOX" && bash "$WATCHER" ) >"$SANDBOX/stdout" 2>"$SANDBOX/stderr"
  echo $?
}

stderr_contains() {
  grep -q -- "$1" "$SANDBOX/stderr"
}
stdout_contains() {
  grep -q -- "$1" "$SANDBOX/stdout"
}
combined_contains() {
  grep -q -- "$1" "$SANDBOX/stdout" "$SANDBOX/stderr" 2>/dev/null
}

# A successful, defended watcher run must NEVER produce these two strings,
# regardless of input — they are the smoking-gun symptoms of the bug.
assert_no_smoking_gun() {
  local label="$1"
  if combined_contains "Cannot index string with string"; then
    assert_pass "$label: no jq 'Cannot index string' error" 0 \
      "found in output: $(grep -m1 'Cannot index' "$SANDBOX/stdout" "$SANDBOX/stderr" 2>/dev/null)"
    return
  fi
  if combined_contains "integer expression expected"; then
    assert_pass "$label: no bash 'integer expression expected'" 0 \
      "found in output: $(grep -m1 'integer expression' "$SANDBOX/stdout" "$SANDBOX/stderr" 2>/dev/null)"
    return
  fi
  assert_pass "$label: no smoking-gun crash markers in output" 1
}

# ─── Test 1: valid empty array ────────────────────────────────────────────
test_valid_empty_array() {
  echo "test 1: gh returns valid [] => watcher proceeds, no errors, no pr_new_comments wisp"
  setup_sandbox
  set_gh_comments_response '[]' '' 0

  run_watcher >/dev/null
  assert_no_smoking_gun "empty-array"

  if [ -f "$SANDBOX/wisps.log" ] && grep -q "pr_new_comments" "$SANDBOX/wisps.log"; then
    assert_pass "empty-array: pr_new_comments wisp NOT written" 0 \
      "wisps.log: $(cat "$SANDBOX/wisps.log")"
  else
    assert_pass "empty-array: pr_new_comments wisp NOT written" 1
  fi

  teardown_sandbox
}

# ─── Test 2: valid populated array ────────────────────────────────────────
test_valid_populated_array() {
  echo "test 2: gh returns array with one new comment => pr_new_comments wisp written"
  setup_sandbox
  set_gh_comments_response \
    '[{"id":1001,"in_reply_to_id":null,"body":"please rename"}]' '' 0

  run_watcher >/dev/null
  assert_no_smoking_gun "populated-array"

  if [ -f "$SANDBOX/wisps.log" ] && grep -q "pr_new_comments" "$SANDBOX/wisps.log"; then
    assert_pass "populated-array: pr_new_comments wisp written" 1
  else
    assert_pass "populated-array: pr_new_comments wisp written" 0 \
      "wisps.log: $(cat "$SANDBOX/wisps.log" 2>/dev/null || echo MISSING)"
  fi

  teardown_sandbox
}

# ─── Test 3: gh returns error string on stdout ────────────────────────────
# Some gh failure modes (rate limit, auth blip) print an error string to
# stdout instead of valid JSON. The watcher must detect and skip.
test_gh_error_string() {
  echo "test 3: gh prints error string => watcher logs skip + no smoking-gun crash"
  setup_sandbox
  set_gh_comments_response 'gh: API rate limit exceeded' '' 0

  run_watcher >/dev/null
  assert_no_smoking_gun "error-string"

  if combined_contains "skipping comment check"; then
    assert_pass "error-string: 'skipping comment check' log line emitted" 1
  else
    assert_pass "error-string: 'skipping comment check' log line emitted" 0 \
      "stderr: $(cat "$SANDBOX/stderr"); stdout: $(cat "$SANDBOX/stdout")"
  fi

  teardown_sandbox
}

# ─── Test 4: gh returns empty stdout ──────────────────────────────────────
test_gh_empty_stdout() {
  echo "test 4: gh returns empty stdout => watcher logs skip + no smoking-gun crash"
  setup_sandbox
  set_gh_comments_response '' '' 0

  run_watcher >/dev/null
  assert_no_smoking_gun "empty-stdout"

  if combined_contains "skipping comment check"; then
    assert_pass "empty-stdout: 'skipping comment check' log line emitted" 1
  else
    assert_pass "empty-stdout: 'skipping comment check' log line emitted" 0 \
      "stderr: $(cat "$SANDBOX/stderr"); stdout: $(cat "$SANDBOX/stdout")"
  fi

  teardown_sandbox
}

# ─── Test 5: gh returns object (valid JSON, wrong shape) ──────────────────
test_gh_object_not_array() {
  echo "test 5: gh returns valid JSON object => watcher logs skip + no smoking-gun crash"
  setup_sandbox
  set_gh_comments_response '{"error":"x"}' '' 0

  run_watcher >/dev/null
  assert_no_smoking_gun "object-not-array"

  if combined_contains "skipping comment check"; then
    assert_pass "object-not-array: 'skipping comment check' log line emitted" 1
  else
    assert_pass "object-not-array: 'skipping comment check' log line emitted" 0 \
      "stderr: $(cat "$SANDBOX/stderr"); stdout: $(cat "$SANDBOX/stdout")"
  fi

  teardown_sandbox
}

# ─── Test 6: gh returns malformed JSON ────────────────────────────────────
test_gh_malformed_json() {
  echo "test 6: gh returns 'not json at all' => watcher logs skip + no smoking-gun crash"
  setup_sandbox
  set_gh_comments_response 'not json at all' '' 0

  run_watcher >/dev/null
  assert_no_smoking_gun "malformed-json"

  if combined_contains "skipping comment check"; then
    assert_pass "malformed-json: 'skipping comment check' log line emitted" 1
  else
    assert_pass "malformed-json: 'skipping comment check' log line emitted" 0 \
      "stderr: $(cat "$SANDBOX/stderr"); stdout: $(cat "$SANDBOX/stdout")"
  fi

  teardown_sandbox
}

# ─── Run ─────────────────────────────────────────────────────────────────
test_valid_empty_array
test_valid_populated_array
test_gh_error_string
test_gh_empty_stdout
test_gh_object_not_array
test_gh_malformed_json

echo
echo "Results: $PASS passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
  echo "Failures:"
  for d in "${FAIL_DETAILS[@]}"; do echo "  - $d"; done
  exit 1
fi
exit 0
