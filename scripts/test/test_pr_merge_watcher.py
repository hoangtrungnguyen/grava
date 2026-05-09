"""Tests for scripts/pr_merge_watcher.py.

Covers the same behaviour parity matrix as the bash test suite
(test-watcher-pidfile.sh, test-watcher-comments.sh) plus the
state-transition branches not previously bash-tested.

Run:
    pytest scripts/test/test_pr_merge_watcher.py -v

The test strategy: monkeypatch `pr_merge_watcher.run` with a programmable
fake that matches incoming `cmd` lists against pre-registered responses.
This keeps tests hermetic — no real grava DB, no real gh, no real ps.
"""

from __future__ import annotations

import json
import os
import sys
from pathlib import Path
from typing import Any

import pytest

# Make scripts/ importable
ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))

import pr_merge_watcher as w  # noqa: E402


# ---------------------------------------------------------------------------
# Programmable fake `run`
# ---------------------------------------------------------------------------

class FakeRun:
    """Configurable subprocess stub.

    `expect(prefix, response)` registers a response (rc, stdout, stderr) to
    return when the next `run()` call's argv starts with `prefix`. Multiple
    prefixes can be registered; the first match wins. Calls without a match
    raise AssertionError so tests can't accidentally hit unmocked code paths.
    """

    def __init__(self) -> None:
        self.expectations: list[tuple[list[str], tuple[int, str, str]]] = []
        self.calls: list[tuple[list[str], str | None]] = []

    def expect(self, prefix: list[str], rc: int = 0, stdout: str = "", stderr: str = "") -> None:
        self.expectations.append((prefix, (rc, stdout, stderr)))

    def __call__(self, cmd: list[str], input_text: str | None = None) -> tuple[int, str, str]:
        self.calls.append((cmd, input_text))
        for prefix, response in self.expectations:
            if cmd[: len(prefix)] == prefix:
                return response
        raise AssertionError(f"unexpected run() call: {cmd}")

    def calls_with_prefix(self, prefix: list[str]) -> list[list[str]]:
        return [cmd for cmd, _ in self.calls if cmd[: len(prefix)] == prefix]


@pytest.fixture
def fake_run(monkeypatch: pytest.MonkeyPatch) -> FakeRun:
    fr = FakeRun()
    monkeypatch.setattr(w, "run", fr)
    return fr


@pytest.fixture
def frozen_time(monkeypatch: pytest.MonkeyPatch) -> int:
    """Pin now_unix and now_iso so tests can assert on stamps."""
    fixed_unix = 1_700_000_000
    monkeypatch.setattr(w, "now_unix", lambda: fixed_unix)
    monkeypatch.setattr(w, "now_iso", lambda: "2023-11-14T22:13:20Z")
    return fixed_unix


# ===========================================================================
# PIDFILE handling — grava-24fa parity matrix
# ===========================================================================

