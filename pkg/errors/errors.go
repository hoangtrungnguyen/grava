// Package errors provides structured error types for the Grava CLI.
//
// All error codes use SCREAMING_SNAKE_CASE with a domain prefix:
//
//	Init/Setup:    NOT_INITIALIZED, SCHEMA_MISMATCH, ALREADY_INITIALIZED
//	Issues:        ISSUE_NOT_FOUND, INVALID_STATUS, MISSING_REQUIRED_FIELD
//	DB/Tx:         DB_UNREACHABLE, COORDINATOR_DOWN, LOCK_TIMEOUT
//	Import/Export: IMPORT_CONFLICT, IMPORT_ROLLED_BACK, FILE_NOT_FOUND
//	Claim:         ALREADY_CLAIMED, CLAIM_CONFLICT
//
// Import with alias to avoid collision with stdlib errors package:
//
//	import gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
package errors

// GravaError is a structured error carrying a machine-readable code,
// a human-readable message, and an optional underlying cause.
type GravaError struct {
	Code    string // SCREAMING_SNAKE_CASE, domain-prefixed
	Message string // lowercase, no trailing period
	Cause   error  // wrapped underlying error (may be nil)
}

// New constructs a GravaError. Always use this constructor — never struct literals.
func New(code, message string, cause error) *GravaError {
	return &GravaError{Code: code, Message: message, Cause: cause}
}

// Error implements the error interface, returning the human-readable message.
func (e *GravaError) Error() string {
	return e.Message
}

// Unwrap returns the underlying cause, enabling errors.Is / errors.As traversal.
func (e *GravaError) Unwrap() error {
	return e.Cause
}
