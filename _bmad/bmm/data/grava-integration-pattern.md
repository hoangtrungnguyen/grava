# Grava CLI Integration Pattern for BMAD Workflows

> **Purpose:** Unified reference for all BMAD workflows that integrate with the Grava CLI.
> Every workflow using Grava commands MUST follow this pattern to ensure consistency,
> idempotency, and graceful degradation.

---

## 1. Availability Guard (Required — Every Workflow)

Before any Grava command, every workflow MUST run this detection block:

```xml
<!-- Grava availability guard -->
<action>Run: `which grava` to check if grava CLI is on PATH</action>
<action>Check if `.grava/` directory exists in the repository root</action>

<check if="grava CLI is available AND .grava/ is initialized">
  <action>Set {{grava_tracking}} = true</action>
</check>

<check if="grava CLI is NOT available OR .grava/ is NOT initialized">
  <action>Set {{grava_tracking}} = false</action>
</check>
```

**Rules:**
- All subsequent Grava commands MUST be wrapped in `<check if="{{grava_tracking}} == true">`
- If any individual Grava command fails, log a warning and **continue the workflow**
- Never abort a workflow because of a Grava failure

---

## 2. Grava Command Reference

### 2.1 Create Issue

```bash
grava create --title "Title" --type <type> --priority <priority> --json
# Optional: --parent <id> --desc "Description" --ephemeral --files file1,file2
```

**Types:** `task`, `bug`, `epic`, `story`, `feature`, `chore`
**Priorities:** `critical`, `high`, `medium`, `low`, `backlog`
**Statuses (DB):** `open`, `in_progress`, `closed`, `blocked`, `tombstone`, `deferred`, `pinned`

**JSON output:**
```json
{
  "id": "abc123def456",
  "status": "created"
}
```

> **Note:** Current JSON output returns `"status": "created"` (a verb), not the DB status.
> Story 2.1 will change this to `"status": "open"`. Parse the `id` field only.

### 2.2 List Issues (for Find-or-Create)

```bash
grava list --type <type> --json
```

**JSON output:** Array of objects:
```json
[
  {
    "id": "abc123def456",
    "title": "Epic 2: Issue Lifecycle",
    "type": "epic",
    "priority": 2,
    "status": "open",
    "created_at": "2026-03-29T..."
  }
]
```

> **Note:** Priority is returned as INT (0=critical, 1=high, 2=medium, 3=low, 4=backlog).

### 2.3 Update Issue Status

```bash
grava update <id> --status <status>
```

**Allowed statuses:** `open`, `in_progress`, `closed`, `blocked`, `tombstone`, `deferred`, `pinned`

> **Important:** Use `closed` (not `done`). Use `in_progress` (not `in-progress`).
> The DB uses underscores, not hyphens.

### 2.4 Add Comment

```bash
grava comment <id> "Comment text here"
```

**Shell escaping:** The comment text is a positional argument (not a `--message` flag).
Wrap in double quotes. Escape internal double quotes as `\"`.
Avoid backticks, single quotes with apostrophes, and special shell characters in comments.

> **Correct:** `grava comment abc123 "Review found 3 issues"`
> **Wrong:** `grava comment abc123 --message "Review found 3 issues"`

### 2.5 Claim Issue

```bash
grava claim <id>
```

Sets status to `in_progress` and assigns to the current actor. Atomic operation.

### 2.6 Show Issue Details

```bash
grava show <id>
```

### 2.7 Create Subtask

```bash
grava create --title "Task title" --type task --parent <parent_id> --json
```

Uses `--parent` flag to establish `subtask-of` dependency relationship.

---

## 3. Find-or-Create Pattern (Required — Idempotency)

**Every `grava create` MUST be preceded by a search to avoid duplicates.**

### Pattern:

```xml
<!-- Find-or-create example: Epic issue -->
<action>Run: `grava list --type epic --json` and search results for title containing "Epic 2"</action>
<check if="matching issue found">
  <action>Set {{grava_epic_id}} = found issue's id</action>
  <output>Found existing Grava epic: {{grava_epic_id}}</output>
</check>
<check if="no matching issue found">
  <action>Run: `grava create --title "Epic 2: Issue Lifecycle" --type epic --priority medium --json`</action>
  <action>Parse JSON output → extract `id` field → set {{grava_epic_id}}</action>
</check>
```

