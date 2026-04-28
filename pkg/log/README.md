# Package: log

Path: `github.com/hoangtrungnguyen/grava/pkg/log`

## Purpose

Thin zerolog wrapper that gives the grava CLI a single configured global
logger plus conventions for using zerolog.Logger as a parameter elsewhere.

## Key Types & Functions

- `Logger zerolog.Logger` — global, initialised by `Init`.
- `Init(level string, jsonMode bool)` — configures `Logger`.
  - `level` parsed by `zerolog.ParseLevel`; falls back to `warn` on error.
  - `jsonMode=true` writes raw JSON lines to stderr; otherwise uses
    `zerolog.ConsoleWriter`.

Log level is sourced from `GRAVA_LOG_LEVEL` (default `warn`).

## Dependencies

- `github.com/rs/zerolog`
- Standard library `os`.

## How It Fits

Initialised once in `pkg/cmd` entry points. Business-logic packages should
accept a `zerolog.Logger` rather than referencing the global so they stay
testable; pass `zerolog.Nop()` in tests.

## Usage

```go
log.Init(os.Getenv("GRAVA_LOG_LEVEL"), false)
log.Logger.Info().Str("cmd", "init").Msg("starting")

// In a sub-package:
func DoWork(logger zerolog.Logger) {
    logger.Debug().Msg("working")
}
```
