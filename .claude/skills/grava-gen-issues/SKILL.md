---
name: grava-gen-issues
description: Generate a full hierarchy of grava issues (epics, stories, tasks, subtasks) from a markdown document or folder of markdowns. Validates that the document covers stories, required services, external APIs, and third-party libraries — asks the user to fill gaps before proceeding. Use when the user says "generate issues from this doc", "create issues from spec", "turn this into grava issues", "parse this document into tasks", "create issues from PRD", or provides a markdown document and wants it broken into trackable work items. Also trigger when the user has a design doc, architecture spec, or feature brief and wants to populate the grava tracker from it.
---

# Generate Grava Issues from Document

Turn a well-structured markdown document (or folder of markdowns) into a complete grava issue hierarchy with dependencies and priorities — after the user approves the plan.

## Prerequisites

- The grava CLI skill (`grava-cli`) provides the mental model. If you haven't loaded it in this session, read `.claude/skills/grava-cli/mental-model.md` for entity hierarchy and state machine context.
- The grava database must be running (`grava doctor` to verify).

## Step 1: Ingest the Document

Read the input. It can be:
- A single markdown file path
- A folder path — read all `*.md` files in it, sorted alphabetically

After reading, identify and extract these four sections from the document content. They don't need to be literal headings — look for the information wherever it appears:

| Required Section | What to Look For |
|-----------------|-----------------|
| **Stories / Features** | User stories, feature descriptions, use cases, acceptance criteria |
| **Required Services** | Backend services, microservices, infrastructure components the system needs |
| **External APIs** | Third-party APIs the system consumes or integrates with (Stripe, SendGrid, OAuth providers, etc.) |
| **Third-Party Libraries** | SDKs, frameworks, packages the system depends on |

## Step 2: Validate Completeness

Check which sections are missing or too vague to generate actionable issues from.

If anything is missing or unclear, **stop and ask the user**. Be specific about what's missing:

```
The document is missing the following:

1. **External APIs** — The document mentions "payment processing" but doesn't
   specify which provider (Stripe, PayPal, etc.) or what operations are needed.

2. **Third-Party Libraries** — No libraries or SDKs are mentioned. What
   frameworks, ORMs, or utility libraries does this project use?

Please provide these details so I can generate complete issues.
```

Keep asking until all four sections have enough detail to generate issues. Don't proceed with gaps — the whole point is to produce actionable work items, not placeholders.

**Exception: Unknown integrations.** If the document mentions a service or API but doesn't describe it in enough detail to know the integration scope, that's OK. These become "requires clarification" issues (see Step 3). They should NOT block the generation process — just flag them and keep going.

## Step 3: Build the Issue Plan

Parse the document and produce a structured plan. Think about the work as a human tech lead would — group related work, identify what blocks what, and assign priorities based on architectural dependencies and business impact.

### Hierarchy Rules

```
Epic (one per major feature area or domain)
 └── Story (user-facing deliverable or integration boundary)
      └── Task (atomic implementation unit)
           └── Subtask (if a task is complex enough to warrant breakdown)
```

### Priority Assignment

Auto-assign priorities using these heuristics:

| Priority | Assign When |
|----------|------------|
| **critical** | Core infrastructure that everything else depends on (auth, database setup, CI/CD) |
| **high** | Stories on the critical path — blocked by nothing, blocking many things |
| **medium** | Standard feature work with clear requirements |
| **low** | Nice-to-haves, polish, non-blocking improvements |
| **backlog** | Exploratory work, "requires clarification" items, future considerations |

### Dependency Rules

Create dependency edges based on real technical relationships:

- **Service setup blocks service consumers.** If Story A sets up a service and Story B calls that service, A `blocks` B.
- **API integration blocks features using the API.** "Integrate Stripe" blocks "Implement checkout flow."
- **Shared infrastructure blocks dependent features.** Auth/database/config blocks everything that needs them.
- **Don't over-connect.** Only create edges where there's a genuine technical dependency. Two independent features that happen to be in the same epic don't need a dependency edge.

### Handling Unknown / Unclear Integrations

When the document mentions a service, API, or library without enough detail:

1. Create the integration issue anyway with type `task`
2. Set priority to `backlog`
3. Prefix the title with `[Clarify]` — e.g., `[Clarify] Integrate notification service — provider TBD`
4. Add a description noting what's unknown and what the user needs to decide
5. Label it `needs-clarification`
6. Do NOT make it block other issues — it's informational until clarified

## Step 4: Present the Plan for Approval

Before creating anything in grava, show the user the complete plan as a markdown table. Group by epic, show the hierarchy with indentation, and include dependencies.

Format:

