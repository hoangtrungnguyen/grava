# Package: doltinstall

Path: `github.com/hoangtrungnguyen/grava/pkg/doltinstall`

## Purpose

Downloads and installs the latest Dolt binary into a project-local
directory (e.g. `.grava/bin`) so Grava can bootstrap its own Dolt without
root, sudo, or a system-wide install.

## Key Types & Functions

- `Options` — configures the install. `DestDir` is required; `GOOS`,
  `GOARCH`, `GitHubAPIURL`, and `DownloadBaseURL` are overridable for
  tests. URLs default to `api.github.com/repos/dolthub/dolt/releases/latest`
  and `github.com/dolthub/dolt/releases/download`.
- `InstallDolt(destDir string) error` — convenience wrapper that calls
  `InstallWithOptions` with defaults.
- `InstallWithOptions(opts Options) error` — full pipeline: resolve
  platform, fetch latest version, download tarball, extract `dolt` binary
  to `destDir`, chmod 0755.
- `PlatformString(goos, goarch string) (string, error)` — maps Go runtime
  values to Dolt's release naming (`darwin-arm64`, `linux-amd64`, etc.).

## Dependencies

Standard library only: `archive/tar`, `compress/gzip`, `encoding/json`,
`net/http`, `os`, `path/filepath`, `runtime`, `strings`.

## How It Fits

Called from Grava's bootstrap commands (e.g. `grava init`) so that running
a fresh checkout produces a usable `.grava/bin/dolt` without operator
intervention. Pairs with `pkg/dolt` (the SQL client) which then connects
to a Dolt server backed by the installed binary.

## Usage

```go
if err := doltinstall.InstallDolt(".grava/bin"); err != nil {
    return fmt.Errorf("dolt install failed: %w", err)
}
```

Tests can inject a mock HTTP server:

```go
err := doltinstall.InstallWithOptions(doltinstall.Options{
    DestDir:         tmp,
    GOOS:            "linux",
    GOARCH:          "amd64",
    GitHubAPIURL:    mockAPI.URL,
    DownloadBaseURL: mockDL.URL,
})
```
