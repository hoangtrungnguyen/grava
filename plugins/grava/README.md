# Grava Plugin

Full grava issue-tracking pipeline for Claude Code: 12 skills, 5 agents, 2 hooks, 5 scripts.

## Install

```bash
# 1. Install the grava binary (once per machine)
go install github.com/hoangtrungnguyen/grava@latest

# 2. In the target project, register the marketplace and install the plugin
cd /path/to/your-project
# (inside Claude Code)
/plugin marketplace add ./                  # if using the grava repo as local marketplace
/plugin install grava@grava

# 3. Bootstrap project-local pieces
grava bootstrap

# 4. Verify
/ship --help                               # skill resolves
ls .git/hooks/commit-msg                   # git hook installed
crontab -l | grep pr-merge-watcher         # cron line live (after paste)
```

Time-to-first-`/ship` on a fresh project: ~2 minutes.

## Usage

| Command | Description |
|---------|-------------|
| `/ship <id>` | Single-issue pipeline: code → review → PR → handoff |
| `/ship <id> --force` | Bypass precondition gate |
| `/ship <id> --retry` | Re-run with PR rejection feedback |
| `/ship <id> --retry --rebase-only` | Rebase stale branch, skip review |
| `/ship` | Auto-discover and ship next ready issue |
| `/plan <doc>` | Generate issues from PRD/spec |
| `/hunt [scope]` | Audit codebase for bugs |

## Manual install (grava monorepo development)

See `plan/phase2B/story-2B.11-settings-and-claude-md.md` for the manual checklist.
This is supported for developing grava itself; prefer the plugin install for other projects.

## Cron setup

`grava bootstrap` prints these lines — paste into `crontab -e`:

```cron
*/5 * * * * cd /path/to/project && /path/to/scripts/pr-merge-watcher.sh >> .grava/watcher.log 2>&1
0 * * * * cd /path/to/project && /path/to/scripts/run-pending-hunts.sh
0 2 * * * cd /path/to/project && claude -p "/hunt since-last-tag" >> .grava/hunt.log 2>&1
```

## Versioning

Plugin version and marketplace plugin-entry version must match. See [CHANGELOG.md](./CHANGELOG.md).

Update with:
```
/plugin marketplace update grava
/plugin update grava@grava
```
