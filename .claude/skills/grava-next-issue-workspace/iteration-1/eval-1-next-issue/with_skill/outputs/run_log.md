# Grava Next Issue Skill - Evaluation Run Log

**Date:** 2026-04-18
**Trigger phrase:** "next issue"
**Skill:** grava-next-issue (SKILL.md)
**Model:** Claude Opus 4.6 (1M context)

---

## Skill Trigger

The skill was read from `/Users/trungnguyenhoang/IdeaProjects/grava/.claude/skills/grava-next-issue/SKILL.md`. The trigger description matches "next issue" correctly. The skill was recognized and its instructions followed.

**Verdict:** Skill triggered correctly from the "next issue" prompt.

---

## Phase 1: Discover

### Command: `grava ready --limit 3 --json`

**Output:**
```json
[
  {
    "Node": {
      "ID": "grava-66ba",
      "Title": "Add missing godoc to reserve.go exported functions",
      "Type": "task",
      "Status": "open",
      "Priority": 2
    },
    "EffectivePriority": 2,
    "Age": 1406110767000,
    "PriorityBoosted": false
  },
  {
    "Node": {
      "ID": "grava-18b2",
      "Title": "Add --format flag to grava stats command",
      "Type": "task",
      "Status": "open",
      "Priority": 2
    },
    "EffectivePriority": 2,
    "Age": 1401110767000,
    "PriorityBoosted": false
  }
]
```

**Candidates displayed:**

| # | ID | Title | Type | Priority |
|---|-----|-------|------|----------|
| 1 | grava-66ba | Add missing godoc to reserve.go exported functions | task | 2 |
| 2 | grava-18b2 | Add --format flag to grava stats command | task | 2 |

**Auto-selected:** #1 (grava-66ba) per skill instructions (highest effective priority, oldest age as tiebreaker).

**Phase 1 Result:** Worked as expected. Candidates were discovered and ranked correctly.

---

## Phase 2: Claim

### Candidate 1: grava-66ba

**Command:** `grava show grava-66ba --json`

Revealed that grava-66ba already had:
- Prior implementation by another agent (Amelia/Gemini-3-Flash)
- `code_review` label already applied
- Comments saying "Implementation complete"
- A `last_commit` already set

**Prerequisites check:** File `pkg/cmd/reserve/reserve.go` exists. No external services needed. Prerequisites pass.

**Command:** `grava claim grava-66ba`

**Output:**
```
Error: worktree directory /Users/trungnguyenhoang/IdeaProjects/grava/.worktree/grava-66ba already exists
```

**Result:** Claim failed because a worktree already existed from prior work. Per skill instructions ("If the claim fails, try the next candidate"), moved to candidate #2.

**Issue noted:** The skill says "prerequisites not met" as the reason to skip, but the actual failure here was a worktree collision -- a different failure mode. The skill could be more explicit about worktree conflicts as a claim failure reason.

### Candidate 2: grava-18b2

**Command:** `grava show grava-18b2 --json`

Clean issue -- no prior work, no labels, no comments.

**Prerequisites check:** Task requires modifying `pkg/cmd/graph/graph.go`. File exists. No external services needed. Prerequisites pass.

**Command:** `grava claim grava-18b2`

**Output:**
```
Claimed grava-18b2 (status: in_progress, actor: Amelia)
Provisioned worktree: .worktree/grava-18b2 (branch: grava/grava-18b2)
```

**Post-claim setup:**

**Command:** `grava wisp write grava-18b2 status "claimed"`

Note: The skill's syntax `--key "status" --value "claimed"` was wrong. The actual CLI syntax is positional: `grava wisp write <id> <key> <value>`. Had to check `grava wisp write --help` to discover the correct syntax.

**Command:** `grava show grava-18b2` -- confirmed status is `in_progress`, assignee is set.

**Phase 2 Result:** Worked, but with two issues:
1. First candidate failed due to worktree collision (not a prerequisite failure)
2. The wisp write syntax in the grava-claim skill (`--key` / `--value` flags) is incorrect -- the CLI uses positional arguments

---

## Phase 3: Implement (Lightweight)

Following grava-dev-epic workflow from Step 2 onward.

### Step 2: Load Context

**Commands run:**
- `grava show grava-18b2 --tree` -- no subtasks
- `grava dep tree grava-18b2` -- no dependencies
- `grava dep list grava-18b2 --json` -- returned error `NODE_NOT_FOUND` / "issue list not found" (no deps is an error, not empty list -- minor CLI issue)
- `grava history grava-18b2` -- clean history, no prior review feedback
- `git log --oneline -20` -- checked recent commits for context

**Context loaded:** The stats command is in `pkg/cmd/graph/graph.go` (lines 886-1093). It currently supports `--json` flag and default table output. Need to add `--format` flag.

**Wisp checkpoint:** `grava wisp write grava-18b2 step "context-loaded"`

### Step 3: Plan Implementation

Small task -- no subtask decomposition needed. Single implementation unit:
- Add `statsFormat` variable and `--format` flag
- Add CSV rendering logic
- Modify JSON check to use format resolution (`--json` becomes shorthand for `--format json`)

### Step 4: Implement

**Changes made to `/Users/trungnguyenhoang/IdeaProjects/grava/.worktree/grava-18b2/pkg/cmd/graph/graph.go`:**

1. Added `"encoding/csv"` import
2. Added `statsFormat string` variable to the package-level var block
3. Replaced the output rendering section (lines 1014-1093) with a format switch:
   - `format := statsFormat` with `--json` shorthand override
   - `case "json"`: existing JSON output (unchanged)
   - `case "csv"`: new CSV output using `encoding/csv` writer with metric/key/value columns
   - `default` ("table"): existing table output (unchanged)
