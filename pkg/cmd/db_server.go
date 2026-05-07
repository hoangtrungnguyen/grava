package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/hoangtrungnguyen/grava/pkg/grava"
	"github.com/hoangtrungnguyen/grava/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(newDBStartCmd())
	rootCmd.AddCommand(newDBStopCmd())
}

func newDBStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "db-start",
		Short: "Start the Dolt SQL server",
		Long: `Start the Dolt SQL server for the current Grava repository.
It uses the repository in .grava/dolt and the port specified in .grava.yaml.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			gravaDir, err := grava.ResolveGravaDir()
			if err != nil {
				return err
			}
			cwd, _ := os.Getwd()
			doltBin, err := utils.ResolveDoltBinary(cwd)
			if err != nil {
				return fmt.Errorf("dolt binary not found: %w", err)
			}

			doltRepoDir := filepath.Join(gravaDir, "dolt")
			
			// Resolve port
			dbURL := viper.GetString("db_url")
			if dbURL == "" {
				dbURL = "root@tcp(127.0.0.1:3306)/dolt?parseTime=true"
			}
			
			// Extract port from URL if possible, otherwise use 3306
			port := 3306
			fmt.Sscanf(dbURL, "root@tcp(127.0.0.1:%d)", &port)

			// Check if already running
			testStore, err := dolt.NewClient(dbURL)
			if err == nil {
				testStore.Close()
				cmd.Printf("✅ Dolt server is already running on port %d\n", port)
				return nil
			}

			cmd.Printf("🚀 Starting Dolt server on port %d...\n", port)
			serverCmd := exec.Command(doltBin, "sql-server", "--port", fmt.Sprintf("%d", port), "--host", "0.0.0.0")
			serverCmd.Dir = doltRepoDir

			logFile, err := os.OpenFile(filepath.Join(gravaDir, "dolt.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err == nil {
				serverCmd.Stdout = logFile
				serverCmd.Stderr = logFile
			}

			if err := serverCmd.Start(); err != nil {
				return fmt.Errorf("failed to start dolt server: %w", err)
			}

			// Wait for server ready
			for i := 0; i < 10; i++ {
				initStore, err := dolt.NewClient(dbURL)
				if err == nil {
					initStore.Close()
					break
				}
				time.Sleep(500 * time.Millisecond)
				if i == 9 {
					return fmt.Errorf("dolt server did not become ready")
				}
			}

			cmd.Printf("✅ Dolt server started successfully.\n")
			return nil
		},
	}
}

func newDBStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db-stop",
		Short: "Stop the Dolt SQL server",
		Long: `Stop the Dolt SQL server running on the port configured for this repository.

Refuses to stop while any non-ephemeral issues are status=in_progress
(claimed by an active /ship run). Stopping mid-flight breaks the
pipeline: the orchestrator's next 'grava signal' write fails and the
issue is left in an inconsistent state. Pass --force to override.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			dbURL := viper.GetString("db_url")
			if dbURL == "" {
				dbURL = "root@tcp(127.0.0.1:3306)/dolt?parseTime=true"
			}

			port := 3306
			fmt.Sscanf(dbURL, "root@tcp(127.0.0.1:%d)", &port)

			// Concurrency guard (concurrency-matrix #4): refuse to stop
			// while any /ship run might be writing wisps. Best-effort —
			// if the DB is unreachable (already down), skip the check.
			if !force {
				if store, err := connectDBFn(); err == nil {
					var inFlight int
					row := store.QueryRow(
						"SELECT COUNT(*) FROM issues WHERE ephemeral = 0 AND status = 'in_progress'",
					)
					if scanErr := row.Scan(&inFlight); scanErr == nil && inFlight > 0 {
						_ = store.Close() //nolint:errcheck
						return fmt.Errorf(
							"refusing to stop: %d issue(s) are status=in_progress "+
								"(active /ship runs would lose state). "+
								"Wait for them to finish, or pass --force to override",
							inFlight,
						)
					}
					_ = store.Close() //nolint:errcheck
				}
			}

			cmd.Printf("🛑 Stopping Dolt server on port %d...\n", port)

			// Find PID using lsof (standard approach on macOS/Linux)
			// We can also use 'ps' and look for 'dolt sql-server' and the port
			lsofCmd := exec.Command("lsof", "-t", fmt.Sprintf("-i:%d", port))
			output, err := lsofCmd.Output()
			if err != nil || len(output) == 0 {
				cmd.Printf("No process found listening on port %d.\n", port)
				return nil
			}

			pid := string(output)
			// Standard kill
			killCmd := exec.Command("kill", "-TERM", strings.TrimSpace(pid))
			if err := killCmd.Run(); err != nil {
				return fmt.Errorf("failed to kill process %s: %w", pid, err)
			}

			// Wait for it to exit
			for i := 0; i < 10; i++ {
				lsofCheck := exec.Command("lsof", "-t", fmt.Sprintf("-i:%d", port))
				checkOutput, _ := lsofCheck.Output()
				if len(checkOutput) == 0 {
					cmd.Printf("✅ Dolt server stopped.\n")
					return nil
				}
				time.Sleep(500 * time.Millisecond)
			}

			return fmt.Errorf("failed to stop server within timeout")
		},
	}
	cmd.Flags().Bool("force", false, "Stop even if issues are in_progress (DANGEROUS — breaks active /ship runs)")
	return cmd
}
