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

// Options configures the Dolt installation. All fields are optional except DestDir.
// Override URL fields for testing with mock HTTP servers.
type Options struct {
	// DestDir is where the dolt binary will be placed (e.g. ".grava/bin").
	DestDir string
	// GOOS overrides runtime.GOOS (for testing).
	GOOS string
	// GOARCH overrides runtime.GOARCH (for testing).
	GOARCH string
	// GitHubAPIURL overrides the GitHub releases API endpoint (for testing).
	GitHubAPIURL string
	// DownloadBaseURL overrides the GitHub download base URL (for testing).
	// In production: https://github.com/dolthub/dolt/releases/download
	// The final URL becomes: <DownloadBaseURL>/<version>/dolt-<platform>.tar.gz
	DownloadBaseURL string
}

// InstallDolt downloads and installs the latest Dolt binary to destDir.
// destDir should be <projectRoot>/.grava/bin.
// No root/sudo required — installs only to the specified local directory.
func InstallDolt(destDir string) error {
	return InstallWithOptions(Options{DestDir: destDir})
}

// InstallWithOptions installs Dolt with configurable options.
// Primarily used for testing with mock HTTP servers.
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

	// 1. Resolve platform string (e.g. "darwin-arm64")
	platform, err := PlatformString(opts.GOOS, opts.GOARCH)
	if err != nil {
		return err
	}

	// 2. Fetch latest release version tag from GitHub API
	version, err := fetchLatestVersion(opts.GitHubAPIURL)
	if err != nil {
		return fmt.Errorf("failed to fetch latest Dolt version: %w", err)
	}

	// 3. Build download URL
	// Real URL: https://github.com/dolthub/dolt/releases/download/v1.82.4/dolt-darwin-arm64.tar.gz
	// Tarball internal path: dolt-{platform}/bin/dolt  (confirmed from official install.sh)
	tarballName := fmt.Sprintf("dolt-%s.tar.gz", platform)
	downloadURL := fmt.Sprintf("%s/%s/%s", opts.DownloadBaseURL, version, tarballName)

	// 4. Download tarball to a temp file
	tmpFile, err := os.CreateTemp("", "dolt-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name()) //nolint:errcheck
	defer tmpFile.Close()           //nolint:errcheck

	if err := downloadFile(downloadURL, tmpFile); err != nil {
		return fmt.Errorf("failed to download Dolt from %s: %w", downloadURL, err)
	}

	// 5. Seek back to start before extracting
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek temp file: %w", err)
	}

	// 6. Ensure destination directory exists
	if err := os.MkdirAll(opts.DestDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", opts.DestDir, err)
	}

	// 7. Extract the dolt binary from the tarball
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

// PlatformString maps GOOS/GOARCH to Dolt's release naming convention.
// Exported so tests can call it directly.
func PlatformString(goos, goarch string) (string, error) {
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
	return "", fmt.Errorf("unsupported platform %s/%s — dolt supports: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64", goos, goarch)
}

// fetchLatestVersion calls the GitHub releases API and returns the tag name (e.g. "v1.82.4").
func fetchLatestVersion(apiURL string) (string, error) {
	resp, err := http.Get(apiURL) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		// Fallback to GitHub HTML releases redirect to avoid rate limits
		if apiURL == defaultGitHubAPIURL {
			client := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}
			fallbackURL := "https://github.com/dolthub/dolt/releases/latest"
			fResp, fErr := client.Head(fallbackURL)
			if fErr == nil {
				fResp.Body.Close() //nolint:errcheck
				if fResp.StatusCode == http.StatusFound {
					loc := fResp.Header.Get("Location")
					parts := strings.Split(loc, "/")
					tag := parts[len(parts)-1]
					if tag != "" {
						return tag, nil
					}
				}
			}
		}
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to decode GitHub response: %w", err)
	}
	if release.TagName == "" {
		return "", fmt.Errorf("empty tag_name in GitHub API response")
	}
	return release.TagName, nil
}

// downloadFile downloads a URL into an open file.
func downloadFile(url string, dest *os.File) error {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return fmt.Errorf("HTTP GET failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d downloading %s", resp.StatusCode, url)
	}
	if _, err = io.Copy(dest, resp.Body); err != nil {
		return fmt.Errorf("failed to write download: %w", err)
	}
	return nil
}

// extractDoltBinary reads a .tar.gz stream and extracts the dolt binary to destPath.
// Dolt release tarballs have the structure: dolt-{platform}/bin/dolt
// (confirmed from https://github.com/dolthub/dolt/releases/latest/download/install.sh)
func extractDoltBinary(r io.Reader, destPath string, platform string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gr.Close() //nolint:errcheck

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tarball: %w", err)
		}

		// Normalise path separators for cross-platform safety
		name := filepath.ToSlash(hdr.Name)

		// Match the real Dolt tarball path: dolt-{platform}/bin/dolt
		// Also handle simpler variants in case layout changes
		if strings.HasSuffix(name, "/bin/dolt") || name == "bin/dolt" || name == "dolt" {
			f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close() //nolint:errcheck
				return fmt.Errorf("failed to write dolt binary: %w", err)
			}
			f.Close() //nolint:errcheck

			// Ensure it is executable regardless of umask
			if err := os.Chmod(destPath, 0755); err != nil {
				return fmt.Errorf("failed to change permissions on dolt binary: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("dolt binary not found inside tarball for platform %s (expected path: dolt-%s/bin/dolt)", platform, platform)
}
