# Story 2B.16: `/ship` Pipeline Test Suite

Dedicated test harness for the `/ship` skill (story 2B.5). `/ship` is the multi-agent pipeline orchestrator — coder → reviewer → pr-creator, plus discover, retry, and re-entry paths. Live runs spawn real Agent subprocesses, mutate Dolt, and call `gh`; that surface is too expensive and non-deterministic to validate by smoke alone (README §Implementation Order steps 9–16 cover the happy path but skip negative branches and contention).

This story builds a deterministic test layer that exercises every phase, every signal, and every recovery branch with **stub agents** and a **disposable Dolt fixture**. The suite is the regression net for stories 2B.5, 2B.9, 2B.12, 2B.14 and any future change to the pipeline contract.

## Test environment

The suite lives in the **grava sandbox repo** at `/Users/trungnguyenhoang/IdeaProjects/gravav6-sandbox` (separate from the main grava repo, already used for Phase 2 multi-branch orchestration scenarios — see `sandbox/README.md` there). Rationale: the sandbox repo already has the harness pattern (subprocess-driving the `grava` binary, throwaway data dirs, scenario fixtures); reusing it keeps "grava integration tests" in one place instead of growing a parallel `tests/` tree inside the main repo. Existing sandbox tests are pytest-based (`test_*.py` + `grava_test_utils.py`) — `/ship` tests adopt the same convention.

## Files (in `gravav6-sandbox/sandbox/ship/`)

- `sandbox/ship/` — pytest test root
  - `test_helpers.py` — unit tests for `last_line`, `parse_signal`, flag parsing (the helper bash blocks are extracted into `lib/ship_helpers.sh` so pytest can `subprocess.run` them in isolation)
  - `test_phase0_discover.py` — discover + precondition gate
  - `test_phase1_code.py` — coder spawn, signal parsing, halt branches
  - `test_phase2_review.py` — review loop (rounds 1–3, exhaustion)
  - `test_phase3_pr.py` — pr-creator delegation
  - `test_phase4_resume.py` — re-entry on `pr_new_comments`, off-scope detection, CI wait
  - `test_phase5_retry.py` — `--retry`, `--retry --rebase-only`, retry cap
  - `test_reentry.py` — `pipeline_phase` switch (claimed / pr_awaiting_merge / failed / complete)
  - `test_negative.py` — Fix 1 (last-line), Fix 8 (forward-only), malformed signals
  - `test_contention.py` — N parallel `/ship` invocations on shared backlog
  - `conftest.py` — pytest fixtures: `dolt_fixture`, `stubbed_agents`, `ship_runner` (subprocess wrapper), `wisp` (read/assert helper)
- `sandbox/ship/lib/`
  - `stub_agent.sh` — fake `Agent` callable; emits canned `<NAME>: <tail>` from `$SHIP_TEST_FIXTURE`
  - `gh_stub.sh` — fake `gh` for `auth status` and `pr checks`
  - `ship_helpers.sh` — extracted from `/ship/SKILL.md` Setup + Helper sections so the helpers are testable without running the full skill
  - `with_stubbed_path.sh` — prepends `lib/` to `$PATH` so `Agent` and `gh` resolve to stubs
- `sandbox/ship/fixtures/`
  - `signals/` — canned agent outputs (`coder-done.txt`, `coder-halted-spec.txt`, `reviewer-blocked-2kb.txt`, …)
  - `issues/` — seed JSON for various preconditions (no-desc, no-AC, has-code-review-label, healthy-task)
  - `pr-comments/` — sample `pr_new_comments` payloads (in-scope, off-scope, mixed)
  - `dolt-seed/` — minimal Dolt schema + seed rows (reuses `gravav6-sandbox/sandbox/scenarios/fixtures/` patterns)
- `sandbox/ship/Makefile` — `make test-ship` target (`pytest sandbox/ship/ -v`)
- `gravav6-sandbox/.github/workflows/ship-tests.yml` — CI job in the **sandbox repo**; triggers on push or scheduled, with a manual `workflow_dispatch` that takes a grava commit SHA so the suite can be run against any main-repo state

> **Why the sandbox repo and not `tests/` in main grava.** Three reasons. (1) The sandbox already pins specific `grava` binary builds for testing — natural fit for `/ship` since the skill calls `grava` subprocess. (2) The sandbox CI is independently scheduled and isn't gated on main-repo CI runtime; this keeps the main-repo PR critical path short. (3) Tests of skill bodies are inherently integration tests (Dolt + bash + Agent stubs) — co-locating with the existing scenario harness avoids duplicate fixture infra.

