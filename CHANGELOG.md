# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [v0.0.7] - 2026-04-11

* fix: resolve critical and high-severity Story 3.4 findings (3b5b3e7)
* fix: resolve 7 findings in Story 3.3 code review (e0d35ba)
* refactor: prune legacy BMAD documentation and workflow files while updating active agent skills and configurations. (161b189)
* fix: update RemoveDependency test to expect all queries and handle duplicate LogEventTx calls (ebf50c1)
* fix: remove unnecessary assignee query mock from ready/blocked tests (f91695c)
* fix: update test mocks to match implementation with metadata column and corrected query expectations (bab0f5a)
* chore: add .DS_Store to gitignore (39c5c48)
* fix: update blocked and ready command tests to match implementation (f9fb8c0)
* chore(data): refresh issues.jsonl export (bed2c99)
* chore(skills): split grava code review into severity-tagged comments (b1291ed)
* chore(skills): streamline complete-dev-story skill (grava-d193) (0782158)
* chore(skills): add grava workflow skills (grava-03bc) (b44c36c)
* fix(cli): allow --last-commit as sole update flag and expose last_commit in show --json (grava-7b39) (dbf8db0)
* chore(config): update gitignore and add deferred work artifact (grava-a3fd) (aa9758c)
* chore(data): refresh issues.jsonl export (4e6d9a2)
* chore(skills): split grava code review into severity-tagged comments (4622db2)
* chore(skills): streamline complete-dev-story skill (grava-d193) (b10ff38)
* chore(skills): add grava workflow skills (grava-03bc) (12711ec)
* fix(cli): allow --last-commit as sole update flag and expose last_commit in show --json (grava-7b39) (00e0f68)
* chore(config): update gitignore and add deferred work artifact (grava-a3fd) (ea84b94)
* feat: implement dependency management and audit logging (Story 4.1) (91c931c)
* feat(cli): add blocked command to query blockers for a specific issue (4-3) (b3c323e)
* docs: standardize mandatory worktree orchestration on .worktree folder and Claude integration (3aae95c)
* feat(cli): add assignee field to ready command JSON output and fix empty state (4-2) (ccfbc79)
* feat(cli): add --remove flag, audited transactions, and validation to dep command (4-1) (17a2b78)
* docs: finalize Epic 6 story implementation specs and sync sprint status (db63bbc)
* clean (937b367)
* docs: finalize implementation stories for Epics 4 and 5; sync roadmap tracking (50abde7)
* docs: reorder phase 1 roadmap for worktree-first strategy and remove onboarding (c3a8540)
* chore: move test docs and scripts into sandbox directory (6f32557)
* docs: correct release process documentation to match script automation (2d7d815)
* docs: restructure README to match explicit project specifications (413841a)


## [v0.0.6] - 2026-04-09

### Added
* feat(issues): implement Epic 2 and 3 — issue lifecycle, claiming, and session tracking
* feat(cli): implement `grava start/stop` for work session tracking (grava-1073)
* feat(cli): implement atomic issue claim and ephemeral state (grava-e4b2)
* feat(lifecycle): implement archive and purge issues (Story 2.6)
* feat(cli): implement label and comment on issues (Story 2.5)
* feat(cli): add db lifecycle commands
* feat(sandbox): implement multi-agent orchestration validation framework
* feat: initialize Serena project configuration and documentation
* feat: implement liveness subsystem with wisp validation and circuit breaking

### Changed
* refactor(cli): modularize issues package and centralize error handling (grava-a0df.1)
* chore(ci): upgrade all GitHub Actions to latest versions
* chore(ci): enable CI build on main branch
* docs: initialize comprehensive project documentation and automated update scripts

### Fixed
* fix(sandbox): implement missing scenario functions and reporting logic
* fix(review): address numerous adversarial code review findings for Epics 1, 2, and 3

## [v0.0.5] - 2026-03-09

