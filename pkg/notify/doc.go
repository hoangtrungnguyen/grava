// Package notify dispatches non-fatal system alerts from Grava commands.
//
// The package defines the Notifier interface (a single Send(title, body)
// method) and a Phase-1 ConsoleNotifier implementation that writes alerts to
// stderr with a [GRAVA ALERT] prefix. Send errors are advisory: callers should
// log and continue, never abort the primary operation when notification fails.
//
// Future phases plan additional sinks (e.g. Telegram, WhatsApp) implementing
// the same interface so commands can be wired to user-preferred channels
// without changing call sites.
package notify