## Test framework

**pytest** (matches the existing `gravav6-sandbox/sandbox/test_*.py` suite). Rationale:

- The sandbox already exercises the `grava` binary via Python subprocess — the same `grava_test_utils.py` patterns (`spawn_dolt`, `seed_issue`, `wait_for_signal`) are reused for `/ship` tests with minimal new infra.
- `/ship/SKILL.md` is bash, but the test boundary is "shell out to a wrapper that sources the skill body" — a polyglot harness is unavoidable. Python wins on subprocess ergonomics, JSON-handling for wisp/issue assertions, and parallelism (`pytest-xdist` for the contention test).
- Speed budget: full suite under 90s on CI (slightly higher than a pure-bash bats budget; pytest startup + Dolt spin-up is the cost). Still fast enough to gate every `/ship` change.

Install via the sandbox's existing `requirements.txt` (`pytest`, `pytest-xdist`); CI uses `setup-python@v5` + `pip install -r requirements.txt`.

## Stub agent contract

`Agent` calls inside `/ship` are the only non-deterministic surface. The stub replaces the tool with a script that:

1. Reads `$SHIP_TEST_FIXTURE` (path to canned output file).
2. Optionally honors `$SHIP_TEST_FIXTURE_<PHASE>` overrides per phase (e.g. `SHIP_TEST_FIXTURE_PHASE1=coder-done.txt`, `SHIP_TEST_FIXTURE_PHASE2=reviewer-blocked.txt`).
3. Emits the file's contents to stdout verbatim.
4. Exits 0.

```bash
# sandbox/ship/lib/stub_agent.sh
#!/bin/bash
# Usage: invoked as `Agent` by /ship. Reads canned output from a fixture file.
PHASE_ENV="SHIP_TEST_FIXTURE_${SHIP_PHASE:-DEFAULT}"
FIXTURE="${!PHASE_ENV:-${SHIP_TEST_FIXTURE:?stub-agent: no fixture set}}"
[ -f "$FIXTURE" ] || { echo "stub-agent: missing fixture $FIXTURE" >&2; exit 1; }
cat "$FIXTURE"
```

A pytest fixture (`stubbed_agents` in `conftest.py`) prepends `sandbox/ship/lib/` to `$PATH` and exports the `SHIP_TEST_FIXTURE_*` env vars from the test parameters, then yields a `ship_runner` callable that invokes the extracted skill body via `subprocess.run`. The skill's bash blocks must call `Agent ...` (already true in 2B.5) — no source-code change needed for the stub to take effect.

> **Note on Agent invocation in 2B.5.** The skill currently shows `Agent({ ... })` pseudo-code (e.g. L274–281). For tests to bind to a stub, those calls must run through a shell-resolvable `Agent` command (or a wrapper function defined in the skill). Story 2B.5 needs a one-line addendum: "Agent invocations are made via the `Agent` shell command, not inline JSON" — so the stub injection works without forking the skill body.

## Dolt fixture lifecycle

```python
# sandbox/ship/conftest.py
import pytest, subprocess, tempfile, os, re, signal, time
from pathlib import Path

@pytest.fixture(scope="module")
def dolt_fixture():
    tmpdir = Path(tempfile.mkdtemp(prefix="ship-test-"))
    seed = Path(__file__).parent / "fixtures" / "dolt-seed"
    subprocess.run(["cp", "-R", f"{seed}/.", str(tmpdir)], check=True)
    log = tmpdir / "server.log"
    proc = subprocess.Popen(
        ["dolt", "sql-server", "--port", "0", "--host", "127.0.0.1",
         "--data-dir", str(tmpdir), "--readonly=false"],
        stdout=open(log, "w"), stderr=subprocess.STDOUT,
    )
    # Poll for "Listening on 127.0.0.1:<port>" within 10s
    deadline = time.time() + 10
    port = None
    while time.time() < deadline and port is None:
        if log.exists():
            m = re.search(r"Listening on 127\.0\.0\.1:(\d+)", log.read_text())
            if m:
                port = m.group(1)
        time.sleep(0.1)
    assert port, f"dolt did not bind in 10s; log={log.read_text()}"
    os.environ["GRAVA_DOLT_DSN"] = f"root@tcp(127.0.0.1:{port})/grava?parseTime=true"
    os.environ["GRAVA_DATA_DIR"] = str(tmpdir)
    yield tmpdir
    proc.send_signal(signal.SIGTERM)
    proc.wait(timeout=5)
    subprocess.run(["rm", "-rf", str(tmpdir)])
```

