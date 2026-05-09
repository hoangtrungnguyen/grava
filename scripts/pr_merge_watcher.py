#!/usr/bin/env python3
"""scripts/pr_merge_watcher.py — async PR merge tracker.

Python rewrite of scripts/pr-merge-watcher.sh. Behavior is preserved verbatim;
this version trades bash idioms for typed code and pytest-based tests.

Run via cron every 5 min:
    */5 * * * * cd /path/to/repo && python3 scripts/pr_merge_watcher.py >> .grava/watcher.log 2>&1

Preserved hardenings (do not regress):
  grava-24fa  PIDFILE owner verification (kill -0 alone is unsafe — recycled PIDs
              on long-uptime hosts pass kill -0 but belong to unrelated processes).
  grava-63f3  PIPELINE_COMPLETE only emitted when grava close succeeded or the
              issue was already closed.
  grava-97ec  Description append guarded — failure defers the rest of the rejection
              recording until next iteration so notes aren't silently lost.
  grava-6ac8  pr_awaiting_merge_since / pr_last_seen_comment_id fall back cleanly
              when wisp is missing (CLI exits 1 with empty stdout).
  grava-431b  COMMENTS_JSON validated as a JSON array before downstream consumption.
  matrix #5   Skip log when previous tick still running (visible in cron logs).
"""

from __future__ import annotations

import json
import logging
import os
import subprocess
import sys
import time
from pathlib import Path
from typing import Any, Optional

MAX_PR_WAIT_HOURS = 72
PIDFILE = ".grava/pr-merge-watcher.pid"
LOG = logging.getLogger("watcher")


# ---------------------------------------------------------------------------
# Time helpers (seam: monkeypatch in tests for deterministic stamps).
# ---------------------------------------------------------------------------

def now_unix() -> int:
    return int(time.time())


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


# ---------------------------------------------------------------------------
# Subprocess seam. ALL external CLI calls flow through `run()` so tests can
# monkeypatch a single function and configure responses per command.
# ---------------------------------------------------------------------------

def run(cmd: list[str], input_text: Optional[str] = None) -> tuple[int, str, str]:
    """Run a command. Returns (returncode, stdout, stderr). Never raises."""
    proc = subprocess.run(
        cmd,
        input=input_text,
        capture_output=True,
        text=True,
        check=False,
    )
    return proc.returncode, proc.stdout, proc.stderr


# ---------------------------------------------------------------------------
# grava CLI wrappers
# ---------------------------------------------------------------------------

def grava_wisp_read(issue_id: str, key: str) -> Optional[str]:
    """Return wisp value, or None if missing.

    The grava CLI exits 1 on a missing wisp with empty stdout (re-verified
    May 2026; see grava-6ac8 / PR #42 saga). We map both rc!=0 and empty
    stdout to None so callers can use `if val is not None` uniformly.
    """
    rc, out, _ = run(["grava", "wisp", "read", issue_id, key])
    if rc != 0:
        return None
    val = out.strip()
    return val or None


def grava_wisp_write(issue_id: str, key: str, value: str) -> bool:
    rc, _, _ = run(["grava", "wisp", "write", issue_id, key, value])
    return rc == 0


def grava_signal(
    kind: str,
    issue_id: str,
    payload: Optional[str] = None,
    actor: str = "watcher",
) -> bool:
    cmd = ["grava", "signal", kind, "--issue", issue_id, "--actor", actor]
    if payload is not None:
        cmd.extend(["--payload", payload])
    rc, _, _ = run(cmd)
    return rc == 0


def grava_label(
    issue_id: str,
    *,
    add: Optional[list[str]] = None,
    remove: Optional[list[str]] = None,
) -> bool:
    cmd = ["grava", "label", issue_id]
    for label in add or []:
        cmd.extend(["--add", label])
    for label in remove or []:
        cmd.extend(["--remove", label])
    rc, _, _ = run(cmd)
    return rc == 0


def grava_close(issue_id: str, actor: str = "watcher") -> bool:
    rc, _, _ = run(["grava", "close", issue_id, "--actor", actor])
    return rc == 0


def grava_show(issue_id: str) -> Optional[dict[str, Any]]:
    rc, out, _ = run(["grava", "show", issue_id, "--json"])
    if rc != 0 or not out.strip():
        return None
    try:
        return json.loads(out)
    except json.JSONDecodeError:
        return None


