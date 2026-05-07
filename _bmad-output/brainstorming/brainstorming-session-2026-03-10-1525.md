---
stepsCompleted: [1, 2, 3]
inputDocuments: []
session_topic: 'Beads-style Conflict Resolution for Grava'
session_goals: 'Implement cell-level merge logic for JSONL issues where different field updates merge automatically but same-field updates trigger human intervention.'
selected_approach: 'ai-recommended'
techniques_used: ['First Principles Thinking']
ideas_generated: []
context_file: ''
---

# Brainstorming Session Results

**Facilitator:** Htnguyen
**Date:** 2026-03-10

## Session Overview

**Topic:** Beads-style Conflict Resolution for Grava
**Goals:** Implement cell-level merge logic for JSONL issues where different field updates merge automatically but same-field updates trigger human intervention.

### Session Setup

We are focusing on evolving the `merge-slot.go` command to handle multi-agent concurrency. The core challenge is bridging the gap between Git's text-based merging and Beads' database-like cell-level reconciliation.

## Technique Selection

**Approach:** AI-Recommended Techniques
**Analysis Context:** Building a schema-aware merge driver for distributed agents.

**Recommended Techniques:**

- **First Principles Thinking:** We will strip away "Git text merging" assumptions and rebuild the logic based on the fundamental truth that a JSONL line is a database record. We'll identify the absolute minimum data needed to resolve a cell-level conflict.

**AI Rationale:** This complex architectural shift requires questioning the basic unit of merging. First Principles Thinking is ideal for breaking down the technical requirements into their most irreducible parts before we build the logic.

## Technique Execution Results

**First Principles Thinking:**

- **Interactive Focus:** We stripped away Git's text-based merge constraints and looked at `issues.jsonl` as a collection of database rows (Issue IDs) containing cells (JSON fields). We then examined how beads uses Dolt to achieve "Cell-Level Merging" and how we can replicate this in JSONL.
- **Key Breakthroughs:** 
    - **Grava Data Anatomy:** A single line of JSONL is NOT the atomic unit of a conflict. The atomic unit is the **JSON Field Value** (Key-Value pair).
    - **True Cell-Level Merging (Beads Style):** If Agent A updates the "Status" field and Agent B updates the "Priority" field of the *same* Issue ID simultaneously, this is NOT a conflict. The merge driver must automatically combine these changes.
    - **Human Interference Trigger:** A conflict *only* occurs if Agent A and Agent B both modify the *exact same field* (e.g., both try to change the "Status") to different values.
    - **Git Hook Integration:** Taking inspiration from beads, we can use Git hooks (`post-merge`, `post-checkout`, `pre-commit`, etc.) as "thin shims" to automatically synchronize the JSONL state with the Dolt database.

- **User Creative Strengths:** Drawing direct analogical connections from beads documentation (Dolt SQL cell-level merges) to our custom Go CLI merge driver.
- **Energy Level:** Highly technical and precise.

### Ideas Generated

**[Category 1]**: Cell-Level JSON Merge Logic
_Concept_: Rewrite `merge-slot.go` to unmarshal (`map[string]interface{}`) the Ancestor, Current, and Other JSON blobs for a specific Issue ID. It will iterate through all JSON keys. If only Current changed a key, apply it. If only Other changed a key, apply it. If BOTH changed the same key to different values, mark it as a conflict.
_Novelty_: Brings database-level conflict resolution (like Dolt/Beads) to a flat JSONL file stored in Git without requiring an external database connection during the `git merge` process.

**[Category 2]**: Human Intervention Marker
_Concept_: When a true cell-level conflict happens (same field modified), instead of just exiting with 1, we could either insert standard Git conflict markers (`<<<<<<<`) *inside* the JSON string value for that field so the human sees exactly what field collided, OR just write the conflicting objects to an `.issues_conflict` file for a side-by-side CLI tool.
_Novelty_: Keeps JSON mostly parseable or provides a superior developer experience compared to reading massive Git text conflicts.

**[Category 3]**: Synchronized Git State (Hooks)
_Concept_: Adopt the beads hook architecture. Install thin shim shell scripts for Git hooks that simply call `grava sync` or `grava import/export`. When `merge-slot` finishes an automatic resolution, these hooks ensure the Dolt database is immediately updated with the new JSONL state without user commands.
_Novelty_: Seamlessly blends the Git workflow with the Grava database backend, completely hiding the synchronization complexity from the agents.

**Further Exploration: How Beads stores changes with Git and Dolt**

- **Interactive Focus:** Digging deeper into the Beads architecture to understand how it bridges the file-based Git world with the SQL-based Dolt world.
- **Key Breakthroughs:**
    - **Dual-Surface Remote Architecture:** Beads uses a "SQL Surface" for Native Dolt remotes (network I/O in the SQL server) and a "CLI Surface" for git-protocol remotes (where a subprocess needs SSH keys). This is complex. For Grava, because we are using a local Dolt instance (as per Epic 2 decisions) and syncing it to a JSONL file, Git handles the remote networking, and Grava just handles local synchronization.
    - **Hook Execution (`bd hooks run <hookname>`):** Beads hooks do not contain logic. They are "thin shims" that call the main binary. This means if we upgrade `grava`, the hook logic upgrades automatically without needing to rewrite `.git/hooks/pre-commit`.
    - **Conflict Tables:** In native Dolt, a conflict populates a system table (`dolt_conflicts_<table>`) with `base`, `our`, and `their` columns. A user must resolve this via SQL (`DOLT_CONFLICTS_RESOLVE`). 

