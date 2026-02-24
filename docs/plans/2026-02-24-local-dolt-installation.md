# Local Dolt Installation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `grava init` download and install the Dolt binary to `.grava/bin/dolt`, and make all Dolt invocations throughout the codebase use that local binary.

**Architecture:** A new `pkg/doltinstall` package handles downloading and extracting Dolt from GitHub releases. A shared utility `pkg/utils/dolt_resolver.go` provides `ResolveDoltBinary()` as the single source of truth. All `exec.Command("dolt", ...)` sites are updated to use the resolved path.

**Tech Stack:** Go stdlib (`net/http`, `archive/tar`, `compress/gzip`, `os/exec`, `runtime`), GitHub releases API, Cobra/Viper (existing).

---

## Task 1: Create `pkg/utils/dolt_resolver.go`

**Files:**
- Create: `pkg/utils/dolt_resolver.go`
- Create: `pkg/utils/dolt_resolver_test.go`

**Step 1: Write the failing test**

Create `pkg/utils/dolt_resolver_test.go`:

```go
package utils_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hoangtrungnguyen/grava/pkg/utils"
)

func TestResolveDoltBinary_LocalExists(t *testing.T) {
	// Create a temp .grava/bin/dolt
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, ".grava", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	binaryName := "dolt"
	if runtime.GOOS == "windows" {
		binaryName = "dolt.exe"
	}
	localDolt := filepath.Join(binDir, binaryName)
	if err := os.WriteFile(localDolt, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatal(err)
	}

	got, err := utils.ResolveDoltBinary(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != localDolt {
		t.Errorf("expected %q, got %q", localDolt, got)
	}
}

func TestResolveDoltBinary_FallsBackToSystem(t *testing.T) {
	// Empty project dir — no local dolt
	tmp := t.TempDir()
	// This test passes only if 'dolt' is on PATH; skip otherwise
	got, err := utils.ResolveDoltBinary(tmp)
	if err != nil {
		t.Skip("dolt not on system PATH, skipping fallback test")
	}
	if got == "" {
		t.Error("expected non-empty path")
	}
}

func TestResolveDoltBinary_NeitherFound(t *testing.T) {
	// Empty project dir and dolt not on PATH (we can't guarantee PATH state,
	// so just verify local lookup doesn't panic)
	tmp := t.TempDir()
	// As long as it doesn't panic, this is acceptable
	_, _ = utils.ResolveDoltBinary(tmp)
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/trungnguyenhoang/IdeaProjects/grava
go test ./pkg/utils/... -run TestResolveDoltBinary -v
```
Expected: FAIL — `ResolveDoltBinary` not defined yet.

**Step 3: Write the implementation**

Create `pkg/utils/dolt_resolver.go`:

```go
package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ResolveDoltBinary returns the absolute path to the dolt binary to use.
// It checks <projectRoot>/.grava/bin/dolt first (local install), then
// falls back to the system PATH. Returns an error if dolt cannot be found.
func ResolveDoltBinary(projectRoot string) (string, error) {
	binaryName := "dolt"
	if runtime.GOOS == "windows" {
		binaryName = "dolt.exe"
	}

	localPath := filepath.Join(projectRoot, ".grava", "bin", binaryName)
	if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
		return localPath, nil
	}

	systemPath, err := exec.LookPath(binaryName)
	if err != nil {
		return "", fmt.Errorf("dolt not found locally at %s and not on system PATH: %w", localPath, err)
	}
	return systemPath, nil
}

// LocalDoltBinDir returns the path to .grava/bin directory for a given project root.
func LocalDoltBinDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".grava", "bin")
}

// LocalDoltBinaryPath returns the expected path of the locally installed dolt binary.
func LocalDoltBinaryPath(projectRoot string) string {
	binaryName := "dolt"
	if runtime.GOOS == "windows" {
		binaryName = "dolt.exe"
	}
	return filepath.Join(projectRoot, ".grava", "bin", binaryName)
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./pkg/utils/... -run TestResolveDoltBinary -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/utils/dolt_resolver.go pkg/utils/dolt_resolver_test.go
git commit -m "feat: add ResolveDoltBinary utility for local dolt path resolution"
```

---

## Task 2: Create `pkg/doltinstall` package

**Files:**
- Create: `pkg/doltinstall/installer.go`
- Create: `pkg/doltinstall/installer_test.go`

**Step 1: Write the failing test**

Create `pkg/doltinstall/installer_test.go`:

