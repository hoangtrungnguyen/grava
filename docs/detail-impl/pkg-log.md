# Module: `pkg/log + pkg/devlog`

**Package role:** Structured logging via zerolog. pkg/devlog is a deprecated no-op stub — all new code should use pkg/log.

> _Auto-generated on 2026-04-08 (commit `baefe84`). Edit `scripts/update-docs.sh` to change the template._

---

## pkg/log

## Files

| File | Lines | Exported Symbols |
|:---|:---|:---|
| `log.go` | 36 | Logger,Init |
| `log_test.go` | 130 | TestInit_DefaultLevelIsWarn,TestInit_DebugLevelEmitsDebug TestInit_InvalidLevelFallsBackToWarn,TestInit_JSONModeEmitsNoANSI TestInit_ErrorLevelSuppressesWarn |

## Public API

```
var Logger zerolog.Logger
func Init(level string, jsonMode bool)
```

## pkg/devlog (DEPRECATED)

> All functions are guaranteed no-ops. Do not add new call sites.

