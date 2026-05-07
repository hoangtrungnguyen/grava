package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current configuration",
	Long: `Display the current configuration settings being used by Grava.
This includes the database URL, actor identity, and agent model.`,
	Run: func(cmd *cobra.Command, args []string) {
		config := map[string]interface{}{
			"db_url":           viper.GetString("db_url"),
			"actor":            viper.GetString("actor"),
			"agent_model":      viper.GetString("agent_model"),
			"config_file_used": viper.ConfigFileUsed(),
		}

		if outputJSON {
			b, _ := json.MarshalIndent(config, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
			return
		}

		cmd.Println("🛠️  Grava Configuration:")
		cmd.Printf("  DB URL:           %s\n", config["db_url"])
		cmd.Printf("  Actor:            %s\n", config["actor"])
		cmd.Printf("  Agent Model:      %s\n", config["agent_model"])
		cmd.Printf("  Config File:      %s\n", config["config_file_used"])

		if config["config_file_used"] == "" {
			cmd.Println("\nℹ️  Note: Using default values (no config file found).")
		}
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