### Search Strategy:

| Looking for | Filter | Match by |
|---|---|---|
| Epic issue | `--type epic` | Title contains "Epic N" where N = epic number |
| Story issue | `--type story` | Title contains "Story X.Y" where X.Y = story key |
| Task issue | `--type task` | Title contains task description AND has parent = story ID |

### JSON Parsing:

```bash
# Extract ID from grava create --json output
grava create --title "..." --type story --json | grep '"id"' | sed 's/.*"id": *"\([^"]*\)".*/\1/'

# Search grava list --json for matching title
grava list --type epic --json | grep -l "Epic 2"  # won't work for structured search

# Recommended: Use jq if available, fallback to grep/sed
grava list --type epic --json | jq -r '.[] | select(.title | test("Epic 2")) | .id'
```

> **Fallback when jq unavailable:** Parse line-by-line. The `id` field always appears first in each object.

---

## 4. ID Propagation Across Workflows

### The Pipeline:

```
create-story          dev-story              code-review
    │                     │                      │
    ├─ Creates epic ID    ├─ Reads IDs           ├─ Reads story ID
    ├─ Creates story ID   ├─ Creates task IDs    ├─ Posts comments
    ├─ Writes to story    ├─ Starts/closes       ├─ Closes story
    │  Dev Agent Record   │  task issues         │  if approved
    │                     │                      │
    └──── IDs stored ─────┴──── IDs read ────────┘
```

### Storage Location: Story File Dev Agent Record

IDs are stored in the story markdown file under `## Dev Agent Record` → `### Completion Notes List`:

```markdown
### Completion Notes List
- Grava Tracking: epic={{grava_epic_id}}, story={{grava_story_id}}
- Grava Tasks: task1={{task1_id}}, task2={{task2_id}}, ...
```

### Reading IDs from Story File:

```xml
<action>Search story file for "Grava Tracking: " line</action>
<action>Extract epic= and story= values using pattern matching</action>
<action>If "Grava Tasks: " line exists, extract task ID mappings</action>
```

### Writing IDs to Story File:

```xml
<action>Append to Completion Notes List: "- Grava Tracking: epic={{grava_epic_id}}, story={{grava_story_id}}"</action>
```

### What If IDs Are Missing?

If a downstream workflow (dev-story, code-review) finds no Grava IDs in the story file:

1. **Try find-or-create** — search `grava list` for matching epic/story issues
2. **If found** — use them and write back to story file for future workflows
3. **If not found AND grava available** — create them now (self-healing)
4. **If grava unavailable** — set `{{grava_tracking}} = false` and proceed

---

## 5. Workflow-Specific Command Usage

### 5.1 create-story Workflow

| When | Command | Purpose |
|---|---|---|
| Step 1b | `grava list --type epic --json` | Find existing epic issue |
| Step 1b | `grava create --type epic --title "Epic N: ..." --json` | Create epic if not found |
| Step 1b | `grava create --type story --title "Story X.Y: ..." --parent <epic_id> --json` | Create story issue |
| Step 5 | Write IDs to Dev Agent Record | Propagate to downstream workflows |
| Step 6 | `grava comment <story_id> "Story created: ..."` | Log creation event |
| Step 6 | `grava comment <epic_id> "Story X.Y created: ..."` | Log on epic |

### 5.2 dev-story Workflow

| When | Command | Purpose |
|---|---|---|
| Step 4b | Read IDs from story file | Get epic/story Grava IDs |
| Step 4b | `grava create --type task --title "Task N: ..." --parent <story_id> --json` | Create task issues |
| Step 5 (each task) | `grava claim <task_id>` | Mark task as in-progress |
| Step 8 (task done) | `grava update <task_id> --status closed` | Mark task complete |
| Step 9 (story done) | `grava update <story_id> --status closed` | Mark story complete |
| Step 9 | `grava comment <story_id> "Development complete: ..."` | Log completion |

### 5.3 code-review Workflow