4. Added `cmd.Flags().StringVar(&statsFormat, "format", "table", "Output format: json, table, or csv")`

**Build check:** `go build ./...` -- passed (no errors)

**Test check:** `go test ./pkg/cmd/graph/... -v -run Stats` -- all 5 existing stats tests passed:
- TestStatsCmd_TextOutput
- TestStatsCmd_JSONOutput
- TestStatsCmd_ZeroBlockedAndStale
- TestStatsCmd_ZeroCycleTime
- TestStatsCmd_NoCycleTimeData

**Wisp checkpoint:** `grava wisp write grava-18b2 step "implemented"`

### Step 5: Validation

**Full test suite:** `go test ./...` -- ALL packages passed (27 packages tested, 0 failures)

**Code quality:** `go vet ./...` -- passed with no issues

**Acceptance criteria check:**
- "Add a --format flag that accepts 'json', 'table', and 'csv'" -- DONE
- "Default to 'table' when --format is not specified" -- DONE (default value is "table")
- "Keep --json as a shorthand for --format json" -- DONE (format resolution logic)

**Wisp checkpoint:** `grava wisp write grava-18b2 step "validated"`

### Step 7: Complete and Hand Off

**Git commit:**
```
git add pkg/cmd/graph/graph.go
git commit -m "chore(stats): add --format flag supporting json, table, csv (grava-18b2)"
```
Commit hash: `f1461e6e0cdd75ffdc4a068c59253e2c6f47440c`

**Post-commit grava commands:**
- `grava update grava-18b2 --last-commit "f1461e6e0cdd75ffdc4a068c59253e2c6f47440c"` -- success
- `grava label grava-18b2 --add code_review` -- success
- `grava comment grava-18b2 -m "Implementation complete. Commit: f1461e6. Ready for code review."` -- success
- `grava commit -m "complete grava-18b2: ready for code review (commit: f1461e6)"` -- success (Dolt hash: dj3tntvr2j4qg0b7pi573t025sqh9rgs)
- `grava wisp write grava-18b2 step "complete"` -- success

**Phase 3 Result:** Worked as expected. Lightweight implementation completed successfully.

---

## Final Issue State

```
ID:          grava-18b2
Title:       Add --format flag to grava stats command
Type:        task
Priority:    medium (2)
Status:      in_progress
Assignee:    Amelia
Last Commit: f1461e6e0cdd75ffdc4a068c59253e2c6f47440c
Labels:      [code_review]
Comments:    2 (completion comments added)
```

---

## Session Summary

```
--- Session Complete ---
Issues completed: 1
  - grava-18b2: Add --format flag to grava stats command
Issues remaining: 1 (grava-66ba -- already has prior implementation, worktree collision)
Stopped because: Test evaluation -- instructed to stop after one issue
```

---

## Evaluation Summary

### What Worked Well

1. **Phase 1 (Discover):** `grava ready --limit 3 --json` worked perfectly. Output was clear, candidates were ranked correctly by priority and age.

2. **Phase 2 (Claim):** The fallback-to-next-candidate logic worked as designed when the first candidate failed. The claim command itself worked cleanly. Wisp heartbeat and confirmation all worked.

3. **Phase 3 (Implement):** The grava-dev-epic workflow was straightforward to follow. Context loading, implementation, validation, commit, and handoff all worked smoothly. All grava CLI commands (wisp write, comment, update, label, commit) worked as expected.

4. **Overall flow:** The three-phase structure (Discover -> Claim -> Implement) is clean and logical. The skill provides clear instructions at each step.

### Issues and Confusion

1. **Wisp write syntax mismatch:** The grava-claim skill says `grava wisp write <id> --key "status" --value "claimed"` but the actual CLI uses positional args: `grava wisp write <id> <key> <value>`. This caused an initial error.

2. **Claim failure reason:** The skill says to skip candidates when "prerequisites not met", but the actual failure for grava-66ba was a worktree collision (`worktree directory already exists`). The skill doesn't explicitly cover this failure mode. It might be worth adding worktree conflicts as a known claim failure reason.

3. **dep list error on no deps:** `grava dep list grava-18b2 --json` returned an error (`NODE_NOT_FOUND`) rather than an empty list when no dependencies exist. This is a minor CLI usability issue -- empty results should return `[]`, not an error.

4. **Already-worked issue in ready queue:** grava-66ba appeared as "ready" (status: open) even though it had implementation complete, code_review label, and a last_commit. This suggests the issue should have been moved to a different status after the prior agent's work. The skill doesn't account for issues that are technically "open" but already worked on.

5. **No explicit step to enter worktree:** The skill says to work in the worktree created by `grava claim`, but doesn't explicitly instruct you to `cd` into it or reference files there. I figured this out but it could be stated more explicitly.

6. **grava-dev-epic Step 6 (DoD checklist):** The checklist.md is heavily oriented toward story files with sections like "Dev Agent Record", "File List", "Change Log" etc. For simple tasks (not stories), many items don't apply. The skill could note that the DoD checklist should be adapted based on issue type.

### Recommendations

- Fix the wisp write syntax in grava-claim SKILL.md (positional args, not flags)
- Add worktree collision to the list of claim failure reasons in grava-next-issue
- Consider having `grava dep list` return empty array instead of error when no deps exist
- Add a note about working in the worktree directory during implementation
- Consider a lighter DoD checklist path for task-type issues vs story-type issues
