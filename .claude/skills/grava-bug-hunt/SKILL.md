---
name: grava-bug-hunt
description: Comprehensive codebase audit that finds bugs and creates grava issues for each one. Use when the user says "find bugs", "hunt for bugs", "audit the code", "comprehensive review", "code audit", "bug hunt", or wants a thorough review of recent changes or the full codebase. Also trigger when the user says "review the codebase", "look for issues in the code", or "check for problems". This is different from grava-code-review which reviews a single issue's commit — this skill reviews across multiple files and commits to find systemic bugs.
---

# Grava Bug Hunt

Conduct a thorough, multi-agent code audit of the codebase (or a targeted subset), classify findings by severity, and create grava issues for each real bug.

**Announce at start:** "Starting grava bug hunt on `<scope>`."

## Why this skill exists

Single-commit code reviews catch local bugs but miss systemic issues — timezone mismatches across packages, false-positive tests, missing keyword escaping, or architectural gaps. A bug hunt reviews across boundaries with fresh eyes and files actionable issues, not just comments.

## On Activation

### Step 1: Determine scope

Ask the user (or infer from context) what to review:

- **Recent changes** (default): `git log --oneline -N` to identify commits since last tag/release, then review all changed files
- **Specific packages**: User names packages like `pkg/cmd/reserve/`, `pkg/utils/`
- **Full codebase**: Every `.go` file (use for periodic audits)
- **PR diff**: Review changes in a specific PR

If unclear, default to recent changes since the last tag:
```bash
git log $(git describe --tags --abbrev=0)..HEAD --oneline
git diff $(git describe --tags --abbrev=0)..HEAD --name-only
```

### Step 2: Launch parallel review agents

Split the files into logical groups (by package or subsystem) and launch **parallel review agents**, each focused on one area. This is the key efficiency gain — reviewing 3-5 areas simultaneously instead of sequentially.

Each agent gets this brief:

```
Review these files in <project-root> for bugs, security issues, race conditions, 
resource leaks, edge cases, logic errors, and correctness problems.

Files to review:
<file list>

Context: <brief description of what this package does>

For each bug found, report:
- File path and line number
- Bug description
- Severity: CRITICAL / HIGH / MEDIUM
- Suggested fix

Rules:
- Report ONLY actual bugs — no style nits, no LOW severity
- Cross-reference related files for consistency issues
- Check SQL for reserved keyword issues
- Check time handling (UTC vs local, timezone mismatches)
- Check error handling (silent swallows, fail-open vs fail-closed)
- Check test assertions (false positives, missing validations)
```

Typical groupings for a Go project:
- Core commands (pkg/cmd/*.go)
- Sub-packages (pkg/cmd/reserve/, pkg/cmd/sync/, etc.)
- Utilities and infrastructure (pkg/utils/, pkg/errors/, etc.)
- Tests (look for false positives, weak assertions)

### Step 3: Consolidate findings

After all agents return, merge their findings into a single list. Deduplicate — different agents may flag the same issue from different angles.

Classify every finding into severity:
- **CRITICAL** — Data loss, security holes, crashes on normal inputs
- **HIGH** — Bugs on edge paths, missing error handling, false-positive tests
- **MEDIUM** — Timezone mismatches, reserved keyword issues, weak assertions, resource leaks

Drop anything that's merely a style nit or LOW-severity cosmetic issue. The goal is actionable bugs, not a laundry list.

### Step 4: Present findings to the user

Show a summary table before creating issues:

```markdown
| # | Severity | File | Bug |
|---|----------|------|-----|
| 1 | HIGH | pkg/cmd/hook.go:377 | ExpiresTS shows "UTC" regardless of timezone |
| 2 | MEDIUM | pkg/utils/worktree_init.go:15 | Doesn't check file vs directory |
...
```

Ask the user: "Found N bugs. Want me to create grava issues for all of them, or do you want to filter first?"

If the user wants to filter, let them pick which findings to keep. Then proceed with only the approved ones.

### Step 5: Create grava issues

For each approved finding, create a grava issue:

```bash
grava create \
  --title "Bug: <concise description>" \
  --type bug \
  --priority <high|medium> \
  --desc "<detailed description with file:line, what's wrong, why it matters, suggested fix>"
```

Map severity to priority:
- CRITICAL → `high` (priority 1)
- HIGH → `high` (priority 1)
- MEDIUM → `medium` (priority 2)

After creating all issues, commit the grava state:
```bash
grava commit -m "bug hunt: create N bug issues from code audit"
```

### Step 6: Summary

Print a final report:

```markdown
## Bug Hunt Complete
- **Scope**: <what was reviewed>
- **Files reviewed**: <count>
- **Bugs found**: <count> (critical=N, high=N, medium=N)
- **Issues created**: <list of grava-XXXX IDs with titles>
- **Next step**: Run `grava ready` to see actionable bugs, or fix them with `/grava-dev-story`
```

## What to look for

These are the categories that produce the highest-value findings, based on real bug hunts in this codebase:

**Cross-package consistency**
- Timezone handling: Go's `time.Now().UTC()` vs SQL `NOW()` — they use different clocks when the MySQL driver defaults to `loc=UTC`
- Reserved SQL keywords used as column names without backtick-quoting (e.g., `exclusive`, `status`, `type`)
- Error propagation: one package returns an error, the caller silently ignores it

**Test quality**
- False-positive tests: assertions that pass regardless of correctness (e.g., checking count > 0 when the real invariant is per-item uniqueness)
- Missing post-condition checks: test sets up data, performs operation, but doesn't verify the important output
- Mock expectations that don't match actual query format (sqlmock regex mismatches)

**Security**
- Network binding: servers listening on 0.0.0.0 instead of 127.0.0.1
- Unauthenticated endpoints
- File descriptor leaks on error paths

**Concurrency**
- Shared mutable state without locking
- Goroutine leaks (launched but never joined)
- TOCTOU races in file operations
