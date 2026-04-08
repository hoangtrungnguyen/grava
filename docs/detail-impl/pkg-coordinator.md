# Module: `pkg/coordinator`

**Package role:** Background goroutine lifecycle manager. Returns a buffered error channel. Future home for gate polling and Wisp expiry cleanup.

> _Auto-generated on 2026-04-08 (commit `baefe84`). Edit `scripts/update-docs.sh` to change the template._

---

## Files

| File | Lines | Exported Symbols |
|:---|:---|:---|
| `coordinator.go` | 55 | Coordinator,New |
| `coordinator_test.go` | 64 | TestCoordinator_Start_ReturnsChannel,TestCoordinator_Start_CtxCancellation_ClosesChannel TestCoordinator_Start_BufferedChannel,TestCoordinator_NoOsExitOrLogFatal |

## Public API

```
type Coordinator struct{ ... }
    func New(n notify.Notifier, log zerolog.Logger) *Coordinator
```

