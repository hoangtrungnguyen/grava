---
description: Validate readiness to start a ticket by checking context, dependencies, and environment
trigger: When user runs '/are-u-ready' or asks "Are you ready to start [Ticket ID]?"
---

# Are You Ready Protocol

Execute the readiness validation protocol defined in `.agent/workflows/are-u-ready.md`.

## Your Task

Follow these steps to validate readiness before writing any code:

### 1. Ticket Identification
- If user provided a Ticket ID, use it
- Otherwise, scan `docs/epics/*.md` for the highest priority 'Todo' or 'Open' item
- Propose the ticket to the user for confirmation

### 2. Context & Dependency Analysis
- Read the ticket from `docs/epics/` or `tracker/`
- Scan for related/blocking tickets in `tracker/` directory
- Check if blocking tickets are marked as 'Done' or 'Closed'
- Summarize the "story so far" based on tracker logs

### 3. Environment & Connectivity Check
- Identify external dependencies (databases, APIs, services)
- Verify `.env` or config files exist
- Perform non-destructive health checks:
  - For Dolt database: `dolt --data-dir .grava/dolt sql -q "SELECT VERSION()"`
  - For APIs: verify config/keys exist (don't call external services unless necessary)

### 4. Output Readiness Checklist

**Format:**
```
## Readiness Checklist for [Ticket ID]
- [ ] Ticket Context Loaded
- [ ] Blockers Resolved (check tracker/)
- [ ] Database Connection Verified
- [ ] Environment Configuration Verified
```

### Decision:

**❌ NOT READY:** If blockers exist or connections fail:
- Stop - do not generate code
- List specific blocking tickets or missing configs
- Recommend: "Shall we switch focus to [Blocking Ticket] or debug the environment?"

**✅ READY:** If all checks pass:
- Confirm: "I am ready to start [Ticket ID]. All dependencies and connections verified."
- Ask user: "How would you like to proceed? (autonomous / step-by-step / plan-first)"
