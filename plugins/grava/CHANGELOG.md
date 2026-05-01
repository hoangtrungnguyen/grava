# Grava Plugin Changelog

## 0.2.0 (2026-04-30)

Initial public plugin release. Packages the full Phase 2B agent-team pipeline.

### Added

- **5 pipeline agents**: `coder`, `reviewer`, `bug-hunter`, `planner`, `pr-creator`
- **3 user-invocable skills**: `/ship`, `/plan`, `/hunt`
- **9 grava-* skills**: `grava-cli`, `grava-dev-task`, `grava-code-review`, `grava-bug-hunt`, `grava-gen-issues`, `grava-claim`, `grava-complete-dev-story`, `grava-dev-epic`, `grava-next-issue`
- **2 pipeline hooks**: `PostToolUse` (sync-pipeline-status), `Stop` (warn-in-progress)
- **5 scripts**: `pr-merge-watcher.sh`, `pre-merge-check.sh`, `run-pending-hunts.sh`, `preflight-gh.sh`, `install-git-hooks.sh`
- **`grava bootstrap`** subcommand for post-install setup
- Plugin marketplace manifest at `.claude-plugin/marketplace.json`

### Pipeline features

- Single-issue pipeline: code → review (up to 3 rounds) → PR → async merge tracking
- PR rejection recovery: `/ship <id> --retry` and `--retry --rebase-only`
- Backlog drain: `/ship` (no id) auto-discovers next ready leaf-type issue
- Precondition gate: validates spec + AC before spawning coder agent
- Async merge watcher: cron-driven, no conversation cache pollution
- Bug-hunt scheduling: commit-msg hook + hourly drain + nightly cron

### Requires

- `grava` binary ≥ 0.2.0 on PATH
- `gh` CLI authenticated with `repo` scope
- `jq` on PATH
- Claude Code with plugin marketplace support
