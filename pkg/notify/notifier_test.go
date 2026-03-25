package notify

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsoleNotifier_Send_WritesToStderr(t *testing.T) {
	// Redirect stderr to capture output
	r, w, err := os.Pipe()
	require.NoError(t, err)

	origStderr := os.Stderr
	os.Stderr = w

	n := NewConsoleNotifier()
	sendErr := n.Send("Test Title", "test body message")

	w.Close()
	os.Stderr = origStderr

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	assert.NoError(t, sendErr, "Send should always return nil")
	expected := fmt.Sprintf("[GRAVA ALERT] Test Title: test body message\n")
	assert.Equal(t, expected, buf.String())
}

func TestConsoleNotifier_Send_ReturnsNil(t *testing.T) {
	n := NewConsoleNotifier()
	err := n.Send("title", "body")
	assert.NoError(t, err, "ConsoleNotifier.Send must always return nil")
}

func TestConsoleNotifier_ImplementsNotifier(t *testing.T) {
	var _ Notifier = NewConsoleNotifier()
}
