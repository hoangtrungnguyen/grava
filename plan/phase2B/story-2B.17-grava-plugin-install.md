# Story 2B.17: Grava Plugin — Install Path for New Projects

Package the grava skill set, agents, hooks, and scripts as a Claude Code **plugin** distributable via the [plugin marketplace](https://code.claude.com/docs/en/plugin-marketplaces) format. A new project gets the full pipeline (`/ship`, `/plan`, `/hunt` plus all `grava-*` skills, the 5 pipeline agents, the 2 PostToolUse/Stop hooks, and the merge/hunt watcher scripts) with three commands: `/plugin marketplace add`, `/plugin install`, and a one-time `grava bootstrap` for the binary + git hook + cron lines.

Today, "install grava in a new project" is a checklist of manual steps in story 2B.11: copy `.claude/skills/grava-*`, copy `.claude/agents/*`, append `settings.json` hook entries by hand, run `./scripts/install-hooks.sh`, paste cron lines. That works for grava-the-monorepo (where the source IS the install) but fails for any other project that wants to consume grava. This story turns the manual checklist into a marketplace install + a single CLI bootstrap.

## Why a plugin (not a copy script, not an npm package)

Researched alternatives:

| Option | Pros | Cons |
|---|---|---|
| `cp -R .claude/skills/grava-* target/.claude/skills/` | Zero infra | No auto-update; doesn't register hooks; doesn't install agents; user has to know which skills are pipeline-active vs ad-hoc |
| Tarball + install script | Versioned, simple | No discovery; user manages updates manually; same hook-registration gap |
| npm package consumed via `source: npm` plugin | Standard ecosystem; `npm publish` workflow | Adds a Node dep for non-Node projects; we don't already publish to npm |
| **Plugin marketplace (`source: github` or `git-subdir`)** | Auto-update via `/plugin marketplace update`; bundles skills + agents + hooks + MCP in one unit; settings registration is automatic; Claude Code copies into versioned cache (`~/.claude/plugins/cache`); supports pinning to ref/sha | Requires authoring `marketplace.json` + `plugin.json` once |

Plugin wins on hook-registration alone. The 2B.9/2B.10 hooks need `.claude/settings.json` entries; the plugin manifest declares them and Claude Code wires them on install. Without that, every new project repeats the manual JSON edit from story 2B.11.

## Files

```
grava/
├── .claude-plugin/
│   └── marketplace.json              # marketplace catalog at repo root
├── plugins/
│   └── grava/
│       ├── .claude-plugin/
│       │   └── plugin.json           # plugin manifest
│       ├── skills/
│       │   ├── grava-cli/            # symlinked to ../../../.claude/skills/grava-cli
│       │   ├── grava-dev-task/
│       │   ├── grava-code-review/
│       │   ├── grava-bug-hunt/
│       │   ├── grava-gen-issues/
│       │   ├── grava-claim/
│       │   ├── grava-complete-dev-story/
│       │   ├── grava-dev-epic/
│       │   ├── grava-next-issue/     # ad-hoc, kept for human use (not pipeline-wired)
│       │   ├── managing-grava-issues/
│       │   ├── ship/                 # 2B.5
│       │   ├── plan/                 # 2B.7
│       │   └── hunt/                 # 2B.8
│       ├── agents/
│       │   ├── coder.md              # 2B.1
│       │   ├── reviewer.md           # 2B.2
│       │   ├── bug-hunter.md         # 2B.3
│       │   ├── planner.md            # 2B.4
│       │   └── pr-creator.md         # 2B.14
│       ├── hooks/
│       │   └── hooks.json            # PostToolUse + Stop wiring (resolves to scripts/hooks/*.sh)
│       └── scripts/
│           ├── hooks/
│           │   ├── sync-pipeline-status.sh   # 2B.9
│           │   └── warn-in-progress.sh       # 2B.10
│           ├── pr-merge-watcher.sh           # 2B.12
│           ├── pre-merge-check.sh            # 2B.13
│           ├── run-pending-hunts.sh          # 2B.15
│           ├── preflight-gh.sh               # 2B.5
│           └── install-git-hooks.sh          # 2B.15
└── pkg/cmd/bootstrap.go              # `grava bootstrap` (new subcommand)
```

> **Why symlinks for skills.** `.claude/skills/grava-*` already exists at the repo root for the local agent loop. Duplicating into `plugins/grava/skills/` would drift. Plugin spec allows the layout the manifest describes, so we symlink. Note the docs caveat: when Claude Code copies the plugin into its cache, symlinks resolve at copy time. Verify with `/plugin install <local-path>` against a sibling directory before publishing — story 2B.17 acceptance includes that smoke test.

## Plugin manifest (`plugins/grava/.claude-plugin/plugin.json`)

```json
{
  "name": "grava",
  "version": "0.2.0",
  "description": "Grava issue-tracking pipeline: skills, agents, hooks, and watchers for the code → review → PR → merge loop.",
  "author": { "name": "grava" },
  "homepage": "https://github.com/<owner>/grava",
  "repository": "https://github.com/<owner>/grava",
  "license": "MIT",
  "keywords": ["grava", "pipeline", "issues", "tdd", "code-review"]
}
```

`strict: true` (default) means `plugin.json` is the authority — skill / agent / hook discovery uses the conventional directories (`skills/`, `agents/`, `hooks/`). No custom paths needed.

## Marketplace catalog (`.claude-plugin/marketplace.json`)

```json
{
  "name": "grava",
  "description": "Grava plugin catalog",
  "owner": { "name": "grava maintainers" },
  "plugins": [
    {
      "name": "grava",
      "source": "./plugins/grava",
      "description": "Full grava pipeline: 12 skills, 5 agents, 2 hooks, 5 scripts.",
      "version": "0.2.0",
      "category": "workflow",
      "tags": ["grava", "pipeline", "ci", "review", "issues"]
    }
  ]
}
```

> **Marketplace name `grava` is unreserved** (the reserved list at `https://code.claude.com/docs/en/plugin-marketplaces#marketplace-schema` covers `agent-skills`, `anthropic-*`, `claude-code-*`, etc., but not project names). Confirm before first publish; if claimed by then, fall back to `grava-pipeline`.

## Hooks file (`plugins/grava/hooks/hooks.json`)

Hooks are declared in the plugin so install registers them automatically — no `.claude/settings.json` editing per project. Story 2B.11 keeps responsibility for the **registered shape** (event names, matchers); this story makes the registration declarative and tied to the plugin lifecycle.

```json
{
  "PostToolUse": [
    {
      "matcher": "Bash",
      "hooks": [
        { "type": "command", "command": "${CLAUDE_PLUGIN_ROOT}/scripts/hooks/sync-pipeline-status.sh" }
      ]
    }
  ],
  "Stop": [
    {
      "hooks": [
        { "type": "command", "command": "${CLAUDE_PLUGIN_ROOT}/scripts/hooks/warn-in-progress.sh" }
      ]
    }
  ]
}
```

> `${CLAUDE_PLUGIN_ROOT}` resolves to the plugin's cache directory (`~/.claude/plugins/cache/<marketplace>/<plugin>/`). This avoids the absolute-path brittleness in 2B.11's example settings — the same hook works whether installed at repo root, at `~/.claude/plugins/`, or via `--add-dir`.

## What the plugin install does NOT cover (and why)

Three install steps are inherently per-project and stay manual or move to `grava bootstrap`:

1. **The `grava` Go binary.** A plugin can't drop binaries into `$PATH`. Solution: `grava bootstrap` (new subcommand) detects whether `grava` is on `$PATH`; if not, prints install hints (`brew install grava/tap/grava` once we publish a tap, or `go install github.com/<owner>/grava@latest`).
2. **The git `commit-msg` hook (story 2B.15).** Lives in `.git/hooks/`, which is intentionally not version-controlled. `grava bootstrap` runs `./scripts/install-git-hooks.sh` (now bundled in the plugin) against the project's `.git/`.
3. **Cron entries for `pr-merge-watcher.sh` and `run-pending-hunts.sh`.** Cron is per-machine, per-user. `grava bootstrap` prints the exact `crontab -e` lines (with `$CLAUDE_PLUGIN_ROOT` resolved); operator pastes them. Auto-installing crons crosses a trust boundary (modifies user's machine state outside the project) — printing is the right level of magic.

## `grava bootstrap` subcommand

```
grava bootstrap [--print-cron] [--skip-git-hooks] [--skip-binary-check]
```

Behavior:

```
$ grava bootstrap
[1/4] Checking grava binary on $PATH...                     OK (grava 0.2.0)
[2/4] Checking Claude Code plugin install...                OK (~/.claude/plugins/cache/grava/grava 0.2.0)
[3/4] Installing git hooks (.git/hooks/commit-msg)...       OK (linked to plugin)
[4/4] Cron lines (paste into `crontab -e`):
      */15 * * * * cd /Users/me/proj && /Users/me/.claude/plugins/cache/grava/grava/scripts/pr-merge-watcher.sh
      0    *  * * * cd /Users/me/proj && /Users/me/.claude/plugins/cache/grava/grava/scripts/run-pending-hunts.sh

Bootstrap complete. Try: /ship <issue-id>
```

`grava bootstrap --print-cron` emits only the cron block (machine-readable; useful for piping into a Makefile target). On failure of any step, prints a recovery hint and exits non-zero so operators don't miss it.

> **Why a CLI subcommand and not a skill.** A skill body would have to ask Claude to run `gh`, `crontab`, etc. — too many moving parts and per-project ambiguity. The CLI is deterministic, scriptable, and the natural owner of "machine-state verification."

## Install path — operator's view

```bash
# 1. Install the binary (once per machine)
brew install grava/tap/grava           # OR: go install github.com/<owner>/grava@latest

# 2. In the target project, register the marketplace and install the plugin
cd /path/to/new-project
claude
/plugin marketplace add <owner>/grava
/plugin install grava@grava

# 3. Bootstrap project-local pieces
grava bootstrap

# 4. Verify
/ship --help                           # skill resolves
ls .git/hooks/commit-msg               # git hook installed
crontab -l | grep pr-merge-watcher     # cron line live (after paste)
```

Time-to-first-`/ship` on a fresh project: ~2 minutes (most of which is `brew install`).

## Versioning + update flow

- Plugin `version` is bumped manually in `plugin.json` and mirrored in `marketplace.json` (the plugin entry's `version`). Both fields must agree — CI check enforces.
- Users update with `/plugin marketplace update grava` then `/plugin update grava@grava`.
- Skills/agents are content-only; updates are safe to take without coordination. Hooks and scripts MAY change interface — release notes in `plugins/grava/CHANGELOG.md` flag breaking changes ("requires `grava` binary ≥ 0.3.0").
- The `grava` Go binary versions independently. Plugin manifest declares a minimum binary version in `plugin.json`'s description (informational); `grava bootstrap` enforces at runtime — mismatch → red error pointing at upgrade path.

## Acceptance Criteria

- `.claude-plugin/marketplace.json` valid against the [marketplace schema](https://code.claude.com/docs/en/plugin-marketplaces#marketplace-schema); validates with `claude plugin lint` (built-in) in CI.
- `plugins/grava/.claude-plugin/plugin.json` declares all 12 skills, 5 agents, 2 hooks; discovery uses default directory layout (no custom `skills`/`agents`/`hooks` paths in the manifest).
- `/plugin marketplace add ./` (run from repo root) registers a marketplace named `grava` with one plugin.
- `/plugin install grava@grava` succeeds; afterwards `/` autocomplete shows `/ship`, `/plan`, `/hunt` and the `grava-*` skills (the user-invocable ones); `coder`, `reviewer`, `bug-hunter`, `planner`, `pr-creator` resolve as subagents (`Agent({ subagent_type: "coder", ... })`).
- After install, the `PostToolUse` and `Stop` hooks fire — verified by emitting a fake `CODER_DONE: abc` from a Bash tool call and asserting `pipeline_phase` wisp advances.
- `${CLAUDE_PLUGIN_ROOT}` resolves correctly inside `hooks.json`; manually grep'ing `.claude/settings.json` after install shows the project did NOT need direct edits.
- Plugin install on a sibling project (`/Users/trungnguyenhoang/IdeaProjects/gravav6-sandbox`) loads all skills + agents + hooks without modifying that project's `.claude/`.
- Symlinks under `plugins/grava/skills/` resolve in the cache copy. CI smoke test: install plugin into a temp dir, list `~/.claude/plugins/cache/grava/grava/skills/`, assert each skill's `SKILL.md` is readable (no dangling symlink).
- `grava bootstrap` exits 0 on a project with binary + plugin installed and prints exactly 4 numbered steps; exits non-zero with a clear hint when any step fails.
- `grava bootstrap --print-cron` emits two valid cron lines (no commentary, no banner) suitable for `crontab -e` paste.
- `grava bootstrap --skip-git-hooks` skips step 3; `--skip-binary-check` skips step 1 — both leave the rest intact.
- `grava bootstrap` is idempotent: running twice on a fresh project produces identical step results (git hooks are symlinks, not copies; cron lines deduplicated against `crontab -l` if `--apply-cron` flag is later added).
- Plugin `version` and marketplace plugin-entry `version` are checked equal by CI; mismatch fails the PR.
- README at `plugins/grava/README.md` documents the install path (the operator-view block above) and links to story 2B.11 for the manual fallback (still supported for grava-the-monorepo development).
- `CHANGELOG.md` exists at `plugins/grava/CHANGELOG.md` with at least an entry for `0.2.0` (initial public version).
- `grava bootstrap` man-page entry / `--help` text describes each step's intent (one line each).
- A new project that has only `claude` + `git` + `gh` installed reaches green `/ship --help` in ≤ 5 user-typed commands. Acceptance test scripted in `tests/install/install.bats` (uses a throwaway tmpdir as the "new project").
- Phase 2B README's "Prerequisites" section gains a top-of-list bullet: "Install via plugin marketplace (preferred) — see story 2B.17. Manual install (story 2B.11) is supported for grava development only."

## Dependencies

- All Phase 2B agent/skill/hook stories (2B.0–2B.15) — the plugin packages their outputs.
- New: `pkg/cmd/bootstrap.go` — bootstrap subcommand. Story implements alongside the manifest.
- New: `tests/install/install.bats` — smoke test for the install path.
- `claude` CLI (Claude Code) ≥ the version that ships plugin marketplaces (Feb 2026 GA).
- `gh`, `git`, `crontab` available on PATH (the bootstrap step shells out to all three).

## Out of Scope

- Publishing to a third-party marketplace registry (`claudemarketplaces.com` listing). Initial distribution is "add our GitHub repo as a marketplace." Listing comes later — separate story when the plugin is stable.
- npm package mirror (`source: npm`). Optional follow-up; only justified if a non-Go consumer base appears.
- Brew tap for the `grava` binary. Tracked separately under release engineering.
- Auto-installing cron lines. Stays manual until we have telemetry showing operators forget — and even then, "auto-edit user's crontab" deserves its own design review.
- Migration tooling for projects that already manually installed via story 2B.11 (rare; documented as "delete the manual `.claude/skills/grava-*` and `claude/agents/{coder,reviewer,…}.md` and re-install via plugin"). One-shot doc note, not a script.

## Signals Tested

The install smoke test (`tests/install/install.bats`) asserts on `claude --version`, `claude plugin list` (JSON), `claude plugin lint`, and `grava bootstrap` exit codes. No new pipeline signals; this story does not touch `/ship`'s signal protocol.

## References

- [Plugin marketplaces](https://code.claude.com/docs/en/plugin-marketplaces)
- [Skills](https://code.claude.com/docs/en/skills) — skill directory layout and frontmatter
- [Plugin manifest schema](https://code.claude.com/docs/en/plugins-reference#plugin-manifest-schema)
- [Plugin caching and file resolution](https://code.claude.com/docs/en/plugins-reference#plugin-caching-and-file-resolution) — confirms `${CLAUDE_PLUGIN_ROOT}` semantics and symlink behavior
- Story 2B.11 — manual install path, kept for monorepo development
- Story 2B.15 — git hook + cron scripts the bootstrap registers
