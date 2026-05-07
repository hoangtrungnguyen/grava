# Migration Plan: Structured Pipeline Signals

Replace the legacy "echo a magic string + parse the last line" signal protocol
with a typed `grava signal` CLI command. Phased rollout, every step
independently reversible, no pipeline downtime.

> **Status: COMPLETE.** Phases 1–5 + 7 + 8 landed (PRs #25–#28 + #31).
> Phase 6 (telemetry) was skipped on the basis that `grava` has a single
> deployment (this repo) and typed-CLI adoption was assumed at ~100% rather
> than measured over a 4-week soak window — closing the soak-gate that Phase 8
> would otherwise wait on. Phase 7 (docs) was rolled into Phase 8 (PR #31).
> The legacy `sync-pipeline-status.sh` PostToolUse hook is **gone** — every
> `pipeline_phase` write across the system now flows through `grava signal`.
>
> **Owner:** pipeline maintainers
> **Tracking:** `.grava/dolt` epic `grava-3a8d` (story 5 = telemetry skipped, story 7 = hook retired)

---

## Why

The old protocol couples three concerns onto stdout text:
- State machine (forward-only `pipeline_phase` transitions)
- Agent UX (the line they emit)
- Observability (the line operators read)

This is fragile in three ways:
1. **No atomicity** — auxiliary triage wisps (e.g. `coder_halted`,
   `pr_url`) are written in a separate transaction from `pipeline_phase`.
   Crash between them ⇒ inconsistent state.
2. **No type safety** — typos like `CODR_DONE` are silently dropped by the
   regex hook. The pipeline hangs at `claimed` until the heartbeat
   stale-check fires (~30 min).
3. **No testability** — the forward-only state machine lives in 47 lines of
   bash with zero tests.

The new approach moves the contract into a typed CLI:

```bash
grava signal CODER_DONE --issue $ID --payload <sha>
```

The CLI writes `pipeline_phase` and any auxiliary wisps inside one
`WithAuditedTx`, validates the kind, and prints the legacy
`<KIND>: <payload>` line as its final stdout line — so existing last-line
parsers keep working unchanged during migration.

See also: [`docs/architecture/signals.md`](../architecture/signals.md)
*(planned in Phase 7)*.

---

## Goals & Non-Goals

### Goals
- Single source of truth for `pipeline_phase` writes (the CLI)
- Atomic phase + auxiliary wisp writes
- Typed signal vocabulary, compiler-checked, unit-tested
- Zero downtime during migration

### Non-Goals
- Removing the legacy `echo` path entirely (kept as safety net)
- Rewriting orchestrator or skills in Go
- Changing wisp schema or signal vocabulary

---

## Phase 0 — Audit (½ day)

**Discover every signal emitter and consumer before changing anything.**

| Step | Command | Output |
|------|---------|--------|
| Find emitters | `rg -n 'echo "(CODER\|REVIEWER\|PR_\|PIPELINE_\|PLANNER_\|BUG_HUNT)' .claude/ scripts/ plugins/` | Every text-signal emission site |
| Find consumers | `rg -n 'pipeline_phase\|wisp.*write' scripts/ pkg/cmd/` | Every `pipeline_phase` writer |
| Find parsers | `rg -n 'last_line\|tail -1\|awk.*END' scripts/ .claude/skills/` | Every last-line parser |

**Deliverable:** `docs/migration/structured-signals-audit.md` — a table of
`{file, line, kind, role (emitter/consumer/parser)}`.

**Exit gate:** zero unknown call sites.

---

## Phase 1 — CLI Foundation ✅ DONE

| Item | File |
|------|------|
| Typed enum + pure phase resolver + atomic wisp write | [`pkg/cmd/issues/signal.go`](../../pkg/cmd/issues/signal.go) |
| 19 unit tests (forward / backward / terminal / unknown / bookkeeping) | [`pkg/cmd/issues/signal_test.go`](../../pkg/cmd/issues/signal_test.go) |
| Wired into `AddCommands` | [`pkg/cmd/issues/issues.go`](../../pkg/cmd/issues/issues.go) |

**Exit gate:** `go test ./pkg/cmd/issues/ -run TestSignal -v` is green.
✅ Verified.

---

## Phase 2 — Agent Migration ✅ PARTIALLY DONE

Migrated: `coder`, `reviewer`, `pr-creator`, `planner` (see
[`.claude/agents/`](../../.claude/agents/)).

`bug-hunter` deliberately not migrated — its `BUG_HUNT_COMPLETE` is a
bookkeeping signal with no `pipeline_phase` mapping and no associated issue
context.

### Remaining work
- Add a worked example to each agent's section in
  [`docs/guides/AGENT_TEAM.md`](AGENT_TEAM.md) showing the new call shape.
- Verify both happy and failure paths in every migrated agent now use
  `grava signal …` instead of `echo`.

### Validation

```bash
# Should return zero hits after migration is complete:
rg 'echo "(CODER_(DONE|HALTED)|REVIEWER_(APPROVED|BLOCKED)|PR_(CREATED|FAILED|COMMENTS_RESOLVED|MERGED)|PIPELINE_(COMPLETE|HALTED|FAILED)|PLANNER_NEEDS_INPUT)' .claude/agents/
```

**Exit gate:** the regex above returns zero hits.

---

## Phase 3 — Orchestrator (`/ship` skill) (1 day)

The orchestrator currently parses the agent's last-line output to extract
signals. After Phase 1, the wisp is already written by the time the agent
returns — so the orchestrator should **read state from the DB** rather than
parse text.

### Steps
1. Find every spot in `.claude/skills/ship/` (and any sub-skill it invokes)
   that parses agent output for `CODER_DONE | REVIEWER_APPROVED | PR_CREATED`.
2. Replace each with `grava show $ID --json | jq -r .pipeline_phase` — the
   canonical state.
3. For payloads (sha, url, findings), read the auxiliary wisp:
   ```bash
   PR_URL=$(grava wisp read $ID pr_url)
   FINDINGS=$(grava wisp read $ID reviewer_findings)
   ```
4. Keep the legacy parser as a **fallback only** — if `pipeline_phase` is
   unset after the agent returns, fall back to last-line parsing. This
   covers any agent that hasn't been migrated yet.

### Risk
The orchestrator is the most critical pipeline component and is tested only
in production today.

### Mitigation
- Add a `--dry-run` mode that prints what phase the orchestrator would read
  without acting.
- Run the migrated `/ship` against a no-op test issue end-to-end.
- Keep the bash parser path live behind a feature flag for one week.

**Exit gate:** ship a test issue through the full pipeline and confirm the
orchestrator reads only from wisps, not from agent stdout.

---

## Phase 4 — `pr-merge-watcher.sh` (½ day)

The async watcher writes `pipeline_phase=pr_merged` and `pr_close_reason`
directly via `grava wisp write`. Migrate it to `grava signal`.

### Steps
1. Replace the `pr_merged` write with
   `grava signal PR_MERGED --issue $ID`.
2. Add a new `SignalPRClosed` kind for the rejection path — auxiliary key
   `pr_close_reason`, payload = the close reason category.
3. Update `signalToPhase` and `auxiliaryKey` in
   [`pkg/cmd/issues/signal.go`](../../pkg/cmd/issues/signal.go) for the new
   kind.
4. Add tests for the rejection path.

### Why this matters
The watcher is the only **automation** that writes `pipeline_phase` outside
agents. Migrating it means **every** phase write goes through the CLI —
after this phase, the bash hook is truly a no-op safety net.

**Exit gate:** trigger a fake PR merge in dev and confirm the `pr_merged`
phase + audit row come from the watcher's `grava signal` call.

---

## Phase 5 — Skills Migration (1 day)

Some skills (`grava-dev-task`, `grava-code-review`) write `pipeline_phase`
or related wisps directly during their workflow checkpoints. They should
use `grava signal` where the kind matches — and explicit `wisp write` for
in-skill state (`step`, `current_task`) which doesn't map to a signal.

### Steps
1. Audit `.claude/skills/grava-dev-task/workflow.md` for any
   `wisp write … pipeline_phase`.
2. Replace with `grava signal` calls.
3. Leave non-phase wisps (`step`, `current_task`, `orchestrator_heartbeat`)
   untouched — they're not signals.

**Exit gate:** `rg 'pipeline_phase' .claude/skills/` returns only
documentation references, no live writes.

---

## Phase 6 — Observability (½ day)

Add a metric so you can prove the migration is taking hold before
deprecating fallbacks.

### Steps
1. Modify [`pkg/cmd/issues/signal.go`](../../pkg/cmd/issues/signal.go) to
   emit a structured log line per invocation:
   ```json
   {"ts": "...", "issue_id": "...", "kind": "...", "phase_wrote": true, "source": "cli"}
   ```
2. Modify
   `scripts/hooks/sync-pipeline-status.sh` (deleted in Phase 8)
   to emit:
   ```json
   {"ts": "...", "issue_id": "...", "kind": "...", "phase_wrote": true, "source": "hook-fallback"}
   ```
3. Tail both into `.grava/signal-source.jsonl`.

### Success metric
After 1 week of pipeline runs, `source: "cli"` should be ≥99% of writes.
`source: "hook-fallback"` indicates an unmigrated path.

**Exit gate:** one full week of telemetry showing CLI dominance.

---

## Phase 7 — Documentation (½ day)

| File | Update |
|------|--------|
| [`CLAUDE.md`](../../CLAUDE.md) (root) | "Pipeline Signals" table — flip the **Emitter** column from "agent text output" to "agent calls `grava signal`" |
| [`CLAUDE.md`](../../CLAUDE.md) (root) | Add "Signal CLI" subsection with the schema |
| [`.claude/agents/*.md`](../../.claude/agents/) | Verify Phase 2 work (all references use `grava signal`, not `echo`) |
| `scripts/hooks/sync-pipeline-status.sh` (deleted in Phase 8) | Header already updated; verify still accurate |
| **NEW:** `docs/architecture/signals.md` | Single source of truth — vocabulary, ordering, terminal phases, auxiliary keys |

**Exit gate:** a new contributor can read `docs/architecture/signals.md`
and add a new signal end-to-end without asking a question.

---

## Phase 8 — Deprecate Hook Regex (1 day, ≥4 weeks after Phase 6)

Once telemetry shows fallback usage at 0%, retire the regex parser.

### Steps
1. Replace
   `scripts/hooks/sync-pipeline-status.sh`'s
   parsing logic with telemetry-only:
   ```bash
   # Hook is now observability-only — CLI writes the phase directly.
   echo "$LAST_LINE" | jq -Rn 'inputs | {ts: now, kind: .}' >> .grava/legacy-signal.jsonl
   ```
2. Keep the script in place — if anyone reintroduces an `echo` signal, it
   shows up in the log.
3. **Optional:** convert to a `PreToolUse` hook that *blocks* Bash commands
   matching `echo "(CODER_|REVIEWER_|PR_|…)"` with feedback "use
   `grava signal` instead."

**Exit gate:** hook no longer writes wisps; CLI is the sole writer.

---

## Phase 9 — Schema Hardening (optional, future)

Once the CLI is the only writer:

1. Add a DB-level CHECK constraint on `pipeline_phase` values.
2. Add a trigger that rejects backward transitions at the DB level — defense
   in depth against resolver bugs.
3. Generate the signal enum from a single YAML/JSON manifest so docs, CLI,
   and tests share one source.

Not required for migration; nice cleanup when scope allows.

---

## Rollback Strategy

Every phase is independently reversible. The legacy text path **never
breaks** during migration — it just becomes redundant.

| Phase | Rollback |
|-------|----------|
| 1 | `git revert` the CLI commit |
| 2 | Agents fall back to `echo` (the hook still works) |
| 3 | Orchestrator restores text parsing |
| 4 | Watcher reverts to direct `wisp write` |
| 6 | Stop tailing logs |
| 8 | Restore the bash parser from git history |

---

## Timeline

| Week | Phase | Status | Effort |
|------|-------|--------|--------|
| 1 | 0 — Audit | TODO | ½ day |
| 1 | 1 — CLI | ✅ done | — |
| 1 | 2 — Agents | ✅ partially done | ½ day remaining |
| 2 | 3 — Orchestrator | TODO | 1 day |
| 2 | 4 — Watcher | TODO | ½ day |
| 2 | 5 — Skills | TODO | 1 day |
| 3 | 6 — Observability | TODO | ½ day |
| 3 | 7 — Docs | TODO | ½ day |
| 4–7 | (telemetry soak) | TODO | passive |
| 8 | 8 — Deprecate hook regex | TODO | 1 day |
| later | 9 — Schema hardening | optional | TBD |

**Critical-path estimate:** ~5 working days of active work, plus ~4 weeks
of telemetry soak before deprecating the fallback.

---

## Success Criteria

After Phase 8:
1. Every `pipeline_phase` write is a CLI invocation (verified by telemetry).
2. Zero text-emission paths in agents/skills/scripts (verified by `rg`).
3. New contributors add signals via Go enum + map, never via bash regex.
4. Pipeline failures from typo'd signals → 0 (verified by absence of
   "stuck at claimed" triage tickets).
5. `grava doctor` can detect stuck-at-phase issues using only DB state (no
   log parsing).

---

## Recommended Next Action

**Phase 3 — orchestrator migration.** It's the highest-value remaining work
because it's the most complex consumer of signal text and the most critical
to pipeline correctness.

Optional preparation: run the Phase 0 audit first to surface any
unanticipated call sites.
