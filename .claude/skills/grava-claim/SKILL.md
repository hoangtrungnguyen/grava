---
name: grava-claim
description: Find and claim best available grava issue. Use when the user says 'claim issue', 'pick up work', 'find next task', or 'claim next issue'.
---

# Grava Claim Issue

## Overview

This skill finds the best available issue from the grava tracker, validates that all prerequisites are met, and claims it atomically. Act as a methodical agent that never claims work it can't start.

**Flow:** Evaluate top 3 ready issues by priority. For each candidate, read the description to identify required services and dependencies, verify they're available, then claim the first that passes all checks. Post-claim, establish a wisp heartbeat and confirm the claim.

**Output:** Claimed issue ID, title, and summary — or a report of why no issue could be claimed.

## On Activation

1. **Find candidates** — run `grava ready --limit 3` to get the top 3 unblocked issues sorted by priority.

2. **Evaluate each candidate** (highest priority first):
   - Run `grava show <id>` to read full issue details (description, comments).
   - Identify any services, tools, or infrastructure mentioned in the description or comments (databases, APIs, external services, MCP servers, etc.).
   - Verify each identified service is reachable or running — use appropriate checks based on what the service is (e.g., HTTP health endpoint, process check, CLI command, port probe). Use judgment on what "available" means for each service type.
   - If any required service is unavailable, skip this candidate and note the reason. Move to the next.

3. **Claim** — for the first candidate that passes all checks:
   ```bash
   grava claim <id>
   ```
   This atomically sets status to `in_progress` and assigns the issue.

4. **Post-claim setup:**
   - Write initial wisp heartbeat: `grava wisp write <id> --key "status" --value "claimed"`
   - Confirm with `grava show <id>`

5. **If all 3 candidates fail** — report what blocked each candidate. Do not claim an issue whose prerequisites aren't met.

6. **If no candidates exist** — notify the user that no ready issues were found and ask them to upload new requirements or create new issues in the tracker.
