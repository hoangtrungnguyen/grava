package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/hoangtrungnguyen/grava/pkg/orchestrator"
	"github.com/spf13/cobra"
)

var orchestrateConfigPath string

var orchestrateCmd = &cobra.Command{
	Use:   "orchestrate",
	Short: "Run the Grava Agent Orchestrator",
	Long: `Start the Grava Agent Orchestrator, which routes tasks from the Grava queue
to a pool of registered agents with load-balancing and automatic failover.

Configuration is read from the specified YAML file (default: .grava/orchestrator.yaml).
Agents are declared in a separate YAML file referenced by 'agents_config' in the main config.

Example config (.grava/orchestrator.yaml):
  poll_interval_secs: 1
  heartbeat_timeout_secs: 15
  task_timeout_secs: 30
  agents_config: orchestrator-agents.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if orchestrateConfigPath == "" {
			orchestrateConfigPath = ".grava/orchestrator.yaml"
		}

		cfg, err := orchestrator.LoadConfig(orchestrateConfigPath)
		if err != nil {
			return err
		}

		agents, err := orchestrator.LoadAgents(cfg.AgentsConfigPath)
		if err != nil {
			return err
		}

		if outputJSON {
			summary := map[string]interface{}{
				"config": map[string]interface{}{
					"poll_interval_secs":    cfg.PollIntervalSecs,
					"heartbeat_timeout_secs": cfg.HeartbeatTimeoutSecs,
					"task_timeout_secs":     cfg.TaskTimeoutSecs,
					"agents_config":         cfg.AgentsConfigPath,
				},
				"agents": len(agents),
				"status": "ready",
			}
			b, _ := json.MarshalIndent(summary, "", "  ")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(),
			"🤖 Grava Agent Orchestrator\n"+
				"   Config:              %s\n"+
				"   Poll interval:       %ds\n"+
				"   Heartbeat timeout:   %ds\n"+
				"   Task timeout:        %ds\n"+
				"   Agents:              %d\n",
			orchestrateConfigPath,
			cfg.PollIntervalSecs,
			cfg.HeartbeatTimeoutSecs,
			cfg.TaskTimeoutSecs,
			len(agents),
		)
		for _, a := range agents {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"     • %s  %s  (max %d tasks)\n",
				a.ID, a.Endpoint, a.MaxConcurrentTasks,
			)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ Orchestrator ready. (polling not yet implemented)")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(orchestrateCmd)
	orchestrateCmd.Flags().StringVar(&orchestrateConfigPath, "config", "", "Path to orchestrator config YAML (default: .grava/orchestrator.yaml)")
}
