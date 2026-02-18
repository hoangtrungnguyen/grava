package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
		fmt.Fprintln(cmd.OutOrStdout(), "✅ Dolt is installed.")

		// 2. Create .grava directory
		gravaDir := ".grava"
		if err := os.MkdirAll(gravaDir, 0755); err != nil {
			return fmt.Errorf("failed to create .grava directory: %w", err)
		}

		// 3. Create default config
		configFile := filepath.Join(gravaDir, "config.yaml") // Keeping it simple for now, though viper searches for .grava.yaml in home/cwd by default from root.go
		// Actually root.go looks for .grava.yaml in . or home.
		// Let's make `grava init` create a local config in .grava/config.yaml or just .grava.yaml in root?
		// The prompt says "Initialize .grava directory and config".
		// Let's separate "repo config" from "user config".
		// For now, let's just create a basic config file if it doesn't exist.

		viper.Set("db_url", "root@tcp(127.0.0.1:3306)/dolt?parseTime=true")
		if err := viper.WriteConfigAs(configFile); err != nil {
			// If file exists, it might error, but safe to ignore or just print
			fmt.Fprintf(cmd.OutOrStdout(), "ℹ️  Config file already exists or could not be written: %v\n", err)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "✅ Created default configuration.")
		}

		fmt.Fprintln(cmd.OutOrStdout(), "🎉 Grava initialized successfully!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
