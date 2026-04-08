# Module: `pkg/cmddeps`

**Package role:** Shared dependency injection container and centralized JSON error emitter. Exists to prevent circular imports between pkg/cmd and pkg/cmd/issues/.

> _Auto-generated on 2026-04-08 (commit `baefe84`). Edit `scripts/update-docs.sh` to change the template._

---

## Files

| File | Lines | Exported Symbols |
|:---|:---|:---|
| `deps.go` | 19 | Deps |
| `json.go` | 53 | GravaError,WriteJSONError |

## Public API

```
func WriteJSONError(w io.Writer, err error) error
type Deps struct{ ... }
type GravaError struct{ ... }
```

