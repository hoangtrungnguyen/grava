# Module: `pkg/notify`

**Package role:** Notification abstraction. ConsoleNotifier writes [GRAVA ALERT] to stderr. Non-fatal contract: Send errors never block the primary workflow.

> _Auto-generated on 2026-04-08 (commit `baefe84`). Edit `scripts/update-docs.sh` to change the template._

---

## Files

| File | Lines | Exported Symbols |
|:---|:---|:---|
| `notifier.go` | 28 | Notifier,ConsoleNotifier NewConsoleNotifier |
| `notifier_test.go` | 42 | TestConsoleNotifier_Send_WritesToStderr,TestConsoleNotifier_Send_ReturnsNil TestConsoleNotifier_ImplementsNotifier |

## Public API

```
type ConsoleNotifier struct{}
    func NewConsoleNotifier() *ConsoleNotifier
type Notifier interface{ ... }
```

