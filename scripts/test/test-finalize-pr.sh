#!/usr/bin/env bash
# Bash smoke tests for scripts/agent-bot/finalize-pr.sh.
#
# Usage:
#   ./scripts/test/test-finalize-pr.sh
#
# Exits 0 on all-pass, non-zero on any failure.
#
# Strategy: inject a mock `grava` via GRAVA_BIN. The mock logs every call
# to a transcript file the test then asserts against. State (wisps, label)
# is held in environment-keyed files so step 7 self-verification reads
# what step 1-3 wrote, exercising the full flow including the precondition
# check in `grava signal PR_CREATED` (mocked, but ordering is real).
#
# These tests run without Dolt — mock-only — so they are safe to invoke
# in CI without bootstrapping a database.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
FINALIZE_SH="$REPO_ROOT/scripts/agent-bot/finalize-pr.sh"

if [ ! -x "$FINALIZE_SH" ]; then
  echo "FAIL: $FINALIZE_SH missing or not executable" >&2
  exit 1
fi

PASS=0
FAIL=0
FAILED_NAMES=()

assert_eq() {
  local desc="$1"; local want="$2"; local got="$3"
  if [ "$want" = "$got" ]; then
    PASS=$((PASS + 1))
    printf '  ✅ %s\n' "$desc"
  else
    FAIL=$((FAIL + 1))
    FAILED_NAMES+=("$desc")
    printf '  ❌ %s\n     want: %s\n     got : %s\n' "$desc" "$want" "$got"
  fi
}

assert_contains() {
  local desc="$1"; local needle="$2"; local haystack="$3"
  if printf '%s' "$haystack" | grep -qF "$needle"; then
    PASS=$((PASS + 1))
    printf '  ✅ %s\n' "$desc"
  else
    FAIL=$((FAIL + 1))
    FAILED_NAMES+=("$desc")
    printf '  ❌ %s\n     missing: %s\n     got    : %s\n' "$desc" "$needle" "$haystack"
  fi
}

# ─── Mock grava factory ────────────────────────────────────────────────────
#
# Writes a fake `grava` shim to $1/grava that logs every call to
# $TRANSCRIPT and stores wisps/labels under $STATE_DIR. The mock
# enforces the PR_CREATED precondition the same way the real CLI does
# so we exercise step ordering end-to-end.
make_mock_grava() {
  local bindir="$1"
  cat >"$bindir/grava" <<'MOCK'
#!/usr/bin/env bash
set -uo pipefail
echo "$(printf '%s ' "$@")" >> "$TRANSCRIPT"

case "$1" in
  wisp)
    sub="$2"; issue="$3"; key="${4:-}"; value="${5:-}"
    if [ "$sub" = "write" ]; then
      printf '%s' "$value" > "$STATE_DIR/${issue}.${key}"
      exit 0
    fi
    if [ "$sub" = "read" ]; then
      f="$STATE_DIR/${issue}.${key}"
      if [ -f "$f" ]; then
        cat "$f"
        exit 0
      fi
      exit 1
    fi
    ;;
  signal)
    kind="$2"; shift 2
    issue=""; payload=""; actor=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --issue)   issue="$2"; shift 2 ;;
        --payload) payload="$2"; shift 2 ;;
        --actor)   actor="$2"; shift 2 ;;
        *)         shift ;;
      esac
    done
    if [ "$kind" = "PR_CREATED" ]; then
      # Enforce precondition like the real CLI.
      missing=""
      [ -f "$STATE_DIR/${issue}.pr_number" ] || missing="$missing pr_number"
      [ -f "$STATE_DIR/${issue}.pr_awaiting_merge_since" ] || missing="$missing pr_awaiting_merge_since"
      if [ -n "$missing" ]; then
        echo "SIGNAL_PRECONDITION_UNMET: missing$missing" >&2
        exit 1
      fi
      printf '%s' "pr_created" > "$STATE_DIR/${issue}.pipeline_phase"
      printf '%s' "$payload"   > "$STATE_DIR/${issue}.pr_url"
    fi
    exit 0
    ;;
  label)
    issue="$2"
    # Crude parse: --add <label>
    while [ $# -gt 0 ]; do
      [ "$1" = "--add" ] && { echo "$2" >> "$STATE_DIR/${issue}.labels"; shift 2; continue; }
      shift
    done
    exit 0
    ;;
  show)
    issue="$2"
    if [ -f "$STATE_DIR/${issue}.labels" ]; then
      labels_json=$(awk 'BEGIN{printf"["} {if(NR>1)printf",";printf"\"%s\"",$0} END{printf"]"}' "$STATE_DIR/${issue}.labels")
    else
      labels_json="[]"
    fi
    printf '{"id":"%s","labels":%s}\n' "$issue" "$labels_json"
    exit 0
    ;;
  commit)
    exit 0
    ;;