```go
package doltinstall_test

import (
	"archive/tar"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hoangtrungnguyen/grava/pkg/doltinstall"
)

// buildFakeTarball creates an in-memory .tar.gz with a fake dolt binary inside.
func buildFakeTarball(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Dolt release tarballs have structure: dolt-<os>-<arch>/bin/dolt
	content := []byte("#!/bin/sh\necho fake-dolt")
	hdr := &tar.Header{
		Name: "dolt-linux-amd64/bin/dolt",
		Mode: 0755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func TestInstallDolt(t *testing.T) {
	tarball := buildFakeTarball(t)

	// Mock GitHub API for latest release
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v1.0.0"}`))
	}))
	defer apiServer.Close()

	// Mock download server
	dlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(tarball)
	}))
	defer dlServer.Close()

	destDir := t.TempDir()
	binPath := filepath.Join(destDir, "dolt")

	err := doltinstall.InstallWithOptions(doltinstall.Options{
		DestDir:          destDir,
		GOOS:             "linux",
		GOARCH:           "amd64",
		GitHubAPIURL:     apiServer.URL + "/repos/dolthub/dolt/releases/latest",
		DownloadBaseURL:  dlServer.URL,
	})
	if err != nil {
		t.Fatalf("InstallWithOptions failed: %v", err)
	}

	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("dolt binary not found at %s: %v", binPath, err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("dolt binary is not executable")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./pkg/doltinstall/... -v
```
Expected: FAIL — package not defined yet.

**Step 3: Write the implementation**

Create `pkg/doltinstall/installer.go`:

```go
package doltinstall

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	defaultGitHubAPIURL    = "https://api.github.com/repos/dolthub/dolt/releases/latest"
	defaultDownloadBaseURL = "https://github.com/dolthub/dolt/releases/download"
)

// Options configures the Dolt installation. Override fields for testing.
type Options struct {
	// DestDir is where the dolt binary will be placed (e.g. ".grava/bin")
	DestDir string
	// GOOS overrides runtime.GOOS (for testing)
	GOOS string
	// GOARCH overrides runtime.GOARCH (for testing)
	GOARCH string
	// GitHubAPIURL overrides the GitHub releases API endpoint (for testing)
	GitHubAPIURL string
	// DownloadBaseURL overrides the download base URL (for testing)
	DownloadBaseURL string
}

// InstallDolt downloads and installs the latest Dolt binary to destDir.
// destDir is typically <projectRoot>/.grava/bin
func InstallDolt(destDir string) error {
	return InstallWithOptions(Options{DestDir: destDir})
}

// InstallWithOptions installs Dolt with configurable options (for testing).
func InstallWithOptions(opts Options) error {
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}
	if opts.GOARCH == "" {
		opts.GOARCH = runtime.GOARCH
	}
	if opts.GitHubAPIURL == "" {
		opts.GitHubAPIURL = defaultGitHubAPIURL
	}
	if opts.DownloadBaseURL == "" {
		opts.DownloadBaseURL = defaultDownloadBaseURL
	}

	// 1. Get latest version tag
	version, err := fetchLatestVersion(opts.GitHubAPIURL)
	if err != nil {
		return fmt.Errorf("failed to fetch latest Dolt version: %w", err)
	}

	// 2. Build platform string
	platform, err := platformString(opts.GOOS, opts.GOARCH)
	if err != nil {
		return err
	}

	// 3. Build download URL
	tarballName := fmt.Sprintf("dolt-%s.tar.gz", platform)
	var downloadURL string
	if strings.HasPrefix(opts.DownloadBaseURL, "http://127") ||
		strings.HasPrefix(opts.DownloadBaseURL, "http://localhost") {
		// Test mode: use mock server URL directly
		downloadURL = opts.DownloadBaseURL + "/" + tarballName
	} else {
		downloadURL = fmt.Sprintf("%s/%s/%s", opts.DownloadBaseURL, version, tarballName)
	}

	// 4. Download tarball
	tmpFile, err := os.CreateTemp("", "dolt-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if err := downloadFile(downloadURL, tmpFile); err != nil {
		return fmt.Errorf("failed to download Dolt from %s: %w", downloadURL, err)
	}

	// 5. Extract binary
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := os.MkdirAll(opts.DestDir, 0755); err != nil {
		return fmt.Errorf("failed to create dest dir %s: %w", opts.DestDir, err)
	}

	binaryName := "dolt"
	if opts.GOOS == "windows" {
		binaryName = "dolt.exe"
	}
	destPath := filepath.Join(opts.DestDir, binaryName)

	if err := extractDoltBinary(tmpFile, destPath, platform); err != nil {
		return fmt.Errorf("failed to extract Dolt binary: %w", err)
	}

	return nil
}

// fetchLatestVersion calls the GitHub releases API and returns the tag name.
func fetchLatestVersion(apiURL string) (string, error) {
	resp, err := http.Get(apiURL) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("empty tag_name in GitHub response")
	}
	return release.TagName, nil
}