The fixture is `module`-scoped so files of related tests share a DB (keeps suite under 90s). Wisps are scoped per issue id, so cross-test bleed is avoided by using unique issue ids (`grava-test-<phase>-<n>`). Function-scoped tests requiring a clean DB use the function-scoped variant `dolt_fixture_clean` (defined alongside).

## Assertion helpers (pytest)

```python
# sandbox/ship/conftest.py (continued)

def assert_wisp(issue_id: str, key: str, expected: str):
    actual = subprocess.run(
        ["grava", "wisp", "read", issue_id, key],
        capture_output=True, text=True,
    ).stdout.strip() or "<missing>"
    assert actual == expected, f"wisp mismatch: {issue_id}.{key} expected={expected!r} actual={actual!r}"

def assert_signal(output: str, expected: str):
    last = next((l for l in reversed(output.splitlines()) if l.strip()), "")
    assert last == expected, f"signal mismatch: expected={expected!r} got={last!r}"

def assert_label(issue_id: str, label: str):
    import json
    issue = json.loads(subprocess.run(
        ["grava", "show", issue_id, "--json"],
        capture_output=True, text=True, check=True,
    ).stdout)
    assert label in (issue.get("labels") or []), f"label {label!r} missing on {issue_id}"
```

These are imported by each `test_*.py` from `conftest.py`.

## Test matrix

Tests are grouped by phase. Every acceptance criterion in story 2B.5 maps to at least one test. Numbering below cross-references the AC list at `story-2B.5-skill-ship.md:590`. File names below match the pytest layout (`test_*.py`).

### Helpers (test_helpers.py)

| # | Test | Validates |
|---|------|-----------|
| H-1 | `last_line` returns final non-empty line; trailing blanks ignored | Fix 1 |
| H-2 | `parse_signal` recognizes 7 expected signals; emits NAME\|TAIL | Phase parser |
| H-3 | `parse_signal` returns `INVALID` for unknown / mid-line / empty input | Negative |
| H-4 | Flag parser: positional id + `--retry`, `--rebase-only`, `--force` order-tolerant | Setup block |
| H-5 | `--rebase-only` without `--retry` → `PIPELINE_FAILED` | AC L613 |
| H-6 | `--retry` without `<id>` → `PIPELINE_FAILED` | AC L615 |

### Phase 0 — discover + precondition gate (phase0_discover.py)

| # | Test | Validates |
|---|------|-----------|
| 0-1 | `/ship` (no id), empty queue → `PIPELINE_INFO`, exit 0, no agent spawned | AC L592 |
| 0-2 | `/ship` (no id), queue with epic + task + bug → auto-picks first leaf-type, prints up-to-2 alts | AC L593 |
| 0-3 | `/ship` (no id), queue with only epics → `PIPELINE_INFO` ("ready queue empty (no task/bug)") | Filter logic |
| 0-4 | `/ship <id>` on missing issue → `PIPELINE_FAILED: <id> not found`, no spawn | Setup validation |
| 0-5 | Auto-pick + missing description → `PIPELINE_HALTED: failed precondition — missing description` + alt list | AC L594 |
| 0-6 | Auto-pick + no AC heuristic match → `PIPELINE_HALTED: ... no acceptance criteria` | AC L594 |
| 0-7 | Auto-pick + `code_review` label on top → halts (defensive guard) | AC L594 |
| 0-8 | Explicit `/ship <id>` on no-desc issue → halts identically (gate is always-on) | AC L594/595 |
| 0-9 | `/ship <id> --force` on no-desc issue → bypasses gate, emits `PIPELINE_INFO: --force set; ...`, proceeds to Phase 1 | AC L596 |
| 0-10 | Halt message includes 3 recovery paths (fix-and-rerun, alt-id, --force) | AC L594 |
| 0-11 | `gh` not authenticated (preflight fails) → `PIPELINE_FAILED: GitHub auth missing`, no agent spawn | AC L598 |

### Phase 1 — code (phase1_code.py)

