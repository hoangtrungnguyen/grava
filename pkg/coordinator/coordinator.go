package coordinator

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/hoangtrungnguyen/grava/pkg/notify"
)

// Coordinator manages background processes and propagates errors via a channel.
// Goroutines inside Coordinator MUST NOT call log.Fatal, os.Exit, or panic.
// All fatal errors propagate via the returned chan error from Start.
type Coordinator struct {
	notifier notify.Notifier
	log      zerolog.Logger
}

// New creates a Coordinator with the given notifier and logger.
func New(n notify.Notifier, log zerolog.Logger) *Coordinator {
	return &Coordinator{notifier: n, log: log}
}

// Start launches the coordinator goroutine and returns a buffered error channel.
//
// The caller MUST select on the channel or ctx.Done():
//
//	ch := coord.Start(ctx)
//	select {
//	case err, ok := <-ch:
//	    if !ok { /* clean shutdown */ }
//	    // handle error
//	case <-ctx.Done():
//	    // graceful shutdown
//	}
//
// Buffer size 1 prevents goroutine leak if caller abandons the channel.
// The channel is closed when the goroutine exits (signals clean shutdown).
func (c *Coordinator) Start(ctx context.Context) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		// Coordinator work loop.
		// ctx cancellation is the graceful shutdown signal.
		// NEVER call log.Fatal, os.Exit, or panic inside this goroutine.
		select {
		case <-ctx.Done():
			c.log.Debug().Msg("coordinator: context cancelled, shutting down")
			return
		}
	}()
	return errCh
}