esac
exit 0
MOCK
  chmod +x "$bindir/grava"
}

# ─── Test cases ────────────────────────────────────────────────────────────

run_case() {
  local name="$1"; shift
  printf '%s\n' "── $name"
  TMP="$(mktemp -d -t finalize-pr-test-XXXXXX)"
  export STATE_DIR="$TMP/state"; mkdir -p "$STATE_DIR"
  export TRANSCRIPT="$TMP/transcript.log"; : > "$TRANSCRIPT"
  local bindir="$TMP/bin"; mkdir -p "$bindir"
  make_mock_grava "$bindir"
  export GRAVA_BIN="$bindir/grava"
  # Make `jq` available for `grava show --json | jq` inside finalize-pr.sh.
  if ! command -v jq >/dev/null 2>&1; then
    echo "SKIP $name: jq not on PATH"
    return
  fi
  "$@"
  rm -rf "$TMP"
}

# Case 1 — happy path: all 7 steps in order, exit 0, transcript shows
# the precondition wisps land BEFORE the signal.
case_happy_path() {
  local issue="grava-test-1"
  local pr_num="42"
  local url="https://github.com/x/y/pull/42"
  local out rc
  out=$("$FINALIZE_SH" "$issue" "$pr_num" "$url" 2>&1); rc=$?
  assert_eq "exit code 0" "0" "$rc"
  assert_contains "summary printed" "✅ $issue PR #$pr_num finalized" "$out"

  # Verify ordering: pr_number, pr_url, pr_awaiting_merge_since BEFORE signal.
  local transcript; transcript=$(cat "$TRANSCRIPT")
  local order; order=$(awk '
    /^wisp write [^ ]+ pr_number / { if(!a)a=NR }
    /^wisp write [^ ]+ pr_url /     { if(!b)b=NR }
    /^wisp write [^ ]+ pr_awaiting_merge_since / { if(!c)c=NR }
    /^signal PR_CREATED / { if(!d)d=NR }
    /^label [^ ]+ --add pr-created/ { if(!e)e=NR }
    /^commit / { if(!f)f=NR }
    END { print a"|"b"|"c"|"d"|"e"|"f }
  ' "$TRANSCRIPT")
  IFS='|' read -r a b c d e f <<<"$order"
  # All 6 step lines must appear AND in monotonically increasing order.
  if [ -n "$a" ] && [ -n "$b" ] && [ -n "$c" ] && [ -n "$d" ] && [ -n "$e" ] && [ -n "$f" ] \
     && [ "$a" -lt "$d" ] && [ "$b" -lt "$d" ] && [ "$c" -lt "$d" ] \
     && [ "$d" -lt "$e" ] && [ "$e" -lt "$f" ]; then
    PASS=$((PASS + 1)); printf '  ✅ step ordering correct (1-3 before 4, 4 before 5 before 6)\n'
  else
    FAIL=$((FAIL + 1)); FAILED_NAMES+=("step ordering")
    printf '  ❌ step ordering wrong\n     transcript:\n%s\n' "$transcript"
  fi
}
run_case "happy path: 7 steps in correct order" case_happy_path

# Case 2 — usage error when args missing.
case_usage_error() {
  local out rc
  out=$("$FINALIZE_SH" only-one-arg 2>&1); rc=$?
  assert_eq "exit code 2 on bad usage" "2" "$rc"
  assert_contains "usage hint printed" "Usage:" "$out"
}
run_case "usage error on wrong arg count" case_usage_error

# Case 3 — fail-fast: step 1 grava failure exits 1 with step number.
case_step1_failure() {
  # Replace mock so wisp write returns error.
  cat >"$STATE_DIR/../bin/grava" <<'BAD'
#!/usr/bin/env bash
echo "fake-failure" >&2
exit 7
BAD
  chmod +x "$STATE_DIR/../bin/grava"
  local out rc
  out=$("$FINALIZE_SH" grava-test-2 99 "https://x/y/pull/99" 2>&1); rc=$?
  assert_eq "exit code 1 on step 1 failure" "1" "$rc"
  assert_contains "step 1 message" "step 1" "$out"
}
run_case "step 1 failure exits 1 with step number" case_step1_failure

# ─── Summary ───────────────────────────────────────────────────────────────

printf '\n── Summary: %d passed, %d failed\n' "$PASS" "$FAIL"
if [ "$FAIL" -gt 0 ]; then
  printf 'Failed:\n'
  for n in "${FAILED_NAMES[@]}"; do printf '  - %s\n' "$n"; done
  exit 1
fi
exit 0
