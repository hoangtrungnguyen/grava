---
name: grava-code-review
description: Use when reviewing code for a grava issue that has been marked for code review. Trigger when the user says "review grava-XXX", "code review for this issue", "review the changes", or any request to review code tied to a grava issue. The issue must have a last_commit stored in its metadata — this skill uses that commit to find changed files and conduct the review.
---

# Grava Code Review

Review code changes associated with a grava issue by inspecting the files changed in the recorded commit, then post findings as a comment and label the issue as reviewed.

**Announce at start:** "Using the grava-code-review skill to review `<issue-id>`."

## Prerequisites

- A valid grava issue ID with `last_commit` stored in its metadata (set by the `grava-complete-dev-story` skill or manually via `grava update <id> --last-commit <hash>`)

## Steps

### 1. Fetch the issue and extract the commit hash

```bash
grava show <issue-id> --json
```

Parse the JSON output to get the `last_commit` field. If it's missing or empty, stop and tell the user: "This issue has no commit hash recorded. Run `grava update <issue-id> --last-commit <hash>` first, or use the grava-complete-dev-story skill to commit and record it."

### 2. Get the changed files from the commit

```bash
git show --stat <commit-hash>
git diff <commit-hash>~1 <commit-hash> --name-only
```

This gives you the list of files that were changed in that commit. If the commit is a merge commit or the first commit, adjust accordingly:

```bash
# For merge commits
git diff <commit-hash>^1 <commit-hash> --name-only

# If ~1 fails (first commit)
git diff --root <commit-hash> --name-only
```

### 3. Read and review each changed file

For each changed file, get the diff:

```bash
git diff <commit-hash>~1 <commit-hash> -- <file-path>
```

Conduct the review focusing on:

- **Correctness**: Does the code do what the issue description says it should?
- **Bugs**: Off-by-one errors, nil/null dereferences, race conditions, resource leaks
- **Security**: Injection vulnerabilities, hardcoded secrets, unsafe input handling
- **Error handling**: Are errors checked and propagated properly?
- **Naming and clarity**: Are names descriptive? Is the logic easy to follow?
- **Tests**: Are there tests for the new code? Do they cover edge cases?
- **Style**: Does the code follow the project's existing conventions?

Cross-reference the changes against the issue's description and acceptance criteria (from `grava show`) to verify completeness.

### 4. Post the review as a comment

Compile findings into a structured review and post it:

```bash
grava comment <issue-id> -m "<review content>"
```

Use this format for the review comment:

```
## Code Review for <commit-short-hash>

### Summary
<1-2 sentence overview of what was changed and overall assessment>

### Findings
<list issues found, categorized by severity>

#### Critical
- <blocking issues that must be fixed>

#### Suggestions
- <non-blocking improvements>

### Verdict: APPROVED | CHANGES_REQUESTED
```

If there are no critical findings, the verdict is `APPROVED`. If there are critical issues, use `CHANGES_REQUESTED`.

### 5. Update labels

Remove the `code_review` label (if present) and add `reviewed`:

```bash
grava label <issue-id> --remove code_review
grava label <issue-id> --add reviewed
```

If the verdict is `CHANGES_REQUESTED`, add `changes_requested` instead of `reviewed`:

```bash
grava label <issue-id> --remove code_review
grava label <issue-id> --add changes_requested
```

### 6. Commit grava changes

```bash
grava commit -m "code review for <issue-id> (commit: <short-hash>)"
```

### 7. Print summary

```
--- Code Review Summary ---
Issue:      <issue-id> — <title>
Commit:     <full-hash>
Files:      <count> files reviewed
Findings:   <count> critical, <count> suggestions
Verdict:    APPROVED | CHANGES_REQUESTED
Label:      reviewed | changes_requested
```

## Error handling

- **Issue not found**: Stop, ask user to verify the ID
- **No last_commit**: Stop, tell user to record a commit hash first
- **Commit hash not found in git**: The commit may have been rebased or force-pushed — tell the user and ask for the correct hash
- **Grava command fails**: Show the error, suggest manual recovery
