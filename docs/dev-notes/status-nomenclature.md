# Status Nomenclature: "paused" vs "open"

**Date:** 2026-04-07
**Origin:** Code review finding (grava-1073.14)

## Context

Early drafts of Epic 02 and Story 2.4 (grava-1073) described `grava stop` as transitioning status to `paused`. During implementation, the architecture audit table and acceptance criteria settled on `open` as the target status for `grava stop`. The current schema validation (`pkg/validation/validation.go`) recognizes: `open`, `in_progress`, `closed`, `archived`, `tombstone`. There is no `paused` status.

## Authoritative Reference

The implementation is authoritative. `grava stop` transitions `in_progress -> open`. The term "paused" was an early draft artifact and does not exist in the codebase.

## Files Affected by Nomenclature

- `docs/epics/Epic_02_Issue_Lifecycle.md` — may still reference "paused" in prose
- `_bmad-output/implementation-artifacts/2-4-track-work-start-and-stop.md` — AC#2 and implementation use "open"
- `pkg/cmd/issues/stop.go` — hardcodes status transition to "open"
- `pkg/validation/validation.go` — no "paused" in AllowedStatuses
