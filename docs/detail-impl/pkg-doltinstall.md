# Module: `pkg/doltinstall`

**Package role:** Downloads and installs the latest Dolt binary to .grava/bin/ without root/sudo. Called by grava init.

> _Auto-generated on 2026-04-08 (commit `baefe84`). Edit `scripts/update-docs.sh` to change the template._

---

## Files

| File | Lines | Exported Symbols |
|:---|:---|:---|
| `installer.go` | 240 | Options,InstallDolt InstallWithOptions,PlatformString |
| `installer_test.go` | 108 | TestInstallDolt,TestPlatformString_Supported TestPlatformString_Unsupported |

## Public API

```
func InstallDolt(destDir string) error
func InstallWithOptions(opts Options) error
func PlatformString(goos, goarch string) (string, error)
type Options struct{ ... }
```

