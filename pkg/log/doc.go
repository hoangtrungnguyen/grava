// Package log is the grava CLI's structured logging wrapper around zerolog.
//
// It exposes a single global Logger initialised by Init(level, jsonMode).
// The level argument is parsed by zerolog.ParseLevel and falls back to
// "warn" when invalid; the GRAVA_LOG_LEVEL environment variable is the
// canonical source for this value. When jsonMode is true the logger writes
// raw JSON lines to stderr (suitable for piped consumers and CI parsing);
// otherwise it uses zerolog's ConsoleWriter for a human-readable stderr
// rendering.
//
// The conventional usage pattern in grava is:
//
//   - In pkg/cmd entry points, call Init once and use the global Logger.
//   - In pkg/ business logic, accept a zerolog.Logger as a parameter rather
//     than reaching for the global, so packages remain testable.
//   - In tests, pass zerolog.Nop() to suppress output.
//
// All logs go to stderr so they do not contaminate stdout, which grava
// reserves for command output that may be parsed by other tools.
package log
