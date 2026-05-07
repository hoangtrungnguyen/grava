# Module: `pkg/errors`

**Package role:** Structured `GravaError` type with machine-readable error codes (SCREAMING_SNAKE_CASE). Supports `errors.Is` / `errors.As` traversal via code-based matching.

> _Auto-generated on 2026-04-08 (commit `baefe84`). Edit `scripts/update-docs.sh` to change the template._

---

## Files

| File | Lines | Exported Symbols |
|:---|:---|:---|
| `errors.go` | 48 | GravaError,New |
| `errors_test.go` | 86 | TestNew_CreatesGravaError,TestNew_WithCause TestGravaError_Error_ReturnsMessage,TestGravaError_Unwrap_ReturnsCause TestGravaError_Unwrap_NilCause |

## Public API

```
type GravaError struct{ ... }
    func New(code, message string, cause error) *GravaError
```

