package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/hoangtrungnguyen/grava/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Dolt SQL server",
	Long:  `Stop the Dolt SQL server running on the configured port.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Get port from config
		dbURL := viper.GetString("db_url")
		if dbURL == "" {
			dbURL = "root@tcp(127.0.0.1:3306)/dolt"
		}

		port := "3306"
		if strings.Contains(dbURL, ":") && strings.Contains(dbURL, ")/") {
			parts := strings.Split(dbURL, ":")
			if len(parts) > 2 {
				portPart := parts[2]
				portParts := strings.Split(portPart, ")/")
				if len(portParts) > 0 {
					port = portParts[0]
				}
			}
		}

		// 2. Find stop script
		scriptPath, err := utils.FindScript("stop_dolt_server.sh")
		if err != nil {
			return err
		}

		// 3. Run stop script
		fmt.Fprintf(cmd.OutOrStdout(), "🛑 Stopping Dolt server on port %s...\n", port)

		stopCmd := exec.Command(scriptPath, port, "-y")
		stopCmd.Stdout = cmd.OutOrStdout()
		stopCmd.Stderr = cmd.ErrOrStderr()

		if err := stopCmd.Run(); err != nil {
			return fmt.Errorf("failed to stop dolt server: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
