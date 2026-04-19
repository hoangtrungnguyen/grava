# Eval 3: Empty Backlog -- grava-next-issue Skill Evaluation

**Date:** 2026-04-18
**Scenario:** User says "what should I work on next" with an empty backlog (no ready issues, no open stories)
**Skill:** grava-next-issue (SKILL.md)

---

## Commands Run and Output

### Phase 1: Discover

**Command:** `grava ready --limit 3 --json`
**Output:**
```
[]
```
**Result:** No ready issues found. Empty JSON array returned.

### Fallback: Check for open stories

**Command:** `grava list --status open --type story`
**Output:**
```
Using config file: /Users/trungnguyenhoang/IdeaProjects/grava/.grava.yaml
ID  Title  Type  Priority  Status  Created
```
**Result:** No open stories found. Only the header row was printed (no data rows).

### Additional diagnostics (for evaluation context)

**Command:** `grava list --status in_progress`
**Output:**
```
ID          Title                                              Type  Priority  Status       Created
grava-d3c3  Validate priority value range in create command    task  2         in_progress  2026-04-18
grava-80d2  Add table header separator to list command output  task  2         in_progress  2026-04-18
grava-18b2  Add --format flag to grava stats command           task  2         in_progress  2026-04-18
```
**Result:** 3 in_progress tasks exist but these are not eligible for claiming.

**Command:** `grava list --status open`
**Output:**
```
ID  Title  Type  Priority  Status  Created
```
**Result:** Zero open issues of any type.

**Command:** `grava stats`
**Output:**
```
Total Issues:       61
Open Issues:        3
Closed Issues:      58
Blocked Issues:     0
Stale In-Progress:  1
```
Note: "Open Issues: 3" in stats refers to the 3 in_progress issues (stats groups in_progress under "open"). All 58 other issues are closed.

### Session Summary (as the skill instructs)

```
--- Session Complete ---
Issues completed: 0
Issues remaining: 0 -- backlog drained
Stopped because: Backlog drained. No more ready issues.
```

---

## Evaluation

### Was the empty-backlog message clear?

**Verdict: Mostly clear, but could be improved.**

The skill's instruction is: *"If still nothing: stop the loop. Report: 'Backlog drained. No more ready issues.'"*

This is concise and unambiguous about what to do -- stop and print a message. However, there are a few gaps:

1. **No mention of in_progress work.** In this scenario, there are 3 in_progress issues. A user hearing "backlog drained" might think all work is done, when in fact 3 tasks are actively being worked on. The message should acknowledge existing in_progress work so the user doesn't think the project is idle.

2. **The "Backlog drained" language is slightly misleading.** The backlog isn't truly drained -- it has in_progress items. A more accurate message would be: "No claimable issues. 3 issues are currently in progress."

3. **No guidance on what to do next.** The user asked "what should I work on next" and the answer is effectively "nothing." The skill doesn't suggest any next actions (e.g., "check if in_progress issues need help," "create new issues," "review completed work," etc.).

### Was the fallback (`grava list --status open --type story`) useful?

**Verdict: Partially useful, but too narrow.**

The fallback only checks for open **stories**. This misses:
- Open **bugs** (which could also be valid work)
- Open **tasks** (which could also be valid work)
- Open **epics** that need stories broken out

In this test, `grava list --status open` (without `--type story`) returned the same empty result, so the narrowness didn't cause a missed issue. But in a scenario where there are open bugs but no open stories, the fallback would give a false "backlog drained" result.

**Recommendation:** The fallback should be `grava list --status open` (all types) or should chain multiple type checks.

### Suggestions for Improving the Empty-Backlog Experience

1. **Show in_progress context.** When reporting "backlog drained," also run `grava list --status in_progress` and display those issues so the user knows what's actively being worked on. Example:

   ```
   Backlog drained. No more ready issues.

   Currently in progress (3):
     - grava-d3c3: Validate priority value range in create command
     - grava-80d2: Add table header separator to list command output
     - grava-18b2: Add --format flag to grava stats command
   ```

2. **Broaden the fallback.** Change `grava list --status open --type story` to `grava list --status open` so bugs and tasks are also surfaced.

3. **Check for stale in_progress.** The stats show `Stale In-Progress: 1`. The skill could suggest investigating stale work as a next action instead of just stopping.

4. **Provide actionable next steps.** Instead of just "backlog drained," offer suggestions:
   - "Run `grava list --status in_progress` to check active work"
   - "Check if any in_progress issues are stale and need to be unblocked"
   - "Create new issues if there's planned work to add"
   - "Run a code review on issues labeled `code_review`"

5. **Include stats in the session summary.** When stopping due to empty backlog, show a quick snapshot from `grava stats` so the user has context about the overall project state.

6. **Handle the edge case where `grava ready` returns `[]` vs an error.** The skill doesn't distinguish between "no results" and "command failed." Both produce different outputs but the skill treats them the same way. An explicit check for errors would make it more robust.

### Overall Assessment of Empty-Backlog Edge Case Handling

**Rating: 6/10 -- Functional but minimal.**

**What works well:**
- The two-step discovery (ready -> fallback) is a reasonable approach
- The stop condition is clearly defined -- no ambiguity about whether to loop or stop
- The session summary format provides useful structure
- The "Backlog drained" language is immediately understandable

**What needs improvement:**
- The skill is too "loop-focused" and treats the empty state as a termination condition rather than an informative state. For a user asking "what should I work on next," getting "Backlog drained" with zero additional context is a poor experience.
- The fallback is too narrow (stories only)
- No awareness of in_progress work that might need help or review
- No actionable suggestions for the user
- The session summary (`Issues completed: 0, Issues remaining: 0`) is technically correct but uninformative -- it says "we did nothing and there's nothing to do" without mentioning the 3 items actively in flight

**The core issue:** The skill is designed as an autopilot work loop, so the empty-backlog path is treated as a simple exit. But when a user specifically asks "what should I work on next," the empty-backlog response needs to be more of a status report and less of a loop terminator. The skill would benefit from a richer "nothing to claim" state that gives the user situational awareness and options.
