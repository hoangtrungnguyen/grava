#!/usr/bin/env bash
# Bash unit tests for the /ship skill's signal-resolution helpers.
#
# Tests both the new wisp-first read_signal_state and the legacy
# last_line/parse_signal fallback path that's gated behind
# SHIP_LEGACY_PARSER=1.
#
# Usage:
#   ./scripts/test/test-ship-helpers.sh
#
# Exit 0 on all-pass, non-zero on any failure.
#
# These helpers are duplicated here from .claude/skills/ship/SKILL.md.
# When changing them in the skill, update them here too — and a CI step
# can `diff` the two to surface drift.

set -uo pipefail

# ─── Helpers under test (mirror .claude/skills/ship/SKILL.md §Helpers) ───

last_line() {
  printf '%s' "$1" | awk 'NF{l=$0} END{print l}'
}

parse_signal() {
  local line="$1"
  case "$line" in
    "CODER_DONE: "*)            echo "CODER_DONE|${line#CODER_DONE: }" ;;
    "CODER_HALTED: "*)          echo "CODER_HALTED|${line#CODER_HALTED: }" ;;
    "REVIEWER_APPROVED")        echo "REVIEWER_APPROVED|" ;;
    "REVIEWER_BLOCKED: "*)      echo "REVIEWER_BLOCKED|${line#REVIEWER_BLOCKED: }" ;;
    "REVIEWER_BLOCKED")         echo "REVIEWER_BLOCKED|" ;;
    "PR_CREATED: "*)            echo "PR_CREATED|${line#PR_CREATED: }" ;;
    "PR_FAILED: "*)             echo "PR_FAILED|${line#PR_FAILED: }" ;;
    *)                          echo "INVALID|no signal in last line" ;;
  esac
}

# Mock grava: read fixtures from $TEST_FIXTURES_DIR rather than the live DB.
# This isolates these tests from Dolt and lets us deterministically vary state.
grava() {
  local sub="$1"; shift
  case "$sub" in
    wisp)
      local op="$1"; local issue="$2"; local key="${3:-}"
      [ "$op" = "read" ] || return 0
      local f="${TEST_FIXTURES_DIR}/${issue}.${key}"
      [ -f "$f" ] && cat "$f" || return 1
      ;;
    show)
      local issue="$1"
      local f="${TEST_FIXTURES_DIR}/${issue}.show.json"
      [ -f "$f" ] && cat "$f" || return 1
      ;;
    *)
      return 0
      ;;
  esac
}

read_signal_state() {
  local agent_result="$1"
  local issue_id="$2"
  local class="$3"

  if [ "${SHIP_LEGACY_PARSER:-0}" = "1" ]; then
    parse_signal "$(last_line "$agent_result")"
    return
  fi

  local phase
  phase=$(grava wisp read "$issue_id" pipeline_phase 2>/dev/null)

  case "$class:$phase" in
    coder:coding_complete)
      local sha
      sha=$(grava show "$issue_id" --json 2>/dev/null | jq -r '.last_commit // ""')
      echo "CODER_DONE|$sha"
      return
      ;;
    coder:coding_halted)
      local reason
      reason=$(grava wisp read "$issue_id" coder_halted 2>/dev/null)
      echo "CODER_HALTED|$reason"
      return
      ;;
    reviewer:review_approved)
      echo "REVIEWER_APPROVED|"
      return
      ;;
    reviewer:review_blocked)
      local findings
      findings=$(grava wisp read "$issue_id" reviewer_findings 2>/dev/null)
      echo "REVIEWER_BLOCKED|$findings"
      return
      ;;
    pr:pr_created|pr:pr_awaiting_merge)
      local url
      url=$(grava wisp read "$issue_id" pr_url 2>/dev/null)
      echo "PR_CREATED|$url"
      return
      ;;
    pr:failed)
      local reason
      reason=$(grava wisp read "$issue_id" pr_failed_reason 2>/dev/null)
      echo "PR_FAILED|$reason"
      return
      ;;
  esac

  echo "PIPELINE_INFO: read_signal_state — pipeline_phase=${phase:-<unset>} did not match class=$class; falling back to legacy parser" >&2
  parse_signal "$(last_line "$agent_result")"
}

# ─── Fixture setup ─────────────────────────────────────────────────────────

TEST_FIXTURES_DIR=$(mktemp -d -t ship-helpers-test-XXXXXX)
trap 'rm -rf "$TEST_FIXTURES_DIR"' EXIT

write_fixture() {
  printf '%s' "$2" > "${TEST_FIXTURES_DIR}/$1"
}

# ─── Test runner ───────────────────────────────────────────────────────────

PASS=0
FAIL=0
FAILED_NAMES=()

assert_eq() {
  local name="$1" expected="$2" actual="$3"
  if [ "$expected" = "$actual" ]; then
    PASS=$((PASS + 1))
    printf '  ✅ %s\n' "$name"
  else
    FAIL=$((FAIL + 1))
    FAILED_NAMES+=("$name")
    printf '  ❌ %s\n' "$name"
    printf '     expected: %q\n' "$expected"
    printf '     actual:   %q\n' "$actual"
  fi
}

# ─── parse_signal: legacy text-line parser ─────────────────────────────────

echo "Group: parse_signal"
assert_eq "CODER_DONE with payload" "CODER_DONE|deadbeef" "$(parse_signal 'CODER_DONE: deadbeef')"
assert_eq "CODER_HALTED with reason" "CODER_HALTED|no spec" "$(parse_signal 'CODER_HALTED: no spec')"
assert_eq "REVIEWER_APPROVED bare" "REVIEWER_APPROVED|" "$(parse_signal 'REVIEWER_APPROVED')"
assert_eq "REVIEWER_BLOCKED with findings" "REVIEWER_BLOCKED|HIGH: oops" "$(parse_signal 'REVIEWER_BLOCKED: HIGH: oops')"
assert_eq "PR_CREATED with url" "PR_CREATED|https://x/y/1" "$(parse_signal 'PR_CREATED: https://x/y/1')"
assert_eq "Unknown signal" "INVALID|no signal in last line" "$(parse_signal 'random text')"

