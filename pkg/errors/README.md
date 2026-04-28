# Package: errors

Path: `github.com/hoangtrungnguyen/grava/pkg/errors`

## Purpose

Defines the structured error type used throughout the Grava CLI. Errors
carry a machine-readable code, a human-readable message, and an optional
underlying cause, so callers can branch on the code while still preserving
the wrapping chain.

Import with an alias to avoid colliding with the stdlib `errors` package:

```go
import gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
```

## Key Types & Functions

- `GravaError` — struct with fields `Code` (SCREAMING_SNAKE_CASE,
  domain-prefixed), `Message` (lowercase, no trailing period), and
  `Cause` (wrapped error, may be nil).
- `New(code, message string, cause error) *GravaError` — the only
  sanctioned constructor; never use struct literals.
- `(*GravaError).Error() string` — returns the human message.
- `(*GravaError).Unwrap() error` — exposes the cause for `errors.Is` /
  `errors.As` traversal.
- `(*GravaError).Is(target error) bool` — matches by `Code`, enabling
  `errors.Is(err, gravaerrors.New("ISSUE_NOT_FOUND", "", nil))`.

## Code Domains

- Init/Setup: `NOT_INITIALIZED`, `SCHEMA_MISMATCH`, `ALREADY_INITIALIZED`.
- Issues: `ISSUE_NOT_FOUND`, `INVALID_STATUS`, `MISSING_REQUIRED_FIELD`,
  `ISSUE_READ_ONLY`.
- DB / transactions: `DB_UNREACHABLE`, `DB_COMMIT_FAILED`,
  `COORDINATOR_DOWN`, `LOCK_TIMEOUT`.
- Import / export: `IMPORT_CONFLICT`, `IMPORT_ROLLED_BACK`,
  `FILE_NOT_FOUND`.
- Claim lifecycle: `ALREADY_CLAIMED`, `CLAIM_CONFLICT`,
  `INVALID_STATE_TRANSITION`, `NOT_YOUR_CLAIM`.

## Dependencies

Standard library only.

## How It Fits

`GravaError` is the error contract the CLI exposes to scripts and agents.
`pkg/cmddeps.WriteJSONError` reads `Code` and `Message` to build the
`{"error": {...}}` JSON envelope, and `pkg/dolt.WithAuditedTx` returns
`DB_UNREACHABLE` / `DB_COMMIT_FAILED` so callers can branch on retryable
failure modes.

## Usage

```go
return gravaerrors.New("ISSUE_NOT_FOUND",
    fmt.Sprintf("issue %s not found", id), nil)

if errors.Is(err, gravaerrors.New("ALREADY_CLAIMED", "", nil)) {
    // surface a friendly message
}
```