def grava_list_pr_created() -> list[str]:
    """Return IDs of issues currently labelled `pr-created`. Empty on any error."""
    rc, out, _ = run(["grava", "list", "--label", "pr-created", "--json"])
    if rc != 0 or not out.strip():
        return []
    try:
        issues = json.loads(out)
    except json.JSONDecodeError:
        return []
    return [item["id"] for item in issues if isinstance(item, dict) and "id" in item]


def grava_commit(message: str) -> bool:
    rc, _, _ = run(["grava", "commit", "-m", message])
    return rc == 0


def grava_comment(issue_id: str, message: str) -> bool:
    rc, _, _ = run(["grava", "comment", issue_id, "-m", message])
    return rc == 0


def grava_update_description_append(issue_id: str, text: str) -> bool:
    rc, _, _ = run(
        ["grava", "update", issue_id, "--description-append-from-stdin"],
        input_text=text + "\n",
    )
    return rc == 0


# ---------------------------------------------------------------------------
# gh CLI wrappers
# ---------------------------------------------------------------------------

def gh_pr_view(pr_number: int, fields: list[str]) -> Optional[dict[str, Any]]:
    rc, out, _ = run(["gh", "pr", "view", str(pr_number), "--json", ",".join(fields)])
    if rc != 0 or not out.strip():
        return None
    try:
        data = json.loads(out)
    except json.JSONDecodeError:
        return None
    return data if isinstance(data, dict) else None


def gh_api_pr_comments(pr_number: int) -> Optional[list[dict[str, Any]]]:
    """Fetch PR review comments. Returns None if response isn't a JSON array.

    grava-431b: gh occasionally returns an error string or a non-array JSON
    object on rate limits / network blips / auth expiry. Validate the shape
    before handing to downstream code so callers can `continue` cleanly.
    """
    rc, out, _ = run(["gh", "api", f"repos/{{owner}}/{{repo}}/pulls/{pr_number}/comments"])
    if rc != 0 or not out.strip():
        return None
    try:
        data = json.loads(out)
    except json.JSONDecodeError:
        return None
    return data if isinstance(data, list) else None


# ---------------------------------------------------------------------------
# PIDFILE handling (grava-24fa)
# ---------------------------------------------------------------------------

def _pid_alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
        return True
    except (OSError, ProcessLookupError):
        return False


def _ps_command(pid: int) -> str:
    rc, out, _ = run(["ps", "-o", "command=", "-p", str(pid)])
    return out.strip() if rc == 0 else ""


def _looks_like_watcher(cmd_line: str) -> bool:
    return "pr_merge_watcher" in cmd_line or "pr-merge-watcher" in cmd_line


def acquire_pidfile(pidfile: str = PIDFILE) -> bool:
    """Try to claim PIDFILE. Returns True on success, False if a real watcher is running.

    grava-24fa: `kill -0 $PID` succeeds for any live PID, including a PID
    that's been recycled and reassigned to an unrelated process (browser,
    daemon) on long-uptime hosts. Verify the PID's command actually looks
    like our watcher before treating it as a live previous run.
    """
    pid_path = Path(pidfile)
    pid_path.parent.mkdir(parents=True, exist_ok=True)

    if pid_path.exists():
        try:
            old_pid = int(pid_path.read_text().strip())
        except (ValueError, OSError):
            old_pid = 0

        if old_pid > 0 and _pid_alive(old_pid):
            cmd = _ps_command(old_pid)
            if _looks_like_watcher(cmd):
                # matrix #5: log so cron tail-f surfaces overlapping ticks.
                LOG.info(
                    "[%s] watcher: previous run (pid %d) still active — skipping this tick",
                    now_iso(), old_pid,
                )
                return False
            LOG.info(
                "[%s] watcher: PIDFILE pid %d is an unrelated process; treating as stale and overwriting",
                now_iso(), old_pid,
            )

    pid_path.write_text(str(os.getpid()))
    return True


def release_pidfile(pidfile: str = PIDFILE) -> None:
    try:
        Path(pidfile).unlink()
    except FileNotFoundError:
        pass


