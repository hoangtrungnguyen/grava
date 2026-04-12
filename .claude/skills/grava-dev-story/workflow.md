# Grava Dev Story Workflow

**Goal:** Implement a grava issue using TDD, tracking all progress through the grava CLI.

**Role:** Developer implementing the issue.

## Ground Rules

- Execute ALL steps in order. Do NOT skip steps.
- Do NOT stop for "milestones", "significant progress", or "session boundaries". Continue until DONE or HALT.
- All issue state changes go through `grava` CLI — never edit the Dolt DB directly.
- Use `--json` flag when parsing grava output programmatically.
- Use `grava wisp write` for crash-recovery checkpoints throughout.

---

## Step 1: Find and Claim the Issue

### If user provides an issue ID or story file path:

```bash
grava show <issue-id> --json
```

If user provides a **story file path** instead of a grava issue ID, read the file to extract the grava issue ID (look for `grava-XXXX` patterns in the file metadata or content). Then proceed with that ID.

### If no issue specified — discover next work:

```bash
grava ready --limit 3 --json
```

Present candidates to user. Let them pick. If only one result, confirm and proceed.

If no ready issues found:
- Show `grava list --status open --type story` as fallback
- If still nothing, HALT: "No ready issues. Create stories first or specify an issue ID."

### Claim it:

```bash
grava claim <issue-id>
```

This atomically sets status to `in_progress` and assigns to current actor.

If already `in_progress` and assigned to you, continue (resuming prior work). If assigned to someone else, HALT and tell user.

### Checkpoint:

```bash
grava wisp write <issue-id> step "claimed"
```

---

## Step 2: Load Context

Gather everything needed to implement well.

### Issue details:

```bash
grava show <issue-id>
grava show <issue-id> --tree
```

Extract: title, description, type, priority, acceptance criteria (from description), subtasks, labels, comments, affected files.

### Dependency context:

```bash
grava dep tree <issue-id>
```

Understand what this issue depends on and what it unblocks. Check if parent epic exists and read its description for broader context.

### Project context:

- Load `**/project-context.md` if it exists — coding standards, architecture patterns
- Load `**/CLAUDE.md` for project instructions
- Check recent git history for related changes: `git log --oneline -20`

### Review continuation check:

Check comments for prior review feedback:

```bash
grava history <issue-id>
```

If there are comments containing review feedback or `code_review` label was previously added then removed, this is a review continuation. Prioritize addressing review findings first.

### Checkpoint:

```bash
grava wisp write <issue-id> step "context-loaded"
```

Output a brief summary:
```
Issue: <id> — <title>
Type: <type> | Priority: <priority>
Subtasks: <count> (<completed>/<total>)
Dependencies: <blocking-count> blocking, <blocked-by-count> blocked by
Context: <what you loaded>
Strategy: <brief implementation approach>
```

---

## Step 3: Plan Implementation

Break down the work based on issue description and subtasks.

### If issue has subtasks:

```bash
grava show <issue-id> --tree
```

Use existing subtasks as the task list. Work through them in order. Check which are already `closed` (completed in prior session).

### If issue has no subtasks but needs decomposition:

Create subtasks for each logical unit of work:

```bash
grava subtask <issue-id> --title "Add error type definitions" --type task
grava subtask <issue-id> --title "Implement handler logic" --type task
grava subtask <issue-id> --title "Add unit tests" --type task
```

Then commit the plan:

```bash
grava commit -m "plan: decompose <issue-id> into subtasks"
```

### If issue is small enough for single implementation:

Skip subtask creation. Treat the whole issue as one task.

---

## Step 4: Implement (Red-Green-Refactor)

For each task/subtask, follow TDD:

### Checkpoint before each task:

```bash
grava wisp write <issue-id> current_task "<subtask-id or description>"
```

### RED — Write failing tests first

- Write tests that define the expected behavior for this task
- Run tests, confirm they fail (validates test correctness)
- If test framework unclear, infer from project structure (`go test`, `npm test`, `pytest`, etc.)

### GREEN — Minimal implementation

- Write the minimum code to make tests pass
- Run tests, confirm they pass
- Handle error conditions and edge cases specified in the issue

### REFACTOR — Clean up

- Improve structure while keeping tests green
- Follow architecture patterns from project context
- Ensure code matches project conventions

### After each task completes:

```bash
# Log progress
grava comment <issue-id> -m "Completed: <brief description of what was done>"

# If working with subtasks, close the subtask
grava update <subtask-id> --status closed

# Checkpoint
grava wisp write <issue-id> step "task-done:<subtask-id>"
```

### HALT conditions:

- New dependencies required beyond what's in the project — HALT, ask user
- 3 consecutive implementation failures — HALT, request guidance
- Required configuration missing — HALT, explain what's needed
- Ambiguous requirements — HALT, ask user to clarify

### Keep going:

Do NOT pause between tasks. Move straight to the next task/subtask until all are complete.

---

## Step 5: Run Full Validation

After all tasks/subtasks done:

### Run full test suite:

```bash
# Detect and run project tests
go test ./...          # Go
npm test               # Node
pytest                 # Python
```

- All existing tests must pass (no regressions)
- All new tests must pass
- If regressions found: fix them before proceeding

### Run code quality checks:

```bash
# If configured in project
go vet ./...           # Go
golangci-lint run      # Go lint
npm run lint           # Node
```

### Validate acceptance criteria:

Re-read the issue description. Verify EVERY acceptance criterion is satisfied by the implementation. If any are not met, go back and implement them.

### Checkpoint:

```bash
grava wisp write <issue-id> step "validated"
```

---

## Step 6: Definition of Done

Run through the [checklist.md](./checklist.md) validation:

- All tasks/subtasks complete
- All acceptance criteria satisfied
- Unit tests for core functionality
- Integration tests where needed
- No regressions
- Code quality checks pass

If any item fails, go back and fix it. Do not proceed until DoD passes.

---

## Step 7: Complete and Hand Off

### Stage and commit code:

```bash
git status
git diff --stat
```

Generate conventional commit message from issue metadata:
- `story` → `feat`, `bug` → `fix`, `task` → `chore`
- Include issue ID in commit message

Stage relevant files (avoid secrets, build artifacts) and commit.

### Record commit on issue:

```bash
COMMIT_HASH=$(git rev-parse HEAD)
grava update <issue-id> --last-commit "$COMMIT_HASH"
```

### Label for code review:

```bash
grava label <issue-id> --add code_review
```

### Add completion comment:

```bash
grava comment <issue-id> -m "Implementation complete. Commit: <short-hash>. Ready for code review."
```

### Commit grava state:

```bash
grava commit -m "complete <issue-id>: ready for code review (commit: <short-hash>)"
```

### Clean up wisp state:

```bash
grava wisp write <issue-id> step "complete"
```

---

## Step 8: Summary

Print completion summary:

```
--- Dev Story Complete ---
Issue:      <issue-id> — <title>
Type:       <type>
Status:     in_progress (pending code review)
Label:      code_review
Git commit: <full-hash>
Subtasks:   <completed>/<total> closed
Tests:      <pass-count> passing, <fail-count> failing
Next step:  Code review (suggest using a different LLM for review)
```

Suggest next steps:
- Run `grava-code-review` or `/grava-code-review` for peer review
- Use a different LLM than the one that implemented for best review quality
- Check `grava ready` for next available work
