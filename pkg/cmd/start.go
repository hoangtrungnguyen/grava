package cmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hoangtrungnguyen/grava/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Dolt SQL server",
	Long:  `Start the Dolt SQL server using the configured port in .grava.yaml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Get port from config
		dbURL := viper.GetString("db_url")
		if dbURL == "" {
			dbURL = "root@tcp(127.0.0.1:3306)/dolt"
		}

		port := "3306"
		lastColon := strings.LastIndex(dbURL, ":")
		if lastColon != -1 {
			afterColon := dbURL[lastColon+1:]
			endParen := strings.Index(afterColon, ")")
			if endParen != -1 {
				port = afterColon[:endParen]
			}
		}

		// 2. Check if port is already in use
		ln, err := net.Listen("tcp", ":"+port)
		if err == nil {
			// Port is free, so server is NOT running (or at least not on this port)
			ln.Close()
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "ℹ️  Server appears to be already running on port %s\n", port)
			return nil
		}

		// 3. Find start script
		scriptPath, err := utils.FindScript("start_dolt_server.sh")
		if err != nil {
			return err
		}

		// 4. Start in background
		fmt.Fprintf(cmd.OutOrStdout(), "🚀 Starting Dolt server on port %s...\n", port)

		serverCmd := exec.Command(scriptPath, port)

		// Redirect output to log file
		gravaDir := ".grava"
		_ = os.MkdirAll(gravaDir, 0755)
		logFile, err := os.OpenFile(filepath.Join(gravaDir, "dolt.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			serverCmd.Stdout = logFile
			serverCmd.Stderr = logFile
		}

		if err := serverCmd.Start(); err != nil {
			return fmt.Errorf("failed to start dolt server: %w", err)
		}

		// We don't wait for it to finish as its a server
		fmt.Fprintln(cmd.OutOrStdout(), "✅ Dolt server started in background. Check .grava/dolt.log for details.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
