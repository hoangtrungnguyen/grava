# Package: coordinator

Path: `github.com/hoangtrungnguyen/grava/pkg/coordinator`

## Purpose

Provides the supervisor goroutine for the Grava CLI's background work. The
coordinator owns a context-bound goroutine and exposes a buffered error
channel so callers can react to failures without the supervisor calling
`log.Fatal`, `os.Exit`, or `panic` directly.

## Key Types & Functions

- `Coordinator` — supervisor struct holding a `notify.Notifier` and a
  `zerolog.Logger`.
- `New(n notify.Notifier, log zerolog.Logger) *Coordinator` — constructor.
- `(*Coordinator).Start(ctx context.Context) <-chan error` — launches the
  supervisor goroutine; returns a buffered (size 1) error channel that is
  closed on clean shutdown. Cancel `ctx` to stop the goroutine.

## Dependencies

- `github.com/rs/zerolog` for structured logging.
- `github.com/hoangtrungnguyen/grava/pkg/notify` for operator alerts.

## How It Fits

The coordinator is the host process for Phase 2 work loops that act on Dolt
state changes. It centralises error propagation and is wired to the global
notifier so future loops can alert humans before the error channel is
drained. The `notifier` field is reserved for upcoming work loops and must
not be removed.

## Usage

```go
coord := coordinator.New(notifier, log)
errCh := coord.Start(ctx)
select {
case err, ok := <-errCh:
    if !ok {
        // clean shutdown
    }
    // handle err
case <-ctx.Done():
    // graceful shutdown requested
}
```