| # | Test | Validates |
|---|------|-----------|
| 1-1 | Stub coder emits `CODER_DONE: abc123` → `pipeline_phase=claimed` written, `LAST_SHA=abc123`, advances to Phase 2 | AC L597 |
| 1-2 | `pipeline_phase=claimed` is seeded **before** the Agent call (assertable via stub fixture that reads wisp first) | AC L597 |
| 1-3 | Stub coder emits `CODER_HALTED: missing spec` → `PIPELINE_HALTED: coder — missing spec`, exit 0 | Phase 1 halt |
| 1-4 | Stub coder emits noise then `CODER_DONE: ...` as last line → advances (last-line semantics) | Fix 1 / AC L599 |
| 1-5 | Stub coder emits `CODER_DONE: ...` mid-output, then unrelated last line → `PIPELINE_FAILED: signal parse failed` | Fix 1 / AC L599 |
| 1-6 | Heartbeat wisp `orchestrator_heartbeat` written before Agent spawn | AC L610 |

### Phase 2 — review loop (phase2_review.py)

| # | Test | Validates |
|---|------|-----------|
| 2-1 | Reviewer `REVIEWER_APPROVED` on round 1 → `APPROVED_SHA` set, advances to Phase 3 | Happy path |
| 2-2 | Reviewer `REVIEWER_BLOCKED: …` round 1, coder `CODER_DONE` round 1, reviewer `APPROVED` round 2 → advances; `review_round_1=blocked` wisp written | AC L600 |
| 2-3 | Three blocked rounds → `PIPELINE_HALTED`, `needs-human` label, `pipeline_halted` wisp set | AC L600 |
| 2-4 | Findings ≤2KB → re-spawn prompt contains inline `FINDINGS:` (assert via stub-captured prompt) | AC L601 |
| 2-5 | Findings >2KB → `FINDINGS_PATH` written under `.worktree/<id>/.review-round-N.md`, prompt references path | AC L601 |
| 2-6 | Re-spawn passes `ROUND: N` token to coder | AC L602 |
| 2-7 | Coder halts mid-loop → `PIPELINE_HALTED: coder halted at review round N`, exit 0 | Mid-loop halt |
| 2-8 | Heartbeat written once per round | AC L610 |

### Phase 3 — PR creation (phase3_pr.py)

| # | Test | Validates |
|---|------|-----------|
| 3-1 | pr-creator `PR_CREATED: https://...` → `pipeline_phase=pr_awaiting_merge`, emits `PIPELINE_HANDOFF`, exit 0 | AC L603/604 |
| 3-2 | pr-creator `PR_FAILED: ...` → `pr-failed` label added, `PIPELINE_FAILED`, exit 1 | Phase 3 fail |
| 3-3 | `/ship` body contains no inline `gh pr create` (grep assertion on the SKILL.md file) | AC L603 |
| 3-4 | No inline poll loop after handoff (grep: no `while true` / `sleep 120` after Phase 3) | AC L604 |

### Phase 4 — resume on PR comments (phase4_resume.py)

| # | Test | Validates |
|---|------|-----------|
| 4-1 | Re-entry: `pipeline_phase=pr_awaiting_merge`, no `pr_new_comments` → `PIPELINE_INFO`, exit 0, no spawn | Setup re-entry |
| 4-2 | Re-entry with `pr_new_comments` → enters Phase 4 fix loop, increments `pr_fix_round` | AC L605 |
| 4-3 | Off-scope feedback (file outside original diff) → `needs-human` label, `pr_off_scope` wisp, halts | AC L607 |
| 4-4 | Coder fix succeeds + stub `gh pr checks` passes → `pr_last_seen_comment_id` bumped to highest in batch, `pr_new_comments` deleted, `pipeline_phase=pr_awaiting_merge` | AC L609 |
| 4-5 | Stub `gh pr checks` fails → `ci_failed` wisp, halts at round N | AC L608 |
| 4-6 | `MAX_PR_FIX_ROUNDS=3` exhausted → `pr_fix_exhausted` wisp, `needs-human` label, halts | AC L606 |
| 4-7 | Coder commit footer carries `[round N]` (assert via stub-captured prompt) | AC L602 |

### Phase 5 — retry on rejected PR (phase5_retry.py)

| # | Test | Validates |
|---|------|-----------|
| 5-1 | `pipeline_phase=failed`, no `--retry` → recovery menu printed, exit 0, no spawn | AC L612 |
| 5-2 | `--retry`: bumps `pr_retry_count`, removes `pr-rejected` label, resets `pipeline_phase=claimed`, re-spawns coder with `RETRY_MODE=full` | AC L613 |
| 5-3 | `--retry --rebase-only`: spawns coder with `RETRY_MODE=rebase-only`, sets `APPROVED_SHA`, jumps directly to Phase 3 (skip Phase 2) | AC L614 |
| 5-4 | `pr_retry_count` over `MAX_PR_RETRIES=2` → `needs-human`, halts | AC L613 |
| 5-5 | Retry coder commit footer carries `[retry N]` | AC L617 |
| 5-6 | Rejection notes >2KB → `REJECTION_NOTES_PATH` written; ≤2KB → inline | AC L601 (parity) |

