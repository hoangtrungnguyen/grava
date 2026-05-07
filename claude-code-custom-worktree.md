# Custom Worktree Folder for Claude Code

By default, Claude Code creates worktrees under `.claude/worktrees/` inside your repository. This guide shows you how to redirect them to any directory you want using the `WorktreeCreate` and `WorktreeRemove` hooks.

---

## How It Works

When Claude Code creates or removes a worktree, it fires two hook events:

| Hook | Trigger |
|---|---|
| `WorktreeCreate` | `claude --worktree <name>` or an agent using `isolation: worktree` |
| `WorktreeRemove` | Session ends with no changes, or user chooses to remove |

Your hook script receives a JSON payload via `stdin`, performs the worktree operation in whatever directory you choose, and **prints the absolute path to stdout** so Claude knows where to start the session.

---

## Setup

### 1. Register the hooks in `.claude/settings.json`

```json
{
  "hooks": {
    "WorktreeCreate": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "bash \"$CLAUDE_PROJECT_DIR\"/.claude/hooks/worktree.sh",
            "timeout": 30
          }
        ]
      }
    ],
    "WorktreeRemove": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "bash \"$CLAUDE_PROJECT_DIR\"/.claude/hooks/worktree.sh",
            "timeout": 15
          }
        ]
      }
    ]
  }
}
```

### 2. Create the hook script at `.claude/hooks/worktree.sh`

The script below places worktrees as **siblings of your repo root** using the pattern `../repo-name@branch-name`. Adjust `WORKTREE_BASE` to any path you prefer.

```bash
#!/bin/bash
#
# Claude Code worktree hook
#
# WorktreeCreate stdin payload:
#   { "hook_event_name": "WorktreeCreate", "cwd": "/path/to/project", "name": "my-feature" }
#
# WorktreeRemove stdin payload:
#   { "hook_event_name": "WorktreeRemove", "cwd": "/path/to/project", "worktree_path": "/path/to/worktree" }
#
# On WorktreeCreate: print the absolute worktree path to stdout.
# Claude Code will start the session in that directory.

set -euo pipefail

INPUT=$(cat)

if ! command -v jq &>/dev/null; then
  echo "jq is required but not installed" >&2
  exit 1
fi

HOOK_EVENT=$(echo "$INPUT" | jq -r '.hook_event_name')
CWD=$(echo "$INPUT"       | jq -r '.cwd')

# ── Logging ────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_FILE="$SCRIPT_DIR/../logs/worktree.log"
mkdir -p "$(dirname "$LOG_FILE")"

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] [$HOOK_EVENT] $*" >> "$LOG_FILE"; }

# ── Config ─────────────────────────────────────────────────────────────────
# Worktrees are created inside .worktree/ at the repo root.
# e.g. .worktree/feature, .worktree/bugfix-123
WORKTREE_BASE="$CWD/.worktree"
mkdir -p "$WORKTREE_BASE"

# ── WorktreeCreate ─────────────────────────────────────────────────────────
worktree_create() {
  local NAME
  NAME=$(echo "$INPUT" | jq -r '.name')

  local BRANCH="worktree-$NAME"
  local WORKTREE_PATH="$WORKTREE_BASE/$NAME"

  log "Creating worktree: path=$WORKTREE_PATH branch=$BRANCH"

  git -C "$CWD" worktree add "$WORKTREE_PATH" -b "$BRANCH"

  log "Created successfully"

  # Required: print the path so Claude knows where to start
  echo "$WORKTREE_PATH"
}

# ── WorktreeRemove ─────────────────────────────────────────────────────────
worktree_remove() {
  local WORKTREE_PATH
  WORKTREE_PATH=$(echo "$INPUT" | jq -r '.worktree_path')

  log "Removing worktree: path=$WORKTREE_PATH"

  if [ ! -d "$WORKTREE_PATH" ]; then
    log "Directory does not exist, skipping"
    exit 0
  fi

  # Derive the main repo path from the worktree's own git metadata
  local MAIN_REPO
  MAIN_REPO=$(git -C "$WORKTREE_PATH" worktree list --porcelain 2>/dev/null \
    | head -1 | sed 's/^worktree //')

  local BRANCH_NAME
  BRANCH_NAME="worktree-$(basename "$WORKTREE_PATH")"

  log "main_repo=$MAIN_REPO branch=$BRANCH_NAME"

  cd "$MAIN_REPO" 2>/dev/null || exit 0
  git worktree remove "$WORKTREE_PATH" --force 2>/dev/null || rm -rf "$WORKTREE_PATH"
  git branch -D "$BRANCH_NAME" 2>/dev/null || true

  log "Removed successfully"
}

# ── Dispatch ───────────────────────────────────────────────────────────────
case "$HOOK_EVENT" in
  WorktreeCreate) worktree_create ;;
  WorktreeRemove) worktree_remove ;;
  *)
    echo "Unknown hook event: $HOOK_EVENT" >&2
    exit 1
    ;;
esac
```

Make it executable:

```bash
chmod +x .claude/hooks/worktree.sh
```

---

## Directory Layout After Setup

Worktrees are created inside `.worktree/` at the Grava repo root:

```
grava/                        ← main repo
├── .worktree/
│   ├── feature/              ← created by: claude -w feature
│   └── bugfix-123/           ← created by: claude -w bugfix-123
├── .claude/
│   ├── settings.json
│   └── hooks/
│       └── worktree.sh
└── src/
```

> Add `.worktree/` to your `.gitignore` to keep worktree directories out of version control:
> ```
> .worktree/
> ```

---

## Verifying It Works

```bash
# Start a worktree session
claude -w my-feature

# In another terminal, confirm the location
git worktree list
```

You should see the new worktree at your custom path instead of `.claude/worktrees/`.

---

## Notes

- **`jq` is required** — install with `brew install jq` (macOS) or `sudo apt install jq` (Linux).
- Logs are written to `.claude/logs/worktree.log` for debugging.
- The hook fully replaces Claude's built-in git behavior — you have complete control.
- This also works for sub-agents that use `isolation: worktree` in their frontmatter.