# ---------------------------------------------------------------------------
# Per-state handlers
# ---------------------------------------------------------------------------

def process_merged(issue_id: str, pr_url: str, now: int) -> None:
    grava_wisp_write(issue_id, "pr_merged_at", str(now))
    grava_signal("PR_MERGED", issue_id)
    grava_label(issue_id, remove=["pr-created"])

    # grava-63f3: only emit PIPELINE_COMPLETE if close succeeded OR the
    # issue is already closed (idempotent re-run). Otherwise the pipeline
    # would falsely report complete while the issue board still says
    # in_progress.
    if not grava_close(issue_id):
        info = grava_show(issue_id) or {}
        if info.get("status") != "closed":
            LOG.info(
                "watcher: failed to close %s (status=%s) — leaving for next iteration",
                issue_id, info.get("status"),
            )
            return

    grava_signal("PIPELINE_COMPLETE", issue_id, payload=issue_id)
    grava_commit(f"watcher: {issue_id} merged + closed")


def _build_rejection_notes(
    pr_url: str,
    closed_by: str,
    reason: str,
    changes_requested: str,
    last_comment: str,
    stamp: str,
) -> str:
    return "\n".join([
        "",
        f"## PR Rejection Notes ({stamp})",
        "",
        f"PR: {pr_url}",
        f"Closed by: {closed_by}",
        f"Reason category: {reason}",
        "",
        "### Reviewer feedback (CHANGES_REQUESTED bodies)",
        changes_requested or "_none recorded_",
        "",
        "### Closing comment",
        last_comment or "_none_",
    ])


def process_closed(issue_id: str, pr_number: int, pr_url: str, now: int) -> None:
    """First-time CLOSED detection records rejection reason; subsequent
    iterations idempotent via pr_rejection_recorded gate."""
    if grava_wisp_read(issue_id, "pr_rejection_recorded") is None:
        # Distil rejection reason from gh
        info = gh_pr_view(pr_number, ["reviews", "closedBy", "author"]) or {}
        reviews = info.get("reviews") or []
        cr_bodies = [
            r.get("body", "")
            for r in reviews
            if isinstance(r, dict) and r.get("state") == "CHANGES_REQUESTED"
        ]
        changes_requested = "\n\n---\n\n".join(cr_bodies)[:4096]

        closed_by_obj = info.get("closedBy") or {}
        closed_by = closed_by_obj.get("login") if isinstance(closed_by_obj, dict) else None
        closed_by = closed_by or "unknown"

        author_obj = info.get("author") or {}
        author = author_obj.get("login") if isinstance(author_obj, dict) else ""
        author = author or ""

        comments_info = gh_pr_view(pr_number, ["comments"]) or {}
        comments = comments_info.get("comments") or []
        last_comment = ""
        if comments and isinstance(comments[-1], dict):
            last_comment = (comments[-1].get("body") or "")[:1024]

        if changes_requested:
            reason = "reviewer-rejected"
        elif closed_by == author:
            reason = "author-abandoned"
        else:
            reason = "unknown"

        notes = _build_rejection_notes(
            pr_url=pr_url,
            closed_by=closed_by,
            reason=reason,
            changes_requested=changes_requested,
            last_comment=last_comment,
            stamp=now_iso(),
        )

        # grava-97ec: guard description write — defer the rest if it fails
        # so the rejection notes aren't silently lost. The pr_rejection_recorded
        # gate is NOT yet set, so re-runs retry cleanly.
        if not grava_update_description_append(issue_id, notes):
            LOG.info(
                "watcher: failed to record rejection notes for %s — will retry next iteration",
                issue_id,
            )
            return

        grava_comment(
            issue_id,
            f"PR closed without merge ({reason}). See description for full notes.",
        )

        grava_wisp_write(issue_id, "pr_rejection_notes", notes)
        grava_wisp_write(issue_id, "pr_closed_at", str(now))
        grava_wisp_write(issue_id, "pr_rejection_recorded", "1")

        # Atomic phase + payload via signal CLI (writes pr_close_reason inside
        # the same transaction). Scoped INSIDE the first-time block so re-runs
        # don't re-emit with blank payload.
        grava_signal("PR_CLOSED", issue_id, payload=reason)

    grava_label(issue_id, add=["pr-rejected"], remove=["pr-created"])
    grava_commit(f"watcher: {issue_id} PR closed without merge")


