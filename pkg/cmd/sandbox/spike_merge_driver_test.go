package sandbox

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSpikeMergeDriver_Registered verifies the scenario is in the global registry.
func TestSpikeMergeDriver_Registered(t *testing.T) {
	s, ok := Find(spikeMergeDriverID)
	require.True(t, ok, "spike-merge-driver must be registered")
	assert.Equal(t, spikeMergeDriverID, s.ID)
	assert.Equal(t, 6, s.EpicGate)
	assert.NotNil(t, s.Run)
}

// TestCheckProcessMerge_Pass verifies the in-process merge checks all pass.
func TestCheckProcessMerge_Pass(t *testing.T) {
	ok, detail := checkProcessMerge()
	assert.True(t, ok, "checkProcessMerge should pass; detail: %s", detail)
	assert.Contains(t, detail, "PASS")
}

// TestCheckProcessMerge_DetailMentionsCases verifies the detail string describes
// all three tested cases so callers can parse what was validated.
func TestCheckProcessMerge_DetailMentionsCases(t *testing.T) {
	_, detail := checkProcessMerge()
	assert.Contains(t, detail, "non-conflicting", "detail should mention non-conflicting case")
	assert.Contains(t, detail, "conflicting", "detail should mention conflicting case")
	assert.Contains(t, detail, "add-both-sides", "detail should mention add-both-sides case")
}

// TestCheckGitInvocation_ReturnsDetail verifies the git check always returns
// a non-empty detail string (regardless of whether grava is on PATH).
func TestCheckGitInvocation_ReturnsDetail(t *testing.T) {
	ok, detail := checkGitInvocation()
	assert.NotEmpty(t, detail, "checkGitInvocation must return a non-empty detail")
	if ok {
		assert.Contains(t, detail, "PASS")
	} else {
		// Either SKIP (no binary) or FAIL (binary exists but something went wrong).
		assert.True(t,
			strings.Contains(detail, "SKIP") || strings.Contains(detail, "FAIL"),
			"non-passing detail must contain SKIP or FAIL: %s", detail)
	}
}

// TestSpikeMergeDriver_BoolHelpers tests helper functions used in report generation.
func TestSpikeMergeDriver_BoolHelpers(t *testing.T) {
	assert.Equal(t, "YES", boolMD(true))
	assert.Equal(t, "NO", boolMD(false))
	assert.Equal(t, "PASS", boolStr(true))
	assert.Equal(t, "SKIP/FAIL", boolStr(false))
}
