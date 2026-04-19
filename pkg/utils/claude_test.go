package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckClaudeInstalled_Succeeds(t *testing.T) {
	// This test passes in any environment where claude CLI is installed.
	// In CI without claude, this test will be skipped.
	err := CheckClaudeInstalled()
	if err != nil {
		t.Skip("claude CLI not installed in this environment")
	}
	assert.NoError(t, err)
}
