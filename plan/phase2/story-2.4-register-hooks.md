# Story 2.4: Register Hooks in Settings

Register all Phase 2 hooks in `.claude/settings.json`.

## File

`.claude/settings.json` — add `hooks` section.

## Changes

Add the following to the project settings:

```json
{
  "hooks": {
    "TaskCompleted": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "./scripts/hooks/validate-task-complete.sh"
          }
        ]
      }
    ],
    "TeammateIdle": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "./scripts/hooks/check-teammate-idle.sh"
          }
        ]
      }
    ],
    "TaskCreated": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "./scripts/hooks/review-loop-guard.sh"
          }
        ]
      }
    ]
  }
}
```

## File Permissions

All scripts must be executable:

```bash
chmod +x scripts/hooks/validate-task-complete.sh
chmod +x scripts/hooks/check-teammate-idle.sh
chmod +x scripts/hooks/review-loop-guard.sh
```

## Directory Structure

```
scripts/
└── hooks/
    ├── validate-task-complete.sh   (Story 2.1)
    ├── check-teammate-idle.sh      (Story 2.2)
    └── review-loop-guard.sh        (Story 2.3)
```

## Acceptance Criteria

- Hooks section added to `.claude/settings.json` (merge with existing content)
- All three hook events registered: TaskCompleted, TeammateIdle, TaskCreated
- Scripts are executable
- Hooks fire correctly when agent team runs
