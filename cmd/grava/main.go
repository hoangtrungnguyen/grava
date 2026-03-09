package main

import (
	"runtime/debug"

	"github.com/hoangtrungnguyen/grava/pkg/cmd"
)

// Version is set via ldflags during release builds:
//
//	go build -ldflags "-X 'main.Version=v1.0.0'" ./cmd/grava/
//
// When installed via `go install`, this is empty and we fall back
// to the module version embedded by the Go toolchain.
var Version string

func main() {
	if Version == "" {
		Version = versionFromBuildInfo()
	}
	cmd.SetVersion(Version)
	cmd.Execute()
}

// versionFromBuildInfo extracts version info from Go's embedded build metadata.
// This is automatically populated when installed via `go install ...@latest`.
func versionFromBuildInfo() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}

	version := info.Main.Version // e.g. "v0.1.0" or "(devel)"
	if version == "(devel)" {
		// Local development build — try to find VCS revision
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && len(setting.Value) >= 7 {
				return "dev-" + setting.Value[:7]
			}
		}
		return "dev"
	}
	return version
}
