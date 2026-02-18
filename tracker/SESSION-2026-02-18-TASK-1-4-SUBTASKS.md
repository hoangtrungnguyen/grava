---
issue: SESSION-2026-02-18-TASK-1-4-SUBTASKS
status: done
Description: Session to verify and close all TASK-1-4 subtasks (CLI scaffold, create, show, update, list commands). Fixed DB name mismatch in start_dolt_server.sh (grava→dolt). Fixed list.go sort order to match acceptance criteria (priority ASC, created_at DESC). All subtasks marked done. All unit tests pass.
---

**Timestamp:** 2026-02-18 10:38:50
**Affected Modules:**
  - pkg/cmd/list.go
  - pkg/cmd/commands_test.go
  - scripts/start_dolt_server.sh
  - tracker/TASK-1-4.subtask-1.md
  - tracker/TASK-1-4.subtask-2.md
  - tracker/TASK-1-4.subtask-3.md
  - tracker/TASK-1-4.subtask-4.md
  - tracker/TASK-1-4.subtask-5.md

---

## Session Details

### Context
Session started with `are-u-ready` check on `TASK-1-4.subtask-1` (Scaffold CLI application structure).

### Readiness Check Results
- **TASK-1-1** (Dolt DB Init): ✅ DONE
- **TASK-1-2** (Core Schema): ✅ DONE
- **TASK-1-3** (Hierarchical ID Generator): ✅ DONE
- **Dolt server**: ✅ Running on port 3306
- **Database exposed**: `dolt` (not `grava` as the start script echo claimed)

### Issues Found & Fixed

#### 1. DB Name Mismatch in `scripts/start_dolt_server.sh`
- **Problem**: The echo message said `mysql://root@127.0.0.1:3306/grava` but the actual Dolt SQL server exposes the database as `dolt` (the repo name). The `pkg/cmd/root.go` default DSN correctly uses `/dolt`.
- **Fix**: Updated the echo string in `start_dolt_server.sh` to say `/dolt`.

#### 2. `grava list` Sort Order Deviation
- **Problem**: `pkg/cmd/list.go` sorted only by `created_at DESC`, but the acceptance criteria for `TASK-1-4.subtask-5` required sorting by `priority/created_at`.
- **Fix**: Changed `ORDER BY created_at DESC` → `ORDER BY priority ASC, created_at DESC` (priority 0=critical sorts first).
- **Test updated**: `TestListCmd` in `commands_test.go` updated to match the new query string.

### Subtask Status Updates
All five subtasks were already implemented; only tracker status needed updating:
| Subtask | Description | Status |
|---|---|---|
| TASK-1-4.subtask-1 | Scaffold CLI (main.go + Cobra) | ✅ done |
| TASK-1-4.subtask-2 | `grava create` command | ✅ done |
| TASK-1-4.subtask-3 | `grava show` command | ✅ done |
| TASK-1-4.subtask-4 | `grava update` command | ✅ done |
| TASK-1-4.subtask-5 | `grava list` command | ✅ done |

### Test Results
All 5 unit tests pass:
```
=== RUN   TestCreateCmd  --- PASS
=== RUN   TestShowCmd    --- PASS
=== RUN   TestListCmd    --- PASS
=== RUN   TestUpdateCmd  --- PASS
=== RUN   TestSubtaskCmd --- PASS
ok  command-line-arguments  1.002s
```

### Next Task
**TASK-1-5** (Schema Validation and Testing) is `in_progress`. One remaining criterion:
- [ ] Performance benchmarks documented
