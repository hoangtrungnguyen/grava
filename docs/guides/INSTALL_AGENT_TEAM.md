# Installing the Agent Team

End-to-end setup for the grava agent pipeline.

> Usage docs live in [AGENT_TEAM.md](AGENT_TEAM.md). This guide covers
> first-time install only. Run it once per project; once done, jump to the
> usage guide.

## Two install paths

| Path | Use when |
|------|----------|
| **A — Working on grava itself** | You cloned this repo to develop grava. Steps 1-8 below. |
| **B — Using the agent team in your own project** | You have a separate project (foo-app, my-service, anything) and want grava + the agent pipeline working inside it. Jump to [Path B](#path-b--installing-into-your-own-project). |

Both paths share the same prerequisites; the difference is whether you're cloning grava as the primary repo or layering the agent team on top of an existing project.

---

## What gets installed

The agent team is **5 components**, all already in the repo. You don't fetch them — you just wire them up:

| Component | Where | Purpose |
|-----------|-------|---------|
| Agents | `.claude/agents/` | Sub-agents Claude spawns: `coder`, `reviewer`, `pr-creator`, `planner`, `bug-hunter`, `golang-pro` |
| Skills | `.claude/skills/` | Entry points (`ship`, `hunt`, `plan`) + workflow primitives (`grava-dev-task`, `grava-code-review`, `grava-gen-issues`, `grava-bug-hunt`, `grava-cli`) |
| `grava` CLI | `/usr/local/bin/grava` (or `$GOPATH/bin`) | Issue tracker + signal protocol |
| Dolt server | `.grava/dolt/` (per repo) | Versioned MySQL-compatible DB |
| Watcher | `scripts/pr-merge-watcher.sh` (cron) | Tracks PR merge/close, closes issues, reports comments |

---

## Prerequisites

| Requirement | Check | Why |
|-------------|-------|-----|
| **macOS or Linux** | `uname` | Bash watcher uses POSIX tools (Windows: WSL2) |
| **Go ≥ 1.22** | `go version` | Build grava + Dolt installer |
| **git ≥ 2.5** | `git --version` | Worktree-per-issue convention requires worktree support |
| **gh CLI** | `gh auth status` | PR creation; must be authed with `repo` scope |
| **jq** | `jq --version` | Pipeline scripts parse `grava show --json` |
| **Claude Code CLI** | `claude --version` | Spawns the agents |
| **lsof** | `which lsof` | `grava db-stop` finds the Dolt PID |

Optional but recommended:
- **flock** — only needed if you customize the watcher; bash watcher uses `ps`-based PID verify which works without flock
- **Python ≥ 3.10** — only if you opt into the experimental Python watcher (`scripts/pr_merge_watcher.py`, when merged)

---

## Step 1 — Clone + build

```bash
git clone https://github.com/hoangtrungnguyen/grava.git
cd grava
go install ./cmd/grava
grava version    # confirm ≥ 0.2.0
```

Or download a release binary from [GitHub Releases](https://github.com/hoangtrungnguyen/grava/releases).

---

## Step 2 — Init Dolt + DB

```bash
grava init             # creates .grava/, downloads Dolt if absent, runs migrations
grava db-start         # starts Dolt sql-server in background
grava doctor           # confirms DB reachable + schema fresh
```

`grava init` also creates `.worktree/` and adds it to `.gitignore`. The Dolt server listens on the port in `.grava.yaml` (default 3306; auto-picks next free port if conflict).

---

## Step 3 — Install git hooks

```bash
./scripts/install-hooks.sh
```

Installs:
- `commit-msg` hook — accepts `bug-hunt: <issue-id>` trailer for issue linking
- `prepare-commit-msg` hook — auto-prefills issue ID from branch name `grava/<id>`

---

## Step 4 — Install cron entries

The pipeline relies on **two** cron jobs:

```cron
# pr-merge-watcher: every 5 min, tracks PR merge/close
*/5 * * * * cd /path/to/grava && ./scripts/pr-merge-watcher.sh >> .grava/watcher.log 2>&1

# Hourly hunt drain (optional — only if /hunt scheduling is in use)
0 * * * * cd /path/to/grava && ./scripts/run-pending-hunts.sh
```

Generate ready-to-paste entries:

```bash
grava bootstrap --print-cron | crontab -
```

**macOS launchd alternative** (if you don't run `cron`):

```bash
# .grava/launchd/com.grava.watcher.plist
launchctl load ~/Library/LaunchAgents/com.grava.watcher.plist
```

A template plist ships at `scripts/launchd/com.grava.watcher.plist.example` — copy and edit `<string>` paths.

---

## Step 5 — Verify Claude Code sees the agents

```bash
claude --list-agents | grep -E "coder|reviewer|pr-creator|planner|bug-hunter"
claude --list-skills | grep -E "ship|hunt|plan|grava-"
```

If any are missing: confirm `.claude/agents/` and `.claude/skills/` are in place at the project root and that you launched `claude` from inside the repo.

---

## Step 6 — Smoke test

```bash
# Create a throwaway issue with AC
grava create --type task --title "smoke test" \
  --desc "## Acceptance Criteria
- [ ] noop"

# Capture the ID printed; then ship it dry-run
SMOKE_ID=$(grava list --limit 1 --json | jq -r '.[0].id')
/ship "$SMOKE_ID" --dry-run
```

Expected: `DRY_RUN: <id> — would read the following wisp state at each phase boundary`. No agents spawned, no mutations.

When ready to fully verify, run a real ship on a small test issue. The pipeline will: spawn coder → reviewer → pr-creator → open a PR → exit handing off to the watcher.

---

## Step 7 — Optional: agent-bot identity

By default, PRs are opened under your personal `gh` account and commits are attributed to your local `git config user.name`. If you want PRs to come from a dedicated bot user:

```bash
./scripts/setup-agent-bot.sh
# Prompts for:
#   - GitHub PAT (with `repo` scope, no expiry recommended)
#   - Bot username
#   - Bot email (e.g. agent-bot+grava@yourcompany.com)
# Stores in .grava/agent-bot.env (gitignored)
```

After setup, `pr-creator` rewrites commit authorship to the bot via `git rebase --exec` and uses the PAT for `gh pr create`. Your personal account stays out of the loop.

Skip this step if you're shipping solo — the default fallback works fine.

---

## Step 8 — Optional: configure shell aliases

Common shortcuts (add to `~/.zshrc` / `~/.bashrc`):

```bash
alias gship='claude -p "/ship"'              # auto-discover next issue
alias ghunt='claude -p "/hunt since-last-tag"'
alias glist='grava list --status open'
alias gready='grava ready --json | jq -r ".[] | \"\(.Node.ID) — \(.Node.Title)\""'
```

---

## Path B — Installing into your own project

You want to use grava's agent team inside `your-project/`, not in a clone of grava itself. The agents and skills are project-scoped under `.claude/`, so they need to be copied. The grava CLI itself is global.

### B.1 — Install grava CLI globally

```bash
# Same as Path A Step 1, but installed once globally instead of per-clone
go install github.com/hoangtrungnguyen/grava/cmd/grava@latest
grava version    # confirm ≥ 0.2.0 on PATH
```

Or download a release binary and put it on PATH.

### B.2 — Init grava in your project

```bash
cd /path/to/your-project
grava init       # creates .grava/, .worktree/, .claude/settings.json
grava db-start
grava doctor
```

This sets up the per-project Dolt DB. `your-project/.grava/` is gitignored by default.

### B.3 — Copy agent-team assets from a grava clone

You need a local clone of grava to source the assets from:

```bash
# Once, anywhere on disk
git clone https://github.com/hoangtrungnguyen/grava.git ~/src/grava
```

Then from your project root:

```bash
GRAVA_SRC=~/src/grava
PROJECT=$(pwd)

# 1. Agents — sub-agents Claude Code spawns
mkdir -p "$PROJECT/.claude/agents"
cp "$GRAVA_SRC"/.claude/agents/{bug-hunter,coder,planner,pr-creator,reviewer,golang-pro}.md \
   "$PROJECT/.claude/agents/"

# 2. Skills — entry points + workflow primitives
mkdir -p "$PROJECT/.claude/skills"
cp -r "$GRAVA_SRC"/.claude/skills/{ship,hunt,plan,grava-bug-hunt,grava-cli,grava-claim,grava-code-review,grava-complete-dev-story,grava-dev-epic,grava-dev-task,grava-gen-issues,grava-next-issue} \
      "$PROJECT/.claude/skills/"

# 3. Pipeline scripts
mkdir -p "$PROJECT/scripts/agent-bot" "$PROJECT/scripts/ship" "$PROJECT/scripts/hooks"
cp "$GRAVA_SRC"/scripts/install-hooks.sh "$PROJECT/scripts/"
cp "$GRAVA_SRC"/scripts/pr-merge-watcher.sh "$PROJECT/scripts/"
cp "$GRAVA_SRC"/scripts/preflight-gh.sh "$PROJECT/scripts/"
cp "$GRAVA_SRC"/scripts/agent-bot-token.sh "$PROJECT/scripts/"
cp "$GRAVA_SRC"/scripts/setup-agent-bot.sh "$PROJECT/scripts/"
cp "$GRAVA_SRC"/scripts/agent-bot/finalize-pr.sh "$PROJECT/scripts/agent-bot/"
cp "$GRAVA_SRC"/scripts/ship/dep-check.sh "$PROJECT/scripts/ship/"
chmod +x "$PROJECT"/scripts/*.sh "$PROJECT"/scripts/*/*.sh

# 4. Optional — regression test suite
cp -r "$GRAVA_SRC"/scripts/test "$PROJECT/scripts/"
```

### B.4 — Set up `CLAUDE.md` for your project

Create or extend `your-project/CLAUDE.md` with the agent-team contract. Minimum required: signal protocol, wisp keys, agent map. Easiest path is to copy the relevant sections from grava's [CLAUDE.md](../../CLAUDE.md):

- `## Agent Team` (skill ↔ agent map)
- `## Pipeline Signals` (v2 protocol + preconditions)
- `## Wisp Keys` (canonical state vocabulary)
- `## Operator Hazards` (concurrency pitfalls)

Adapt project-specific details: paths to `.grava/dolt/`, test commands, build commands.

### B.5 — Install hooks + cron in your project

```bash
cd "$PROJECT"
./scripts/install-hooks.sh

# Cron (replace /path/to/your-project with absolute path)
crontab -l | { cat; cat <<EOF
*/5 * * * * cd /path/to/your-project && ./scripts/pr-merge-watcher.sh >> .grava/watcher.log 2>&1
EOF
} | crontab -
```

### B.6 — Verify

```bash
cd "$PROJECT"
claude --list-agents | grep -E "coder|reviewer|pr-creator|planner|bug-hunter"
claude --list-skills | grep -E "ship|hunt|plan|grava-"

# Optional: run portable regression suite to confirm scripts work
bash scripts/test/test-finalize-pr.sh
bash scripts/test/test-watcher-pidfile.sh
bash scripts/test/test-dep-check.sh
```

### B.7 — Smoke test — first ship in your project

```bash
cd "$PROJECT"
grava create --type task --title "smoke test" \
  --desc "## Acceptance Criteria
- [ ] noop"

SMOKE_ID=$(grava list --limit 1 --json | jq -r '.[0].id')
/ship "$SMOKE_ID" --dry-run
```

If the dry-run prints wisp state without errors, the install is wired correctly.

### Updating later

To pull in upstream agent / skill / script changes after a grava release:

```bash
cd ~/src/grava && git pull origin main
# Re-run B.3 to overwrite. Diff first to see what changed:
diff -ru ~/src/grava/.claude/agents your-project/.claude/agents
```

A future `grava bootstrap --copy-agent-team` flag will automate this — for now, the manual `cp` block above is the canonical path.

### What does NOT need copying

| Asset | Why |
|-------|-----|
| `.grava/dolt/` | Per-project DB; created by `grava init` |
| `pkg/` Go code | Built into the grava CLI binary; not project-scoped |
| `Dockerfile`, `Makefile`, etc. | grava-internal build infra; your project keeps its own |
| `docs/` | Reference docs live in grava itself; link to them, don't fork |

---

## Operator hazards

These are operator-managed. The pipeline cannot enforce all of them.

| Hazard | Why | Avoid |
|--------|-----|-------|
| `grava db-stop` while issues `in_progress` | Active /ship runs lose state on next signal write | Now refuses with `--force` required (concurrency-matrix #4) |
| Editing `.grava.yaml` mid-flight | Config re-read on each CLI call; in-flight `/ship` may pick up new value mid-pipeline | Edit only when `grava list --status in_progress` is empty |
| `/ship X` from two terminals | Second halts at `ALREADY_CLAIMED` (safe) | Check `grava show X --json \| jq .status` first if unsure |
| `git push --force` to `grava/<id>` from two terminals | Same branch from two worktrees → conflict / lost commits | Don't force-push grava/* outside the pipeline |
| Manually editing files inside `.worktree/<id>/` while agent is running | Agent's view diverges from disk; commit may include unintended changes | Don't touch `.worktree/<id>/` while issue is `in_progress` |
| Restarting Dolt on a different port | New PID, old `.grava.yaml` still points at old port | Update `.grava.yaml` first, then `db-start` |

---

## Verifying the install end-to-end

Run the regression suite to confirm everything wired correctly:

```bash
go test ./... 2>&1 | tail -5             # all green
bash scripts/test/test-finalize-pr.sh    # 7/7 pass
bash scripts/test/test-watcher-pidfile.sh # 3/3 pass
bash scripts/test/test-watcher-comments.sh # passes
bash scripts/test/test-dep-check.sh      # 11/11 pass
bash scripts/test/test-ship-helpers.sh   # 22/22 pass
```

If all pass, the install is complete.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `grava init` errors with `dolt binary not found` | Network blocked during install | Set `DOLT_BINARY=/path/to/dolt` env var or place binary in `.grava/bin/dolt` |
| `claude CLI not found on PATH` during `grava init` | Tests run in CI without Claude | `export GRAVA_SKIP_PREFLIGHT=1` (test/CI only) |
| `gh auth status` shows missing `repo` scope | PAT or browser auth lacks repo perms | `gh auth refresh -h github.com -s repo` |
| Watcher logs `previous run still active` indefinitely | Stale PIDFILE on long-uptime host (PID recycled) | Already handled by grava-24fa fix in `scripts/pr-merge-watcher.sh` (verifies `ps` command before treating as live) |
| `/ship` halts at `ALREADY_CLAIMED` | Issue claimed by another terminal or stale claim | Run `grava stop <id>` to release; if stale heartbeat (>1h), claim auto-recovers |
| PR opens but `pipeline_phase` stays `claimed` | pr-creator skipped finalize-pr.sh | Already enforced by signal precondition (grava-fddd) — run `grava signal PR_CREATED` requires aux wisps; should reject loudly now |

---

## Next

- Read [AGENT_TEAM.md](AGENT_TEAM.md) for the full usage reference (signals, phases, re-entry patterns)
- Read [STRUCTURED_SIGNALS_MIGRATION.md](STRUCTURED_SIGNALS_MIGRATION.md) for the v2 signal protocol details
- Run `/hunt since-last-tag` to seed the backlog, then `/ship` to drain it
