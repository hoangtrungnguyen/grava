# Package: grava

Path: `github.com/hoangtrungnguyen/grava/pkg/grava`

## Purpose

Resolve the active `.grava/` directory for the current CLI process,
following the ADR-004 priority chain (env var, redirect file, upward walk).

## Key Types & Functions

- `ResolveGravaDir() (string, error)` — returns the absolute path of the
  `.grava/` directory grava should use, or a typed error
  (`NOT_INITIALIZED`, `REDIRECT_STALE`).

Resolution order:

1. `GRAVA_DIR` environment variable (must point at an existing directory).
2. Nearest `.grava/redirect` walking upward from CWD; content is treated
   as a path (relative paths resolve against the containing `.grava/`).
3. Nearest `.grava/` directory walking upward from CWD.

## Dependencies

- `github.com/hoangtrungnguyen/grava/pkg/errors` for typed error codes.
- Standard library `os`, `path/filepath`, `strings`, `fmt`.

## How It Fits

Entry point used by every subcommand to locate state under `.grava/`
(notably `.grava/dolt/`). Worktree-based agent workflows set
`GRAVA_DIR` or write a `.grava/redirect` so multiple checkouts share the
same Dolt database.

## Usage

```go
dir, err := grava.ResolveGravaDir()
if err != nil {
    return err
}
doltDir := filepath.Join(dir, "dolt")
```
