// Package log provides a zerolog-based structured logger for the Grava CLI.
//
// Usage:
//   - In pkg/cmd entry points: use the global Logger directly.
//   - In pkg/ business logic: receive zerolog.Logger as a parameter (never use global).
//   - In tests: pass zerolog.Nop() to suppress output.
//
// Log level is controlled via the GRAVA_LOG_LEVEL environment variable (default: "warn").
package log

import (
	"io"
	"os"

	"github.com/rs/zerolog"
)

// Logger is the global zerolog logger. Initialised by Init; safe to use after init.
var Logger zerolog.Logger

// Init configures the global Logger.
//
// level is parsed from GRAVA_LOG_LEVEL; falls back to warn on parse error.
// jsonMode switches from pretty console output to plain-text JSON-friendly output.
func Init(level string, jsonMode bool) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.WarnLevel
	}

	var writer io.Writer
	if jsonMode {
		// No colour/formatting when in JSON output mode — cleaner for piped consumers
		writer = zerolog.ConsoleWriter{Out: os.Stderr, NoColor: true}
	} else {
		writer = zerolog.ConsoleWriter{Out: os.Stderr}
	}

	Logger = zerolog.New(writer).Level(lvl).With().Timestamp().Logger()
}
