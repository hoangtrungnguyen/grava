package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

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
  agents_config: orchestrator-agents.yaml

Environment variables:
  LOG_FORMAT   Log output format: 'json' (default) or 'text' (human-readable)
  LOG_LEVEL    Log verbosity: 'info' (default) or 'debug'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := orchestrateConfigPath
		if cfgPath == "" {
			cfgPath = ".grava/orchestrator.yaml"
		}

		cfg, err := orchestrator.LoadConfig(cfgPath)
		if err != nil {
			return err
		}

		// Resolve agents config path relative to the orchestrator config file
		// directory so that a relative path like "agents.yaml" works when the
		// config lives in .grava/ and the user runs grava from the project root.
		agentsPath := cfg.AgentsConfigPath
		if !filepath.IsAbs(agentsPath) {
			agentsPath = filepath.Join(filepath.Dir(cfgPath), agentsPath)
		}

		agents, err := orchestrator.LoadAgents(agentsPath)
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
			cfgPath,
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
		// Configure structured logger: JSON by default, text when LOG_FORMAT=text.
		// LOG_LEVEL=debug enables debug-level output.
		// Configured before the dry-run guard so tests can exercise this path.
		logLevel := slog.LevelInfo
		if os.Getenv("LOG_LEVEL") == "debug" {
			logLevel = slog.LevelDebug
		}
		handlerOpts := &slog.HandlerOptions{Level: logLevel}
		var handler slog.Handler
		if os.Getenv("LOG_FORMAT") == "text" {
			handler = slog.NewTextHandler(os.Stderr, handlerOpts)
		} else {
			handler = slog.NewJSONHandler(os.Stderr, handlerOpts)
		}
		slog.SetDefault(slog.New(handler))

		// When Store is nil (e.g. dry-run / tests), print config summary and exit.
		if Store == nil {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ Orchestrator ready.")
			return nil
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ Orchestrator ready. Starting poll loop...")

		pool := orchestrator.NewAgentPool(agents)
		orc := orchestrator.NewOrchestrator(Store, pool, cfg)

		// Cancel context on SIGTERM / SIGINT for graceful shutdown.
		cmdCtx := cmd.Context()
		if cmdCtx == nil {
			cmdCtx = context.Background()
		}
		ctx, cancel := context.WithCancel(cmdCtx)
		defer cancel()
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		defer signal.Stop(sigCh)
		go func() {
			select {
			case <-sigCh:
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "\nSignal received — draining in-flight tasks...")
				cancel()
			case <-ctx.Done():
			}
		}()

		orc.Run(ctx)
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ Orchestrator exited cleanly.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(orchestrateCmd)
	orchestrateCmd.Flags().StringVar(&orchestrateConfigPath, "config", "", "Path to orchestrator config YAML (default: .grava/orchestrator.yaml)")
}
