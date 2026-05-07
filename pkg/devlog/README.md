# Package: devlog

Path: `github.com/hoangtrungnguyen/grava/pkg/devlog`

## Purpose

No-op compatibility shim retained while remaining call sites migrate to
`pkg/log` (zerolog). Every exported function is a guaranteed no-op — no
files are opened, no output is written.

## Key Types & Functions

All functions are deprecated stubs:

- `Init(_ bool, _ string) error` — returns nil.
- `Close() error` — returns nil.
- `Printf(_ string, _ ...interface{})` — does nothing.
- `Println(_ ...interface{})` — does nothing.

## Dependencies

None.

## How It Fits

Historically `devlog` wrote rotating dev-log files. All real logging now
lives in `pkg/log` (zerolog-based, structured). This package is kept only
so existing imports keep compiling. New code MUST NOT depend on it; the
package will be deleted once the last caller is gone.

## Usage

Do not add new callers. To migrate an existing call site, replace it with
the equivalent `pkg/log` call (typically `log.Logger.Info().Msg(...)` or
`log.Logger.Debug().Msgf(...)`).
