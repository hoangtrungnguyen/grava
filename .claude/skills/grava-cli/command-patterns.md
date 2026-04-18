# Grava Command Composition Patterns

## Core Principle

Grava commands follow a **discover → act → verify** pattern. Always confirm state after mutations.

## Pattern 1: Discovery → Claim

Find unblocked work and take ownership.

```bash
# 1. Discover what's ready
grava ready --limit 5 --json

# 2. Inspect the best candidate
grava show grava-42 --json
grava dep tree grava-42          # check what blocks it

# 3. Claim atomically (assigns + sets in_progress)
grava claim grava-42

# 4. Verify
grava show grava-42 --json       # confirm status=in_progress, assignee=you
```

**Why not just `start`?** `claim` is atomic (assign + status change in one transaction). `start` only changes status. Use `claim` when picking up new work; use `start` when resuming work already assigned to you.

## Pattern 2: Implementation Loop

Checkpoint progress while working on an issue.

```bash
# Set checkpoint after each phase
grava wisp write grava-42 step "tests-written"

# Add context via comments (visible to other agents/humans)
grava comment grava-42 -m "Added unit tests for auth middleware"

# Track affected files
grava update grava-42 --files "pkg/auth/middleware.go,pkg/auth/middleware_test.go"

# Record commit hash when code is committed
grava update grava-42 --last-commit "abc123def"
```

**When to use wisps vs comments:**
- Wisps = machine state, crash recovery, temporary data
- Comments = human-readable progress, decisions, context

## Pattern 3: Finalization

Close out completed work.

```bash
# 1. Label for review
grava label grava-42 --add code_review

# 2. Comment with summary
grava comment grava-42 -m "Implementation complete. All tests passing."

# 3. Snapshot grava state
grava commit -m "complete: grava-42 auth middleware"

# 4. Close (or let reviewer close after review)
grava close grava-42
```

**Always `grava commit` after meaningful state changes.** Without it, changes exist only in the running database and won't survive a `db-stop`/`db-start` cycle or be visible to other clones.

## Pattern 4: Triage

Assess and organize incoming issues.

```bash
# 1. List unprocessed issues
grava list --status open --sort priority --json

# 2. Prioritize
grava update grava-99 --priority high

# 3. Set dependencies
grava dep grava-99 grava-42 --type blocks    # grava-99 blocks grava-42

# 4. Assign
grava assign grava-99 --actor alice

# 5. Add labels
grava label grava-99 --add backend --add urgent

# 6. Create subtasks for complex issues
grava subtask grava-99 -t "Design API schema"
grava subtask grava-99 -t "Implement endpoints"
grava subtask grava-99 -t "Write integration tests"
```

## Pattern 5: Impact Analysis

Understand blast radius before making changes.

```bash
# What does this issue block?
grava dep impact grava-42

# What blocks this issue? (ancestry tree)
grava dep tree grava-42

# Is there a blocking chain between two issues?
grava dep path grava-10 grava-42

# Overall graph health
grava graph health              # cycles, orphans, stats
grava graph cycle               # just cycle detection

# What's stuck?
grava blocked --depth 2         # blocked tasks + their blockers' blockers

# Visualize (for documentation)
grava graph visualize --format mermaid
```

## Pattern 6: Sync & Backup

Move issue data between environments.

```bash
# Export all issues to JSONL
grava export -f backup.jsonl --include-wisps

# Import into another environment
grava import -f backup.jsonl --overwrite

# Or skip existing issues (additive merge)
grava import -f backup.jsonl --skip-existing

# Always commit after import
grava commit -m "import: restored from backup"
```

## Pattern 7: Cleanup & Maintenance

Keep the tracker healthy.

```bash
# Purge old ephemeral wisps (default: older than 7 days)
grava compact --days 7

# Purge all wisps
grava compact --days 0

# Clear archived issues (permanent)
grava clear --force

# Clear by date range
grava clear --from 2026-01-01 --to 2026-02-01 --force

# Health check
grava doctor

# Review recent activity
grava stats --days 14
grava cmd_history --limit 20 --actor alice
```

## Pattern 8: Multi-Agent Coordination

When multiple agents work concurrently.

```bash
# Reserve files before editing
grava reserve "pkg/auth/**/*.go" --exclusive --ttl 60 --reason "refactoring auth"

# Check existing reservations
grava reserve --list

# Release when done
grava reserve --release res-a1b2c3

# Use wisps for inter-agent signaling
grava wisp write grava-42 deploy_url "https://staging.example.com"
grava wisp read grava-42 deploy_url
```

## Anti-Patterns

| Mistake | Why It's Wrong | Do Instead |
|---------|---------------|------------|
| Creating stories without `--parent` | Orphan stories lack epic context | `grava create -t "..." --type story --parent grava-1` |
| Forgetting `grava commit` | Changes only in running DB, lost on restart | Commit after every meaningful state change |
| Using `start` to pick up new work | Doesn't assign to you | Use `claim` instead |
| Polling `list` to find work | Ignores dependency graph | Use `ready` — it only shows unblocked work |
| Claiming without checking `dep tree` | May depend on unfinished work | Always inspect dependencies first |
| Writing long data to comments | Clutters issue history | Use wisps for machine state, comments for human context |
| Using `drop --all` casually | Nuclear reset, archives everything | Use targeted `drop <id>` |
| Skipping `--json` in automation | Human-readable output breaks parsers | Always use `--json` when parsing output |
