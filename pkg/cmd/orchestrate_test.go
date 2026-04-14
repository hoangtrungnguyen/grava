package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeOrchestratorConfig(t *testing.T, dir string) (cfgPath, agentsPath string) {
	t.Helper()
	agentsPath = filepath.Join(dir, "agents.yaml")
	require.NoError(t, os.WriteFile(agentsPath, []byte(`
agents:
  - id: agent-1
    endpoint: http://localhost:8001
    max_concurrent_tasks: 2
  - id: agent-2
    endpoint: http://localhost:8002
    max_concurrent_tasks: 3
`), 0644))

	cfgPath = filepath.Join(dir, "orchestrator.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
poll_interval_secs: 1
heartbeat_timeout_secs: 15
task_timeout_secs: 30
agents_config: `+agentsPath+`
`), 0644))
	return cfgPath, agentsPath
}

func TestOrchestratCmd_TextOutput(t *testing.T) {
	cfgPath, _ := writeOrchestratorConfig(t, t.TempDir())

	buf := &bytes.Buffer{}
	orchestrateConfigPath = cfgPath
	outputJSON = false
	t.Cleanup(func() { orchestrateConfigPath = ""; outputJSON = false })

	orchestrateCmd.SetOut(buf)
	orchestrateCmd.SetErr(buf)

	err := orchestrateCmd.RunE(orchestrateCmd, []string{})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Grava Agent Orchestrator")
	assert.Contains(t, out, "agent-1")
	assert.Contains(t, out, "agent-2")
	assert.Contains(t, out, "Poll interval:")
	assert.Contains(t, out, "Orchestrator ready")
}

func TestOrchestratCmd_JSONOutput(t *testing.T) {
	cfgPath, _ := writeOrchestratorConfig(t, t.TempDir())

	buf := &bytes.Buffer{}
	orchestrateConfigPath = cfgPath
	outputJSON = true
	t.Cleanup(func() { outputJSON = false; orchestrateConfigPath = "" })

	orchestrateCmd.SetOut(buf)
	orchestrateCmd.SetErr(buf)

	err := orchestrateCmd.RunE(orchestrateCmd, []string{})
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, "ready", result["status"])
	assert.Equal(t, float64(2), result["agents"])
}

func TestOrchestrateCmd_MissingAgentsConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "orchestrator.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
poll_interval_secs: 1
agents_config: /nonexistent/path/agents.yaml
`), 0644))

	orchestrateConfigPath = cfgPath
	outputJSON = false
	t.Cleanup(func() { orchestrateConfigPath = "" })

	buf := &bytes.Buffer{}
	orchestrateCmd.SetOut(buf)
	orchestrateCmd.SetErr(buf)

	err := orchestrateCmd.RunE(orchestrateCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read agents config")
}

func TestOrchestrateCmd_DefaultConfigPath(t *testing.T) {
	// When no --config is given, command defaults to .grava/orchestrator.yaml
	// If the file doesn't exist it returns a descriptive error.
	orchestrateConfigPath = ""
	outputJSON = false
	t.Cleanup(func() { orchestrateConfigPath = "" })

	buf := &bytes.Buffer{}
	orchestrateCmd.SetOut(buf)
	orchestrateCmd.SetErr(buf)

	t.Chdir(t.TempDir()) // no .grava dir → file not found
	err := orchestrateCmd.RunE(orchestrateCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config")
}

func TestOrchestrateCmd_RelativeAgentsConfigPath(t *testing.T) {
	// Verifies that a relative agents_config path is resolved relative to the
	// config file's directory, not the CWD. This covers the review finding:
	// users expect 'agents_config: agents.yaml' to resolve relative to the
	// orchestrator config file (e.g. .grava/), not the project root.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agents.yaml"), []byte(`
agents:
  - id: agent-1
    endpoint: http://localhost:8001
`), 0644))

	cfgPath := filepath.Join(dir, "orchestrator.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
poll_interval_secs: 1
heartbeat_timeout_secs: 15
task_timeout_secs: 30
agents_config: agents.yaml
`), 0644))

	orchestrateConfigPath = cfgPath
	outputJSON = false
	t.Cleanup(func() { orchestrateConfigPath = ""; outputJSON = false })

	buf := &bytes.Buffer{}
	orchestrateCmd.SetOut(buf)
	orchestrateCmd.SetErr(buf)

	// CWD is different from dir — relative path must resolve against dir
	t.Chdir(t.TempDir())
	err := orchestrateCmd.RunE(orchestrateCmd, []string{})
	require.NoError(t, err, "relative agents_config should resolve relative to config file dir")
	assert.Contains(t, buf.String(), "agent-1")
}