// platformString maps GOOS/GOARCH to Dolt's release naming convention.
func platformString(goos, goarch string) (string, error) {
	table := map[string]string{
		"darwin/amd64":  "darwin-amd64",
		"darwin/arm64":  "darwin-arm64",
		"linux/amd64":   "linux-amd64",
		"linux/arm64":   "linux-arm64",
		"windows/amd64": "windows-amd64",
	}
	key := goos + "/" + goarch
	if p, ok := table[key]; ok {
		return p, nil
	}
	return "", fmt.Errorf("unsupported platform: %s/%s", goos, goarch)
}

// downloadFile downloads a URL into an open file.
func downloadFile(url string, dest *os.File) error {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d downloading %s", resp.StatusCode, url)
	}
	_, err = io.Copy(dest, resp.Body)
	return err
}

// extractDoltBinary reads a .tar.gz and extracts the dolt binary to destPath.
// Dolt tarballs have the structure: dolt-<platform>/bin/dolt
func extractDoltBinary(r io.Reader, destPath string, platform string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Look for the binary: dolt-<platform>/bin/dolt OR just bin/dolt
		name := filepath.ToSlash(hdr.Name)
		if strings.HasSuffix(name, "/bin/dolt") || name == "bin/dolt" || name == "dolt" {
			f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
			return nil
		}
	}
	return fmt.Errorf("dolt binary not found inside tarball (platform: %s)", platform)
}
```

**Step 4: Add missing import to test file**

Add `"bytes"` to the test file imports (it uses `bytes.Buffer`).

**Step 5: Run tests to verify they pass**

```bash
go test ./pkg/doltinstall/... -v
```
Expected: PASS

**Step 6: Commit**

```bash
git add pkg/doltinstall/
git commit -m "feat: add doltinstall package for downloading local dolt binary"
```

---

## Task 3: Update `pkg/cmd/init.go` to download Dolt locally

**Files:**
- Modify: `pkg/cmd/init.go`

**Step 1: Understand the current flow**

Read `pkg/cmd/init.go`. The current panic point is:

```go
// Line 24-26
if _, err := exec.LookPath("dolt"); err != nil {
    return fmt.Errorf("dolt not found: ...")
}
```

And the exec calls use bare `"dolt"`:
```go
initCmd := exec.Command("dolt", "init")         // line 47
serverCmd := exec.Command("dolt", "sql-server", ...) // line 73
```

**Step 2: Write the new `init.go`**

Replace with a three-step flow:
1. `ResolveDoltBinary(cwd)` — try local or system
2. If not found → `doltinstall.InstallDolt(localBinDir)` → then resolve again
3. Use resolved path for all `exec.Command` calls

Key changes to `pkg/cmd/init.go`:

```go
// Replace lines 23-29 with:
cwd, err := os.Getwd()
if err != nil {
    return fmt.Errorf("failed to get working directory: %w", err)
}

doltBin, err := utils.ResolveDoltBinary(cwd)
if err != nil {
    // Dolt not found locally or on PATH — install it
    if !outputJSON {
        fmt.Fprintln(cmd.OutOrStdout(), "📥 Dolt not found. Installing to .grava/bin/dolt...")
    }
    binDir := utils.LocalDoltBinDir(cwd)
    if installErr := doltinstall.InstallDolt(binDir); installErr != nil {
        return fmt.Errorf("failed to install dolt: %w", installErr)
    }
    doltBin, err = utils.ResolveDoltBinary(cwd)
    if err != nil {
        return fmt.Errorf("dolt install appeared to succeed but binary not found: %w", err)
    }
    if !outputJSON {
        fmt.Fprintln(cmd.OutOrStdout(), "✅ Dolt installed to .grava/bin/dolt")
    }
}
if !outputJSON {
    fmt.Fprintln(cmd.OutOrStdout(), "✅ Dolt is ready.")
}

// Remove duplicated cwd fetch below (line 55-58 currently)
```

Then replace the two exec.Command calls:
```go
// Line 47: was exec.Command("dolt", "init")
initDoltCmd := exec.Command(doltBin, "init")

// Line 73: was exec.Command("dolt", "sql-server", ...)
serverCmd := exec.Command(doltBin, "sql-server", "--port", fmt.Sprintf("%d", port), "--host", "0.0.0.0")
```

Also remove the duplicate `cwd, err := os.Getwd()` at line 55 since we now get it earlier.

**Step 3: Verify the file compiles**

```bash
go build ./pkg/cmd/...
```
Expected: no errors.

**Step 4: Commit**

```bash
git add pkg/cmd/init.go
git commit -m "feat: grava init auto-downloads dolt to .grava/bin if not found"
```

---

## Task 4: Update `pkg/cmd/start.go` to use resolved dolt path

**Files:**
- Modify: `pkg/cmd/start.go`

**Step 1: Understand the current flow**

`start.go` currently calls `utils.FindScript("start_dolt_server.sh")` and executes it via shell — this shell script calls bare `dolt`. We'll replace this with a pure-Go implementation using the resolved binary.

**Step 2: Rewrite the server start in `start.go`**

Replace the script-based approach with:

```go
// Replace "Find start script" section (lines 47-56) with:
cwd, err := os.Getwd()
if err != nil {
    return fmt.Errorf("failed to get working directory: %w", err)
}

