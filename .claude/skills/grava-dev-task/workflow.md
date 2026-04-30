# Grava Dev Task Workflow

**Goal:** Implement ONE grava task issue (leaf-level: type `task` or a subtask) using TDD, tracking all progress through the grava CLI.

**Role:** Developer implementing a single, scoped unit of work — not a full story.

## Ground Rules

- Execute steps in order. Do NOT skip.
- One task only. Do NOT auto-pick up the next sibling task when done.
- All issue state changes go through `grava` CLI — never edit Dolt directly.
- Use `--json` when parsing grava output programmatically.
- Use `grava wisp write` for crash-recovery checkpoints throughout.
- Tests are **scoped**: run only tests covering the files you touched (and any direct callers). Full-suite is the parent story's job.

---

## Step 1: Resolve the Task ID

### 1a. ID provided

Skip to Step 2 (Fetch).

### 1b. Story file path / story ID provided

Read the story to find candidate **subtasks** (status `open`, type `task`). Multiple → list and ask user to pick. Do NOT auto-pick.

### 1c. Nothing provided

Prefer the typed list (filtered server-side):

```bash
grava list --type task --status open --json
```

Fallback if your grava version differs:

```bash
grava ready --limit 10 --json
# then filter results to entries where .type == "task"
```

Present 3-5 candidates to user with id + title + priority. Wait for the user's pick. Do NOT auto-claim the top result.

If no ready task issues, HALT: *"No ready task-level issues. Either decompose a story first or specify an ID."*

---

## Step 2: Fetch the Issue and Validate Scope

```bash
grava show <issue-id> --json
```

### Scope check (CRITICAL)

Inspect `.type` from the JSON:

- `task` (or other leaf type) → proceed.
- `story` or `epic` AND has subtasks → HALT: *"This is a `<type>` with subtasks — use `grava-dev-epic` instead, or pick a specific subtask."* List subtasks via `grava show <id> --tree` to help the user pick.
- `story` with NO subtasks AND clearly small → ask user *"Implement as single task? (y/n)"* — proceed only on confirmation.

### Spec-presence check (CRITICAL — do BEFORE claiming)

Claiming mutates state. Don't claim something you can't act on. Concrete heuristic:

- `.description` is non-empty (≥30 chars of intent), OR
- `.comments` contains at least one body that scopes the work, OR
- The parent issue's description scopes this task unambiguously (see Step 3 for parent lookup if needed).

If NONE pass, HALT before claiming:

> *"Issue `<id>` has no acceptance criteria, description, or scoping context I can act on. I haven't claimed it. Add a description / AC, or point me at the spec."*

Why: `grava claim` first + HALT later leaves the issue stuck `in_progress`, blocking ready-queue ordering. Confirming spec exists is cheap; unwinding a stale claim is not.

---

## Step 3: Claim and Load Context

### Resume detection

If `.status` is already `in_progress` and `.assignee` is the current actor, this is a resume. Read the wisp checkpoint:

```bash
grava wisp read <issue-id>
```

Use the recovered `step` value to skip back to the right place in this workflow. Do not re-claim.

If `.status` is `in_progress` and assigned to someone else, HALT.

### Claim (fresh start only)

```bash
grava claim <issue-id>
grava wisp write <issue-id> step "claimed"
```

**Worktree heads-up.** `grava claim` may auto-provision a git worktree at `.worktree/<issue-id>` and switch the active branch there. Implementation work (RED/GREEN/REFACTOR, scoped tests, the `git commit`) happens **inside the worktree**. But all `grava ...` subcommands must be invoked from the **repo root**, not the worktree — running them inside the worktree fails with *"failed to connect to database"* because the dolt config sits at the root. Pattern:

```bash
# code/test/commit happens here
cd .worktree/<issue-id>

# grava state changes happen here
( cd /path/to/repo-root && grava <subcommand> ... )
```

The worktree may also contain freshly-provisioned `.claude/` or other harness artifacts. Don't stage them — Step 7 says "stage only files this task touched", which excludes provisioning noise.

### Parent context (lightweight)

Grava's CLI does not expose parent_id directly. Two reliable paths:

1. **ID prefix inference (preferred for subtasks):** if the ID has a `.<n>` suffix (e.g. `grava-d217.1`), the parent ID is the prefix (`grava-d217`). Fetch `grava show <parent-id> --json`. If `.description` is non-empty, read it for the broader "why". If empty, just note "parent has no description" and move on — don't block.
2. **No prefix:** the issue may be a top-level task with no parent. Skip parent context.

Note: `grava dep tree <leaf>` only shows the issue itself for a leaf — not the parent. Don't rely on it for parent lookup.

### Issue audit + review continuation

```bash
grava history <issue-id>
```

