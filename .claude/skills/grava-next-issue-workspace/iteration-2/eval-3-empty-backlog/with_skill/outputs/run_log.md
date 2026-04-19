# grava-next-issue Skill Evaluation: Empty Backlog Scenario (Iteration 2, Eval 3)

**Date:** 2026-04-18
**Skill version:** iteration-2
**Scenario:** User asks "what should I work on next" with a ready queue full of stale/already-implemented issues
**Model:** Claude Opus 4.6 (1M context)

---

## Commands Executed and Output

### Step 1: Discover -- `grava ready --limit 3 --json`

**Result:** 3 issues returned:

| ID | Title | Type | Priority | Age |
|----|-------|------|----------|-----|
| grava-18b2 | Add --format flag to grava stats command | task | 2 (medium) | ~22h |
| grava-80d2 | Add table header separator to list command output | task | 2 (medium) | ~21h |
| grava-d3c3 | Validate priority value range in create command | task | 2 (medium) | ~21h |

### Step 2: Stale Check -- `grava show <id> --json` for each candidate

**grava-18b2:**
- Labels: `["code_review"]`
- Comments: "Completed: added --format flag..." / "Implementation complete. Commit: f1461e6. Ready for code review."
- **Verdict: STALE -- already implemented, skip**

**grava-80d2:**
- Labels: `["code_review"]`
- Comments: "Implementation complete. Commit: 65107bf. Ready for code review."
- **Verdict: STALE -- already implemented, skip**

**grava-d3c3:**
- Labels: `["code_review"]`
- Comments: "Implementation complete. Commit: 0913757. Ready for code review."
- **Verdict: STALE -- already implemented, skip**

All 3 candidates are already-implemented with `code_review` labels. Per SKILL.md: "If all candidates fail, report blockers and stop the loop."

### Step 3: Fallback -- Check open issues across all types

```bash
grava list --status open --json
```

**Result:** 3 open issues (same 3 stale ones from the ready queue). All have `code_review` labels and implementation commits. None are actionable.

### Step 4: Fallback -- Check in-progress issues

```bash
grava list --status in_progress --json
```

**Result:** 1 in-progress issue:

| ID | Title | Type | Priority | Status | Assignee |
|----|-------|------|----------|--------|----------|
| grava-e814 | Add --verbose flag to grava show command | task | 2 | in_progress | Amelia |

This issue also has a `code_review` label and an "Implementation complete" comment (commit: 16dc7b8), suggesting it's done but status wasn't transitioned to closed.

### Step 5: Stats -- `grava stats --json`

```json
{
  "total_issues": 62,
  "open_issues": 4,
  "closed_issues": 58,
  "blocked_count": 0,
  "stale_in_progress_count": 0,
  "by_status": { "closed": 58, "in_progress": 1, "open": 3 },
  "by_priority": { "1": 15, "2": 46, "3": 1 },
  "by_author": { "Amelia": 51, "htnguyen": 10, "unknown": 1 },
  "created_by_date": {
    "2026-04-12": 3, "2026-04-13": 7, "2026-04-17": 30,
    "2026-04-18": 7, "2026-04-19": 1
  },
  "closed_by_date": {
    "2026-04-12": 3, "2026-04-13": 10, "2026-04-17": 31, "2026-04-18": 4
  }
}
```

---

## Session Report (as SKILL.md prescribes)

```
--- Session Complete ---
Issues completed: 0
Issues skipped: 3
  - grava-18b2: Already implemented (code_review label, commit f1461e6)
  - grava-80d2: Already implemented (code_review label, commit 65107bf)
  - grava-d3c3: Already implemented (code_review label, commit 0913757)
In progress (other agents): 1
  - grava-e814: Add --verbose flag to grava show command (Amelia/Gemini-3-Flash)
Issues remaining in ready queue: 0 (3 shown as ready but all stale)
Stopped because: All candidates already implemented -- nothing claimable
```

### Overall Stats
- 62 total issues, 58 closed (93.5% completion rate)
- 4 remaining (3 open + 1 in_progress), all appear to be awaiting code review, not new work
- 0 blocked issues
- Recommendation: Run code reviews on the 4 pending issues to close them out. No new implementation work available.

---

## Quality Assessment

### Does the broadened fallback (all types, not just stories) work?

**YES.** The SKILL.md instructs `grava list --status open --json` (no type filter), which returns all issue types. The 3 open issues are all type `task`. In iteration-1, if the skill only looked for stories, these tasks would have been invisible. The broadened fallback correctly surfaces them.

### Does it show in_progress issues?

**YES.** The skill explicitly checks `grava list --status in_progress --json` and found grava-e814. This gives the user visibility into work that's still being tracked elsewhere (Amelia/Gemini-3-Flash agent).

### Does it run `grava stats` for context?

**YES.** Stats were retrieved and included: 62 total, 58 closed, 4 remaining, 0 blocked. This gives the user a clear picture of overall project health.

### Is the response actionable vs just "nothing found"?

**YES -- significantly improved.** Instead of a bare "nothing found", the skill produces:

1. **Stale detection** -- identifies WHY the ready queue is empty (all candidates already implemented, not "no issues exist")
2. **Skip report** -- lists each skipped issue with the specific reason
3. **In-progress visibility** -- shows what's still being worked on
4. **Stats summary** -- 62 total / 58 closed / 93.5% completion gives project context
5. **Actionable recommendation** -- "Run code reviews on pending issues to close them out"

### Comparison to Iteration-1 Behavior

| Aspect | Iteration 1 | Iteration 2 |
|--------|-------------|-------------|
| Stale detection | Not specified; agent might spin trying to claim stale issues | Explicit: check labels/comments before claiming, skip if code_review |
| Fallback scope | Stories only (missed tasks, bugs, subtasks) | All types via `grava list --status open --json` |
| In-progress visibility | Not checked | Explicitly checked and reported |
| Stats context | Not gathered | `grava stats --json` provides project-wide numbers |
| Empty response quality | "No ready issues" with no context | Full session report with skip reasons, in-progress work, stats, recommendations |
| Spin prevention | Could loop on unclaimable stale issues | "If all candidates fail, report blockers and stop the loop" |

### Edge Case: Status vs Label Mismatch

One notable finding: grava-e814 is `in_progress` status but has `code_review` label and "Implementation complete" comment. This is a status/label mismatch -- the issue is done but wasn't transitioned. The skill correctly surfaces this for the user, who can then take action (run code review, close it).

Similarly, the 3 open issues (grava-18b2, grava-80d2, grava-d3c3) are `open` status but already implemented with `code_review` labels. This suggests either the status transition was skipped or the code review workflow hasn't consumed them yet.

### Overall Grade: PASS

The iteration-2 skill handles the empty/stale backlog scenario well. It does not spin, does not attempt to claim already-implemented issues, and produces an actionable report with full context. The broadened fallback, in-progress check, and stats gathering all work as designed.
