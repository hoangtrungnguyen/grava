---
title: 'Grava Dev Task — Definition of Done'
validation-target: 'Single task issue (grava `task` type or subtask)'
validation-criticality: 'HIGH'
required-inputs:
  - 'Claimed task issue with `in_progress` status assigned to current actor'
  - 'Implementation diff scoped to the task'
  - 'New/updated tests covering the task behavior'
optional-inputs:
  - 'Test output (scoped run)'
  - 'Lint output (scoped run)'
validation-rules:
  - 'Scope stays at the single task — no decomposition into subtasks, no rolling forward to siblings'
  - 'Every acceptance criterion in the task description satisfied'
  - 'Scoped tests pass; no regressions in directly-touched packages'
  - 'Issue labeled `code_review` and last_commit recorded before declaring done'
---

# Grava Dev Task — Definition of Done

A task is done only when ALL items pass. If any fail, fix before labeling `code_review`.

## Pre-Claim Gates (Step 2)

- [ ] **Scope validation passed:** issue type confirmed `task` (or other leaf) — not a `story`/`epic` with subtasks
- [ ] **Spec-presence gate passed:** description ≥30 chars OR a comment scopes the work OR the parent's description scopes it

## Scope Discipline

- [ ] **Single task only:** no sibling tasks claimed, no scope expansion beyond the task description
- [ ] **No decomposition:** task implemented as one unit; if it felt big, you halted and asked the user instead
- [ ] **Parent identified (when applicable):** parent ID resolved via `.<n>` suffix inference and parent description read for context — or explicitly noted "no parent"

## Implementation Completion

- [ ] **Acceptance criteria:** every AC in the task description is satisfied by the diff
- [ ] **Edge cases:** error and edge cases stated in the task are handled
- [ ] **Dependencies:** no new external dependencies added (or, if added, user explicitly approved)

## Testing & Quality

- [ ] **New tests:** tests added/updated for the behavior this task introduces or changes
- [ ] **Scoped pass:** tests in the touched package(s) and direct callers all pass
- [ ] **No scoped regressions:** no existing test in the touched scope regressed
- [ ] **Lint/static checks:** scoped lint/vet runs clean (when configured)

## Grava State

- [ ] **last-commit recorded:** `grava update <id> --last-commit <hash>` ran and points at the implementation commit
- [ ] **Completion comment:** `grava comment <id>` records implementation and short hash
- [ ] **`code_review` label added:** the hand-off signal is set
- [ ] **Wisp checkpoint:** final `grava wisp write <id> step "complete"` written
- [ ] **Grava state committed:** `grava commit -m "complete <id> ..."` ran

## Final Status

- [ ] **No HALT conditions outstanding:** nothing blocking, nothing partial
- [ ] **Summary printed:** end-of-task summary block emitted for the user

## Output

```
Definition of Done: {{PASS/FAIL}}

Task: {{issue-id}} — {{title}}
Parent: {{parent-id}}
Commit: {{short-hash}}
Tests (scoped): {{pass}}/{{total}}
Label: code_review
```

**If FAIL:** list the failing items and what needs to happen before the task can be labeled `code_review`.

**If PASS:** task is ready. The reviewer (different LLM recommended) picks it up via the `code_review` label.
