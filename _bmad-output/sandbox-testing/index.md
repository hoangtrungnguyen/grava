# Sandbox Testing — Artifact Index

**Project:** grava
**Created:** 2026-03-21

This folder contains planning-level requirements and test plans for sandbox validation of the Grava system. It complements the existing sandbox implementation guide and scenario definitions.

---

## Relationship to Existing Sandbox

The live sandbox implementation already exists at [sandbox/SANDBOX_VALIDATION_GUIDE.md](../../sandbox/SANDBOX_VALIDATION_GUIDE.md), which defines **8 Phase 2 release-gate scenarios** with scripts, CI integration, and troubleshooting guides.

This folder provides the **planning layer** on top of that:

| This folder | sandbox/ (existing) |
|-------------|---------------------|
| Requirements (what must be validated and why) | Scenarios (how to validate, scripts, CI) |
| Acceptance criteria in Given/When/Then format | Scenario implementation files |
| NFR thresholds and chaos requirements | `run-scenarios.sh` execution |
| Phase 1 + Phase 2 traceability to PRD | Phase 2 release gate focus |

---

## Documents

| File | Purpose | Status |
|------|---------|--------|
| [requirements.md](requirements.md) | Sandbox testing requirements (FRs, NFRs, constraints) mapped to PRD + Architecture | Draft |
| [test-plan.md](test-plan.md) | Test scenarios and acceptance criteria in Given/When/Then format | Draft |

---

## Scope

### Phase 1 (Core Substrate MVP)
- Single repository, local machine
- Up to 30 concurrent autonomous agents
- 10,000+ active and ephemeral issues
- Concurrent `claim`, `ready`, `dep`, `update` operations
- Git hook triggers, merge driver activation, worktree lifecycle

### Phase 2 (Multi-Agent Orchestration — existing sandbox gate)
- 8 scenarios defined in [sandbox/SANDBOX_VALIDATION_GUIDE.md](../../sandbox/SANDBOX_VALIDATION_GUIDE.md)
- CI-gated release validation
- Multi-branch concurrent agent orchestration

**Out of scope for this folder:**
- Phase 3 daemon/RPC mode
- Telegram/WhatsApp notifiers (Phase 2 implementation detail)
