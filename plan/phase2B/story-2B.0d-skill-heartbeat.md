# Story 2B.0d: `grava-dev-task` Inline Heartbeat

Skill prerequisite for the Phase 2B pipeline. The longest window in the pipeline is **Phase 1 (coding)**, which spans the full TDD loop inside `grava-dev-task` — typically tens of minutes for a non-trivial task, occasionally over an hour. `grava doctor`'s stale-claim detection relies on `orchestrator_heartbeat`. Today only `/ship` writes that wisp, and only at phase boundaries — so during Phase 1 the heartbeat freezes at the value seeded right before the coder spawn, and a 30-minute task looks indistinguishable from a 30-minute crash.

This story amends `grava-dev-task/workflow.md` to write `orchestrator_heartbeat` at every workflow checkpoint, including inside the RED → GREEN → REFACTOR loop. After this story lands, `/ship` is no longer the sole writer of the heartbeat wisp; the contract is **shared** with the skill, with the skill carrying the load during the implementation window.

## Why heartbeat in the skill, not bumped threshold

Two alternatives were considered:

| Option | Why rejected |
|--------|--------------|
| Bump `grava doctor` stale threshold from 30 min to 90 min | Hides genuine 60-minute crashes. The pipeline already has two well-defined phases (coding vs. waiting); the right fix is to instrument the long phase, not relax the alarm. |
| Heartbeat from `/ship` while waiting on the coder | `/ship` is blocked on the `Agent` call — it cannot run a side-loop. A timer thread would require shell trickery (`&` + trap) that breaks under sub-shell isolation. |

Inline heartbeat in the skill is the natural shape: the agent **is** the long-running phase, and it already writes wisps at every checkpoint. Adding one more wisp write per checkpoint is essentially free.

## File

`.claude/skills/grava-dev-task/workflow.md` (existing — adds 8 heartbeat lines)

## Required surface

At each of the following workflow points, write `orchestrator_heartbeat` immediately after the existing checkpoint operation. Use a small shell helper to keep the diff small:

```bash
# helper convention — paste at top of the workflow's bash blocks, or reference inline
HEARTBEAT() { grava wisp write "$1" orchestrator_heartbeat "$(date -u +%s)"; }
```

Insertion points (matched to existing workflow structure):

| Step | Location | Existing checkpoint | New write |
|------|----------|---------------------|-----------|
| 3 | After `grava claim` | `step "claimed"` | `HEARTBEAT $ID` |
| 3 | End of context load | `step "context-loaded"` | `HEARTBEAT $ID` |
| 4 | Pre-implementation | `current_task "..."` | `HEARTBEAT $ID` |
| 4 | RED entry (start of failing-test write) | (none) | `HEARTBEAT $ID` |
| 4 | GREEN entry (start of impl) | (none) | `HEARTBEAT $ID` |
| 4 | REFACTOR entry | (none) | `HEARTBEAT $ID` |
| 5 | After scoped validation | `step "validated"` | `HEARTBEAT $ID` |
| 7 | After commit recorded | `step "complete"` | `HEARTBEAT $ID` |

This produces a ≤5-minute heartbeat cadence under normal Phase 1 flow (the RED-GREEN-REFACTOR cycle for a single test rarely exceeds 5 min in TDD; if it does, the skill has bigger problems than heartbeats). A task that takes 60 minutes will write the heartbeat ~12 times.

## Why these specific points (not a timer)

- **Logical, not wall-clock.** Heartbeats fire when the skill makes meaningful progress, not on a fixed schedule. A 90-second pause for a slow test is fine; a 60-minute pause means the agent is stuck and the heartbeat correctly stops advancing.
- **No background threads.** Bash-style `( while sleep 60; do HEARTBEAT; done ) &` requires PID tracking and trap cleanup; the skill is not the right place for that complexity.
- **Cheap.** One `grava wisp write` per checkpoint; no measurable cost on top of work the skill already does.

## Acceptance Criteria

- `grava-dev-task/workflow.md` writes `orchestrator_heartbeat` at all 8 insertion points listed above
- All writes use the same wisp key (`orchestrator_heartbeat`) and same value format (UTC unix timestamp from `date -u +%s`) as `/ship`
- A 60-minute simulated Phase 1 (RED→GREEN→REFACTOR for ≥5 small tests) produces ≥10 heartbeat writes — verifiable by `grava wisp read <id> orchestrator_heartbeat` returning a value ≤5 min behind wall clock at any sample point during the run
- `grava doctor` does NOT flag a Phase 1 issue as stale during normal flow (heartbeat advances faster than the 30-min stale threshold)
- `grava doctor` DOES flag a Phase 1 issue as stale within ~30 min of the agent crashing or hanging (heartbeat freezes; threshold trips)
- The `step` wisp continues to be written at the existing checkpoints — heartbeat is **additive**, not a replacement
- No new dependencies (no need for daemons, timers, or external state)

## Dependencies

- None for the skill change itself
- Story 2B.5 (`/ship`) — its heartbeat write at Phase 1 entry remains as the pre-coder seed; the skill's first heartbeat (at `step "claimed"`) overwrites it within a few seconds. No coordination needed.

## Pipeline impact

- Story 2B.5: `pipeline_phase` seeding contract unchanged; the seed line `grava wisp write "$ISSUE_ID" orchestrator_heartbeat ...` becomes "first writer" rather than "sole writer"
- Story 2B.11: the Wisp Keys table's `orchestrator_heartbeat` row gains `grava-dev-task` skill as a co-owner
- `grava doctor`: no change required — it reads the wisp and applies the threshold; it does not care who writes it

## Out of scope

- Heartbeats inside other skills (`grava-code-review`, `grava-bug-hunt`) — those phases are short (Phase 2 review, ad-hoc hunt). If they grow long, file a follow-up story per skill.
- Configurable heartbeat cadence — not needed; checkpoint-driven cadence is sufficient.
- Tunable stale threshold — `grava doctor` keeps its 30-minute threshold; this story makes that threshold survivable for Phase 1.
