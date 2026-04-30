---
name: grava-dev-task
description: 'Implement a single grava task (subtask / leaf issue) end-to-end with TDD: claim, build, scoped tests, commit, and label for code review. Use when the user says "dev this task <id>", "implement task grava-XXXX", "do task <id>", "work the next task on grava-YYYY", or asks to pick up a single task-scoped issue. Use this — not grava-dev-epic — whenever the target is one task or subtask rather than a full story or epic.'
---

# Grava Dev Task

Implement one grava task issue: claim it, build it with TDD, run scoped tests, commit, and label `code_review`. Stops after one task — does not roll forward to siblings.

**Announce at start:**
- If user gave an ID: "Using grava-dev-task to implement `<issue-id>`."
- If no ID was given: "Using grava-dev-task — discovering a candidate task." Then, after the user picks one, re-announce: "Implementing `<issue-id>`."

Follow [workflow.md](./workflow.md). Validate with [checklist.md](./checklist.md) before labeling complete.

**Scope rule:** if the target turns out to be a `story` or `epic` with subtasks, HALT and tell the user to use `grava-dev-epic` instead (or pick a specific subtask).
