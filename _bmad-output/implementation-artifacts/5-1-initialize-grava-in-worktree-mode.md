# Story 5.1: Initialize Grava in Worktree Mode

Status: ready-for-dev

## Story

As a developer,
I want to initialize Grava with worktree mode enabled,
So that the coordinator starts managing the Dolt server lifecycle and each agent gets an isolated branch and directory.

## Acceptance Criteria

1. **AC#1 -- Worktree Mode Opt-in**
   Given a Git repository where `grava init` has been run or is being run,
   When I run `grava init --enable-worktrees`,
   Then `.grava/config.yaml` (or .json) is updated/created with `worktrees_enabled: true`,
   And the command returns `{"status": "initialized", "worktrees_enabled": true}` in `--json` mode.

2. **AC#2 -- Coordinator CLI**
   There MUST be a command `grava coordinator start`,
   When executed, it launches the long-running coordinator process (managed in `pkg/coordinator`),
   And it manages the Dolt SQL server lifecycle (start/stop),
   And it executes any pending schema migrations exclusively.

3. **AC#3 -- Coordinator Error Handling**
   The coordinator MUST use the `Start(ctx) <-chan error` pattern (ADR-FM3),
   And no goroutine inside the coordinator may call `log.Fatal`, `os.Exit`, or `panic`,
   And errors from the background loop are propagated to the CLI for structured reporting.

4. **AC#4 -- Idempotency**
   When `grava init --enable-worktrees` is run on a repository already configured for worktrees,
   Then it returns `{"status": "already_enabled"}` and makes no changes.

5. **AC#5 -- Git Requirement**
   Running `grava init --enable-worktrees` in a directory that is NOT a Git repository returns `{"error": {"code": "NOT_A_GIT_REPO", ...}}`.

## Tasks / Subtasks

- [ ] Task 1: Update `grava init` logic
  - [ ] 1.1 Add `--enable-worktrees` flag to `newInitCmd` in `pkg/cmd/init.go`.
  - [ ] 1.2 Update `viper` to save `worktrees_enabled` to the persistent config file.
  - [ ] 1.3 Add validation check for `.git` directory existence.

- [ ] Task 2: Implement `grava coordinator start` Command
  - [ ] 2.1 Create `pkg/cmd/coordinator.go` (or add to `pkg/cmd/maintenance/`).
  - [ ] 2.2 Implement `runCoordinatorStart` which instantiates `coordinator.New(...)`.
  - [ ] 2.3 Wire the `Start(ctx)` channel to a select loop that waits for SIGINT/SIGTERM or background errors.

- [ ] Task 3: Implement Dolt Server Lifecycle in Coordinator
  - [ ] 3.1 Use `os/exec` or a library to manage `dolt sql-server`.
  - [ ] 3.2 Ensure the server is shut down gracefully when the coordinator stops.

- [ ] Task 4: Unit Testing
  - [ ] 4.1 Mock the coordinator loop to verify error propagation.
  - [ ] 4.2 Test `init` config persistence.

## Dev Notes

### Config Persistence
Viper should handle the config file. Ensure `viper.WriteConfig()` is called during `init`.

### Architecture Patterns
- **Coordinator:** See existing stub in `pkg/coordinator/coordinator.go`.
- **Dolt Server:** The coordinator is the "owner" of the shared Dolt server in worktree mode.

### References
- [Source: pkg/coordinator/coordinator.go]
- [Source: _bmad-output/planning-artifacts/epics/epic-05-worktree.md#Story-5.1]