### Re-entry switch (reentry.py)

| # | Test | Validates |
|---|------|-----------|
| R-1 | `pipeline_phase=complete` → `PIPELINE_COMPLETE: <id>`, exit 0 | AC L634 |
| R-2 | `pipeline_phase=pr_awaiting_merge` + `pr_new_comments` set → jumps to Phase 4 (not Phase 1) | Re-entry routing |
| R-3 | `pipeline_phase=failed` + `--retry` → enters Phase 5 retry block | Re-entry routing |
| R-4 | `pipeline_phase=failed` + `--retry --force` → both flags accepted (force is no-op at Phase 5; documented as redundant) | AC L596 |

### Negative / regression (negative.py)

| # | Test | Validates |
|---|------|-----------|
| N-1 | Agent emits `CODER_DONE` mid-output, `noise` last line → no wisp advance, `PIPELINE_FAILED: signal parse failed` | Fix 1 |
| N-2 | Coder re-spawn after `review_blocked` emits `CODER_DONE` → wisp does NOT regress to `claimed`; phase stays `review_blocked` until next `REVIEWER_APPROVED` | Fix 8 |
| N-3 | Unknown signal `WAT: ...` last line → `PIPELINE_FAILED: signal parse failed`, exit 1 | parse_signal default branch |
| N-4 | `Agent` returns empty stdout → `INVALID` parse → `PIPELINE_FAILED` | Edge |
| N-5 | Wisp write fails (simulate by chmod-ing dolt readonly) → `/ship` surfaces error, does not proceed silently | Fail-loud |

### Contention (contention.py)

| # | Test | Validates |
|---|------|-----------|
| C-1 | Two `/ship` (no id) processes against a 4-issue queue → each picks a different issue (verified via wisps) | README step 14 |
| C-2 | Four `/ship` (no id) processes, 2 ready issues → 2 succeed, 2 emit `PIPELINE_INFO` (queue empty after first two claim) | Atomic claim |
| C-3 | `grava claim` race: two terminals target same id → one wins (`CODER_DONE`), other gets `claim contended` HALT | DB-layer atomicity |

## CI integration

