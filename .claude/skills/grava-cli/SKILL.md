---
name: grava-cli
description: Context loader that teaches agents the grava CLI mental model, command composition patterns, error recovery, and JSON output contracts. Use when an agent needs to understand grava before working with it, or when asked "how does grava work", "explain grava", "what grava command should I use".
---

# Grava CLI — Agent Context Loader

This skill loads grava CLI knowledge into your context. It does NOT execute commands — it teaches you how grava works so you can reason about it.

## On Activation

Load the sub-document most relevant to the agent's need:

| Need | Document |
|------|----------|
| How grava works conceptually (entities, state machine, hierarchy) | [mental-model.md](mental-model.md) |
| How to chain commands for workflows (recipes, anti-patterns) | [command-patterns.md](command-patterns.md) |
| What went wrong and how to recover (error codes, retries) | [error-handling.md](error-handling.md) |
| What JSON shape a command returns (for parsing `--json` output) | [output-contracts.md](output-contracts.md) |

For command **syntax** (flags, arguments, usage), see the separate skill: `managing-grava-issues`.

## Quick Orientation

Grava is a git-native issue tracker backed by Dolt (a SQL database with git-like versioning). Key facts:

- **All state lives in the database** — issues, dependencies, comments, labels, wisps, reservations
- **`grava commit`** snapshots database state into Dolt version history (like `git commit` for issues)
- **`--json` flag** on any command produces structured output for programmatic use
- **`--actor` flag** identifies who is performing an action (defaults to `GRAVA_ACTOR` env var)
- **Commands are atomic** — write operations use transactions with audit logging
- **The dependency graph drives work selection** — `grava ready` surfaces only unblocked, high-priority tasks
- **Always run `grava` from the repo root** — Dolt config lives in the main checkout's `.grava/`. From inside a worktree (`.worktree/<id>/`) wrap calls in a subshell: `( cd "$REPO_ROOT" && grava ... )`. Details in [command-patterns.md](command-patterns.md).

## When To Load This Skill

- Agent is about to work on a grava-tracked project for the first time
- Agent needs to understand *why* a command exists, not just *how* to call it
- Agent encounters a grava error and needs to reason about recovery
- Agent needs to parse command output programmatically
