package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTemp writes content to a temp file and returns its path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func TestLoadConfig_HappyPath(t *testing.T) {
	path := writeTemp(t, `
poll_interval_secs: 2
heartbeat_timeout_secs: 20
task_timeout_secs: 60
agents_config: agents.yaml
`)
	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, 2, cfg.PollIntervalSecs)
	assert.Equal(t, 20, cfg.HeartbeatTimeoutSecs)
	assert.Equal(t, 60, cfg.TaskTimeoutSecs)
	assert.Equal(t, "agents.yaml", cfg.AgentsConfigPath)
}

func TestLoadConfig_Defaults(t *testing.T) {
	// Only agents_config provided; all numeric fields default.
	path := writeTemp(t, `agents_config: agents.yaml`)
	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, 1, cfg.PollIntervalSecs)
	assert.Equal(t, 15, cfg.HeartbeatTimeoutSecs)
	assert.Equal(t, 30, cfg.TaskTimeoutSecs)
}

func TestLoadConfig_MissingAgentsConfig(t *testing.T) {
	path := writeTemp(t, `poll_interval_secs: 1`)
	_, err := LoadConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agents_config is required")
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config")
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	path := writeTemp(t, `{not: [valid yaml`)
	_, err := LoadConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config")
}

func TestLoadAgents_HappyPath(t *testing.T) {
	path := writeTemp(t, `
agents:
  - id: agent-1
    endpoint: http://localhost:8001
    timeout_secs: 30
    heartbeat_interval_secs: 5
    max_concurrent_tasks: 3
  - id: agent-2
    endpoint: http://localhost:8002
`)
	agents, err := LoadAgents(path)
	require.NoError(t, err)
	require.Len(t, agents, 2)
	assert.Equal(t, "agent-1", agents[0].ID)
	assert.Equal(t, "http://localhost:8001", agents[0].Endpoint)
	assert.Equal(t, 30, agents[0].TimeoutSecs)
	assert.Equal(t, 5, agents[0].HeartbeatIntervalSecs)
	assert.Equal(t, 3, agents[0].MaxConcurrentTasks)
	// agent-2 uses defaults
	assert.Equal(t, 30, agents[1].TimeoutSecs)
	assert.Equal(t, 5, agents[1].HeartbeatIntervalSecs)
	assert.Equal(t, 3, agents[1].MaxConcurrentTasks)
}

func TestLoadAgents_Empty(t *testing.T) {
	path := writeTemp(t, `agents: []`)
	_, err := LoadAgents(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one agent")
}

func TestLoadAgents_MissingID(t *testing.T) {
	path := writeTemp(t, `
agents:
  - endpoint: http://localhost:8001
`)
	_, err := LoadAgents(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id is required")
}

func TestLoadAgents_MissingEndpoint(t *testing.T) {
	path := writeTemp(t, `
agents:
  - id: agent-1
`)
	_, err := LoadAgents(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint is required")
}

func TestLoadAgents_FileNotFound(t *testing.T) {
	_, err := LoadAgents(filepath.Join(t.TempDir(), "missing.yaml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read agents config")
}