### Architectural Decision: Dolt = Source of Truth, JSONL = Git Transport

**CONFIRMED:** Dolt is the primary read/write store. JSONL is a derived export committed to Git so other users see changes. The sync cycle:
1. **Work locally** → all reads/writes go to Dolt
2. **pre-commit hook** → `grava export` writes Dolt → JSONL, so the file is fresh when pushed
3. **Pull/Merge** → `merge-slot` does cell-level merge on JSONL, then `post-merge` hook imports merged JSONL → Dolt
4. **Other users** → pull JSONL via Git, their `post-merge` hook hydrates their local Dolt

### Refined Brainstorming Ideas based on Beads Architecture:

**[Category 4]**: Dolt-in-Git Sync Model (Beads-Inspired)
_Concept_: Adopt the `dolt-in-git` sync model from beads. Dolt is the local truth. On commit, export to JSONL. On pull/merge, the merge driver resolves JSONL conflicts, then import back into Dolt. Git carries the JSONL across the network — Dolt never needs to talk to remotes.
_Novelty_: Simpler than beads' full architecture (no Dual-Surface Remotes, no DoltHub push). Git is the only network transport. Grava's merge driver gives us cell-level resolution that beads doesn't have at the JSONL layer.

**[Category 5]**: Thin Shim Hook Installer
_Concept_: Just like `bd hooks install --chain`, our `grava install` command shouldn't write massive bash scripts into `.git/hooks/`. It should write a 2-line script: `#!/bin/sh` \n `grava hook run pre-commit`. If a hook already exists, it creates `<hookname>.old` and chains it.
_Novelty_: Makes Grava installation incredibly safe and future-proofs the hook logic inside the Go binary.

**[Category 6]**: Conflict Resolution Output
_Concept_: Instead of forcing users to write SQL like Dolt (`DOLT_CONFLICTS_RESOLVE`), if `grava merge-slot` hits an unresolvable cell-level conflict, it writes the conflict to a temporary JSON file (e.g., `.grava/conflicts.json`) and prints a command for the user: "Run `grava resolve` to fix."
_Novelty_: Keeps the JSONL file perfectly pristine until the user resolves the conflict using a guided CLI prompt, bypassing Git's messy text markers entirely.

**[Category 7]**: Dual Safety Check (CONFIRMED — A + C combined)
_Concept_: Before `post-merge` hook imports JSONL into Dolt, perform TWO checks:
- **Check A (JSONL Content Hash):** SHA-256 of `issues.jsonl` stored in `.grava/last_sync_hash`. Compare current file hash against stored hash.
- **Check C (Dolt Commit Tracking):** Dolt HEAD commit hash stored in `.grava/last_sync_dolt_commit`. Compare current Dolt HEAD against stored hash.
The 2x2 safety matrix:
- Both match → Safe to import
- JSONL hash changed, Dolt unchanged → JSONL modified externally (hand edit or failed hook)
- JSONL unchanged, Dolt HEAD changed → Local DB has unexported changes
- Both changed → Most dangerous: both sides diverged independently
_Novelty_: Catches ALL drift scenarios that neither check alone can cover. Prevents silent data loss from any direction. Beads doesn't have this explicit dual guard.

**[Category 8]**: Sorted-Key Export for Hash Stability
_Concept_: To prevent false positives in Check A (where Go map iteration order changes the JSON field ordering), the `grava export` command must sort JSON keys deterministically before writing each line. This ensures the same data always produces the same hash.
_Novelty_: A small implementation detail that makes the entire hash-based safety system reliable.

**[Category 9]**: `grava sync-status` Command
_Concept_: Expose the dual safety check as a user-facing command. `grava sync-status` reads the two stored hashes, compares them against current state, and prints a clear status: "In sync ✅", "JSONL drifted ⚠️", "Dolt drifted ⚠️", or "Both diverged 🚨". Useful for debugging and CI pipelines.
_Novelty_: Gives visibility into the sync state without needing to understand the internal mechanics. Similar to `bd doctor` in beads but focused specifically on the JSONL↔Dolt relationship.

### Action Items & Next Steps (Created in Issue Tracker)
During this session, we converted the brainstorming ideas into concrete, tracked issues inside Epic 3 (`grava-6f19`):
1. **`grava-6f19.7`**: Deterministic Sorted-Key Export (Ensures field stability for hash checks).
2. **`grava-6f19.8`**: Conflict Resolution Output & CLI (Write `.grava/conflicts.json` and build `grava resolve`).
3. **`grava-6f19.9`**: Dual Safety Check for Import Hooks (Check A + Check C).
4. **`grava-6f19.10`**: `grava sync-status` Command (User-facing sync visibility).

### Edge Cases Identified for Future Exploration
- **Dependency Arrays:** How cell-level merging should handle `dep` arrays. Need to ensure unions are clean.
- **Comment Arrays:** How concurrent comment appends are handled.

---
*Brainstorming and Ideation Phase Complete. Transitioning to Idea Organization.*
