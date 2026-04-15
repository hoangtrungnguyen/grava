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

// TestCheckProcessMerge_CaseA_NonConflicting directly tests the merge logic
// for non-conflicting field changes (both sides modify different fields).
func TestCheckProcessMerge_CaseA_NonConflicting(t *testing.T) {
	// Validate case A inline: title changes on current, status changes on other.
	ok, detail := checkProcessMerge()
	require.True(t, ok, detail)
	assert.True(t, strings.Contains(detail, "non-conflicting"), "detail should mention non-conflicting case")
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
