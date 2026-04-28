# Package: notify

Path: `github.com/hoangtrungnguyen/grava/pkg/notify`

## Purpose

Provides a uniform `Notifier` interface for emitting non-fatal system alerts
from Grava commands, plus a default stderr-backed implementation
(`ConsoleNotifier`).

## Key Types & Functions

- `Notifier` — interface with `Send(title, body string) error`. Send failures
  are advisory; the calling command must always succeed regardless of
  notifier state.
- `ConsoleNotifier` — Phase-1 implementation. Writes `[GRAVA ALERT] <title>:
  <body>` to stderr and returns nil.
- `NewConsoleNotifier()` — constructor returning a ready-to-use
  `*ConsoleNotifier`.

## Dependencies

- Standard library only (`fmt`, `os`).

## How It Fits

Long-running Grava workflows (orchestrator watchdog, claim monitoring, merge
driver) need to surface degraded-but-non-fatal conditions without crashing.
They depend on the `Notifier` interface so deployments can swap stderr for a
chat-channel sink in later phases. Tests use `pkg/notify/mock.MockNotifier`
to assert call sequences.

## Usage

```go
n := notify.NewConsoleNotifier()
if err := someStep(); err != nil {
    _ = n.Send("step failed", err.Error()) // non-fatal
}
```