```markdown
## Issue Generation Plan

### Epic: User Authentication
| # | Type | Title | Priority | Depends On |
|---|------|-------|----------|------------|
| 1 | epic | User Authentication | high | — |
| 2 | story | Email/password registration | high | — |
| 3 | task | Set up auth database schema | critical | — |
| 4 | task | Implement registration endpoint | high | #3 |
| 5 | task | Email verification flow | medium | #4 |
| 6 | story | OAuth2 social login | medium | #2 |
| 7 | task | [Clarify] Integrate social auth provider — TBD | backlog | — |

### Epic: Payment Processing
| # | Type | Title | Priority | Depends On |
|---|------|-------|----------|------------|
| 8 | epic | Payment Processing | high | #1 |
| 9 | story | Integrate Stripe API | high | — |
| 10 | task | Set up Stripe SDK and credentials | critical | — |
| 11 | task | Implement payment intent creation | high | #10 |
| ...

**Summary:** 4 epics, 12 stories, 28 tasks, 5 subtasks = 49 issues total
**Clarification needed:** 3 items marked [Clarify]
**Cross-epic dependencies:** Epic "Payment" depends on Epic "Auth" (user identity required)
```

Then ask:

```
Does this plan look right? You can:
- Approve as-is → I'll create all issues in grava
- Request changes → Tell me what to adjust (add/remove/reorder/reprioritize)
- Remove items → "Drop #7 and #15"
- Change priorities → "#9 should be critical"
```

Wait for explicit approval. Do not proceed without it.

## Step 5: Execute the Plan

Once approved, create issues in grava in dependency order (parents before children, blockers before blocked).

### Execution sequence:

1. **Create epics first:**
   ```bash
   grava create -t "User Authentication" --type epic --priority high --desc "..." --json
   ```
   Capture the returned `id` — you'll need it for child stories.

2. **Create stories with `--parent`:**
   ```bash
   grava create -t "Email/password registration" --type story --priority high --parent <epic-id> --desc "..." --json
   ```

3. **Create tasks as subtasks:**
   ```bash
   grava subtask <story-id> -t "Set up auth database schema" --type task --priority critical --json
   ```

4. **Add dependencies:**
   ```bash
   grava dep <blocker-id> <blocked-id> --type blocks
   ```

5. **Label clarification items:**
   ```bash
   grava label <id> --add needs-clarification
   ```

6. **Commit the grava state:**
   ```bash
   grava commit -m "gen: created issues from <document-name>"
   ```

### Error Recovery

If a `grava create` or `grava dep` command fails mid-execution:
- Log which issues were already created (keep a running list)
- Report the error and what was created so far
- Ask the user whether to continue with remaining issues or stop

Do NOT silently skip failures.

## Step 6: Summary Report

After all issues are created, produce two outputs:

### 6a. Issue Manifest File

Write a markdown file to `tracker/gen-<document-name>-<YYYY-MM-DD>.md` containing the full mapping from plan numbers to actual grava IDs. This is the persistent record of what was generated.

```markdown
# Issue Manifest: <document-name>

Generated: <date>
Source: <document-path>

## Issues

| Plan # | Grava ID | Type | Title | Priority | Depends On |
|--------|----------|------|-------|----------|------------|
| 1 | grava-101 | epic | User Authentication | high | — |
| 2 | grava-102 | story | Email/password registration | high | — |
| 3 | grava-101.1 | task | Set up auth database schema | critical | — |
| 4 | grava-101.2 | task | Implement registration endpoint | high | grava-101.1 |
| 5 | grava-101.3 | task | Email verification flow | medium | grava-101.2 |
| ... | ... | ... | ... | ... | ... |

## Dependencies

| From (blocks) | To (blocked) | Type |
|---------------|-------------|------|
| grava-101.1 | grava-101.2 | blocks |
| grava-101.2 | grava-101.3 | blocks |
| grava-101 | grava-105 | blocks |
| ... | ... | ... |

## Clarification Needed

| Grava ID | Title | What's Unknown |
|----------|-------|---------------|
| grava-110 | [Clarify] Integrate notification service — provider TBD | Provider not specified |
| ... | ... | ... |
```

### 6b. Console Summary

Print a summary to the console:

```markdown
## Generation Complete

**Source:** docs/feature-spec.md
**Created:** 49 issues (4 epics, 12 stories, 28 tasks, 5 subtasks)
**Dependencies:** 23 edges created
**Needs clarification:** 3 items labeled `needs-clarification`
**Manifest saved to:** tracker/gen-feature-spec-2026-04-18.md

| Epic | Grava ID | Stories | Tasks | Status |
|------|----------|---------|-------|--------|
| User Authentication | grava-101 | 3 | 8 | Ready |
| Payment Processing | grava-105 | 4 | 10 | Ready |
| Notification System | grava-112 | 2 | 5 | 1 needs clarification |
| Admin Dashboard | grava-118 | 3 | 5 | 2 need clarification |

**Next steps:**
- Run `grava ready` to see unblocked work
- Address `[Clarify]` items: `grava list --label needs-clarification`
- Assign work: `grava assign <id> --actor <name>`
- Full issue map: `cat tracker/gen-feature-spec-2026-04-18.md`
```

## Edge Cases

- **Very large documents (50+ potential issues):** Break execution into batches of 10-15 issues. Show progress between batches.
- **Circular dependencies in the document:** If the described work implies cycles (A needs B, B needs C, C needs A), flag it in the plan and ask the user to break the cycle before creating issues.
- **Duplicate work:** Before creating, run `grava search "<title>"` for each planned issue to check if similar issues already exist. Flag potential duplicates in the plan.
- **Multiple documents in a folder:** Treat the folder as one logical document. If files cover different domains, each file likely maps to one epic. Note the source file in each epic's description.