If comments contain prior review feedback or `code_review` was previously added then removed, treat as a review continuation — address review findings first.

### Project context

- `**/project-context.md` — coding standards, architecture
- `**/CLAUDE.md` — project instructions
- `git log --oneline -10` — recent related changes

### Checkpoint

```bash
grava wisp write <issue-id> step "context-loaded"
```

Output a brief summary:
```
Task:       <id> — <title>
Parent:     <parent-id> — <parent-title> (or "none")
Files:      <expected files to touch>
Strategy:   <one-line approach>
```

---

## Step 4: Implement (Red-Green-Refactor)

A task is one logical unit. Do NOT decompose into subtasks — if it feels big enough to need that, HALT and tell the user the task should be split at story level first.

### Pre-implementation checkpoint

```bash
grava wisp write <issue-id> current_task "<short description>"
```

### RED — failing test first

- Write test(s) covering the behavior the task adds/changes.
- Run **only** the new test(s); confirm they fail.
- Infer test framework from project structure:
  - Go: `go test`
  - Node: `npm test` or `npx jest`
  - Python: `pytest <path>` — if `pytest` not on PATH, fall back to `python3 -m pytest <path>`

### GREEN — minimal implementation

- Minimum code to pass the test.
- Re-run the new test(s); confirm pass.
- Handle error/edge cases stated in the task.

### REFACTOR

- Improve structure while keeping tests green.
- Match project conventions.

### HALT conditions

- New external dependency required → HALT, ask user.
- 3 consecutive implementation failures → HALT, request guidance.
- Required configuration missing → HALT, explain what's needed.
- Task scope unclear or seems to span multiple stories → HALT, ask user to clarify.
- Task expands beyond stated AC during work → HALT, surface scope creep.

### Keep going

Within a single task, do NOT pause between RED→GREEN→REFACTOR. Push straight through.

---

## Step 5: Run Scoped Validation

Full-suite regression is the parent story's responsibility. For a single task, validate scoped only.

### Scoped tests

Run tests in the package(s) you touched, plus any package that directly imports them.

```bash
# Go
go test ./path/to/touched/pkg/...
go test ./path/to/direct/caller/...

# Node
npm test -- --findRelatedTests <changed-files>

# Python
pytest path/to/touched/tests/
# fallback if pytest not on PATH:
python3 -m pytest path/to/touched/tests/
```

- All scoped tests must pass.
- If you broke a direct caller's test, fix it before proceeding (still in-scope).
- If a more distant test fails, note it in a comment and let the parent story's full-suite catch it. Do NOT chase regressions outside the task scope unless user instructs.

### Code quality (scoped)

```bash
# Go
go vet ./path/to/touched/...
golangci-lint run ./path/to/touched/...

# Node
npx eslint <changed-files>
```

### Acceptance criteria check

Re-read the task description. Every AC must be satisfied. If not, return to Step 4.

### Checkpoint

```bash
grava wisp write <issue-id> step "validated"
```

---

## Step 6: Definition of Done

Run [checklist.md](./checklist.md). If any item fails, fix before proceeding.

---

## Step 7: Commit and Hand Off for Review

### Stage and commit

```bash
git status
git diff --stat
```

Conventional commit message:
- Map task intent → `feat` / `fix` / `chore` / `docs` / `test` (read the description; don't blindly map by type field).
- Include the issue ID in the message subject or trailer.
- Reference parent ID if relevant.

Stage only files this task touched. No secrets, no unrelated work. Commit.

### Record commit on issue

```bash
COMMIT_HASH=$(git rev-parse HEAD)
grava update <issue-id> --last-commit "$COMMIT_HASH"
```

### Label for code review (the hand-off signal)

```bash
grava label <issue-id> --add code_review
```

### Completion comment (issue audit trail)

```bash
grava comment <issue-id> -m "Implementation complete. Commit: <short-hash>. Ready for code review."
```

### Commit grava state (dolt audit log)

```bash
grava commit -m "complete <issue-id>: ready for code review (commit: <short-hash>)"
```

Note: the comment + the grava commit message are intentional duplication. The comment lives on the issue forever; the dolt commit is the cross-issue diff history. Both are useful for different audiences.

### Final wisp checkpoint

```bash
grava wisp write <issue-id> step "complete"
```

---

## Step 8: Summary

Print:

```
--- Dev Task Complete ---
Task:       <issue-id> — <title>
Parent:     <parent-id>
Status:     in_progress (pending code review)
Label:      code_review
Git commit: <full-hash>
Files:      <count> changed
Tests:      <pass-count> passing (scoped)
Next step:  Reviewer picks up via code_review label.
```

Suggest next steps:
- `grava-code-review` for peer review (different LLM recommended).
- Do NOT auto-claim the next sibling task — let the parent story coordinator decide.
