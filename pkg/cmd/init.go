package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hoangtrungnguyen/grava/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Grava repository",
	Long: `Initialize a new Grava repository in the current directory.
This command creates a .grava directory and a default configuration file.
It also checks if the dolt CLI is installed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Check for Dolt
		if _, err := exec.LookPath("dolt"); err != nil {
			return fmt.Errorf("dolt not found: %w\nPlease install Dolt at https://docs.dolthub.com/introduction/installation", err)
		}
		if !outputJSON {
			fmt.Fprintln(cmd.OutOrStdout(), "✅ Dolt is installed.")
		}

		// 2. Create .grava directory
		gravaDir := ".grava"
		if err := os.MkdirAll(gravaDir, 0755); err != nil {
			return fmt.Errorf("failed to create .grava directory: %w", err)
		}

		// 3. Initialize Dolt Repo in .grava/dolt
		doltRepoDir := filepath.Join(gravaDir, "dolt")
		if err := os.MkdirAll(doltRepoDir, 0755); err != nil {
			return fmt.Errorf("failed to create dolt directory: %w", err)
		}

		if _, err := os.Stat(filepath.Join(doltRepoDir, ".dolt")); os.IsNotExist(err) {
			if !outputJSON {
				fmt.Fprintln(cmd.OutOrStdout(), "📦 Initializing Dolt database...")
			}
			initCmd := exec.Command("dolt", "init")
			initCmd.Dir = doltRepoDir
			if err := initCmd.Run(); err != nil {
				return fmt.Errorf("failed to initialize dolt: %w", err)
			}
		}

		// 4. Find Available Port
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		port, err := utils.AllocatePort(cwd, 3306)
		if err != nil {
			return err
		}
		if !outputJSON && port != 3306 {
			fmt.Fprintf(cmd.OutOrStdout(), "ℹ️  Port 3306 is busy, using port %d\n", port)
		}

		// 5. Start Dolt Server in background
		if !outputJSON {
			fmt.Fprintf(cmd.OutOrStdout(), "🚀 Starting Dolt server on port %d...\n", port)
		}

		serverCmd := exec.Command("dolt", "sql-server", "--port", fmt.Sprintf("%d", port), "--host", "0.0.0.0")
		serverCmd.Dir = doltRepoDir

		// Redirect output to log file
		logFile, err := os.OpenFile(filepath.Join(gravaDir, "dolt.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			serverCmd.Stdout = logFile
			serverCmd.Stderr = logFile
		}

		if err := serverCmd.Start(); err != nil {
			return fmt.Errorf("failed to start dolt server: %w", err)
		}

		// 6. Create default config
		configFile := ".grava.yaml" // Default used by root.go in CWD
		dbURL := fmt.Sprintf("root@tcp(127.0.0.1:%d)/dolt?parseTime=true", port)
		viper.Set("db_url", dbURL)

		if err := viper.WriteConfigAs(configFile); err != nil {
			// If it fails because file exists, we might want to update it
			if os.IsExist(err) || err.Error() == "file already exists" {
				// For now just overwrite if we are initializing?
				// The prompt says "Initialize .grava directory and config"
				_ = os.Remove(configFile)
				_ = viper.WriteConfigAs(configFile)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "ℹ️  Note: %v\n", err)
			}
		}

		if !outputJSON {
			fmt.Fprintln(cmd.OutOrStdout(), "✅ Created configuration in .grava.yaml")
		}

		if outputJSON {
			resp := map[string]interface{}{
				"status": "initialized",
				"port":   port,
				"db_url": dbURL,
			}
			b, _ := json.MarshalIndent(resp, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), "🎉 Grava initialized successfully and server is running!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
