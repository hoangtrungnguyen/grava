# Release Process

This document outlines the steps to release a new version of Grava.

## Prerequisites
- Push access to the [Grava repository](https://github.com/hoangtrungnguyen/grava).
- `git` installed.
- `go` installed (1.21 or later).

## Automated Release Process

The `scripts/release.sh` script automates the build process for multiple platforms.

### 1. Verify Tests

Before releasing, ensure all tests pass:

```bash
go test ./...
```

### 2. Tag a New Version

Grava uses [Semantic Versioning](https://semver.org) (vX.Y.Z).

1. Determine the next version number.
2. Create and push a git tag:

```bash
# Example for version 0.1.0
git tag v0.1.0
git push origin v0.1.0
```

### 3. Build Binaries

Run the `release.sh` script to generate cross-platform binaries. You can optionally pass the version explicitly, otherwise it defaults to `git describe --tags`.

```bash
./scripts/release.sh v0.1.0
```

This will create a `bin/` directory containing:
- `grava-darwin-amd64` (macOS Intel)
- `grava-darwin-arm64` (macOS Apple Silicon)
- `grava-linux-amd64` (Linux Intel)
- `grava-linux-arm64` (Linux ARM)
- `grava-windows-amd64.exe` (Windows)

### 4. Create GitHub Release

1. Go to [Releases](https://github.com/hoangtrungnguyen/grava/releases).
2. Click **Draft a new release**.
3. Select the tag you just pushed (e.g., `v0.1.0`).
4. **Title**: `Grava v0.1.0` (or similar).
5. **Description**: Add release notes, changelog, etc.
6. **Binaries**: Upload all files from the `bin/` directory generated in step 3.
7. Click **Publish release**.

## User Installation

Once released, users can install the latest version using the installation script:

```bash
curl -sL https://raw.githubusercontent.com/hoangtrungnguyen/grava/main/scripts/install.sh | bash
```

Alternatively, users can download the binary for their platform from the GitHub Releases page.
