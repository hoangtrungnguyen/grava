package cmddeps

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
)

// GravaError is the standard structured error for JSON output.
type GravaError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteJSONError formats an error into a JSON envelope and writes it to w.
func WriteJSONError(w io.Writer, err error) error {
	code := "INTERNAL_ERROR"
	msg := err.Error()

	// 1. Resolve GravaError if present
	var gerr *gravaerrors.GravaError
	if errors.As(err, &gerr) {
		code = gerr.Code
		msg = gerr.Message
	} else {
		// 2. Resolve common fallback error codes from raw strings
		errStr := err.Error()
		switch {
		case strings.Contains(errStr, "not found"):
			code = "ISSUE_NOT_FOUND"
		case strings.Contains(errStr, "ALREADY_CLAIMED"):
			code = "ALREADY_CLAIMED"
		case strings.Contains(errStr, "NOT_YOUR_CLAIM"):
			code = "NOT_YOUR_CLAIM"
		case strings.Contains(errStr, "INVALID_STATE_TRANSITION"):
			code = "INVALID_STATE_TRANSITION"
		}
	}

	envelope := map[string]GravaError{
		"error": {
			Code:    code,
			Message: msg,
		},
	}
	b, _ := json.MarshalIndent(envelope, "", "  ")
	_, _ = fmt.Fprintln(w, string(b))
	return nil
}
