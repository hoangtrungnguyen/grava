package doltinstall_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hoangtrungnguyen/grava/pkg/doltinstall"
)

// buildFakeTarball creates an in-memory .tar.gz mimicking the real Dolt release structure:
// dolt-{platform}/bin/dolt
func buildFakeTarball(t *testing.T, platform string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("#!/bin/sh\necho fake-dolt")
	hdr := &tar.Header{
		Name: "dolt-" + platform + "/bin/dolt",
		Mode: 0755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	tw.Close() //nolint:errcheck
	gw.Close() //nolint:errcheck
	return buf.Bytes()
}

func TestInstallDolt(t *testing.T) {
	platform := "linux-amd64"
	tarball := buildFakeTarball(t, platform)

	// Mock GitHub API for latest release
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v1.0.0"}`)) //nolint:errcheck
	}))
	defer apiServer.Close()

	// Mock download server — serves the tarball for any path
	dlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(tarball) //nolint:errcheck
	}))
	defer dlServer.Close()

	destDir := t.TempDir()
	binPath := filepath.Join(destDir, "dolt")

	err := doltinstall.InstallWithOptions(doltinstall.Options{
		DestDir:         destDir,
		GOOS:            "linux",
		GOARCH:          "amd64",
		GitHubAPIURL:    apiServer.URL,
		DownloadBaseURL: dlServer.URL,
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

func TestPlatformString_Supported(t *testing.T) {
	cases := []struct {
		goos, goarch string
		want         string
	}{
		{"darwin", "amd64", "darwin-amd64"},
		{"darwin", "arm64", "darwin-arm64"},
		{"linux", "amd64", "linux-amd64"},
		{"linux", "arm64", "linux-arm64"},
		{"windows", "amd64", "windows-amd64"},
	}
	for _, c := range cases {
		got, err := doltinstall.PlatformString(c.goos, c.goarch)
		if err != nil {
			t.Errorf("PlatformString(%q, %q) error: %v", c.goos, c.goarch, err)
		}
		if got != c.want {
			t.Errorf("PlatformString(%q, %q) = %q, want %q", c.goos, c.goarch, got, c.want)
		}
	}
}

func TestPlatformString_Unsupported(t *testing.T) {
	_, err := doltinstall.PlatformString("plan9", "386")
	if err == nil {
		t.Error("expected error for unsupported platform")
	}
}