The CI workflow lives in the **sandbox repo** (`gravav6-sandbox/.github/workflows/ship-tests.yml`), not the main grava repo. It runs against a checked-out copy of grava pinned by SHA (input parameter or repository-dispatch from grava's main-repo CI when paths in the filter list change).

```yaml
# gravav6-sandbox/.github/workflows/ship-tests.yml
name: ship-tests
on:
  push:
    paths:
      - 'sandbox/ship/**'
  workflow_dispatch:
    inputs:
      grava_sha:
        description: 'grava commit SHA to test against'
        required: false
        default: 'main'
  repository_dispatch:
    types: [grava-ship-changed]    # fired by main grava repo when /ship paths change
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4                          # checks out gravav6-sandbox
      - uses: actions/checkout@v4
        with:
          repository: <owner>/grava
          ref: ${{ github.event.inputs.grava_sha || github.event.client_payload.sha || 'main' }}
          path: grava-src
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - uses: actions/setup-python@v5
        with: { python-version: '3.12' }
      - uses: dolthub/dolt-action@v1
      - run: |
          cd grava-src && go build -o "$GITHUB_WORKSPACE/grava_test_bin" ./cmd/grava
          echo "$GITHUB_WORKSPACE" >> "$GITHUB_PATH"
      - run: pip install -r requirements.txt
      - run: make -C sandbox/ship test-ship
        env:
          SHIP_TEST_GH_STUB: 1
          GRAVA_SKILL_DIR: ${{ github.workspace }}/grava-src/.claude/skills/ship
```

The main grava repo gains a tiny workflow `notify-ship-tests.yml` that fires `repository_dispatch` to the sandbox repo when any path in the filter list changes (the filter list lives in **one** place: the sandbox workflow's `paths` block, and is documented in `sandbox/ship/README.md`).

`sandbox/ship/Makefile`:

```makefile
test-ship:
	@command -v pytest >/dev/null || { echo "install: pip install -r ../../requirements.txt"; exit 1; }
	@command -v dolt >/dev/null || { echo "install dolt: brew install dolt"; exit 1; }
	@command -v grava >/dev/null || { echo "grava binary not on PATH (point to grava_test_bin)"; exit 1; }
	pytest -v .
```

## Acceptance Criteria

- `make -C sandbox/ship test-ship` (run from inside `gravav6-sandbox`) exits 0 on a clean checkout; full suite completes in <90s on CI.
- Suite is hosted in the sandbox repo at `gravav6-sandbox/sandbox/ship/`; no `tests/` tree is added to the main grava repo.
- Every acceptance criterion in `story-2B.5-skill-ship.md` (lines 590–617) is tagged in the test matrix above by exactly one test (or marked N/A with rationale in the test file's header comment).
- Stub agent harness honors per-phase fixture overrides (`SHIP_TEST_FIXTURE_PHASE1`, …) so a single test can drive coder→reviewer→pr-creator with three different canned outputs.
- Dolt fixture spins up on a random port and tears down cleanly; running the suite leaves no residual processes (`pgrep dolt` after teardown returns empty).
- Tests run without network access (no real `gh` calls; `gh` is shimmed via `tests/ship/lib/gh-stub.sh` for `gh auth status` and `gh pr checks`).
- Negative tests for Fix 1 (last-line) and Fix 8 (forward-only) reproduce the regressions they exist to prevent: removing the relevant defenses from `/ship` causes the negative tests to fail.
- Contention test runs N=2 by default; flag `SHIP_TEST_CONTENTION_N=4` opts into the larger run for nightly CI (kept off PR critical path to bound runtime).
- CI workflow `ship-tests.yml` (in the sandbox repo) is required-status on grava-side PRs via `repository_dispatch` round-trip: main grava repo's `notify-ship-tests.yml` triggers it, sandbox runs the suite, GitHub status check on grava's PR is updated by a follow-up commit-status API call. (If the round-trip turns out to be too brittle, fallback is to keep the suite advisory and gate via a nightly required check.)
- Test fixture issues use ids prefixed `grava-test-` so they never collide with real issues if the suite is run against a non-disposable Dolt by mistake.
- Suite includes a regression test for the Phase 0 auto-pick alts list: when only 1 candidate exists, output omits the alts section (no trailing empty bullets).
- Test failures emit the captured stub-Agent prompt for the failing phase to make signal-parse / context-construction bugs debuggable without rerunning.

## Dependencies

- Story 2B.5 (`/ship` skill) — unit under test; needs the addendum noted above to make `Agent` shell-resolvable so the stub binds.
- Story 2B.0b (`grava wisp delete`) — used in Phase 4 cleanup assertions.
- Story 2B.0d (heartbeat) — referenced when asserting heartbeat writes across phases.
- Story 2B.9 (`sync-pipeline-status.sh` hook) — the suite runs `/ship` with the hook **disabled** by default (test asserts `/ship`'s in-band wisp writes); a single integration test in `test_phase1_code.py` re-enables the hook and asserts it does not double-write or regress phase.
- `pytest`, `pytest-xdist`, `dolt`, `jq`, `gh` (stubbed), `grava` (built from grava-src in CI). Go toolchain required only on CI to build the test binary; not required to run the suite locally if a `grava` binary is already on `$PATH`.
- Sandbox repo (`gravav6-sandbox`) — checked out alongside grava in CI; suite lives there, not in main grava.

## Signals Tested (catalogue)

`/ship`-emitted (asserted on stdout):

- `PR_CREATED:` `PR_COMMENTS_RESOLVED:` `PIPELINE_HANDOFF:` `PIPELINE_HALTED:` `PIPELINE_FAILED:` `PIPELINE_INFO:` `PIPELINE_COMPLETE:`

Agent-emitted (consumed via stub fixtures):

- `CODER_DONE:` `CODER_HALTED:` `REVIEWER_APPROVED` `REVIEWER_BLOCKED[: …]` `PR_CREATED:` `PR_FAILED:`

The catalogue matches `parse_signal` in 2B.5 L150–162 — if either side adds a signal, both must update.

## Out of Scope

- Live end-to-end run against real `gh` and real Anthropic Agent API — covered by README §Implementation Order step 9 (manual smoke).
- Watcher tests (story 2B.12) — separate suite. This story tests only `/ship`'s re-entry contract with the watcher's wisp outputs (which we mint directly in the fixture, not by running the watcher).
- Performance/load — the contention test asserts correctness under N=2/4, not throughput.
