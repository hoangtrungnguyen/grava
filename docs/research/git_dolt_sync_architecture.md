# Research: Git and Dolt Sync Architecture

## Introduction

This document outlines the architectural research and decisions for integrating the Dolt database with Git for the Grava issue tracking system. The core goal is to provide a robust, version-controlled issue tracking system that works seamlessly alongside the user's source code, while supporting concurrent modifications by autonomous AI agents.

## Architectural Approaches Considered

### Option 1: The Hybrid Approach (Git + JSONL Sync)
In this approach, a single `issues.jsonl` file stored in the project's root `.grava/` directory acts as the Git-tracked source of truth, while Dolt serves as a high-performance local query engine. 
* **Mechanism:** 
  * Git hooks (`pre-commit`) automatically export the local Dolt database state to `issues.jsonl` and stage it.
  * Git hooks (`post-merge`, `post-checkout`) read `issues.jsonl` from the checked-out branch and synchronize the local Dolt database.
  * A custom Git merge driver (`grava merge-slot`) is registered via `.gitattributes` to handle concurrent edits to `issues.jsonl`, merging at the JSON field level to bypass text conflicts.
* **Pros:** 
  * Keeps Grava data directly inside the user's standard project Git repository.
  * Leverages Git's delta compression (packfiles) and zlib, making `issues.jsonl` incredibly space-efficient (even at 100k commits).
  * Checking out a Git branch automatically updates the issues to match that branch's state.
* **Cons:** Requires building Git integration hooks and a custom 3-way JSONL merge driver.

### Option 2: The Pure Dolt Approach (Separate Histories)
In this approach, Grava relies entirely on Dolt's native version control ("Git for Data"), ignoring the project's `.git` repository entirely.
* **Mechanism:** Grava CLI commands act as wrappers around `dolt commit`, `dolt push`, etc. Users and agents interact solely with the `.dolt` database. To avoid massive repo bloat from tracking Dolt's binary chunks, `.dolt/` must be added to `.gitignore`.
* **Pros:** Leverages Dolt's powerful native cell-level merge resolution; less custom sync code required.
* **Cons:** Separates the issue history from the source code history. Checking out a feature branch in Git does not automatically change the issue state; users/agents must separately run `grava branch` / `dolt branch`.

**Conclusion:** The Hybrid Approach (Option 1) is selected for Grava to ensure that issue tracking data remains versioned alongside code efficiently, which is critical for tying issues directly to specific codebase states.

---

## Agentic Conflict Resolution

A key requirement for Grava is supporting multiple autonomous agents working concurrently in different repositories (or worktrees) and merging back to a common Epic repository.

### Scenario
Agent A and Agent B clone the Epic Repo and independently modify the same issue (e.g., Task #10). Agent A merges its branch successfully. Agent B then attempts to merge its branch, resulting in a potential conflict.

### Resolution Mechanism: The Grava Merge Driver (`grava merge-slot`)

To prevent agents from getting stuck resolving standard Git text conflicts (which they handle poorly), Grava implements a custom 3-way merge driver.

When Git detects a merge involving `*.jsonl`, it delegates to `grava merge-slot --ancestor %O --current %A --other %B`.

#### Field-Level Merging
The driver parses the Ancestor, Current (Ours), and Other (Theirs) files into JSON objects in memory.
* If Agent A updated the `status` and Agent B updated the `title`, the driver securely combines both changes into a single valid JSON object without conflict.

#### True Conflict Resolution Rules
If both agents modified the *exact same field* differently, the driver applies deterministic rules to resolve the conflict automatically, such that Agent B never sees a text conflict marker:
1. **Timestamp Precedence:** The modification with the newer `updated_at` timestamp typically wins.
2. **State Machine Precedence:** Terminal states override intermediate states (e.g., `closed` > `blocked` > `in_progress` > `open`).
3. **Array Appends:** For lists like `comments` or `audit_logs`, the driver merges the arrays natively, keeping both sets of changes.

### Best Practices for Autonomous Agents under Conflict
If the driver cannot resolve a complex conflict and falls back to a failure state:
1. **Pull with Rebase First:** Agents should execute `git pull --rebase origin main` before attempting to push, applying upstream changes locally first.
2. **Idempotent Updates:** Agents should not attempt to parse raw Git conflict markers in JSON strings. Instead, they should abort the merge (`git merge --abort`), fetch the latest state, and simply re-run the `grava update` command intended, allowing Dolt/Grava to apply the change cleanly to the fresh database.
