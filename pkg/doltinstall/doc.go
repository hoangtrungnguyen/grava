// Package doltinstall downloads and installs the latest Dolt binary into a
// project-local directory (typically .grava/bin) without requiring root or
// sudo.
//
// The package resolves the host platform via runtime.GOOS / runtime.GOARCH
// (overridable for tests), queries the dolthub/dolt GitHub releases API for
// the latest tag, downloads the matching dolt-<platform>.tar.gz, and
// extracts only the dolt binary from the tarball. If the GitHub API rate
// limits the request, it falls back to the HTTPS releases redirect to
// recover the tag. The extracted binary is chmoded 0755 so it is executable
// regardless of the caller's umask.
//
// Grava uses this package during `grava init` and similar bootstrap flows
// so that fresh checkouts can stand up a working Dolt without operators
// having to install Dolt globally. All network endpoints are configurable
// through Options, which lets tests substitute mock HTTP servers.
//
// Supported platforms are the intersection of Dolt's published releases and
// Go's runtime values: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64,
// and windows/amd64. PlatformString returns an error for anything else.
package doltinstall
