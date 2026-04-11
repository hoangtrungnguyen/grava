---
name: grava-complete-dev-story
description: Use when a developer or agent has finished implementing a grava story/task and wants to wrap up the work — commit code, record the commit hash, and mark it ready for code review. Trigger whenever someone says "complete story", "finish task", "done with grava-XXX", "wrap up this issue", "mark for review", or similar completion phrases that reference a grava issue.
---

# Complete a Grava Dev Story

Wrap up a finished implementation: run tests, commit code, record the commit hash on the grava issue, and label it for code review.

**Announce at start:** "Using the grava-complete-dev-story skill to wrap up `<issue-id>`."

## Prerequisites

- A valid grava issue ID that exists in the Dolt database
- Code changes ready to commit (staged or unstaged)

## Steps

### 1. Validate the grava issue

Fetch the issue and confirm it exists and is `in_progress`:

```bash
grava show <issue-id> --json
```

If the issue doesn't exist, stop and tell the user. If the status isn't `in_progress`, flag it but continue — the user may have forgotten to claim it.

### 2. Run unit tests

Run the project's test suite before committing. Detect the test runner from the project:

```bash
# Go projects
go test ./...

# Node projects
npm test

# Python projects
pytest

# Rust projects
cargo test
```

If tests fail, stop immediately. Show the failures and do not proceed. The user needs to fix them first.

### 3. Stage and commit code

Review what's changed, then create a commit:

```bash
git status
git diff --stat
```

Generate a conventional commit message from the grava issue's title and description. The format:

```
<type>(<scope>): <short summary> (<issue-id>)

<body — what changed and why, derived from the issue description>
```

Where:
- `type` is inferred from the issue type: `story` → `feat`, `bug` → `fix`, `task` → `chore`, `epic` → `feat`
- `scope` is inferred from affected files or the area of work
- `short summary` is derived from the issue title
- `body` summarizes the implementation based on the diff and issue description

Stage relevant files (avoid secrets, build artifacts) and commit.

### 4. Record the commit hash on the grava issue

After committing, capture the hash and store it:

```bash
COMMIT_HASH=$(git rev-parse HEAD)
grava update <issue-id> --last-commit "$COMMIT_HASH"
```

This links the grava issue to the exact commit for traceability.

### 5. Label for code review

Add the `code_review` label so reviewers can find it:

```bash
grava label <issue-id> --add code_review
```

Keep the status as `in_progress` — the issue stays in progress until the review is done and the code is merged.

### 6. Add a completion comment

Leave a comment summarizing what was done:

```bash
grava comment <issue-id> -m "Implementation complete. Commit: <short-hash>. Labeled for code review."
```

### 7. Commit grava changes

Commit the grava database changes so the label and metadata are versioned:

```bash
grava commit -m "mark <issue-id> for code review (commit: <short-hash>)"
```

## Summary output

After all steps succeed, print a summary:

```
--- Story Completion Summary ---
Issue:      <issue-id> — <title>
Status:     in_progress (pending code review)
Label:      code_review
Git commit: <full-hash>
Next step:  Code review
```

## Error handling

- **Issue not found**: Stop, ask user to verify the ID
- **Tests fail**: Stop, show failures, do not commit
- **Nothing to commit**: Skip git commit, still update labels and metadata
- **Grava command fails**: Show the error, suggest manual recovery
