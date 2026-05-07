// Package coordinator manages background processes for the Grava CLI and
// propagates fatal errors via channels rather than process termination.
//
// The package exposes a single Coordinator type whose Start method launches a
// goroutine bound to a context.Context and returns a buffered error channel.
// Goroutines inside the coordinator are forbidden from calling log.Fatal,
// os.Exit, or panic; instead, all errors flow through the returned channel so
// the caller can decide how to react (graceful shutdown, retry, alerting).
//
// In the broader Grava system, the coordinator is the long-running supervisor
// that hosts work loops which interact with the Dolt-backed shared state and
// notify operators through pkg/notify when something goes wrong. Phase 2 work
// loops will use the embedded notify.Notifier to alert before signalling
// errors to the channel; the field is intentionally retained.
//
// Typical use:
//
//	coord := coordinator.New(notifier, log)
//	errCh := coord.Start(ctx)
//	select {
//	case err, ok := <-errCh:
//	    // handle err or detect clean shutdown via !ok
//	case <-ctx.Done():
//	    // graceful shutdown
//	}
package coordinator
