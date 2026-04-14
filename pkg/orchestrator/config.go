package orchestrator

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the orchestrator runtime configuration.
type Config struct {
	// PollIntervalSecs is how often (in seconds) the orchestrator polls Grava
	// for open tasks.
	PollIntervalSecs int `yaml:"poll_interval_secs"`

	// HeartbeatTimeoutSecs is the number of seconds without a heartbeat before
	// an agent is declared dead.
	HeartbeatTimeoutSecs int `yaml:"heartbeat_timeout_secs"`

	// TaskTimeoutSecs is the maximum seconds a claimed task can remain without
	// a status update before it is reassigned.
	TaskTimeoutSecs int `yaml:"task_timeout_secs"`

	// AgentsConfigPath is the path to the agents YAML file.
	AgentsConfigPath string `yaml:"agents_config"`

	// StatusPort is the TCP port for the /status HTTP endpoint.
	// 0 disables the endpoint. Defaults to 9090.
	StatusPort int `yaml:"status_port"`
}

// AgentsFile is the top-level structure of the agents YAML.
type AgentsFile struct {
	Agents []AgentConfig `yaml:"agents"`
}

// AgentConfig holds per-agent settings loaded from the agents YAML.
type AgentConfig struct {
	ID                    string `yaml:"id"`
	Endpoint              string `yaml:"endpoint"`
	TimeoutSecs           int    `yaml:"timeout_secs"`
	HeartbeatIntervalSecs int    `yaml:"heartbeat_interval_secs"`
	MaxConcurrentTasks    int    `yaml:"max_concurrent_tasks"`
}

// Validate returns an error if any required field is missing or invalid.
func (c *Config) Validate() error {
	if c.AgentsConfigPath == "" {
		return fmt.Errorf("agents_config is required")
	}
	if c.PollIntervalSecs <= 0 {
		c.PollIntervalSecs = 1
	}
	if c.HeartbeatTimeoutSecs <= 0 {
		c.HeartbeatTimeoutSecs = 15
	}
	if c.TaskTimeoutSecs <= 0 {
		c.TaskTimeoutSecs = 30
	}
	if c.StatusPort == 0 {
		c.StatusPort = 9090
	}
	return nil
}

// LoadConfig reads and parses an orchestrator config file at the given path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &cfg, nil
}

// LoadAgents reads and parses the agents YAML file at the given path.
func LoadAgents(path string) ([]AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read agents config %s: %w", path, err)
	}
	var af AgentsFile
	if err := yaml.Unmarshal(data, &af); err != nil {
		return nil, fmt.Errorf("failed to parse agents config %s: %w", path, err)
	}
	if len(af.Agents) == 0 {
		return nil, fmt.Errorf("agents_config must define at least one agent")
	}
	for i, a := range af.Agents {
		if a.ID == "" {
			return nil, fmt.Errorf("agent[%d]: id is required", i)
		}
		if a.Endpoint == "" {
			return nil, fmt.Errorf("agent %s: endpoint is required", a.ID)
		}
		if af.Agents[i].TimeoutSecs <= 0 {
			af.Agents[i].TimeoutSecs = 30
		}
		if af.Agents[i].HeartbeatIntervalSecs <= 0 {
			af.Agents[i].HeartbeatIntervalSecs = 5
		}
		if af.Agents[i].MaxConcurrentTasks <= 0 {
			af.Agents[i].MaxConcurrentTasks = 3
		}
	}
	return af.Agents, nil
}
