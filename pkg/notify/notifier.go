package notify

import (
	"fmt"
	"os"
)

// Notifier is the interface for system-level alerts.
// Send errors are non-fatal — the primary operation always completes regardless of notifier state.
// Phase 1: ConsoleNotifier (stderr). Phase 2: TelegramNotifier, WhatsAppNotifier.
type Notifier interface {
	Send(title, body string) error
}

// ConsoleNotifier implements Notifier for Phase 1 — writes to stderr with [GRAVA ALERT] prefix.
// Send never returns an error.
type ConsoleNotifier struct{}

// NewConsoleNotifier returns a new ConsoleNotifier.
func NewConsoleNotifier() *ConsoleNotifier {
	return &ConsoleNotifier{}
}

// Send writes the alert to stderr. Always returns nil (non-fatal contract).
func (n *ConsoleNotifier) Send(title, body string) error {
	fmt.Fprintf(os.Stderr, "[GRAVA ALERT] %s: %s\n", title, body)
	return nil
}
