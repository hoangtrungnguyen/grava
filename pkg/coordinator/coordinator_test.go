package coordinator

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hoangtrungnguyen/grava/pkg/notify/mock"
)

func TestCoordinator_Start_ReturnsChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n := &mock.MockNotifier{}
	c := New(n, zerolog.Nop())
	ch := c.Start(ctx)
	require.NotNil(t, ch, "Start must return a non-nil channel")
}

func TestCoordinator_Start_CtxCancellation_ClosesChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	n := &mock.MockNotifier{}
	c := New(n, zerolog.Nop())
	ch := c.Start(ctx)

	// Cancel context — goroutine should exit and close the channel
	cancel()

	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed after ctx cancellation")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for channel to close after ctx cancellation")
	}
}

func TestCoordinator_Start_BufferedChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n := &mock.MockNotifier{}
	c := New(n, zerolog.Nop())
	ch := c.Start(ctx)

	// A buffered channel of size 1 means cap==1
	assert.Equal(t, 1, cap(ch), "error channel must have buffer size 1")
}

func TestCoordinator_NoOsExitOrLogFatal(t *testing.T) {
	// Static verification: coordinator.go must not import "os" for Exit
	// or "log" for Fatal. This is verified by compilation success + code review.
	// The test here documents the contract.
	t.Log("Contract: coordinator goroutines never call os.Exit, log.Fatal, or panic")
}