| When | Command | Purpose |
|---|---|---|
| Step 1 | Read story ID from Dev Agent Record | Get Grava story ID |
| Step 1 | Fallback: `grava list --type story --json` | Search if ID missing |
| Step 4 | `grava comment <story_id> "Code Review: X High, Y Medium, Z Low"` | Post summary |
| Step 4 | `grava comment <story_id> "[HIGH] finding description"` | Post per-finding |
| Step 5 (approved) | `grava update <story_id> --status closed` | Mark done |
| Step 5 (approved) | `grava comment <story_id> "Review APPROVED"` | Log outcome |
| Step 5 (changes needed) | `grava comment <story_id> "Review CHANGES REQUESTED"` | Log outcome |

---

## 6. Error Handling Rules

### Non-Blocking Execution

```xml
<!-- CORRECT: Non-blocking with warning -->
<action>Run: `grava comment <id> "..."`</action>
<check if="grava command fails">
  <output>Warning: Grava comment failed — continuing.</output>
</check>

<!-- WRONG: Blocking the workflow -->
<action>Run: `grava comment <id> "..."`</action>
<check if="grava command fails">
  <action>ABORT workflow</action>  <!-- NEVER DO THIS -->
</check>
```

### Common Failure Modes

| Failure | Cause | Handling |
|---|---|---|
| `grava` not on PATH | Not installed / not in environment | `{{grava_tracking}} = false`, skip all |
| `.grava/` not found | Repo not initialized with `grava init` | `{{grava_tracking}} = false`, skip all |
| `grava create` fails | DB not running (`grava start` needed) | Log warning, continue without tracking |
| `grava list` returns empty | No matching issues exist | Proceed to create |
| `grava update` fails on ID | Issue was deleted/compacted | Log warning, continue |
| JSON parse fails | Unexpected output format | Log warning, set ID = empty, continue |
| Shell escaping breaks comment | Special characters in text | Sanitize before passing |

---

## 7. Shell Escaping Guidelines

When constructing Grava commands dynamically:

```bash
# Safe: Simple alphanumeric text
grava comment abc123 "Review complete, 3 issues found"

# Dangerous: User-generated content or code references
grava comment abc123 "Found SQL injection in `query` on line 42"  # backtick breaks shell

# Safe alternative: Escape or strip problematic characters
# Replace backticks with single quotes, strip control characters
SAFE_MSG=$(echo "$MSG" | tr '`' "'" | tr -d '\n')
grava comment abc123 "$SAFE_MSG"
```

**Characters to watch:**
- Backticks `` ` `` — use single quotes `'` instead
- Double quotes `"` — escape as `\"`
- Dollar signs `$` — escape as `\$`
- Newlines — flatten to spaces or use `\n`
- Exclamation marks `!` — may trigger history expansion in some shells

---

## 8. Dual Bookkeeping: Grava + sprint-status.yaml

Both systems track status. Keep them in sync:

| Event | sprint-status.yaml | Grava |
|---|---|---|
| Story created | `ready-for-dev` | `grava create --type story` → `open` |
| Dev starts | `in-progress` | `grava claim <story_id>` → `in_progress` |
| Dev completes | `review` | `grava comment` (no status change) |
| Review approved | `done` | `grava update --status closed` |
| Review rejected | `in-progress` | `grava comment` (status stays) |

> **sprint-status.yaml is the source of truth for sprint tracking.**
> Grava provides richer operational tracking (comments, task-level, audit trail).
> If they diverge, sprint-status.yaml wins for sprint/epic status decisions.

---

## 9. Quick Reference Card

```
GUARD:    which grava && test -d .grava/
CREATE:   grava create --title "..." --type <type> [--parent <id>] --json
LIST:     grava list --type <type> --json
UPDATE:   grava update <id> --status <status>
COMMENT:  grava comment <id> "text"
CLAIM:    grava claim <id>

STATUSES: open | in_progress | closed | blocked | tombstone | deferred | pinned
TYPES:    task | bug | epic | story | feature | chore
PRIORITY: critical | high | medium | low | backlog

ID STORAGE: Story file → Dev Agent Record → Completion Notes List
FORMAT:     "Grava Tracking: epic=<id>, story=<id>"
```
