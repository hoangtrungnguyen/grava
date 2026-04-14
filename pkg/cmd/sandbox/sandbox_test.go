package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- framework tests ---

func TestResultJSON_PassFields(t *testing.T) {
	r := pass("TS-99", "all checks passed", "elapsed=5ms")
	r.DurationMs = 5

	b, err := json.Marshal(r)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(b, &decoded))

	assert.Equal(t, "TS-99", decoded["scenario"])
	assert.Equal(t, "pass", decoded["status"])
	assert.Equal(t, float64(5), decoded["duration_ms"])
	details, ok := decoded["details"].([]any)
	require.True(t, ok)
	assert.Len(t, details, 2)
	// error field must be absent for passing results
	_, hasError := decoded["error"]
	assert.False(t, hasError, "passing result must not include error field")
}

func TestResultJSON_FailFields(t *testing.T) {
	r := fail("TS-99", "something broke", "detail A")
	b, err := json.Marshal(r)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(b, &decoded))

	assert.Equal(t, "fail", decoded["status"])
	assert.Equal(t, "something broke", decoded["error"])
}

func TestRegistry_AllRegistered(t *testing.T) {
	all := All()
	require.NotEmpty(t, all, "at least one scenario must be registered")
	for _, s := range all {
		assert.NotEmpty(t, s.ID, "scenario ID must not be empty")
		assert.NotEmpty(t, s.Name, "scenario Name must not be empty")
		assert.NotNil(t, s.Run, "scenario Run func must not be nil")
	}
}

func TestRegistry_TS01Registered(t *testing.T) {
	s, ok := Find("TS-01")
	require.True(t, ok, "TS-01 must be registered")
	assert.Equal(t, "TS-01", s.ID)
	assert.Equal(t, 3, s.EpicGate)
}

func TestFind_NotFound(t *testing.T) {
	_, ok := Find("TS-NOTEXIST")
	assert.False(t, ok)
}

func TestRun_TimingInjected(t *testing.T) {
	s := Scenario{
		ID:   "TS-MOCK",
		Name: "Mock",
		Run: func(_ context.Context, _ dolt.Store) Result {
			return pass("TS-MOCK", "ok")
		},
	}
	r := Run(context.Background(), nil, s)
	assert.Equal(t, "pass", r.Status)
	assert.Equal(t, "TS-MOCK", r.Scenario)
	assert.GreaterOrEqual(t, r.DurationMs, int64(0))
}

func TestRun_ScenarioIDPropagated(t *testing.T) {
	// If the Run func returns an empty Scenario field, Run() fills it in.
	s := Scenario{
		ID:   "TS-FILL",
		Name: "Fill",
		Run: func(_ context.Context, _ dolt.Store) Result {
			return Result{Status: "pass"} // empty Scenario field
		},
	}
	r := Run(context.Background(), nil, s)
	assert.Equal(t, "TS-FILL", r.Scenario)
}

func TestCountFailed(t *testing.T) {
	results := []Result{
		{Status: "pass"},
		{Status: "fail"},
		{Status: "pass"},
		{Status: "fail"},
	}
	assert.Equal(t, 2, countFailed(results))
}

func TestScenarioIDsNotEmpty(t *testing.T) {
	ids := scenarioIDs()
	assert.NotEmpty(t, ids)
	assert.Contains(t, ids, "TS-01")
}

func TestFailResult_ErrorInDetails(t *testing.T) {
	r := fail("TS-X", "db unavailable", "check connectivity")
	assert.Equal(t, "fail", r.Status)
	assert.Equal(t, "db unavailable", r.Error)
	assert.Equal(t, []string{"check connectivity"}, r.Details)
	_ = errors.New(r.Error)
}
