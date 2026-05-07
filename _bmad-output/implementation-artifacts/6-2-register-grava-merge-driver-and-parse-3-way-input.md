# Story 6.2: Register `grava-merge` Driver and Parse 3-Way Input

Status: ready-for-dev

## Story

As a developer,
I want `grava init` to register the `grava-merge` driver via `.gitattributes`,
So that Git automatically invokes the driver on `issues.jsonl` conflicts without manual configuration.

## Acceptance Criteria

1. **AC#1 -- Automatic Registration**
   When `grava init` (or a dedicated `grava config --setup-merge-driver`) is run,
   Then `.gitattributes` MUST be updated idempotently with `issues.jsonl merge=grava-merge`.
   And the local Git config (`.git/config`) MUST be updated with the `merge.grava-merge` definition.

2. **AC#2 -- 3-Way JSONL Parsing**
   The `merge-driver` command MUST parse the three provided files (`%O`, `%A`, `%B`) as JSONL.
   It MUST build an in-memory adjacency map keyed by `issue_id` to allow quick lookup of all three versions of a single issue.

3. **AC#3 -- Trivial Merge (No Conflict)**
   For issues that exist ONLY in `%A` or ONLY in `%B` (clean additions/deletes), the driver MUST include them in the merged output without entering conflict resolution logic.

4. **AC#4 -- Binary Robustness**
   The driver MUST handle missing files gracefully (e.g. if an issue didn't exist in the ancestor `%O`).

5. **AC#5 -- Dry Run Support**
   `grava merge-driver --dry-run %O %A %B` MUST output the proposed merge plan (items to add, remove, or resolve) without actually writing to the `%A` file path.

## Tasks / Subtasks

- [ ] Task 1: Update `grava init`
  - [ ] 1.1 Implement `.gitattributes` writing logic in `pkg/cmd/init.go`.
  - [ ] 1.2 Implement `git config` invocation to register the driver binary.

- [ ] Task 2: Implement JSONL Parser
  - [ ] 2.1 Create a generic 3-way JSONL mapper that loads three files into `map[string]*Triple`.
  - [ ] 2.2 Define the `Triple` struct: `{Base, Ours, Theirs}` Issue objects.

- [ ] Task 3: Implement Initial Merge Loop
  - [ ] 3.1 Loop over all keys in the Triple map.
  - [ ] 3.2 For non-conflicting keys, write directly to the merged output buffer.

## Dev Notes

### JSONL Format
The `issues.jsonl` file contains one JSON issue object per line.
The parser should be robust to empty lines or malformed JSON (though those should technically not exist in a valid workspace).

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-06-advanced-merge-driver.md#Story-6.2]
