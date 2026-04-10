# Epic 5: Mandatory Worktree Orchestration

**Status:** Planned
**Grava ID:** grava-add5
**Matrix Score:** 4.15
**FRs covered:** FR23, FR5, FR4

## Goal

`grava claim` is transformed from a simple database update to a **Mandatory Worktree Provisioner**. Every successful claim MUST result in an isolated Git worktree and branch. This ensures that no agent can work in the project root, protecting the stability of the master branch.

## Directory Standard: `.worktree/`

To ensure consistency between manual Git operations, Grava commands, and Claude sessions, **all worktrees MUST be stored in a unified hidden directory at the project root:**

- **Base Path:** `{{root}}/.worktree/`
- **Isolation Path:** `{{root}}/.worktree/<issue-id>/`
- **Branch Naming:** `grava/<issue-id>`

## Orchestration Modes

| Mode | Trigger | Storage Path | Branch Name |
| :--- | :--- | :--- | :--- |
| **Standard (Default)** | `grava claim <id>` | `.worktree/<id>/` | `grava/<id>` |
| **Claude-Native** | `grava claim <id> --launch` | `.worktree/<id>/` | `grava/<id>` |

### Implementation Note for Claude Mode
Since Claude defaults to `.claude/worktrees/`, Grava's `init` command will attempt to configure a **WorktreeCreate hook** or use the `--worktree` flag with explicit paths to ensure Claude honors the `.worktree/` directory at the project root.

## Commands Delivered

| Command | FR | Description |
| :--- | :--- | :--- |
| `grava claim <id>` | FR5 | **Default:** Calls `git worktree add .worktree/<id> -b grava/<id>`. |
| `grava claim <id> --launch` | FR5 | **Claude Mode:** Prepares state, then executes `claude --worktree <id>`. |
| `grava close <id>` | FR4 | Atomic teardown: deletes `.worktree/<id>/` and branch. |
| `grava stop <id>` | FR4 | Pause: deletes directory, keeps branch. |
| `grava init` (Enhanced) | FR23 | Configures `.worktree` as the global storage and bootstraps `.claude/settings.json`. |

## Stories

### Story 5.1: Automatic Master-Root Resolution *(grava-355a)*
As a developer working in an isolated worktree, I want Grava to automatically link to the database in my parent workspace.

### Story 5.2: Claim -> `.worktree/` Provisioning *(grava-02b0)*
As an agent, I want `grava claim` to automatically provision a Git worktree in the `.worktree/` folder, ensuring a clean root.

### Story 5.3: Claude Integration with Custom Path *(grava-5c5a)*
As a user of Claude Code, I want Grava to ensure that `claude --worktree` uses the project's `.worktree/` folder instead of its default location.

### Story 5.4: Recursive Lifecycle Cleanup *(grava-e2b4)*
As a developer, I want `grava close` to safely delete my isolated `.worktree/<id>` directory.

### Story 5.5: Configure Claude & Git on Init *(grava-f1a2)*
As a developer, I want `grava init` to automatically configure my Git and Claude settings to use the `.worktree/` folder at the project root for all isolated sessions.
