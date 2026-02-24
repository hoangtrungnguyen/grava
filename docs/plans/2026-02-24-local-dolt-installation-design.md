# Design: Project-Local Dolt Installation

**Date:** 2026-02-24  
**Status:** Approved

---

## Problem

`grava init` currently fails immediately if `dolt` is not installed in the system `$PATH`. This creates a friction that requires users to manually install Dolt before using Grava. Additionally, even if the user has Dolt installed globally, there is no guarantee the version matches what Grava expects, and team environments may have version drift.

## Goal

Download and install the Dolt binary **inside the project folder** at `.grava/bin/dolt` during `grava init`. All subsequent Grava commands that invoke Dolt must use this local binary, not the system one.

---

## Architecture

### Directory Layout (after init)

```
.grava/                   ← gitignored (already)
  bin/
    dolt                  ← downloaded binary (chmod +x)
  dolt/                   ← dolt database files (existing)
  dolt.log                ← server log (existing)
```

### New Package: `pkg/doltinstall`

Responsible for:
1. Detecting `GOOS` + `GOARCH`
2. Fetching the latest Dolt release tag from GitHub API
3. Downloading the correct tarball from GitHub releases
4. Extracting the `dolt` binary to `.grava/bin/dolt`
5. Setting execute permissions (`0755`)

Supported platforms: `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`.

### New Utility: `pkg/utils/dolt_resolver.go`

```go
// ResolveDoltBinary returns the path to the dolt binary to use.
// It checks .grava/bin/dolt first (local), then falls back to system PATH.
func ResolveDoltBinary() (string, error)
```

This is the **single source of truth** for getting the dolt binary path. All `exec.Command("dolt", ...)` callers must migrate to use this.

---

## Changes Required

### Go Code

| File | Change |
|------|--------|
| `pkg/utils/dolt_resolver.go` | **New** — `ResolveDoltBinary()` |
| `pkg/doltinstall/installer.go` | **New** — download + extract logic |
| `pkg/doltinstall/installer_test.go` | **New** — unit tests |
| `pkg/cmd/init.go` | Replace `LookPath("dolt")` check with download flow; use resolved path |
| `pkg/cmd/start.go` | Go pure-Go; use resolved dolt path instead of shell script |

### Shell Scripts

| File | Change |
|------|--------|
| `scripts/start_dolt_server.sh` | Accept `DOLT_BIN` env var, fallback to `dolt` |
| `scripts/init_dolt.sh` | Accept `DOLT_BIN` env var, fallback to `dolt` |
| `scripts/apply_schema.sh` | Accept `DOLT_BIN` env var, fallback to `dolt` |

---

## Download Flow

```
http GET https://api.github.com/repos/dolthub/dolt/releases/latest
  → parse tag_name (e.g. "v1.43.9")

http GET https://github.com/dolthub/dolt/releases/download/{tag}/dolt-{os}-{arch}.tar.gz
  → stream to temp file
  → extract dolt binary
  → move to .grava/bin/dolt
  → chmod 0755
```

Platform mapping:
- `darwin/amd64`  → `dolt-darwin-amd64`
- `darwin/arm64`  → `dolt-darwin-arm64`
- `linux/amd64`   → `dolt-linux-amd64`
- `linux/arm64`   → `dolt-linux-arm64`

---

## Resolution Priority (ResolveDoltBinary)

1. `.grava/bin/dolt` exists and is executable → **use it**
2. `dolt` found in system `$PATH` → **use it**
3. Neither → return error (caller = init, which will trigger install)

---

## Gitignore

`.grava/` is already present in `.gitignore`. No changes needed.

---

## Testing Strategy

- Unit test `ResolveDoltBinary()` with a temp dir simulating `.grava/bin/dolt`
- Unit test installer with a mock HTTP server serving a fake tarball
- Integration test: run `grava init` in a temp dir without dolt in PATH, assert `.grava/bin/dolt` exists and is executable
