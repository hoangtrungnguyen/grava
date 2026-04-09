# Release Process

This document outlines the steps to release a new version of Grava.

## Prerequisites
- Push access to the [Grava repository](https://github.com/hoangtrungnguyen/grava).
- `git` installed.
- `go` installed (**1.24** or later).
- A clean working directory (no uncommitted changes).

## Automated Release Process

The `scripts/release.sh` script automates the changelog generation, version tagging, and cross-platform builds.

### 1. Verify Quality

Before releasing, ensure all tests pass and check the system health:

```bash
# Run unit tests
go test ./...

# Run diagnostics
go run ./cmd/grava doctor
```

### 2. Run the Release Script

Grava uses [Semantic Versioning](https://semver.org) (vX.Y.Z). To release a new version, run the release script with the new version number as an argument.

```bash
# Example for version 0.1.0
./scripts/release.sh v0.1.0
```

**What the script does:**
1.  Verifies the git working directory is clean.
2.  Generates a changelog since the last tag and prepends it to `CHANGELOG.md`.
3.  Commits the `CHANGELOG.md` change.
4.  Creates an annotated git tag for the version.
5.  Builds binaries for multiple platforms in the `bin/` directory:
    - `grava-darwin-amd64` (macOS Intel)
    - `grava-darwin-arm64` (macOS Apple Silicon)
    - `grava-linux-amd64` (Linux Intel)
    - `grava-linux-arm64` (Linux ARM)
    - `grava-windows-amd64.exe` (Windows)

### 3. Push Changes and Tag

Push the new commit and the tag to the remote repository:

```bash
git push origin main
git push origin v0.1.0
```

### 4. Create GitHub Release

1. Go to [Releases](https://github.com/hoangtrungnguyen/grava/releases).
2. Click **Draft a new release**.
3. Select the tag you just pushed (e.g., `v0.1.0`).
4. **Title**: `Grava v0.1.0`.
5. **Description**: Copy the latest entries from `CHANGELOG.md`.
6. **Binaries**: Upload all files from the `bin/` directory.
7. Click **Publish release**.

## User Installation

Once released, users can install the latest version using the installation script:

```bash
curl -sL https://raw.githubusercontent.com/hoangtrungnguyen/grava/main/scripts/install.sh | bash
```

Alternatively, users can download the binary for their platform from the GitHub Releases page.