class TestPidfile:
    def test_no_pidfile_acquires(self, tmp_path: Path, fake_run: FakeRun) -> None:
        pidfile = tmp_path / ".grava/pr-merge-watcher.pid"
        assert w.acquire_pidfile(str(pidfile)) is True
        assert pidfile.read_text() == str(os.getpid())

    def test_dead_pid_is_overwritten(self, tmp_path: Path, fake_run: FakeRun) -> None:
        pidfile = tmp_path / "watcher.pid"
        pidfile.parent.mkdir(parents=True, exist_ok=True)
        # PID 1 typically alive on every system, BUT we want a dead PID — pick
        # a very high one that won't exist.
        pidfile.write_text("99999999")
        # _pid_alive on 99999999 returns False, so we should acquire cleanly.
        assert w.acquire_pidfile(str(pidfile)) is True
        assert pidfile.read_text() == str(os.getpid())

    def test_live_unrelated_pid_overwritten(
        self, tmp_path: Path, fake_run: FakeRun, monkeypatch: pytest.MonkeyPatch, caplog
    ) -> None:
        """grava-24fa: live PID whose `ps` command doesn't look like the
        watcher gets treated as stale and overwritten."""
        pidfile = tmp_path / "watcher.pid"
        pidfile.parent.mkdir(parents=True, exist_ok=True)
        pidfile.write_text("4242")
        monkeypatch.setattr(w, "_pid_alive", lambda pid: True)
        fake_run.expect(["ps", "-o", "command=", "-p", "4242"], stdout="/Applications/Browser.app/Contents/MacOS/Browser\n")

        with caplog.at_level("INFO"):
            assert w.acquire_pidfile(str(pidfile)) is True
        assert pidfile.read_text() == str(os.getpid())
        assert any("unrelated process" in rec.message for rec in caplog.records)

    def test_live_real_watcher_skips(
        self, tmp_path: Path, fake_run: FakeRun, monkeypatch: pytest.MonkeyPatch, caplog
    ) -> None:
        pidfile = tmp_path / "watcher.pid"
        pidfile.parent.mkdir(parents=True, exist_ok=True)
        pidfile.write_text("4242")
        monkeypatch.setattr(w, "_pid_alive", lambda pid: True)
        fake_run.expect(
            ["ps", "-o", "command=", "-p", "4242"],
            stdout="python3 scripts/pr_merge_watcher.py\n",
        )

        with caplog.at_level("INFO"):
            assert w.acquire_pidfile(str(pidfile)) is False
        # PIDFILE NOT overwritten — preserves the running watcher's claim
        assert pidfile.read_text() == "4242"
        assert any("still active" in rec.message for rec in caplog.records)

    def test_legacy_bash_watcher_name_still_recognised(
        self, tmp_path: Path, fake_run: FakeRun, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        """During the bash→python rollout both names may appear in `ps`."""
        pidfile = tmp_path / "watcher.pid"
        pidfile.parent.mkdir(parents=True, exist_ok=True)
        pidfile.write_text("4242")
        monkeypatch.setattr(w, "_pid_alive", lambda pid: True)
        fake_run.expect(
            ["ps", "-o", "command=", "-p", "4242"],
            stdout="bash scripts/pr-merge-watcher.sh\n",
        )
        assert w.acquire_pidfile(str(pidfile)) is False

    def test_release_removes_pidfile(self, tmp_path: Path) -> None:
        pidfile = tmp_path / "watcher.pid"
        pidfile.parent.mkdir(parents=True, exist_ok=True)
        pidfile.write_text("123")
        w.release_pidfile(str(pidfile))
        assert not pidfile.exists()

    def test_release_missing_pidfile_does_not_raise(self, tmp_path: Path) -> None:
        w.release_pidfile(str(tmp_path / "nope.pid"))  # no exception


# ===========================================================================
# grava CLI wrappers
# ===========================================================================

class TestGravaWrappers:
    def test_wisp_read_present(self, fake_run: FakeRun) -> None:
        fake_run.expect(["grava", "wisp", "read"], stdout="hello\n")
        assert w.grava_wisp_read("grava-x", "k") == "hello"

    def test_wisp_read_missing_returns_none(self, fake_run: FakeRun) -> None:
        # CLI exits 1 on missing wisp (re-verified May 2026)
        fake_run.expect(["grava", "wisp", "read"], rc=1, stdout="")
        assert w.grava_wisp_read("grava-x", "k") is None

    def test_wisp_read_blank_stdout_returns_none(self, fake_run: FakeRun) -> None:
        fake_run.expect(["grava", "wisp", "read"], rc=0, stdout="   \n")
        assert w.grava_wisp_read("grava-x", "k") is None

    def test_signal_passes_payload(self, fake_run: FakeRun) -> None:
        fake_run.expect(["grava", "signal"], stdout="OK")
        assert w.grava_signal("PR_CLOSED", "grava-x", payload="reviewer-rejected") is True
        assert fake_run.calls[-1][0] == [
            "grava", "signal", "PR_CLOSED",
            "--issue", "grava-x",
            "--actor", "watcher",
            "--payload", "reviewer-rejected",
        ]

    def test_signal_omits_payload_when_none(self, fake_run: FakeRun) -> None:
        fake_run.expect(["grava", "signal"], stdout="OK")
        w.grava_signal("PR_MERGED", "grava-x")
        assert "--payload" not in fake_run.calls[-1][0]

    def test_label_combines_add_remove(self, fake_run: FakeRun) -> None:
        fake_run.expect(["grava", "label"], stdout="OK")
        w.grava_label("grava-x", add=["a", "b"], remove=["c"])
        assert fake_run.calls[-1][0] == [
            "grava", "label", "grava-x",
            "--add", "a", "--add", "b",
            "--remove", "c",
        ]

    def test_list_pr_created_parses_ids(self, fake_run: FakeRun) -> None:
        fake_run.expect(
            ["grava", "list"],
            stdout=json.dumps([{"id": "grava-1", "title": "x"}, {"id": "grava-2"}]),
        )
        assert w.grava_list_pr_created() == ["grava-1", "grava-2"]

    def test_list_pr_created_handles_garbage(self, fake_run: FakeRun) -> None:
        fake_run.expect(["grava", "list"], stdout="not json")
        assert w.grava_list_pr_created() == []

    def test_list_pr_created_handles_empty(self, fake_run: FakeRun) -> None:
        fake_run.expect(["grava", "list"], stdout="")
        assert w.grava_list_pr_created() == []


# ===========================================================================
# gh CLI wrappers — including grava-431b array shape gate
# ===========================================================================

class TestGhWrappers:
    def test_pr_view_returns_dict(self, fake_run: FakeRun) -> None:
        fake_run.expect(
            ["gh", "pr", "view"],
            stdout=json.dumps({"state": "OPEN"}),
        )
        assert w.gh_pr_view(123, ["state"]) == {"state": "OPEN"}

    def test_pr_view_handles_invalid_json(self, fake_run: FakeRun) -> None:
        fake_run.expect(["gh", "pr", "view"], stdout="rate limited")
        assert w.gh_pr_view(123, ["state"]) is None

    def test_api_comments_valid_array(self, fake_run: FakeRun) -> None:
        fake_run.expect(
            ["gh", "api"],
            stdout=json.dumps([{"id": 1, "body": "x"}]),
        )
        result = w.gh_api_pr_comments(123)
        assert result == [{"id": 1, "body": "x"}]

    def test_api_comments_empty_array_ok(self, fake_run: FakeRun) -> None:
        fake_run.expect(["gh", "api"], stdout="[]")
        assert w.gh_api_pr_comments(123) == []

    def test_api_comments_error_string_returns_none(self, fake_run: FakeRun) -> None:
        """grava-431b: gh occasionally prints an error string instead of JSON."""
        fake_run.expect(["gh", "api"], stdout="API rate limit exceeded")
        assert w.gh_api_pr_comments(123) is None

    def test_api_comments_object_returns_none(self, fake_run: FakeRun) -> None:
        """grava-431b: gh sometimes returns a {"message":"..."} object."""
        fake_run.expect(
            ["gh", "api"],
            stdout=json.dumps({"message": "Not Found", "documentation_url": "..."}),
        )
        assert w.gh_api_pr_comments(123) is None

    def test_api_comments_empty_stdout_returns_none(self, fake_run: FakeRun) -> None:
        fake_run.expect(["gh", "api"], stdout="")
        assert w.gh_api_pr_comments(123) is None

    def test_api_comments_nonzero_exit_returns_none(self, fake_run: FakeRun) -> None:
        fake_run.expect(["gh", "api"], rc=1, stdout="")
        assert w.gh_api_pr_comments(123) is None


# ===========================================================================
# process_merged — happy path + grava-63f3 close-failure handling
# ===========================================================================

class TestProcessMerged:
    def test_happy_path_emits_full_sequence(self, fake_run: FakeRun, frozen_time: int) -> None:
        fake_run.expect(["grava", "wisp", "write"], stdout="OK")
        fake_run.expect(["grava", "signal"], stdout="OK")
        fake_run.expect(["grava", "label"], stdout="OK")
        fake_run.expect(["grava", "close"], stdout="OK")
        fake_run.expect(["grava", "commit"], stdout="OK")

        w.process_merged("grava-1", "https://x/y/1", frozen_time)

        kinds = [c[2] for c in fake_run.calls_with_prefix(["grava", "signal"])]
        assert "PR_MERGED" in kinds
        assert "PIPELINE_COMPLETE" in kinds
        assert any("removed" in c or "--remove" in c for c in fake_run.calls_with_prefix(["grava", "label"])[0])

    def test_close_fails_status_open_skips_pipeline_complete(
        self, fake_run: FakeRun, frozen_time: int, caplog
    ) -> None:
        """grava-63f3: don't fire PIPELINE_COMPLETE if close failed and issue
        isn't already closed — leave for next iteration."""
        fake_run.expect(["grava", "wisp", "write"], stdout="OK")
        fake_run.expect(["grava", "signal", "PR_MERGED"], stdout="OK")
        fake_run.expect(["grava", "label"], stdout="OK")
        fake_run.expect(["grava", "close"], rc=1, stderr="DB locked")
        fake_run.expect(
            ["grava", "show"],
            stdout=json.dumps({"id": "grava-1", "status": "in_progress"}),
        )

        with caplog.at_level("INFO"):
            w.process_merged("grava-1", "https://x/y/1", frozen_time)

        signal_kinds = [c[2] for c in fake_run.calls_with_prefix(["grava", "signal"])]
        assert "PR_MERGED" in signal_kinds
        assert "PIPELINE_COMPLETE" not in signal_kinds
        assert any("failed to close" in r.message for r in caplog.records)

    def test_close_fails_but_status_already_closed_proceeds(
        self, fake_run: FakeRun, frozen_time: int
    ) -> None:
        """grava-63f3: close failed because issue was ALREADY closed — proceed."""
        fake_run.expect(["grava", "wisp", "write"], stdout="OK")
        fake_run.expect(["grava", "signal"], stdout="OK")
        fake_run.expect(["grava", "label"], stdout="OK")
        fake_run.expect(["grava", "close"], rc=1, stderr="already closed")
        fake_run.expect(
            ["grava", "show"],
            stdout=json.dumps({"id": "grava-1", "status": "closed"}),
        )
        fake_run.expect(["grava", "commit"], stdout="OK")

        w.process_merged("grava-1", "https://x/y/1", frozen_time)

        signal_kinds = [c[2] for c in fake_run.calls_with_prefix(["grava", "signal"])]
        assert "PIPELINE_COMPLETE" in signal_kinds


# ===========================================================================
# process_closed — first-time vs idempotent re-run, grava-97ec guard
# ===========================================================================

class TestProcessClosed:
    def _setup_changes_requested(self, fake_run: FakeRun) -> None:
        # pr_rejection_recorded missing → first time
        fake_run.expect(["grava", "wisp", "read", "grava-1", "pr_rejection_recorded"], rc=1, stdout="")
        fake_run.expect(
            ["gh", "pr", "view", "5", "--json", "reviews,closedBy,author"],
            stdout=json.dumps({
                "reviews": [{"state": "CHANGES_REQUESTED", "body": "fix the thing"}],
                "closedBy": {"login": "reviewer-bot"},
                "author": {"login": "agent-bot"},
            }),
        )
        fake_run.expect(
            ["gh", "pr", "view", "5", "--json", "comments"],
            stdout=json.dumps({"comments": [{"body": "thanks!"}]}),
        )

    def test_first_time_records_reviewer_rejected(
        self, fake_run: FakeRun, frozen_time: int
    ) -> None:
        self._setup_changes_requested(fake_run)
        fake_run.expect(["grava", "update"], stdout="OK")
        fake_run.expect(["grava", "comment"], stdout="OK")
        fake_run.expect(["grava", "wisp", "write"], stdout="OK")
        fake_run.expect(["grava", "signal"], stdout="OK")
        fake_run.expect(["grava", "label"], stdout="OK")
        fake_run.expect(["grava", "commit"], stdout="OK")

        w.process_closed("grava-1", 5, "https://x/y/5", frozen_time)

        # signal payload must be reviewer-rejected
        signal_calls = fake_run.calls_with_prefix(["grava", "signal"])
        pr_closed_calls = [c for c in signal_calls if c[2] == "PR_CLOSED"]
        assert len(pr_closed_calls) == 1
        assert "reviewer-rejected" in pr_closed_calls[0]

    def test_first_time_author_abandoned(
        self, fake_run: FakeRun, frozen_time: int
    ) -> None:
        fake_run.expect(["grava", "wisp", "read", "grava-1", "pr_rejection_recorded"], rc=1, stdout="")
        fake_run.expect(
            ["gh", "pr", "view", "5", "--json", "reviews,closedBy,author"],
            stdout=json.dumps({
                "reviews": [],
                "closedBy": {"login": "agent-bot"},
                "author": {"login": "agent-bot"},
            }),
        )
        fake_run.expect(["gh", "pr", "view", "5", "--json", "comments"], stdout=json.dumps({"comments": []}))
        fake_run.expect(["grava", "update"], stdout="OK")
        fake_run.expect(["grava", "comment"], stdout="OK")
        fake_run.expect(["grava", "wisp", "write"], stdout="OK")
        fake_run.expect(["grava", "signal"], stdout="OK")
        fake_run.expect(["grava", "label"], stdout="OK")
        fake_run.expect(["grava", "commit"], stdout="OK")

        w.process_closed("grava-1", 5, "https://x/y/5", frozen_time)

        pr_closed = [c for c in fake_run.calls_with_prefix(["grava", "signal"]) if c[2] == "PR_CLOSED"]
        assert len(pr_closed) == 1
        assert "author-abandoned" in pr_closed[0]

    def test_grava_97ec_description_write_failure_defers(
        self, fake_run: FakeRun, frozen_time: int, caplog
    ) -> None:
        """grava-97ec: if description-append fails, defer the rest of the
        recording — don't set the idempotency gate, don't emit signal."""
        self._setup_changes_requested(fake_run)
        # description append FAILS
        fake_run.expect(["grava", "update"], rc=1, stderr="DB blip")

        with caplog.at_level("INFO"):
            w.process_closed("grava-1", 5, "https://x/y/5", frozen_time)

        # No signal should fire, no idempotency gate written, no commit.
        assert not fake_run.calls_with_prefix(["grava", "signal"])
        assert not fake_run.calls_with_prefix(["grava", "comment"])
        # pr_rejection_recorded must NOT have been written
        rejection_writes = [
            c for c in fake_run.calls_with_prefix(["grava", "wisp", "write"])
            if "pr_rejection_recorded" in c
        ]
        assert rejection_writes == []
        assert any("will retry next iteration" in r.message for r in caplog.records)

    def test_idempotent_rerun_skips_signal(
        self, fake_run: FakeRun, frozen_time: int
    ) -> None:
        """When pr_rejection_recorded is already 1, only label/commit run.
        No re-signaling (which would overwrite pr_close_reason with blank)."""
        fake_run.expect(["grava", "wisp", "read", "grava-1", "pr_rejection_recorded"], stdout="1\n")
        fake_run.expect(["grava", "label"], stdout="OK")
        fake_run.expect(["grava", "commit"], stdout="OK")

        w.process_closed("grava-1", 5, "https://x/y/5", frozen_time)

        # Confirm no signal call happened
        assert not fake_run.calls_with_prefix(["grava", "signal"])
        # No gh calls either
        assert not fake_run.calls_with_prefix(["gh"])


# ===========================================================================
# process_open — stale cap + comments diff + grava-431b gate
# ===========================================================================

class TestProcessOpen:
    def test_stale_cap_triggers_at_72h(
        self, fake_run: FakeRun, frozen_time: int
    ) -> None:
        # since = NOW - 72h
        old_since = frozen_time - (72 * 3600)
        fake_run.expect(
            ["grava", "wisp", "read", "grava-1", "pr_awaiting_merge_since"],
            stdout=str(old_since),
        )
        fake_run.expect(["grava", "wisp", "write"], stdout="OK")
        fake_run.expect(["grava", "label"], stdout="OK")
        fake_run.expect(["grava", "commit"], stdout="OK")

        w.process_open("grava-1", 7, "url", frozen_time)

        # No gh calls — short-circuited
        assert not fake_run.calls_with_prefix(["gh"])
        labels = fake_run.calls_with_prefix(["grava", "label"])
        assert any("needs-human" in c for c in labels)

    def test_grava_6ac8_missing_since_falls_back_to_now(
        self, fake_run: FakeRun, frozen_time: int
    ) -> None:
        """grava-6ac8: pr_awaiting_merge_since wisp absent → use NOW so
        AGE_HRS=0, gate doesn't trip."""
        fake_run.expect(
            ["grava", "wisp", "read", "grava-1", "pr_awaiting_merge_since"],
            rc=1, stdout="",
        )
        # Fresh PR — empty comments array → no further action
        fake_run.expect(["gh", "api"], stdout="[]")
        fake_run.expect(
            ["grava", "wisp", "read", "grava-1", "pr_last_seen_comment_id"],
            rc=1, stdout="",
        )
        fake_run.expect(["gh", "pr", "view"], stdout=json.dumps({"reviewDecision": ""}))

        w.process_open("grava-1", 7, "url", frozen_time)

        # Stale-cap path NOT taken — no needs-human label, no pr_stale wisp
        assert not fake_run.calls_with_prefix(["grava", "label"])
        wisp_writes = fake_run.calls_with_prefix(["grava", "wisp", "write"])
        assert all("pr_stale" not in c for c in wisp_writes)

    def test_grava_431b_non_array_gh_response_skips(
        self, fake_run: FakeRun, frozen_time: int, caplog
    ) -> None:
        """grava-431b: gh returned non-array → log + skip comment check,
        don't crash on downstream processing."""
        fake_run.expect(
            ["grava", "wisp", "read", "grava-1", "pr_awaiting_merge_since"],
            rc=1, stdout="",
        )
        fake_run.expect(["gh", "api"], stdout="API rate limit exceeded")

        with caplog.at_level("INFO"):
            w.process_open("grava-1", 7, "url", frozen_time)

        # No subsequent grava operations
        assert not fake_run.calls_with_prefix(["grava", "wisp", "write"])
        assert not fake_run.calls_with_prefix(["grava", "label"])
        assert any("non-array" in r.message for r in caplog.records)

    def test_new_comments_writes_wisps_and_commits(
        self, fake_run: FakeRun, frozen_time: int
    ) -> None:
        fake_run.expect(
            ["grava", "wisp", "read", "grava-1", "pr_awaiting_merge_since"],
            stdout=str(frozen_time - 60),
        )
        comments = [
            {"id": 100, "in_reply_to_id": None, "body": "comment 100"},
            {"id": 101, "in_reply_to_id": None, "body": "comment 101"},
            {"id": 102, "in_reply_to_id": 100, "body": "reply"},  # filtered out
        ]
        fake_run.expect(["gh", "api"], stdout=json.dumps(comments))
        fake_run.expect(
            ["grava", "wisp", "read", "grava-1", "pr_last_seen_comment_id"],
            stdout="50",
        )
        fake_run.expect(["gh", "pr", "view"], stdout=json.dumps({"reviewDecision": ""}))
        fake_run.expect(["grava", "wisp", "write"], stdout="OK")
        fake_run.expect(["grava", "commit"], stdout="OK")

        w.process_open("grava-1", 7, "url", frozen_time)

        wisp_writes = fake_run.calls_with_prefix(["grava", "wisp", "write"])
        # cmd shape: ["grava","wisp","write",<id>,<key>,<value>]
        keys_written = {c[4] for c in wisp_writes}
        assert "pr_new_comments" in keys_written
        assert "pr_last_seen_comment_id" in keys_written
        # Highest = max(100,101,102) = 102 (note: includes replies in highest watermark, parity with bash)
        last_seen_call = next(c for c in wisp_writes if c[4] == "pr_last_seen_comment_id")
        assert last_seen_call[5] == "102"

    def test_changes_requested_triggers_even_without_new_comments(
        self, fake_run: FakeRun, frozen_time: int
    ) -> None:
        """When reviewDecision flips to CHANGES_REQUESTED, watcher records
        even if NEW_COUNT==0 — operator needs to see it."""
        fake_run.expect(
            ["grava", "wisp", "read", "grava-1", "pr_awaiting_merge_since"],
            stdout=str(frozen_time - 60),
        )
        fake_run.expect(["gh", "api"], stdout="[]")
        fake_run.expect(
            ["grava", "wisp", "read", "grava-1", "pr_last_seen_comment_id"],
            rc=1, stdout="",
        )
        fake_run.expect(
            ["gh", "pr", "view"],
            stdout=json.dumps({"reviewDecision": "CHANGES_REQUESTED"}),
        )
        fake_run.expect(["grava", "wisp", "write"], stdout="OK")
        fake_run.expect(["grava", "commit"], stdout="OK")

        w.process_open("grava-1", 7, "url", frozen_time)

        wisp_writes = fake_run.calls_with_prefix(["grava", "wisp", "write"])
        keys = {c[4] for c in wisp_writes}
        assert "pr_new_comments" in keys


# ===========================================================================
# process_issue dispatch
# ===========================================================================

class TestProcessIssueDispatch:
    def test_no_pr_number_skips_silently(self, fake_run: FakeRun, frozen_time: int) -> None:
        fake_run.expect(["grava", "wisp", "read", "grava-1", "pr_number"], rc=1, stdout="")
        w.process_issue("grava-1", frozen_time)
        assert not fake_run.calls_with_prefix(["gh"])

    def test_invalid_pr_number_skips(self, fake_run: FakeRun, frozen_time: int) -> None:
        fake_run.expect(["grava", "wisp", "read", "grava-1", "pr_number"], stdout="not-a-number")
        w.process_issue("grava-1", frozen_time)
        assert not fake_run.calls_with_prefix(["gh"])

    def test_dispatches_to_merged(
        self, fake_run: FakeRun, frozen_time: int, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        called = []
        monkeypatch.setattr(w, "process_merged", lambda *a, **kw: called.append(("merged", a)))
        fake_run.expect(["grava", "wisp", "read", "grava-1", "pr_number"], stdout="42")
        fake_run.expect(["grava", "wisp", "read", "grava-1", "pr_url"], stdout="https://x/y/42")
        fake_run.expect(["gh", "pr", "view"], stdout=json.dumps({"state": "MERGED"}))

        w.process_issue("grava-1", frozen_time)

        assert called and called[0][0] == "merged"

    def test_dispatches_to_closed(
        self, fake_run: FakeRun, frozen_time: int, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        called = []
        monkeypatch.setattr(w, "process_closed", lambda *a, **kw: called.append("closed"))
        fake_run.expect(["grava", "wisp", "read", "grava-1", "pr_number"], stdout="42")
        fake_run.expect(["grava", "wisp", "read", "grava-1", "pr_url"], stdout="url")
        fake_run.expect(["gh", "pr", "view"], stdout=json.dumps({"state": "CLOSED"}))

        w.process_issue("grava-1", frozen_time)

        assert called == ["closed"]

    def test_dispatches_to_open_for_unknown_state(
        self, fake_run: FakeRun, frozen_time: int, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        """Bash-version parity: any non-MERGED/CLOSED state falls through to open."""
        called = []
        monkeypatch.setattr(w, "process_open", lambda *a, **kw: called.append("open"))
        fake_run.expect(["grava", "wisp", "read", "grava-1", "pr_number"], stdout="42")
        fake_run.expect(["grava", "wisp", "read", "grava-1", "pr_url"], stdout="url")
        fake_run.expect(["gh", "pr", "view"], stdout=json.dumps({"state": "DRAFT"}))

        w.process_issue("grava-1", frozen_time)

        assert called == ["open"]


# ===========================================================================
# Helper: _safe_int
# ===========================================================================

class TestSafeInt:
    @pytest.mark.parametrize("v,expected", [
        (0, 0),
        (42, 42),
        ("17", 17),
        ("0", 0),
        ("not", None),
        ("", None),
        (None, None),
        ([], None),
        ({}, None),
    ])
    def test_safe_int(self, v: Any, expected: Any) -> None:
        assert w._safe_int(v) == expected
