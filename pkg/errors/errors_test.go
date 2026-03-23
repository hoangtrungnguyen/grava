package errors_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
)

func TestNew_CreatesGravaError(t *testing.T) {
	err := gravaerrors.New("ISSUE_NOT_FOUND", "issue abc123 not found", nil)
	require.NotNil(t, err)
	assert.Equal(t, "ISSUE_NOT_FOUND", err.Code)
	assert.Equal(t, "issue abc123 not found", err.Message)
	assert.Nil(t, err.Cause)
}

func TestNew_WithCause(t *testing.T) {
	cause := fmt.Errorf("original db error")
	err := gravaerrors.New("DB_UNREACHABLE", "failed to connect to database", cause)
	require.NotNil(t, err)
	assert.Equal(t, "DB_UNREACHABLE", err.Code)
	assert.Equal(t, "failed to connect to database", err.Message)
	assert.Equal(t, cause, err.Cause)
}

func TestGravaError_Error_ReturnsMessage(t *testing.T) {
	err := gravaerrors.New("SCHEMA_MISMATCH", "schema version mismatch", nil)
	assert.Equal(t, "schema version mismatch", err.Error())
}

func TestGravaError_Unwrap_ReturnsCause(t *testing.T) {
	cause := fmt.Errorf("underlying error")
	err := gravaerrors.New("DB_UNREACHABLE", "db failed", cause)
	assert.Equal(t, cause, errors.Unwrap(err))
}

func TestGravaError_Unwrap_NilCause(t *testing.T) {
	err := gravaerrors.New("NOT_INITIALIZED", "not initialized", nil)
	assert.Nil(t, errors.Unwrap(err))
}

func TestErrorsIs_WorksThroughWrap(t *testing.T) {
	sentinel := fmt.Errorf("sentinel")
	err := gravaerrors.New("DB_UNREACHABLE", "db failed", sentinel)
	// errors.Is should traverse Unwrap chain
	assert.True(t, errors.Is(err, sentinel))
}

func TestErrorsAs_WorksForGravaError(t *testing.T) {
	wrapped := fmt.Errorf("wrap: %w", gravaerrors.New("ISSUE_NOT_FOUND", "not found", nil))
	var gravaErr *gravaerrors.GravaError
	require.True(t, errors.As(wrapped, &gravaErr))
	assert.Equal(t, "ISSUE_NOT_FOUND", gravaErr.Code)
}

func TestGravaError_ImplementsErrorInterface(t *testing.T) {
	var err error = gravaerrors.New("TEST_CODE", "test message", nil)
	assert.NotNil(t, err)
	assert.Equal(t, "test message", err.Error())
}
