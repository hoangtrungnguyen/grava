# Story 5.2: Claim -> .worktree/ Provisioning

Status: ready-for-dev

## Story

As an agent,
I want `grava claim` to automatically provision a Git worktree in the `.worktree/` folder,
Ensuring a clean root for my primary workspace.

## Acceptance Criteria

1. **AC#1 -- Mandatory Worktree Creation in .worktree/**
   By default, `grava claim <id>` MUST execute:
   `git worktree add .worktree/<id> -b grava/<id>`
   
2. **AC#2 -- Identity Enforcement**
   The identifier `<id>` used for the directory name MUST match the issue ID precisely.
   The branch MUST be prefixed with `grava/`.

3. **AC#3 -- Conflict Resolution**
   If a directory at `.worktree/<id>` or a branch `grava/<id>` exists, Grava MUST abort the claim to prevent state corruption.

4. **AC#4 -- Atomicity**
   Database state (Set `claimed_by`) and File System state (`git worktree add`) MUST be atomic. If the worktree fails, the DB claim is rolled back.

## Tasks / Subtasks

- [ ] Task 1: Update Claim CLI
  - [ ] 1.1 Use the path resolved from `git config grava.worktreeDir`.
  - [ ] 1.2 Implement the `git worktree add` orchestration.

- [ ] Task 2: Testing
  - [ ] 2.1 Write integration tests that mock Git failures to verify DB rollbacks.

## Dev Notes

### Working Directory
The `git worktree add` command should be executed with the project root as the CWD.

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-05-worktree.md#Story-5.2]