def process_open(
    issue_id: str,
    pr_number: int,
    pr_url: str,
    now: int,
) -> None:
    """OPEN-state branch: stale cap (72h) → comments diff → notify hook."""
    # Stale cap — fallback to NOW when wisp missing (grava-6ac8).
    since_str = grava_wisp_read(issue_id, "pr_awaiting_merge_since")
    try:
        since = int(since_str) if since_str else now
    except ValueError:
        since = now
    age_hrs = (now - since) // 3600
    if age_hrs >= MAX_PR_WAIT_HOURS:
        grava_wisp_write(issue_id, "pr_stale", "true")
        grava_label(issue_id, add=["needs-human"])
        grava_commit(f"watcher: {issue_id} stale (>{MAX_PR_WAIT_HOURS}h)")
        return

    # New comments — gh validation gate (grava-431b)
    comments = gh_api_pr_comments(pr_number)
    if comments is None:
        LOG.info(
            "[%s] watcher: gh api returned non-array for %s PR_NUMBER=%d — skipping comment check this tick",
            now_iso(), issue_id, pr_number,
        )
        return

    last_seen_str = grava_wisp_read(issue_id, "pr_last_seen_comment_id")
    try:
        last_seen = int(last_seen_str) if last_seen_str else 0
    except ValueError:
        last_seen = 0

    new = [
        c for c in comments
        if isinstance(c, dict)
        and c.get("in_reply_to_id") is None
        and (_safe_int(c.get("id")) or 0) > last_seen
    ]
    new_count = len(new)

    review = gh_pr_view(pr_number, ["reviewDecision"]) or {}
    review_decision = review.get("reviewDecision", "")

    if new_count > 0 or review_decision == "CHANGES_REQUESTED":
        ids = [_safe_int(c.get("id")) or 0 for c in comments if isinstance(c, dict)]
        highest = max(ids) if ids else 0
        grava_wisp_write(issue_id, "pr_new_comments", json.dumps(new))
        grava_wisp_write(issue_id, "pr_last_seen_comment_id", str(highest))
        grava_commit(f"watcher: {issue_id} new PR comments ({new_count})")

        notify = Path("scripts/hooks/notify-pr-comments.sh")
        if notify.is_file() and os.access(notify, os.X_OK):
            run([str(notify), issue_id, pr_url])


def _safe_int(v: Any) -> Optional[int]:
    if isinstance(v, int):
        return v
    if isinstance(v, str):
        try:
            return int(v)
        except ValueError:
            return None
    return None


# ---------------------------------------------------------------------------
# Main loop
# ---------------------------------------------------------------------------

def process_issue(issue_id: str, now: int) -> None:
    pr_number_str = grava_wisp_read(issue_id, "pr_number")
    if not pr_number_str:
        return
    try:
        pr_number = int(pr_number_str)
    except ValueError:
        return
    pr_url = grava_wisp_read(issue_id, "pr_url") or ""

    info = gh_pr_view(pr_number, ["state"])
    state = (info or {}).get("state", "") or ""

    if state == "MERGED":
        process_merged(issue_id, pr_url, now)
    elif state == "CLOSED":
        process_closed(issue_id, pr_number, pr_url, now)
    else:
        # OPEN (or unknown — preserve bash behaviour: any non-MERGED non-CLOSED
        # state falls through to the open-branch checks).
        process_open(issue_id, pr_number, pr_url, now)


def main(argv: Optional[list[str]] = None) -> int:
    logging.basicConfig(format="%(message)s", level=logging.INFO, stream=sys.stderr)

    repo_root = os.environ.get("CLAUDE_PROJECT_DIR") or os.getcwd()
    try:
        os.chdir(repo_root)
    except OSError:
        return 1

    if not acquire_pidfile():
        return 0
    try:
        now = now_unix()
        for issue_id in grava_list_pr_created():
            try:
                process_issue(issue_id, now)
            except Exception as exc:  # noqa: BLE001 — never let one bad issue kill the tick
                LOG.exception("watcher: error processing %s: %s", issue_id, exc)
        return 0
    finally:
        release_pidfile()


if __name__ == "__main__":
    sys.exit(main())