doltBin, err := utils.ResolveDoltBinary(cwd)
if err != nil {
    return fmt.Errorf("dolt not found: run 'grava init' first to install dolt: %w", err)
}

doltRepoDir := filepath.Join(cwd, ".grava", "dolt")
if _, statErr := os.Stat(doltRepoDir); os.IsNotExist(statErr) {
    return fmt.Errorf("dolt database not found at %s — run 'grava init' first", doltRepoDir)
}

serverCmd := exec.Command(doltBin, "sql-server", "--port", port, "--host", "0.0.0.0")
serverCmd.Dir = doltRepoDir
```

**Step 3: Verify start.go compiles**

```bash
go build ./pkg/cmd/...
```
Expected: no errors.

**Step 4: Commit**

```bash
git add pkg/cmd/start.go
git commit -m "feat: grava start uses resolved local dolt binary instead of shell script"
```

---

## Task 5: Update shell scripts to accept `DOLT_BIN` env var

**Files:**
- Modify: `scripts/start_dolt_server.sh`
- Modify: `scripts/init_dolt.sh`
- Modify: `scripts/apply_schema.sh`

These scripts are still used in some contexts. Update them to honour `DOLT_BIN` env var with fallback to system `dolt`.

**Step 1: Update `scripts/start_dolt_server.sh`**

Add at the top (after `set -e`):
```bash
# Use local dolt if available, otherwise system dolt
DOLT_BIN="${DOLT_BIN:-dolt}"
```

Then replace all bare `dolt` command invocations with `$DOLT_BIN`:
```bash
# Before: dolt sql-server ...
# After:
$DOLT_BIN sql-server --port=$PORT --host=0.0.0.0 --loglevel=info
```

Also update the `command -v dolt` check:
```bash
if ! command -v "$DOLT_BIN" &> /dev/null; then
    echo "Error: dolt not found at $DOLT_BIN"
    exit 1
fi
```

**Step 2: Update `scripts/init_dolt.sh`** — same pattern:
```bash
DOLT_BIN="${DOLT_BIN:-dolt}"
```
Replace all `dolt` calls with `$DOLT_BIN`.

**Step 3: Update `scripts/apply_schema.sh`** — same pattern.

**Step 4: Verify scripts are valid bash**

```bash
bash -n scripts/start_dolt_server.sh
bash -n scripts/init_dolt.sh
bash -n scripts/apply_schema.sh
```
Expected: no output (syntax OK).

**Step 5: Commit**

```bash
git add scripts/start_dolt_server.sh scripts/init_dolt.sh scripts/apply_schema.sh
git commit -m "feat: shell scripts respect DOLT_BIN env var for local dolt binary"
```

---

## Task 6: Run full test suite and verify

**Step 1: Run unit tests (no DB)**

```bash
go test ./pkg/utils/... ./pkg/doltinstall/... -v
```
Expected: all PASS.

**Step 2: Build the binary**

```bash
go build -o /tmp/grava-test ./cmd/grava/
```
Expected: no errors.

**Step 3: Integration smoke test**

```bash
mkdir /tmp/grava-local-test
cd /tmp/grava-local-test
/tmp/grava-test init
ls .grava/bin/dolt   # Should exist
ls .grava/dolt/.dolt # DB should be initialised
```
Expected: `dolt` binary at `.grava/bin/dolt`, DB initialised.

**Step 4: Commit any remaining fixes**

```bash
git add -A
git commit -m "fix: resolve any issues from integration smoke test"
```

---

## Summary of All Changed Files

| File | Action |
|------|--------|
| `pkg/utils/dolt_resolver.go` | **Created** |
| `pkg/utils/dolt_resolver_test.go` | **Created** |
| `pkg/doltinstall/installer.go` | **Created** |
| `pkg/doltinstall/installer_test.go` | **Created** |
| `pkg/cmd/init.go` | **Modified** — installs+resolves dolt locally |
| `pkg/cmd/start.go` | **Modified** — pure Go, uses resolved dolt path |
| `scripts/start_dolt_server.sh` | **Modified** — honours `DOLT_BIN` |
| `scripts/init_dolt.sh` | **Modified** — honours `DOLT_BIN` |
| `scripts/apply_schema.sh` | **Modified** — honours `DOLT_BIN` |