* chore: ignore .grava.yaml (9c8396c)
* feat(debug): implement persistent development logging system (grava-8892) (a4eadff)
* docs: reorganize folder structure and update internal paths (grava-2c31) (7910176)
* feat(docker): add sandbox support and github release publish (grava-ec13.4) (eba688d)
* docs: add agent workflow documentation (grava-273a) (d1d3ab8)
* docs: rewrite README with Quick Start and Troubleshooting sections (grava-ec13.3) (ed5b633)
* feat(install): add go install support with auto version detection (grava-ec13.1) (2ff09ba)
* feat(install): default to ~/.local/bin, remove sudo requirement (grava-ec13.2) (fe79f11)

## [v0.0.2] - 2026-02-23

* fix(scripts): inject version via -ldflags in build.sh (af83007)
* feat(graph): implement persistent updates for status and priority with audit logging (grava-0637.6) (f1af74d)
* feat(graph): add store and session metadata to AdjacencyDAG (1b97c80)
* docs: add design and plan for persistent graph updates (bd7300d)
* feat(graph): implement metadata integrity in graph loading (grava-0637.5) (89b64d8)
* feat(graph): implement CRUD audit for create operations and wisp indicators (grava-0637.4) (6d2eec4)
* feat(audit): implement audit logging for create operations and fix related tests (grava-0637.4) (8e20e6a)
* feat(cli): add 'grava dep tree' and 'grava dep path' commands (grava-1f45.4) (a458ebd)
* test(graph): add scale benchmarks and verify performance targets (grava-1f45.3) (62386f3)
* feat(graph): implement incremental cache updates and granular invalidation (grava-1f45.2) (36c0333)
* feat(graph): implement transitive reduction and blocking path algorithms (grava-1f45.1) (649eedd)
* docs: Update CLI reference with new graph and dependency commands (94ba101)
* Implement Phase 6: CLI Integration for graph mechanics (4631b73)
* feat(graph): update gates implementation (grava-b389.1, grava-b389.2) (55bdc02)
* feat(graph): define standard errors and CycleError struct (grava-2c49.1, grava-2c49.2) (7337803)
* feat(graph): finalize ready engine implementation and tests (grava-2867) (080c0c6)
* docs: finalize graph implementation plan document (8c8d3cb)
* feat(graph): implement core graph data structures and ready engine (grava-b9c9, grava-7aa1, grava-bf26) (d363cf2)
* feat: introduce an optimized agent scheduler, add performance benchmarks, and provide optimization documentation. (b709530)
* feat(epic-2): Complete AgentScheduler implementation and comprehensive benchmarking (e73d8d2)
* feat(cli): Support dynamic port discovery and persistence in grava init (grava-0999) (d8f5f72)
* chore(cli): set default version to v0.0.1 (373fe37)
* feat(cli): add version command (grava-e912) (581af95)
* docs: document start and stop commands and implementation plan (grava-831f) (f063dca)
* test: add integration tests for grava start and stop commands (e6a8afe)
* fix: improve port extraction and stop script robustness (a5c7445)
* feat: implement grava start and stop commands (857df01)
* feat: add utility to find scripts relative to binary or CWD (85e3adf)
* refactor: enhance scripts to support port arguments and non-interactive stop (e8200e0)
* chore: ignore .worktrees directory (ec74bde)
* feat: finalize init enhancement and config command (grava-4f4c) (f40c683)
* docs: update init and config command documentation (a5b6f6b)
* feat: add grava config command (7d10b67)
* feat: add available port detection utility (c3c1f37)
* docs: document database migration system (acb5ba6)
* feat(cli): automate database migrations using goose (grava-b777) (a61c892)
* add doc one liner download (786fd6b)
* chore: implement automated changelog generation and tagging for release (ac35911)
* docs: add history and undo commands to README (f218a11)
* update safe guard (e698dea)
* fix(cli): fix subtask command priority, undo behavior, and test isolation (f9f069b)
* feat(cli): Implement history, undo, and commit commands (grava-de78.11) (1d552fe)
* feat(cli): Implement stats command (grava-de78.12) (874c48f)
* feat(cli): Implement Soft Delete and Export/Import commands (grava-de78.10, grava-de78.15) (4532610)
* docs: document --sort flag for list command (grava-fe76) (5253add)
* feat(cli): integrate --sort flag into grava list (grava-fe76) (f66abc9)
* feat(cli): add sort flag parser logic (grava-fe76) (b1ebc01)
* docs: implementation plan for list command sorting (67aa912)
* docs: design for list command sorting (grava-fe76) (9bfb4f7)
* docs(workflow): Add documentation step to landing-the-plane (1e68fca)
* docs: Add missing clear command documentation and fix headers (6b73207)
* feat(cli): Implement global --json output flag (grava-de78.8) (1873b21)
* docs(plan): Add implementation plan for JSON output flag (grava-de78.8) (894c1c9)
* feat(validation): Implement input validation layer (grava-de78.6) (01c54fb)
* feat(skill): Add 'getting-highest-priority-issue' skill (grava-6575) (4c16392)
* feat(docs): Add release process documentation and scripts (grava-6db7) (45cedf9)
* feat: install agent skills and claude code templates (a1d8245)
* feat: Add Dolt SQL server logs, create grava, and update agent workflow. (38ec069)
* feat(cli): Add files support to subtask command and update CLI docs related to affected_files (48f61f8)
* feat(cli): Implement grava drop and clear with transaction safety (11af8c2)
* feat: Add database testing safeguard rule, update the landing plane workflow with command warnings, and remove the `grava sync` plan. (8862dda)
* feat(scripts): add build script (3138632)
* feat(cli): implement clear command and add audit columns (grava-863c) (467035b)
* docs(epic): rename epic doc to .md and add agent rules (e53c9f2)
* chore(workflow): add Grava-native landing-the-plane workflow (4bb6a2e)
* feat(cli): implement grava drop command (grava-863c.1) (36c90fd)
* chore(tracker): session log + tracker update for TASK-1-8 (2026-02-18) (4f16430)
* feat(cli): implement search, quick, doctor commands (TASK-1-8) (fdbe2ef)
* chore(landing): update README with CLI quick-start; archive session log for TASK-1-7 (6318fd4)
* refactor(scripts): move test scripts into scripts/test/ subfolder (e36579c)
* test(e2e): add e2e_test_all_commands.sh — smoke tests all CLI commands against live Dolt (d3be148)
* feat(cli): implement comment, dep, label, assign commands (TASK-1-7) (edd1a90)
* chore(tracker): add session log for TASK-1-6.subtask-2 (2026-02-18) (09bcf22)
* feat(cli): implement grava compact — Wisp compaction with tombstone tracking (f1d9557)
* plan for skill storage (a1cb464)
* test(integration): Add comprehensive foreign key constraint tests (2d14bee)
* feat(benchmark): Add comprehensive insert performance benchmarks (1119dd8)
* fix(cli): fix DB name mismatch in start script, sort list by priority+date (f30eaa5)
* docs: Add CLI reference guide (43bfcce)
* feat(cli): Implement basic CRUD and subtasks, setup test infrastructure (43a529f)
* Update Epic 1 and tasks with comprehensive Beads commands requirements (b99d4a3)
* chore: configure dolt user for datagrip compatibility (aba65ba)
* feat: implement hierarchical ID generator with atomic Dolt counters (c60471b)
* docs: complete Task 1-1 and add remaining Epic 1 tasks (de13d64)
* feat: Initialize Dolt database and documentation (Task 1-1) (2ab5783)
* update plan (f8f3378)
* update cmd (ce13e59)
* update readme (91b9a9a)
* chore(docs): update epics structure, rename beads to grava, add log saver epic (e1b2e92)
* initial commit: project architecture and MVP epics (64e34be)
* Initial commit (2fbb0a4)

