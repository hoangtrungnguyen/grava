# Module: `pkg/grava`

**Package role:** Core domain bootstrap. Implements `ResolveGravaDir()` (ADR-004 priority chain) for locating the .grava/ directory.

> _Auto-generated on 2026-04-08 (commit `baefe84`). Edit `scripts/update-docs.sh` to change the template._

---

## Files

| File | Lines | Exported Symbols |
|:---|:---|:---|
| `resolver.go` | 71 | ResolveGravaDir |
| `resolver_test.go` | 164 | TestResolveGravaDir |

## Public API

```
func ResolveGravaDir() (string, error)
```