# ─── last_line: takes last non-empty line ──────────────────────────────────

echo "Group: last_line"
assert_eq "single line" "hello" "$(last_line 'hello')"
assert_eq "trailing newline" "hello" "$(last_line 'hello
')"
assert_eq "trailing blank lines" "hello" "$(last_line 'hello

')"
assert_eq "multi-line picks last" "world" "$(last_line 'hello
world')"
assert_eq "blank between picks last" "world" "$(last_line 'hello

world')"

# ─── read_signal_state: wisp-first path ────────────────────────────────────

echo "Group: read_signal_state (wisp-first)"

# Coder happy path — pipeline_phase=coding_complete, last_commit in show.json
write_fixture "ID-A.pipeline_phase" "coding_complete"
write_fixture "ID-A.show.json" '{"id":"ID-A","last_commit":"abc123"}'
unset SHIP_LEGACY_PARSER
assert_eq "coder coding_complete → CODER_DONE|sha" \
  "CODER_DONE|abc123" \
  "$(read_signal_state '' ID-A coder)"

# Coder halt path
write_fixture "ID-B.pipeline_phase" "coding_halted"
write_fixture "ID-B.coder_halted" "no spec found"
assert_eq "coder coding_halted → CODER_HALTED|reason" \
  "CODER_HALTED|no spec found" \
  "$(read_signal_state '' ID-B coder)"

# Reviewer approved
write_fixture "ID-C.pipeline_phase" "review_approved"
assert_eq "reviewer review_approved → REVIEWER_APPROVED|" \
  "REVIEWER_APPROVED|" \
  "$(read_signal_state '' ID-C reviewer)"

# Reviewer blocked with findings
write_fixture "ID-D.pipeline_phase" "review_blocked"
write_fixture "ID-D.reviewer_findings" "[HIGH] sql injection in handler"
assert_eq "reviewer review_blocked → REVIEWER_BLOCKED|findings" \
  "REVIEWER_BLOCKED|[HIGH] sql injection in handler" \
  "$(read_signal_state '' ID-D reviewer)"

# PR created
write_fixture "ID-E.pipeline_phase" "pr_created"
write_fixture "ID-E.pr_url" "https://github.com/x/y/pull/42"
assert_eq "pr pr_created → PR_CREATED|url" \
  "PR_CREATED|https://github.com/x/y/pull/42" \
  "$(read_signal_state '' ID-E pr)"

# PR awaiting merge (same handler)
write_fixture "ID-F.pipeline_phase" "pr_awaiting_merge"
write_fixture "ID-F.pr_url" "https://github.com/x/y/pull/43"
assert_eq "pr pr_awaiting_merge → PR_CREATED|url" \
  "PR_CREATED|https://github.com/x/y/pull/43" \
  "$(read_signal_state '' ID-F pr)"

# PR failed
write_fixture "ID-G.pipeline_phase" "failed"
write_fixture "ID-G.pr_failed_reason" "git push rejected"
assert_eq "pr failed → PR_FAILED|reason" \
  "PR_FAILED|git push rejected" \
  "$(read_signal_state '' ID-G pr)"

# ─── read_signal_state: fallback when wisp absent ──────────────────────────

echo "Group: read_signal_state (fallback when wisp absent)"

# No pipeline_phase fixture for ID-H — falls back to last_line(agent_result)
assert_eq "wisp unset → falls back to text last_line" \
  "CODER_DONE|deadbeef" \
  "$(read_signal_state 'some preamble
CODER_DONE: deadbeef' ID-H coder 2>/dev/null)"

# Class mismatch — pipeline_phase exists but doesn't match the requested class.
# Should fall back too (e.g. caller asks for "pr" but state is review_approved).
write_fixture "ID-I.pipeline_phase" "review_approved"
assert_eq "class mismatch → falls back to text last_line" \
  "PR_CREATED|https://example.com/pr/1" \
  "$(read_signal_state 'PR_CREATED: https://example.com/pr/1' ID-I pr 2>/dev/null)"

# ─── read_signal_state: SHIP_LEGACY_PARSER=1 forces fallback ───────────────

echo "Group: SHIP_LEGACY_PARSER=1 regression flag"

# Even with a perfectly good wisp, the env var forces text parsing.
write_fixture "ID-J.pipeline_phase" "coding_complete"
write_fixture "ID-J.show.json" '{"id":"ID-J","last_commit":"xyz999"}'
SHIP_LEGACY_PARSER=1 assert_eq "flag=1 forces parse_signal even with valid wisp" \
  "CODER_DONE|legacysha" \
  "$(SHIP_LEGACY_PARSER=1 read_signal_state 'CODER_DONE: legacysha' ID-J coder)"

# Without the flag, same fixture uses the wisp path.
unset SHIP_LEGACY_PARSER
assert_eq "flag unset → uses wisp (sanity check)" \
  "CODER_DONE|xyz999" \
  "$(read_signal_state 'CODER_DONE: legacysha' ID-J coder)"

# ─── Summary ────────────────────────────────────────────────────────────────

echo
echo "─────────────────────────────"
echo "  Passed: $PASS"
echo "  Failed: $FAIL"
if [ "$FAIL" -gt 0 ]; then
  echo "  Failures:"
  for n in "${FAILED_NAMES[@]}"; do
    echo "    - $n"
  done
  exit 1
fi
echo "  All ship helper tests passed."
exit 0
