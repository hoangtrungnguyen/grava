# Module: `pkg/cmd`

**Package role:** CLI command registration layer. Wires all Cobra commands, manages lifecycle (PersistentPreRunE / PersistentPostRunE), and builds the shared `cmddeps.Deps` container.

> _Auto-generated on 2026-04-08 (commit `baefe84`). Edit `scripts/update-docs.sh` to change the template._

---

## Sub-commands (pkg/cmd/issues/)

| File | Command |
|:---|:---|
| `assign.go` | `grava assign <id>` |
| `claim.go` | `grava claim <issue-id>` |
| `comment.go` | `grava comment <id> [text]` |
| `create.go` | `grava create` |
| `drop.go` | `grava drop [id]` |
| `history.go` | `grava history <issue-id>` |
| `issues.go` | `grava show <id>` |
| `label.go` | `grava label <id>` |
| `start.go` | `grava start <id>` |
| `stop.go` | `grava stop <id>` |
| `subtask.go` | `grava subtask <parent_id>` |
| `undo.go` | `grava undo <id>` |
| `update.go` | `grava update <id>` |
| `wisp.go` | `grava wisp` |

## Files

| File | Lines | Exported Symbols |
|:---|:---|:---|
| `audit_integration_test.go` | 103 | TestCreateAuditIntegration |
| `commands_benchmark_test.go` | 214 | BenchmarkCreateBaseIssue,BenchmarkCreateSubtask BenchmarkBulkInsert1000,BenchmarkMixedWorkload BenchmarkSequentialInserts |
| `commands_test.go` | 1643 | TestCreateCmd,TestShowCmd TestCreateEphemeralCmd,TestListCmd TestListWispCmd |
| `config.go` | 45 | — |
| `history_undo_integration_test.go` | 234 | TestHistoryIntegration,TestUndoIntegration TestUndoAffectedFilesIntegration,TestSessionUndoIntegration |
| `history_undo_test.go` | 175 | TestUndoCmd_Dirty,TestUndoCmd_Clean TestUndoCmd_NoHistory,TestUndoCmd_NotFound |
| `init.go` | 181 | — |
| `ready_test.go` | 66 | TestReadyCmd,TestBlockedCmd |
| `root.go` | 191 | Execute,SetVersion |
| `version.go` | 19 | — |

