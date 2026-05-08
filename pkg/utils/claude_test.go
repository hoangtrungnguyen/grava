package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestCheckClaudeInstalled_BypassedBySkipPreflight verifies that setting
// GRAVA_SKIP_PREFLIGHT=1 bypasses the claude CLI check. This is the
// test-friendly bypass used by init_test.go and CI jobs that don't have
// claude installed.
func TestCheckClaudeInstalled_BypassedBySkipPreflight(t *testing.T) {
	// Force PATH to an empty dir so claude is guaranteed missing.
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GRAVA_SKIP_PREFLIGHT", "1")

	require.NoError(t, CheckClaudeInstalled(),
		"GRAVA_SKIP_PREFLIGHT=1 must bypass the claude CLI check")
}

// TestCheckClaudeInstalled_StrictWithoutBypass verifies that the preflight
// remains strict when neither bypass env var is set. Confirms end-user
// behavior is preserved — users must still install claude CLI.
func TestCheckClaudeInstalled_StrictWithoutBypass(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GRAVA_SKIP_PREFLIGHT", "")
	t.Setenv("GRAVA_SKIP_CLAUDE_CHECK", "")

	err := CheckClaudeInstalled()
	require.Error(t, err, "preflight must fail when claude is absent and no bypass is set")
	assert.Contains(t, err.Error(), "claude CLI not found")
}
