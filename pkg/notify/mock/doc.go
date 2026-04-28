// Package mock provides a test double for notify.Notifier.
//
// MockNotifier records every Send invocation in its Calls slice and returns a
// configurable Error value (nil by default, mirroring the non-fatal
// production contract). Tests use it to assert that orchestrator and command
// code emits the expected alerts without writing to stderr or external
// channels.
package mock
